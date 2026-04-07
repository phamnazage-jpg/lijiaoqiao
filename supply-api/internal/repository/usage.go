package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"lijiaoqiao/supply-api/internal/domain"
)

// UsageRepository 用量记录仓储
type UsageRepository struct {
	pool *pgxpool.Pool
}

// NewUsageRepository 创建用量记录仓储
func NewUsageRepository(pool *pgxpool.Pool) *UsageRepository {
	return &UsageRepository{pool: pool}
}

// ListRecords 查询收益记录列表
func (r *UsageRepository) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*domain.EarningRecord, int, error) {
	// 解析日期
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid start date: %w", err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid end date: %w", err)
	}

	// 查询总数
	countQuery := `
		SELECT COUNT(*)
		FROM supply_usage_records
		WHERE supplier_user_id = $1
		AND started_at >= $2
		AND started_at < $3
	`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, supplierID, start, end.AddDate(0, 0, 1)).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count records: %w", err)
	}

	// 查询记录
	offset := (page - 1) * pageSize
	query := `
		SELECT
			id,
			supplier_user_id,
			total_cost,
			started_at,
			platform,
			model,
			total_tokens,
			success
		FROM supply_usage_records
		WHERE supplier_user_id = $1
		AND started_at >= $2
		AND started_at < $3
		ORDER BY started_at DESC
		LIMIT $4 OFFSET $5
	`

	rows, err := r.pool.Query(ctx, query, supplierID, start, end.AddDate(0, 0, 1), pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query records: %w", err)
	}
	defer rows.Close()

	var records []*domain.EarningRecord
	for rows.Next() {
		var id int64
		var supplierUserID int64
		var totalCost float64
		var startedAt time.Time
		var platform, model string
		var totalTokens int64
		var success bool

		if err := rows.Scan(&id, &supplierUserID, &totalCost, &startedAt, &platform, &model, &totalTokens, &success); err != nil {
			return nil, 0, fmt.Errorf("failed to scan record: %w", err)
		}

		status := "available"
		if !success {
			status = "pending"
		}

		records = append(records, &domain.EarningRecord{
			ID:         id,
			SupplierID: supplierUserID,
			Amount:     totalCost,
			EarningsType: "usage",
			Status:     status,
			Description: fmt.Sprintf("%s %s %d tokens", platform, model, totalTokens),
			EarnedAt:   startedAt,
		})
	}

	return records, total, nil
}

// GetBillingSummary 获取账单汇总
func (r *UsageRepository) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
	// 解析日期
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}

	// 查询汇总数据
	query := `
		SELECT
			COALESCE(SUM(total_cost), 0) as total_revenue,
			COUNT(*) as total_orders,
			COALESCE(SUM(total_tokens), 0) as total_usage,
			COUNT(*) as total_requests,
			COALESCE(AVG(CASE WHEN success THEN 100.0 ELSE 0.0 END), 0) as avg_success_rate
		FROM supply_usage_records
		WHERE supplier_user_id = $1
		AND started_at >= $2
		AND started_at < $3
	`

	var totalRevenue float64
	var totalOrders, totalUsage, totalRequests int64
	var avgSuccessRate float64

	err = r.pool.QueryRow(ctx, query, supplierID, start, end.AddDate(0, 0, 1)).Scan(
		&totalRevenue, &totalOrders, &totalUsage, &totalRequests, &avgSuccessRate,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query billing summary: %w", err)
	}

	// 平台费用假设为 1%
	platformFee := totalRevenue * 0.01
	netEarnings := totalRevenue - platformFee

	// 查询按平台分组的统计数据
	platformQuery := `
		SELECT
			platform,
			COALESCE(SUM(total_cost), 0) as revenue,
			COUNT(*) as orders,
			COALESCE(SUM(total_tokens), 0) as tokens,
			COALESCE(AVG(CASE WHEN success THEN 100.0 ELSE 0.0 END), 0) as success_rate
		FROM supply_usage_records
		WHERE supplier_user_id = $1
		AND started_at >= $2
		AND started_at < $3
		GROUP BY platform
		ORDER BY revenue DESC
	`

	platformRows, err := r.pool.Query(ctx, platformQuery, supplierID, start, end.AddDate(0, 0, 1))
	if err != nil {
		return nil, fmt.Errorf("failed to query platform stats: %w", err)
	}
	defer platformRows.Close()

	var byPlatform []domain.PlatformStat
	for platformRows.Next() {
		var platform string
		var revenue float64
		var orders int
		var tokens int64
		var successRate float64

		if err := platformRows.Scan(&platform, &revenue, &orders, &tokens, &successRate); err != nil {
			return nil, fmt.Errorf("failed to scan platform stat: %w", err)
		}

		byPlatform = append(byPlatform, domain.PlatformStat{
			Platform:    platform,
			Revenue:     revenue,
			Orders:      orders,
			Tokens:      tokens,
			SuccessRate: successRate,
		})
	}

	return &domain.BillingSummary{
		Period: domain.BillingPeriod{
			Start: startDate,
			End:   endDate,
		},
		Summary: domain.BillingTotal{
			TotalRevenue:   totalRevenue,
			TotalOrders:    int(totalOrders),
			TotalUsage:     totalUsage,
			TotalRequests:  totalRequests,
			AvgSuccessRate: avgSuccessRate,
			PlatformFee:    platformFee,
			NetEarnings:    netEarnings,
		},
		ByPlatform: byPlatform,
	}, nil
}
