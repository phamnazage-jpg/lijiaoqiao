package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"lijiaoqiao/supply-api/internal/audit/model"
)

// EventFilter 事件查询过滤器（仓储层定义，避免循环依赖）
type EventFilter struct {
	TenantID   int64
	OperatorID int64
	Category   string
	EventName  string
	StartTime  *time.Time
	EndTime    *time.Time
	Limit      int
	Offset     int
}

// AuditRepository 审计事件仓储接口
type AuditRepository interface {
	// Emit 发送审计事件
	Emit(ctx context.Context, event *model.AuditEvent) error
	// Query 查询审计事件
	Query(ctx context.Context, filter *EventFilter) ([]*model.AuditEvent, int64, error)
	// GetByIdempotencyKey 根据幂等键获取事件
	GetByIdempotencyKey(ctx context.Context, key string) (*model.AuditEvent, error)
}

// PostgresAuditRepository PostgreSQL实现的审计仓储
type PostgresAuditRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAuditRepository 创建PostgreSQL审计仓储
func NewPostgresAuditRepository(pool *pgxpool.Pool) *PostgresAuditRepository {
	return &PostgresAuditRepository{pool: pool}
}

// Ensure interface
var _ AuditRepository = (*PostgresAuditRepository)(nil)

// Emit 发送审计事件
func (r *PostgresAuditRepository) Emit(ctx context.Context, event *model.AuditEvent) error {
	// 生成事件ID
	if event.EventID == "" {
		event.EventID = uuid.New().String()
	}

	// 设置时间戳
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	event.TimestampMs = event.Timestamp.UnixMilli()

	// 序列化扩展字段
	var extensionsJSON []byte
	if event.Extensions != nil {
		var err error
		extensionsJSON, err = json.Marshal(event.Extensions)
		if err != nil {
			return fmt.Errorf("failed to marshal extensions: %w", err)
		}
	}

	// 序列化安全标记
	securityFlagsJSON, err := json.Marshal(event.SecurityFlags)
	if err != nil {
		return fmt.Errorf("failed to marshal security flags: %w", err)
	}

	// 序列化状态变更
	var beforeStateJSON, afterStateJSON []byte
	if event.BeforeState != nil {
		beforeStateJSON, err = json.Marshal(event.BeforeState)
		if err != nil {
			return fmt.Errorf("failed to marshal before state: %w", err)
		}
	}
	if event.AfterState != nil {
		afterStateJSON, err = json.Marshal(event.AfterState)
		if err != nil {
			return fmt.Errorf("failed to marshal after state: %w", err)
		}
	}

	query := `
		INSERT INTO audit_events (
			event_id, event_name, event_category, event_sub_category,
			timestamp, timestamp_ms,
			request_id, trace_id, span_id,
			idempotency_key,
			operator_id, operator_type, operator_role,
			tenant_id, tenant_type,
			object_type, object_id,
			action, action_detail,
			credential_type, credential_id, credential_fingerprint,
			source_type, source_ip, source_region, user_agent,
			target_type, target_endpoint, target_direct,
			result_code, result_message, success,
			before_data, after_data,
			security_flags, risk_score,
			compliance_tags, invariant_rule,
			extensions,
			version, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
			$31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41
		)
	`

	_, err = r.pool.Exec(ctx, query,
		event.EventID, event.EventName, event.EventCategory, event.EventSubCategory,
		event.Timestamp, event.TimestampMs,
		event.RequestID, event.TraceID, event.SpanID,
		event.IdempotencyKey,
		event.OperatorID, event.OperatorType, event.OperatorRole,
		event.TenantID, event.TenantType,
		event.ObjectType, event.ObjectID,
		event.Action, event.ActionDetail,
		event.CredentialType, event.CredentialID, event.CredentialFingerprint,
		event.SourceType, event.SourceIP, event.SourceRegion, event.UserAgent,
		event.TargetType, event.TargetEndpoint, event.TargetDirect,
		event.ResultCode, event.ResultMessage, event.Success,
		beforeStateJSON, afterStateJSON,
		securityFlagsJSON, event.RiskScore,
		event.ComplianceTags, event.InvariantRule,
		extensionsJSON,
		1, time.Now(),
	)

	if err != nil {
		// 检查幂等键重复
		if strings.Contains(err.Error(), "idempotency_key") && strings.Contains(err.Error(), "unique") {
			return ErrDuplicateIdempotencyKey
		}
		return fmt.Errorf("failed to emit audit event: %w", err)
	}

	return nil
}

