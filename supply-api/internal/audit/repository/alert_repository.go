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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"lijiaoqiao/supply-api/internal/audit/alerterr"
	"lijiaoqiao/supply-api/internal/audit/model"
)

var errAlertRepositoryPoolRequired = errors.New("postgres alert repository requires a pool")

// PostgresAlertRepository PostgreSQL实现的告警仓储。
type PostgresAlertRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAlertRepository 创建PostgreSQL告警仓储。
func NewPostgresAlertRepository(pool *pgxpool.Pool) *PostgresAlertRepository {
	return &PostgresAlertRepository{pool: pool}
}

// Create 创建告警。
func (r *PostgresAlertRepository) Create(ctx context.Context, alert *model.Alert) error {
	if err := r.requirePool(); err != nil {
		return err
	}
	if alert == nil {
		return alerterr.ErrInvalidAlertInput
	}

	now := time.Now()
	if alert.AlertID == "" {
		alert.AlertID = generateAlertID()
	}
	if alert.Status == "" {
		alert.Status = model.AlertStatusActive
	}
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = now
	}
	if alert.UpdatedAt.IsZero() {
		alert.UpdatedAt = alert.CreatedAt
	}
	if alert.FirstSeenAt.IsZero() {
		alert.FirstSeenAt = alert.CreatedAt
	}
	if alert.LastSeenAt.IsZero() {
		alert.LastSeenAt = alert.UpdatedAt
	}

	eventIDsJSON, notifyChannelsJSON, metadataJSON, tagsJSON, err := marshalAlertCollections(alert)
	if err != nil {
		return err
	}

	const query = `
		INSERT INTO audit_alerts (
			alert_id, alert_name, alert_type, alert_level, tenant_id, supplier_id,
			title, message, description,
			event_id, event_ids,
			trigger_condition, threshold, current_value,
			status, resolved_at, resolved_by, resolve_note,
			notify_enabled, notify_channels,
			created_at, updated_at, first_seen_at, last_seen_at,
			metadata, tags
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9,
			$10, $11,
			$12, $13, $14,
			$15, $16, $17, $18,
			$19, $20,
			$21, $22, $23, $24,
			$25, $26
		)
	`

	_, err = r.pool.Exec(ctx, query,
		alert.AlertID, alert.AlertName, alert.AlertType, alert.AlertLevel, alert.TenantID, nullableInt64(alert.SupplierID),
		alert.Title, alert.Message, alert.Description,
		nullableString(alert.EventID), eventIDsJSON,
		alert.TriggerCondition, alert.Threshold, alert.CurrentValue,
		alert.Status, alert.ResolvedAt, nullableString(alert.ResolvedBy), nullableString(alert.ResolveNote),
		alert.NotifyEnabled, notifyChannelsJSON,
		alert.CreatedAt, alert.UpdatedAt, alert.FirstSeenAt, alert.LastSeenAt,
		metadataJSON, tagsJSON,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return alerterr.ErrAlertConflict
		}
		return fmt.Errorf("create alert: %w", err)
	}

	return nil
}

// GetByID 根据ID查询告警。
func (r *PostgresAlertRepository) GetByID(ctx context.Context, alertID string) (*model.Alert, error) {
	if err := r.requirePool(); err != nil {
		return nil, err
	}

	const query = `
		SELECT
			alert_id, alert_name, alert_type, alert_level, tenant_id, supplier_id,
			title, message, description,
			event_id, event_ids,
			trigger_condition, threshold, current_value,
			status, resolved_at, resolved_by, resolve_note,
			notify_enabled, notify_channels,
			created_at, updated_at, first_seen_at, last_seen_at,
			metadata, tags
		FROM audit_alerts
		WHERE alert_id = $1
	`

	alert, err := scanAlert(r.pool.QueryRow(ctx, query, alertID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, alerterr.ErrAlertNotFound
		}
		return nil, fmt.Errorf("get alert by id: %w", err)
	}
	return alert, nil
}

