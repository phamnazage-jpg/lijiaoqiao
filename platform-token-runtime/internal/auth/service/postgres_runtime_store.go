package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type runtimeStoreRow interface {
	Scan(dest ...any) error
}

type runtimeStoreDB interface {
	Exec(ctx context.Context, query string, args ...any) error
	QueryRow(ctx context.Context, query string, args ...any) runtimeStoreRow
}

type pgxRuntimeStoreDB struct {
	pool *pgxpool.Pool
}

func (db *pgxRuntimeStoreDB) Exec(ctx context.Context, query string, args ...any) error {
	_, err := db.pool.Exec(ctx, query, args...)
	return err
}

func (db *pgxRuntimeStoreDB) QueryRow(ctx context.Context, query string, args ...any) runtimeStoreRow {
	return db.pool.QueryRow(ctx, query, args...)
}

type PostgresRuntimeStore struct {
	db runtimeStoreDB
}

func NewPostgresRuntimeStore(pool *pgxpool.Pool) *PostgresRuntimeStore {
	return newPostgresRuntimeStoreWithDB(&pgxRuntimeStoreDB{pool: pool})
}

func newPostgresRuntimeStoreWithDB(db runtimeStoreDB) *PostgresRuntimeStore {
	return &PostgresRuntimeStore{db: db}
}

func (s *PostgresRuntimeStore) Save(ctx context.Context, record TokenRecord, idempotencyKey, requestHash string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.db == nil {
		return errors.New("postgres runtime store is not configured")
	}

	scopeJSON, err := json.Marshal(record.Scope)
	if err != nil {
		return err
	}

	tokenFingerprint := ""
	if strings.TrimSpace(record.AccessToken) != "" {
		tokenFingerprint = accessTokenFingerprint(record.AccessToken)
	} else {
		existingFingerprint, ok, err := s.lookupTokenFingerprint(ctx, record.TokenID)
		if err != nil {
			return err
		}
		if ok {
			tokenFingerprint = existingFingerprint
		}
	}
	if tokenFingerprint == "" {
		return errors.New("token fingerprint is required")
	}

	var revokedAt any
	if record.Status == TokenStatusRevoked {
		revokedAt = time.Now().UTC()
	}

	const query = `
INSERT INTO auth_platform_tokens (
    token_id,
    token_fingerprint,
    hash_algo,
    subject_id,
    role_code,
    scope_json,
    status,
    issued_at,
    expires_at,
    revoked_reason,
    revoked_at,
    issue_request_id,
    issue_idempotency_key,
    issue_request_hash
) VALUES (
    $1,
    NULLIF($2, ''),
    'SHA-256',
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    NULLIF($9, ''),
    $10,
    NULLIF($11, ''),
    NULLIF($12, ''),
    NULLIF($13, '')
)
ON CONFLICT (token_id) DO UPDATE SET
    token_fingerprint = COALESCE(NULLIF(EXCLUDED.token_fingerprint, ''), auth_platform_tokens.token_fingerprint),
    subject_id = EXCLUDED.subject_id,
    role_code = EXCLUDED.role_code,
    scope_json = EXCLUDED.scope_json,
    status = EXCLUDED.status,
    issued_at = EXCLUDED.issued_at,
    expires_at = EXCLUDED.expires_at,
    revoked_reason = NULLIF(EXCLUDED.revoked_reason, ''),
    revoked_at = CASE
        WHEN EXCLUDED.status = 'revoked' THEN COALESCE(auth_platform_tokens.revoked_at, EXCLUDED.revoked_at, CURRENT_TIMESTAMP)
        ELSE NULL
    END,
    issue_request_id = COALESCE(NULLIF(EXCLUDED.issue_request_id, ''), auth_platform_tokens.issue_request_id),
    issue_idempotency_key = COALESCE(NULLIF(EXCLUDED.issue_idempotency_key, ''), auth_platform_tokens.issue_idempotency_key),
    issue_request_hash = COALESCE(NULLIF(EXCLUDED.issue_request_hash, ''), auth_platform_tokens.issue_request_hash),
    updated_at = CURRENT_TIMESTAMP
`

	return s.db.Exec(ctx, query,
		record.TokenID,
		tokenFingerprint,
		record.SubjectID,
		record.Role,
		scopeJSON,
		string(record.Status),
		record.IssuedAt,
		record.ExpiresAt,
		strings.TrimSpace(record.RevokedReason),
		revokedAt,
		strings.TrimSpace(record.RequestID),
		strings.TrimSpace(idempotencyKey),
		strings.TrimSpace(requestHash),
	)
}

func (s *PostgresRuntimeStore) GetByTokenID(ctx context.Context, tokenID string) (*TokenRecord, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.querySingleRecord(ctx, `
SELECT token_id, subject_id, role_code, scope_json, status, issued_at, expires_at, issue_request_id, COALESCE(revoked_reason, '')
FROM auth_platform_tokens
WHERE token_id = $1
`, strings.TrimSpace(tokenID))
}

func (s *PostgresRuntimeStore) GetByAccessToken(ctx context.Context, accessToken string) (*TokenRecord, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.querySingleRecord(ctx, `
SELECT token_id, subject_id, role_code, scope_json, status, issued_at, expires_at, issue_request_id, COALESCE(revoked_reason, '')
FROM auth_platform_tokens
WHERE token_fingerprint = $1
`, accessTokenFingerprint(accessToken))
}

func (s *PostgresRuntimeStore) LookupIdempotency(ctx context.Context, idempotencyKey string) (IdempotencyEntry, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.db == nil {
		return IdempotencyEntry{}, false, errors.New("postgres runtime store is not configured")
	}

	var entry IdempotencyEntry
	err := s.db.QueryRow(ctx, `
SELECT COALESCE(issue_request_hash, ''), token_id
FROM auth_platform_tokens
WHERE issue_idempotency_key = $1
`, strings.TrimSpace(idempotencyKey)).Scan(&entry.RequestHash, &entry.TokenID)
	if errors.Is(err, pgx.ErrNoRows) {
		return IdempotencyEntry{}, false, nil
	}
	if err != nil {
		return IdempotencyEntry{}, false, err
	}
	return entry, true, nil
}

func (s *PostgresRuntimeStore) querySingleRecord(ctx context.Context, query string, arg string) (*TokenRecord, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, errors.New("postgres runtime store is not configured")
	}

	var scopeJSON []byte
	var status string
	record := TokenRecord{}
	err := s.db.QueryRow(ctx, query, arg).Scan(
		&record.TokenID,
		&record.SubjectID,
		&record.Role,
		&scopeJSON,
		&status,
		&record.IssuedAt,
		&record.ExpiresAt,
		&record.RequestID,
		&record.RevokedReason,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(scopeJSON, &record.Scope); err != nil {
		return nil, false, err
	}
	record.Status = TokenStatus(status)
	return &record, true, nil
}

func (s *PostgresRuntimeStore) lookupTokenFingerprint(ctx context.Context, tokenID string) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, errors.New("postgres runtime store is not configured")
	}

	var tokenFingerprint string
	err := s.db.QueryRow(ctx, `
SELECT token_fingerprint
FROM auth_platform_tokens
WHERE token_id = $1
`, strings.TrimSpace(tokenID)).Scan(&tokenFingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return tokenFingerprint, true, nil
}

func accessTokenFingerprint(accessToken string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(accessToken)))
	return hex.EncodeToString(sum[:])
}
