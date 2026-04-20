package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type fakeRuntimeRow struct {
	values []any
	err    error
}

func (r fakeRuntimeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = r.values[i].(string)
		case *[]byte:
			*d = append((*d)[:0], r.values[i].([]byte)...)
		case *time.Time:
			*d = r.values[i].(time.Time)
		default:
			tpanic("unsupported scan destination")
		}
	}
	return nil
}

type fakeRuntimeDB struct {
	lastExecSQL   string
	lastExecArgs  []any
	lastQuerySQL  string
	lastQueryArgs []any
	execFunc      func(query string, args ...any) error
	queryRowFunc  func(query string, args ...any) runtimeStoreRow
	execErr       error
}

func (db *fakeRuntimeDB) Exec(_ context.Context, query string, args ...any) error {
	db.lastExecSQL = query
	db.lastExecArgs = append([]any(nil), args...)
	if db.execFunc != nil {
		return db.execFunc(query, args...)
	}
	return db.execErr
}

func (db *fakeRuntimeDB) QueryRow(_ context.Context, query string, args ...any) runtimeStoreRow {
	db.lastQuerySQL = query
	db.lastQueryArgs = append([]any(nil), args...)
	if db.queryRowFunc == nil {
		return fakeRuntimeRow{err: pgx.ErrNoRows}
	}
	return db.queryRowFunc(query, args...)
}

func TestPostgresRuntimeStore_SavePersistsTokenRecord(t *testing.T) {
	now := time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC)
	db := &fakeRuntimeDB{}
	store := newPostgresRuntimeStoreWithDB(db)

	err := store.Save(context.Background(), TokenRecord{
		TokenID:       "tok_123",
		AccessToken:   "ptk_secret",
		SubjectID:     "2001",
		Role:          "owner",
		Scope:         []string{"supply:*"},
		IssuedAt:      now,
		ExpiresAt:     now.Add(15 * time.Minute),
		Status:        TokenStatusActive,
		RequestID:     "req-1",
		RevokedReason: "",
	}, "idem-1", "hash-1")
	if err != nil {
		t.Fatalf("save record: %v", err)
	}
	if db.lastExecSQL == "" {
		t.Fatal("expected save query to be executed")
	}
	if got := db.lastExecArgs[1].(string); got == "" {
		t.Fatal("expected token fingerprint to be persisted")
	}
	if got := db.lastExecArgs[10].(string); got != "req-1" {
		t.Fatalf("unexpected request id: got=%s want=req-1", got)
	}
	if got := db.lastExecArgs[11].(string); got != "idem-1" {
		t.Fatalf("unexpected idempotency key: got=%s want=idem-1", got)
	}
	if got := db.lastExecArgs[12].(string); got != "hash-1" {
		t.Fatalf("unexpected request hash: got=%s want=hash-1", got)
	}
}

func TestPostgresRuntimeStore_LookupsUseFingerprintAndIdempotencyHash(t *testing.T) {
	now := time.Date(2026, 4, 17, 9, 5, 0, 0, time.UTC)
	db := &fakeRuntimeDB{
		queryRowFunc: func(query string, args ...any) runtimeStoreRow {
			switch {
			case strings.Contains(query, "token_fingerprint"):
				return fakeRuntimeRow{values: []any{
					"tok_123",
					"2001",
					"owner",
					[]byte(`["supply:*"]`),
					string(TokenStatusActive),
					now,
					now.Add(10 * time.Minute),
					"req-1",
					"",
				}}
			case strings.Contains(query, "issue_idempotency_key"):
				return fakeRuntimeRow{values: []any{"hash-1", "tok_123"}}
			default:
				return fakeRuntimeRow{err: pgx.ErrNoRows}
			}
		},
	}
	store := newPostgresRuntimeStoreWithDB(db)

	record, ok, err := store.GetByAccessToken(context.Background(), "ptk_secret")
	if err != nil {
		t.Fatalf("get by access token: %v", err)
	}
	if !ok {
		t.Fatal("expected access token lookup to succeed")
	}
	if record.TokenID != "tok_123" {
		t.Fatalf("unexpected token id: got=%s want=tok_123", record.TokenID)
	}
	entry, ok, err := store.LookupIdempotency(context.Background(), "idem-1")
	if err != nil {
		t.Fatalf("lookup idempotency: %v", err)
	}
	if !ok {
		t.Fatal("expected idempotency lookup to succeed")
	}
	if entry.RequestHash != "hash-1" {
		t.Fatalf("unexpected request hash: got=%s want=hash-1", entry.RequestHash)
	}
}

