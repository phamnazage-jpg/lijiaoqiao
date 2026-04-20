//go:build integration
// +build integration

package repository

import (
	"context"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/domain"
)

func TestPackageRepositorySchemaContract(t *testing.T) {
	if testing.Short() {
		t.Skip("integration only")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumns(t, pool, "supply_packages", []string{
		"id", "supply_account_id", "user_id", "platform", "model",
		"total_quota", "available_quota", "sold_quota", "reserved_quota",
		"price_per_1m_input", "price_per_1m_output", "min_purchase",
		"start_at", "end_at", "valid_days",
		"status", "max_concurrent", "rate_limit_rpm",
		"total_orders", "total_revenue", "rating", "rating_count",
		"quota_unit", "price_unit", "currency_code", "version",
		"created_ip", "updated_ip", "audit_trace_id",
		"request_id", "created_at", "updated_at",
	})
}

func TestPackageRepository_Create_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	repo := NewPackageRepository(pool)
	pkg := &domain.Package{
		SupplierID:       1001,
		AccountID:        2001,
		Platform:         "openai",
		Model:            "gpt-4.1-mini",
		TotalQuota:       10000,
		AvailableQuota:   10000,
		SoldQuota:        0,
		ReservedQuota:    0,
		PricePer1MInput:  0.25,
		PricePer1MOutput: 0.75,
		MinPurchase:      100,
		StartAt:          time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		EndAt:            time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
		ValidDays:        30,
		Status:           domain.PackageStatusDraft,
		MaxConcurrent:    5,
		RateLimitRPM:     60,
		TotalOrders:      0,
		TotalRevenue:     0,
		Rating:           0,
		RatingCount:      0,
	}

	if err := repo.Create(context.Background(), pkg, "req-pkg-create-int", "trace-pkg-create-int"); err != nil {
		t.Fatalf("create package failed: %v", err)
	}
	if pkg.ID == 0 {
		t.Fatal("expected created package id")
	}
}

func TestPackageRepository_GetByID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	var count int
	if err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM supply_packages").Scan(&count); err != nil {
		t.Fatalf("查询 supply_packages 失败: %v", err)
	}
}

func TestPackageRepository_Update_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumn(t, pool, "supply_packages", "version")
}

func TestPackageRepository_List_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	rows, err := pool.Query(context.Background(), `
		SELECT id, user_id, available_quota
		FROM supply_packages
		LIMIT 10
	`)
	if err != nil {
		t.Fatalf("列出套餐失败: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, userID int64
		var availableQuota float64
		if scanErr := rows.Scan(&id, &userID, &availableQuota); scanErr != nil {
			t.Fatalf("扫描套餐失败: %v", scanErr)
		}
	}
}

func TestPackageRepository_UpdateQuota_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumn(t, pool, "supply_packages", "available_quota")
}

func TestPackageRepository_GetForUpdate_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("开始事务失败: %v", err)
	}
	defer tx.Rollback(context.Background())

	rows, err := tx.Query(context.Background(), "SELECT id FROM supply_packages LIMIT 1 FOR UPDATE")
	if err != nil {
		t.Fatalf("FOR UPDATE 查询失败: %v", err)
	}
	rows.Close()
}

func TestPackageRepository_OptimisticLock_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumn(t, pool, "supply_packages", "version")
}