// Update 更新告警。
func (r *PostgresAlertRepository) Update(ctx context.Context, alert *model.Alert) error {
	if err := r.requirePool(); err != nil {
		return err
	}
	if alert == nil || alert.AlertID == "" {
		return alerterr.ErrInvalidAlertInput
	}
	if alert.UpdatedAt.IsZero() {
		alert.UpdatedAt = time.Now()
	}

	eventIDsJSON, notifyChannelsJSON, metadataJSON, tagsJSON, err := marshalAlertCollections(alert)
	if err != nil {
		return err
	}

	const query = `
		UPDATE audit_alerts
		SET
			alert_name = $2,
			alert_type = $3,
			alert_level = $4,
			tenant_id = $5,
			supplier_id = $6,
			title = $7,
			message = $8,
			description = $9,
			event_id = $10,
			event_ids = $11,
			trigger_condition = $12,
			threshold = $13,
			current_value = $14,
			status = $15,
			resolved_at = $16,
			resolved_by = $17,
			resolve_note = $18,
			notify_enabled = $19,
			notify_channels = $20,
			updated_at = $21,
			first_seen_at = $22,
			last_seen_at = $23,
			metadata = $24,
			tags = $25
		WHERE alert_id = $1
	`

	tag, err := r.pool.Exec(ctx, query,
		alert.AlertID,
		alert.AlertName, alert.AlertType, alert.AlertLevel, alert.TenantID, nullableInt64(alert.SupplierID),
		alert.Title, alert.Message, alert.Description,
		nullableString(alert.EventID), eventIDsJSON,
		alert.TriggerCondition, alert.Threshold, alert.CurrentValue,
		alert.Status, alert.ResolvedAt, nullableString(alert.ResolvedBy), nullableString(alert.ResolveNote),
		alert.NotifyEnabled, notifyChannelsJSON,
		alert.UpdatedAt, alert.FirstSeenAt, alert.LastSeenAt,
		metadataJSON, tagsJSON,
	)
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return alerterr.ErrAlertNotFound
	}
	return nil
}

