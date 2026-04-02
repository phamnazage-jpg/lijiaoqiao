package middleware

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// DatabaseAuditEmitter 实现 AuditEmitter 接口，将审计事件存入数据库
type DatabaseAuditEmitter struct {
	db  *sql.DB
	mu  sync.RWMutex
	now func() time.Time
}

// NewDatabaseAuditEmitter 创建数据库审计发射器
func NewDatabaseAuditEmitter(dsn string, now func() time.Time) (*DatabaseAuditEmitter, error) {
	if now == nil {
		now = time.Now
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	emitter := &DatabaseAuditEmitter{
		db:  db,
		now: now,
	}

	// 初始化表
	if err := emitter.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return emitter, nil
}

// initSchema 创建审计表
func (e *DatabaseAuditEmitter) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS token_audit_events (
		event_id VARCHAR(64) PRIMARY KEY,
		event_name VARCHAR(128) NOT NULL,
		request_id VARCHAR(128) NOT NULL,
		token_id VARCHAR(128),
		subject_id VARCHAR(128),
		route VARCHAR(256) NOT NULL,
		result_code VARCHAR(64) NOT NULL,
		client_ip VARCHAR(64),
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_token_audit_request_id ON token_audit_events(request_id);
	CREATE INDEX IF NOT EXISTS idx_token_audit_token_id ON token_audit_events(token_id);
	CREATE INDEX IF NOT EXISTS idx_token_audit_subject_id ON token_audit_events(subject_id);
	CREATE INDEX IF NOT EXISTS idx_token_audit_created_at ON token_audit_events(created_at);
	`
	_, err := e.db.Exec(schema)
	return err
}

// Emit 实现 AuditEmitter 接口
func (e *DatabaseAuditEmitter) Emit(_ context.Context, event AuditEvent) error {
	if event.EventID == "" {
		event.EventID = fmt.Sprintf("evt-%d", e.now().UnixNano())
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = e.now()
	}

	query := `
		INSERT INTO token_audit_events (event_id, event_name, request_id, token_id, subject_id, route, result_code, client_ip, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := e.db.Exec(query,
		event.EventID,
		event.EventName,
		event.RequestID,
		nullString(event.TokenID),
		nullString(event.SubjectID),
		event.Route,
		event.ResultCode,
		nullString(event.ClientIP),
		event.CreatedAt,
	)
	return err
}

// Close 关闭数据库连接
func (e *DatabaseAuditEmitter) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

// nullString 安全处理空字符串
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}