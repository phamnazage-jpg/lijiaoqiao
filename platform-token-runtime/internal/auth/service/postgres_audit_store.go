package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type auditStoreRows interface {
	Close()
	Err() error
	Next() bool
	Scan(dest ...any) error
}

type auditStoreDB interface {
	Exec(ctx context.Context, query string, args ...any) error
	Query(ctx context.Context, query string, args ...any) (auditStoreRows, error)
}

type pgxAuditStoreDB struct {
	pool *pgxpool.Pool
}

func (db *pgxAuditStoreDB) Exec(ctx context.Context, query string, args ...any) error {
	_, err := db.pool.Exec(ctx, query, args...)
	return err
}

func (db *pgxAuditStoreDB) Query(ctx context.Context, query string, args ...any) (auditStoreRows, error) {
	return db.pool.Query(ctx, query, args...)
}

type PostgresAuditStore struct {
	db auditStoreDB
}

func NewPostgresAuditStore(pool *pgxpool.Pool) *PostgresAuditStore {
	return newPostgresAuditStoreWithDB(&pgxAuditStoreDB{pool: pool})
}

func newPostgresAuditStoreWithDB(db auditStoreDB) *PostgresAuditStore {
	return &PostgresAuditStore{db: db}
}

func (s *PostgresAuditStore) Emit(ctx context.Context, event AuditEvent) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.db == nil {
		return errors.New("postgres audit store is not configured")
	}
	if event.EventID == "" {
		eventID, err := generateEventID()
		if err != nil {
			return err
		}
		event.EventID = eventID
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	const query = `
INSERT INTO auth_token_audit_events (
    event_id,
    event_name,
    request_id,
    token_id,
    subject_id,
    route,
    result_code,
    client_ip,
    created_at
) VALUES (
    $1,
    $2,
    $3,
    NULLIF($4, ''),
    NULLIF($5, ''),
    $6,
    $7,
    NULLIF($8, '')::inet,
    $9
)
`

	return s.db.Exec(ctx, query,
		event.EventID,
		event.EventName,
		event.RequestID,
		event.TokenID,
		event.SubjectID,
		event.Route,
		event.ResultCode,
		strings.TrimSpace(event.ClientIP),
		event.CreatedAt,
	)
}

func (s *PostgresAuditStore) QueryEvents(ctx context.Context, filter AuditEventFilter) ([]AuditEvent, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.db == nil {
		return nil, errors.New("postgres audit store is not configured")
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	query := `
SELECT
    event_id,
    event_name,
    request_id,
    COALESCE(token_id, ''),
    COALESCE(subject_id, ''),
    route,
    result_code,
    COALESCE(host(client_ip), ''),
    created_at
FROM auth_token_audit_events
WHERE 1=1
`
	args := make([]any, 0, 6)
	if filter.RequestID != "" {
		args = append(args, strings.TrimSpace(filter.RequestID))
		query += " AND request_id = $" + strconvArg(len(args))
	}
	if filter.TokenID != "" {
		args = append(args, strings.TrimSpace(filter.TokenID))
		query += " AND token_id = $" + strconvArg(len(args))
	}
	if filter.SubjectID != "" {
		args = append(args, strings.TrimSpace(filter.SubjectID))
		query += " AND subject_id = $" + strconvArg(len(args))
	}
	if filter.EventName != "" {
		args = append(args, strings.TrimSpace(filter.EventName))
		query += " AND event_name = $" + strconvArg(len(args))
	}
	if filter.ResultCode != "" {
		args = append(args, strings.TrimSpace(filter.ResultCode))
		query += " AND result_code = $" + strconvArg(len(args))
	}
	args = append(args, limit)
	query += " ORDER BY created_at DESC LIMIT $" + strconvArg(len(args))

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]AuditEvent, 0, limit)
	for rows.Next() {
		var event AuditEvent
		if err := rows.Scan(
			&event.EventID,
			&event.EventName,
			&event.RequestID,
			&event.TokenID,
			&event.SubjectID,
			&event.Route,
			&event.ResultCode,
			&event.ClientIP,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, nil
}

func strconvArg(position int) string {
	return strconv.Itoa(position)
}