func TestPostgresRuntimeStore_SavePreservesExistingFingerprintWhenAccessTokenMissing(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	db := newSimulatedRuntimeDB()
	store := newPostgresRuntimeStoreWithDB(db)

	initial := TokenRecord{
		TokenID:       "tok_2001",
		AccessToken:   "ptk_secret_2001",
		SubjectID:     "2001",
		Role:          "owner",
		Scope:         []string{"supply:*"},
		IssuedAt:      now,
		ExpiresAt:     now.Add(2 * time.Minute),
		Status:        TokenStatusActive,
		RequestID:     "req-issue",
		RevokedReason: "",
	}
	if err := store.Save(context.Background(), initial, "idem-2001", "hash-2001"); err != nil {
		t.Fatalf("save initial token: %v", err)
	}

	updated := initial
	updated.AccessToken = ""
	updated.Status = TokenStatusRevoked
	updated.RevokedReason = "operator_request"
	if err := store.Save(context.Background(), updated, "", ""); err != nil {
		t.Fatalf("save updated token without access token: %v", err)
	}

	record, ok, err := store.GetByAccessToken(context.Background(), initial.AccessToken)
	if err != nil {
		t.Fatalf("lookup by access token: %v", err)
	}
	if !ok {
		t.Fatal("expected lookup by original access token to succeed")
	}
	if record.Status != TokenStatusRevoked {
		t.Fatalf("unexpected status after update: got=%s want=%s", record.Status, TokenStatusRevoked)
	}
	if record.RevokedReason != "operator_request" {
		t.Fatalf("unexpected revoke reason: got=%s want=operator_request", record.RevokedReason)
	}
}