// Query 查询审计事件
func (r *PostgresAuditRepository) Query(ctx context.Context, filter *EventFilter) ([]*model.AuditEvent, int64, error) {
	// 构建查询条件
	conditions := []string{}
	args := []interface{}{}
	argIndex := 1

	if filter.TenantID != 0 {
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", argIndex))
		args = append(args, filter.TenantID)
		argIndex++
	}

	if filter.Category != "" {
		conditions = append(conditions, fmt.Sprintf("event_category = $%d", argIndex))
		args = append(args, filter.Category)
		argIndex++
	}

	if filter.EventName != "" {
		conditions = append(conditions, fmt.Sprintf("event_name = $%d", argIndex))
		args = append(args, filter.EventName)
		argIndex++
	}

	if filter.OperatorID != 0 {
		conditions = append(conditions, fmt.Sprintf("operator_id = $%d", argIndex))
		args = append(args, filter.OperatorID)
		argIndex++
	}

	if filter.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argIndex))
		args = append(args, *filter.StartTime)
		argIndex++
	}

	if filter.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argIndex))
		args = append(args, *filter.EndTime)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_events %s", whereClause)
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count audit events: %w", err)
	}

	// 查询事件列表
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT
			event_id, event_name, event_category, event_sub_category,
			timestamp, timestamp_ms,
			request_id, trace_id, span_id,
			idempotency_key,
			operator_id, operator_type, operator_role,
			tenant_id, tenant_type,
			object_type, object_id,
			action, action_detail,
			credential_type, credential_id, credential_fingerprint,
			source_type, source_ip, source_region, user_agent,
			target_type, target_endpoint, target_direct,
			result_code, result_message, success,
			before_data, after_data,
			security_flags, risk_score,
			compliance_tags, invariant_rule,
			extensions,
			version, created_at
		FROM audit_events
		%s
		ORDER BY timestamp DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query audit events: %w", err)
	}
	defer rows.Close()

	var events []*model.AuditEvent
	for rows.Next() {
		event, err := r.scanAuditEvent(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit event: %w", err)
		}
		events = append(events, event)
	}

	return events, total, nil
}

// GetByIdempotencyKey 根据幂等键获取事件
func (r *PostgresAuditRepository) GetByIdempotencyKey(ctx context.Context, key string) (*model.AuditEvent, error) {
	query := `
		SELECT
			event_id, event_name, event_category, event_sub_category,
			timestamp, timestamp_ms,
			request_id, trace_id, span_id,
			idempotency_key,
			operator_id, operator_type, operator_role,
			tenant_id, tenant_type,
			object_type, object_id,
			action, action_detail,
			credential_type, credential_id, credential_fingerprint,
			source_type, source_ip, source_region, user_agent,
			target_type, target_endpoint, target_direct,
			result_code, result_message, success,
			before_data, after_data,
			security_flags, risk_score,
			compliance_tags, invariant_rule,
			extensions,
			version, created_at
		FROM audit_events
		WHERE idempotency_key = $1
	`

	row := r.pool.QueryRow(ctx, query, key)
	event, err := r.scanAuditEventRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get event by idempotency key: %w", err)
	}

	return event, nil
}