// Delete 删除告警。
func (r *PostgresAlertRepository) Delete(ctx context.Context, alertID string) error {
	if err := r.requirePool(); err != nil {
		return err
	}

	tag, err := r.pool.Exec(ctx, `DELETE FROM audit_alerts WHERE alert_id = $1`, alertID)
	if err != nil {
		return fmt.Errorf("delete alert: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return alerterr.ErrAlertNotFound
	}
	return nil
}

// List 查询告警列表。
func (r *PostgresAlertRepository) List(ctx context.Context, filter *model.AlertFilter) ([]*model.Alert, int64, error) {
	if err := r.requirePool(); err != nil {
		return nil, 0, err
	}
	if filter == nil {
		filter = &model.AlertFilter{}
	}

	conditions := make([]string, 0, 8)
	args := make([]any, 0, 8)
	nextArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.TenantID > 0 {
		conditions = append(conditions, "tenant_id = "+nextArg(filter.TenantID))
	}
	if filter.SupplierID > 0 {
		conditions = append(conditions, "supplier_id = "+nextArg(filter.SupplierID))
	}
	if filter.AlertType != "" {
		conditions = append(conditions, "alert_type = "+nextArg(filter.AlertType))
	}
	if filter.AlertLevel != "" {
		conditions = append(conditions, "alert_level = "+nextArg(filter.AlertLevel))
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = "+nextArg(filter.Status))
	}
	if !filter.StartTime.IsZero() {
		conditions = append(conditions, "created_at >= "+nextArg(filter.StartTime))
	}
	if !filter.EndTime.IsZero() {
		conditions = append(conditions, "created_at <= "+nextArg(filter.EndTime))
	}
	if filter.Keywords != "" {
		kw := "%" + strings.ToLower(filter.Keywords) + "%"
		placeholder := nextArg(kw)
		conditions = append(conditions, "(LOWER(title) LIKE "+placeholder+" OR LOWER(message) LIKE "+placeholder+")")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM audit_alerts" + whereClause
	var total int64
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count alerts: %w", err)
	}

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

	listArgs := append(append([]any{}, args...), limit, offset)
	query := `
		SELECT
			alert_id, alert_name, alert_type, alert_level, tenant_id, supplier_id,
			title, message, description,
			event_id, event_ids,
			trigger_condition, threshold, current_value,
			status, resolved_at, resolved_by, resolve_note,
			notify_enabled, notify_channels,
			created_at, updated_at, first_seen_at, last_seen_at,
			metadata, tags
		FROM audit_alerts` + whereClause + `
		ORDER BY created_at DESC, alert_id DESC
		LIMIT $` + fmt.Sprintf("%d", len(args)+1) + `
		OFFSET $` + fmt.Sprintf("%d", len(args)+2)

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	alerts := make([]*model.Alert, 0, limit)
	for rows.Next() {
		alert, scanErr := scanAlert(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("scan alert: %w", scanErr)
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate alerts: %w", err)
	}

	return alerts, total, nil
}

func (r *PostgresAlertRepository) requirePool() error {
	if r == nil || r.pool == nil {
		return errAlertRepositoryPoolRequired
	}
	return nil
}

type alertScanner interface {
	Scan(dest ...any) error
}

func scanAlert(scanner alertScanner) (*model.Alert, error) {
	var (
		alert        model.Alert
		supplierID   *int64
		eventID      *string
		resolvedBy   *string
		resolveNote  *string
		resolvedAt   *time.Time
		eventIDsJSON []byte
		notifyJSON   []byte
		metadataJSON []byte
		tagsJSON     []byte
	)

	err := scanner.Scan(
		&alert.AlertID, &alert.AlertName, &alert.AlertType, &alert.AlertLevel, &alert.TenantID, &supplierID,
		&alert.Title, &alert.Message, &alert.Description,
		&eventID, &eventIDsJSON,
		&alert.TriggerCondition, &alert.Threshold, &alert.CurrentValue,
		&alert.Status, &resolvedAt, &resolvedBy, &resolveNote,
		&alert.NotifyEnabled, &notifyJSON,
		&alert.CreatedAt, &alert.UpdatedAt, &alert.FirstSeenAt, &alert.LastSeenAt,
		&metadataJSON, &tagsJSON,
	)
	if err != nil {
		return nil, err
	}

	if supplierID != nil {
		alert.SupplierID = *supplierID
	}
	if eventID != nil {
		alert.EventID = *eventID
	}
	if resolvedBy != nil {
		alert.ResolvedBy = *resolvedBy
	}
	if resolveNote != nil {
		alert.ResolveNote = *resolveNote
	}
	if resolvedAt != nil {
		alert.ResolvedAt = resolvedAt
	}

	if err := unmarshalJSONSlice(eventIDsJSON, &alert.EventIDs); err != nil {
		return nil, fmt.Errorf("decode event_ids: %w", err)
	}
	if err := unmarshalJSONSlice(notifyJSON, &alert.NotifyChannels); err != nil {
		return nil, fmt.Errorf("decode notify_channels: %w", err)
	}
	if err := unmarshalJSONMap(metadataJSON, &alert.Metadata); err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}
	if err := unmarshalJSONSlice(tagsJSON, &alert.Tags); err != nil {
		return nil, fmt.Errorf("decode tags: %w", err)
	}

	return &alert, nil
}

func marshalAlertCollections(alert *model.Alert) ([]byte, []byte, []byte, []byte, error) {
	eventIDsJSON, err := marshalJSONOrDefault(alert.EventIDs, []string{})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("marshal event ids: %w", err)
	}
	notifyChannelsJSON, err := marshalJSONOrDefault(alert.NotifyChannels, []string{})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("marshal notify channels: %w", err)
	}
	metadataJSON, err := marshalJSONOrDefault(alert.Metadata, map[string]any{})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("marshal metadata: %w", err)
	}
	tagsJSON, err := marshalJSONOrDefault(alert.Tags, []string{})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("marshal tags: %w", err)
	}
	return eventIDsJSON, notifyChannelsJSON, metadataJSON, tagsJSON, nil
}

func marshalJSONOrDefault(value any, defaultValue any) ([]byte, error) {
	if value == nil {
		return json.Marshal(defaultValue)
	}
	return json.Marshal(value)
}

func unmarshalJSONSlice(data []byte, target *[]string) error {
	if len(data) == 0 {
		*target = []string{}
		return nil
	}
	return json.Unmarshal(data, target)
}

func unmarshalJSONMap(data []byte, target *map[string]any) error {
	if len(data) == 0 {
		*target = map[string]any{}
		return nil
	}
	return json.Unmarshal(data, target)
}

func generateAlertID() string {
	return "ALT-" + uuid.New().String()[:8]
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt64(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