func TestInMemoryTokenRuntimeWithPostgresStore_RefreshAndRevokePersistLifecycle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 9, 10, 0, 0, time.UTC)
	store := newPostgresRuntimeStoreWithDB(newSimulatedRuntimeDB())
	runtime := NewInMemoryTokenRuntimeWithStore(func() time.Time { return now }, store)

	issued, err := runtime.Issue(context.Background(), IssueTokenInput{
		SubjectID: "2008",
		Role:      "owner",
		Scope:     []string{"supply:*"},
		TTL:       2 * time.Minute,
		RequestID: "req-runtime-issue",
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	previousExpiresAt := issued.ExpiresAt

	now = now.Add(15 * time.Second)
	refreshed, err := runtime.Refresh(context.Background(), issued.TokenID, 5*time.Minute)
	if err != nil {
		t.Fatalf("refresh token: %v", err)
	}
	if !refreshed.ExpiresAt.After(previousExpiresAt) {
		t.Fatalf("expected refreshed token to extend ttl: previous=%s refreshed=%s", previousExpiresAt, refreshed.ExpiresAt)
	}

	introspected, err := runtime.Introspect(context.Background(), issued.AccessToken)
	if err != nil {
		t.Fatalf("introspect after refresh: %v", err)
	}
	if introspected.Status != TokenStatusActive {
		t.Fatalf("unexpected status after refresh introspection: got=%s want=%s", introspected.Status, TokenStatusActive)
	}

	revoked, err := runtime.Revoke(context.Background(), issued.TokenID, "operator_request")
	if err != nil {
		t.Fatalf("revoke token: %v", err)
	}
	if revoked.Status != TokenStatusRevoked {
		t.Fatalf("unexpected revoke status: got=%s want=%s", revoked.Status, TokenStatusRevoked)
	}

	introspected, err = runtime.Introspect(context.Background(), issued.AccessToken)
	if err != nil {
		t.Fatalf("introspect after revoke: %v", err)
	}
	if introspected.Status != TokenStatusRevoked {
		t.Fatalf("unexpected status after revoke introspection: got=%s want=%s", introspected.Status, TokenStatusRevoked)
	}

	claims, err := runtime.Verify(context.Background(), issued.AccessToken)
	if err != nil {
		t.Fatalf("verify after revoke: %v", err)
	}
	status, err := runtime.Resolve(context.Background(), claims.TokenID)
	if err != nil {
		t.Fatalf("resolve after revoke: %v", err)
	}
	if status != TokenStatusRevoked {
		t.Fatalf("unexpected status after verify/resolve: got=%s want=%s", status, TokenStatusRevoked)
	}
}

type simulatedRuntimeDB struct {
	records map[string]simulatedTokenRecord
}

type simulatedTokenRecord struct {
	tokenID          string
	tokenFingerprint string
	subjectID        string
	roleCode         string
	scopeJSON        []byte
	status           string
	issuedAt         time.Time
	expiresAt        time.Time
	requestID        string
	revokedReason    string
	idempotencyKey   string
	idempotencyHash  string
}

func newSimulatedRuntimeDB() *simulatedRuntimeDB {
	return &simulatedRuntimeDB{
		records: make(map[string]simulatedTokenRecord),
	}
}

func (db *simulatedRuntimeDB) Exec(_ context.Context, query string, args ...any) error {
	if !strings.Contains(query, "INSERT INTO auth_platform_tokens") {
		return errors.New("unexpected exec query")
	}

	tokenID := args[0].(string)
	tokenFingerprint := strings.TrimSpace(args[1].(string))
	if tokenFingerprint == "" {
		return errors.New(`null value in column "token_fingerprint" violates not-null constraint`)
	}

	record := simulatedTokenRecord{
		tokenID:          tokenID,
		tokenFingerprint: tokenFingerprint,
		subjectID:        args[2].(string),
		roleCode:         args[3].(string),
		scopeJSON:        append([]byte(nil), args[4].([]byte)...),
		status:           args[5].(string),
		issuedAt:         args[6].(time.Time),
		expiresAt:        args[7].(time.Time),
		requestID:        args[10].(string),
		idempotencyKey:   args[11].(string),
		idempotencyHash:  args[12].(string),
	}
	if revokedReason := strings.TrimSpace(args[8].(string)); revokedReason != "" {
		record.revokedReason = revokedReason
	}
	if existing, ok := db.records[tokenID]; ok {
		if record.requestID == "" {
			record.requestID = existing.requestID
		}
		if record.idempotencyKey == "" {
			record.idempotencyKey = existing.idempotencyKey
		}
		if record.idempotencyHash == "" {
			record.idempotencyHash = existing.idempotencyHash
		}
	}
	db.records[tokenID] = record
	return nil
}

func (db *simulatedRuntimeDB) QueryRow(_ context.Context, query string, args ...any) runtimeStoreRow {
	switch {
	case strings.Contains(query, "SELECT token_fingerprint"):
		record, ok := db.records[strings.TrimSpace(args[0].(string))]
		if !ok {
			return fakeRuntimeRow{err: pgx.ErrNoRows}
		}
		return fakeRuntimeRow{values: []any{record.tokenFingerprint}}
	case strings.Contains(query, "WHERE token_id = $1"):
		record, ok := db.records[strings.TrimSpace(args[0].(string))]
		if !ok {
			return fakeRuntimeRow{err: pgx.ErrNoRows}
		}
		return fakeRuntimeRow{values: []any{
			record.tokenID,
			record.subjectID,
			record.roleCode,
			record.scopeJSON,
			record.status,
			record.issuedAt,
			record.expiresAt,
			record.requestID,
			record.revokedReason,
		}}
	case strings.Contains(query, "WHERE token_fingerprint = $1"):
		tokenFingerprint := strings.TrimSpace(args[0].(string))
		for _, record := range db.records {
			if record.tokenFingerprint == tokenFingerprint {
				return fakeRuntimeRow{values: []any{
					record.tokenID,
					record.subjectID,
					record.roleCode,
					record.scopeJSON,
					record.status,
					record.issuedAt,
					record.expiresAt,
					record.requestID,
					record.revokedReason,
				}}
			}
		}
		return fakeRuntimeRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "WHERE issue_idempotency_key = $1"):
		idempotencyKey := strings.TrimSpace(args[0].(string))
		for _, record := range db.records {
			if record.idempotencyKey == idempotencyKey {
				return fakeRuntimeRow{values: []any{record.idempotencyHash, record.tokenID}}
			}
		}
		return fakeRuntimeRow{err: pgx.ErrNoRows}
	default:
		return fakeRuntimeRow{err: pgx.ErrNoRows}
	}
}

func tpanic(msg string) {
	panic(msg)
}
