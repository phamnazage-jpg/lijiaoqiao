package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PartitionConfig 分区配置
type PartitionConfig struct {
	TableName        string
	PartitionType    string // RANGE, LIST
	PartitionKey     string
	RetentionMonths  int    // 0 = 永久保留
	PreCreateMonths  int
}

// PartitionManager 分区管理器
type PartitionManager struct {
	pool   *pgxpool.Pool
	config map[string]*PartitionConfig
}

// NewPartitionManager 创建分区管理器
func NewPartitionManager(pool *pgxpool.Pool) *PartitionManager {
	return &PartitionManager{
		pool: pool,
		config: map[string]*PartitionConfig{
			"audit_events": {
				TableName:        "audit_events",
				PartitionType:    "RANGE",
				PartitionKey:     "timestamp",
				RetentionMonths:  12,
				PreCreateMonths:  3,
			},
			"supply_usage_records": {
				TableName:        "supply_usage_records",
				PartitionType:    "RANGE",
				PartitionKey:     "started_at",
				RetentionMonths:  3,
				PreCreateMonths:  3,
			},
			"supply_idempotency_records": {
				TableName:        "supply_idempotency_records",
				PartitionType:    "RANGE",
				PartitionKey:     "expires_at",
				RetentionMonths:  1, // 保留1个月
				PreCreateMonths:  1,
			},
		},
	}
}

// EnsureFuturePartitions 确保未来分区已创建
func (m *PartitionManager) EnsureFuturePartitions(ctx context.Context) error {
	for tableName, cfg := range m.config {
		for i := 0; i <= cfg.PreCreateMonths; i++ {
			futureDate := time.Now().AddDate(0, i, 0)
			if err := m.createPartition(ctx, tableName, futureDate); err != nil {
				return fmt.Errorf("failed to create partition for %s at %s: %w", tableName, futureDate.Format("2006-01"), err)
			}
		}
	}
	return nil
}

// createPartition 创建单个分区
func (m *PartitionManager) createPartition(ctx context.Context, tableName string, partitionDate time.Time) error {
	startDate := time.Date(partitionDate.Year(), partitionDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0)
	partitionName := fmt.Sprintf("%s_%s", tableName, startDate.Format("2006_01"))

	// 检查分区是否已存在
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM pg_class WHERE relname = $1)`
	if err := m.pool.QueryRow(ctx, checkQuery, partitionName).Scan(&exists); err != nil {
		return fmt.Errorf("failed to check partition existence: %w", err)
	}
	if exists {
		return nil // 分区已存在
	}

	// 创建分区
	createQuery := fmt.Sprintf(
		"CREATE TABLE %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')",
		partitionName, tableName, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"),
	)

	if _, err := m.pool.Exec(ctx, createQuery); err != nil {
		return fmt.Errorf("failed to create partition: %w", err)
	}

	return nil
}

// DropOldPartitions 删除过期分区
func (m *PartitionManager) DropOldPartitions(ctx context.Context, tableName string) (int, error) {
	cfg, ok := m.config[tableName]
	if !ok {
		return 0, fmt.Errorf("unknown table: %s", tableName)
	}
	if cfg.RetentionMonths == 0 {
		return 0, nil // 永久保留，不删除
	}

	cutoffDate := time.Now().AddDate(0, -cfg.RetentionMonths, 0)
	cutoffPrefix := fmt.Sprintf("%s_%s", tableName, cutoffDate.Format("2006_01"))

	var droppedCount int

	// 查询所有该表的分区
	query := `
		SELECT relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'
		AND relname ~ $1
		AND relname < $2
	`

	rows, err := m.pool.Query(ctx, query, fmt.Sprintf("^%s_[0-9]{4}_[0-9]{2}$", tableName), cutoffPrefix)
	if err != nil {
		return 0, fmt.Errorf("failed to query partitions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var partitionName string
		if err := rows.Scan(&partitionName); err != nil {
			continue
		}

		// 删除分区
		dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s", partitionName)
		if _, err := m.pool.Exec(ctx, dropQuery); err != nil {
			return droppedCount, fmt.Errorf("failed to drop partition %s: %w", partitionName, err)
		}
		droppedCount++
	}

	return droppedCount, nil
}

// ListPartitions 列出表的所有分区
func (m *PartitionManager) ListPartitions(ctx context.Context, tableName string) ([]string, error) {
	query := `
		SELECT relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'
		AND relname ~ $1
		ORDER BY relname
	`

	rows, err := m.pool.Query(ctx, query, fmt.Sprintf("^%s_[0-9]{4}_[0-9]{2}$", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to query partitions: %w", err)
	}
	defer rows.Close()

	var partitions []string
	for rows.Next() {
		var partitionName string
		if err := rows.Scan(&partitionName); err != nil {
			continue
		}
		partitions = append(partitions, partitionName)
	}

	return partitions, nil
}

// IsPartitioned 检查表是否已分区
func (m *PartitionManager) IsPartitioned(ctx context.Context, tableName string) (bool, error) {
	query := `
		SELECT relkind
		FROM pg_class
		WHERE relname = $1
	`

	var relkind string
	if err := m.pool.QueryRow(ctx, query, tableName).Scan(&relkind); err != nil {
		return false, fmt.Errorf("failed to check table: %w", err)
	}

	// 'p' = partitioned table
	return relkind == "p", nil
}