// scanAuditEvent 扫描审计事件行
func (r *PostgresAuditRepository) scanAuditEvent(rows pgx.Rows) (*model.AuditEvent, error) {
	var event model.AuditEvent
	var eventSubCategory, traceID, spanID, idempotencyKey, operatorRole string
	var beforeData, afterData, extensions []byte
	var securityFlagsJSON []byte
	var complianceTags []string

	err := rows.Scan(
		&event.EventID, &event.EventName, &event.EventCategory, &eventSubCategory,
		&event.Timestamp, &event.TimestampMs,
		&event.RequestID, &traceID, &spanID,
		&idempotencyKey,
		&event.OperatorID, &event.OperatorType, &operatorRole,
		&event.TenantID, &event.TenantType,
		&event.ObjectType, &event.ObjectID,
		&event.Action, &event.ActionDetail,
		&event.CredentialType, &event.CredentialID, &event.CredentialFingerprint,
		&event.SourceType, &event.SourceIP, &event.SourceRegion, &event.UserAgent,
		&event.TargetType, &event.TargetEndpoint, &event.TargetDirect,
		&event.ResultCode, &event.ResultMessage, &event.Success,
		&beforeData, &afterData,
		&securityFlagsJSON, &event.RiskScore,
		&complianceTags, &event.InvariantRule,
		&extensions,
		&event.Version, &event.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	event.EventSubCategory = eventSubCategory
	event.TraceID = traceID
	event.SpanID = spanID
	event.IdempotencyKey = idempotencyKey
	event.OperatorRole = operatorRole
	event.ComplianceTags = complianceTags

	// 反序列化JSON字段
	if beforeData != nil {
		json.Unmarshal(beforeData, &event.BeforeState)
	}
	if afterData != nil {
		json.Unmarshal(afterData, &event.AfterState)
	}
	if securityFlagsJSON != nil {
		json.Unmarshal(securityFlagsJSON, &event.SecurityFlags)
	}
	if extensions != nil {
		json.Unmarshal(extensions, &event.Extensions)
	}

	return &event, nil
}

// scanAuditEventRow 扫描单行审计事件
func (r *PostgresAuditRepository) scanAuditEventRow(row pgx.Row) (*model.AuditEvent, error) {
	var event model.AuditEvent
	var eventSubCategory, traceID, spanID, idempotencyKey, operatorRole string
	var beforeData, afterData, extensions []byte
	var securityFlagsJSON []byte
	var complianceTags []string

	err := row.Scan(
		&event.EventID, &event.EventName, &event.EventCategory, &eventSubCategory,
		&event.Timestamp, &event.TimestampMs,
		&event.RequestID, &traceID, &spanID,
		&idempotencyKey,
		&event.OperatorID, &event.OperatorType, &operatorRole,
		&event.TenantID, &event.TenantType,
		&event.ObjectType, &event.ObjectID,
		&event.Action, &event.ActionDetail,
		&event.CredentialType, &event.CredentialID, &event.CredentialFingerprint,
		&event.SourceType, &event.SourceIP, &event.SourceRegion, &event.UserAgent,
		&event.TargetType, &event.TargetEndpoint, &event.TargetDirect,
		&event.ResultCode, &event.ResultMessage, &event.Success,
		&beforeData, &afterData,
		&securityFlagsJSON, &event.RiskScore,
		&complianceTags, &event.InvariantRule,
		&extensions,
		&event.Version, &event.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	event.EventSubCategory = eventSubCategory
	event.TraceID = traceID
	event.SpanID = spanID
	event.IdempotencyKey = idempotencyKey
	event.OperatorRole = operatorRole
	event.ComplianceTags = complianceTags

	// 反序列化JSON字段
	if beforeData != nil {
		json.Unmarshal(beforeData, &event.BeforeState)
	}
	if afterData != nil {
		json.Unmarshal(afterData, &event.AfterState)
	}
	if securityFlagsJSON != nil {
		json.Unmarshal(securityFlagsJSON, &event.SecurityFlags)
	}
	if extensions != nil {
		json.Unmarshal(extensions, &event.Extensions)
	}

	return &event, nil
}

// errors
var (
	ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
)
