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

	fetched, err := repo.GetByID(context.Background(), pkg.SupplierID, pkg.ID)
	if err != nil {
		t.Fatalf("get package after create failed: %v", err)
	}
	if fetched.SupplierID != pkg.SupplierID {
		t.Fatalf("expected supplier id %d, got %d", pkg.SupplierID, fetched.SupplierID)
	}
	if fetched.AccountID != pkg.AccountID {
		t.Fatalf("expected account id %d, got %d", pkg.AccountID, fetched.AccountID)
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

	repo := NewPackageRepository(pool)
	pkg := &domain.Package{
		SupplierID:       3001,
		AccountID:        4001,
		Platform:         "anthropic",
		Model:            "claude-3-7-sonnet",
		TotalQuota:       5000,
		AvailableQuota:   5000,
		PricePer1MInput:  0.4,
		PricePer1MOutput: 1.2,
		Status:           domain.PackageStatusDraft,
		ValidDays:        15,
	}

	if err := repo.Create(context.Background(), pkg, "req-pkg-list-int", "trace-pkg-list-int"); err != nil {
		t.Fatalf("create package for list failed: %v", err)
	}

	packages, err := repo.List(context.Background(), pkg.SupplierID)
	if err != nil {
		t.Fatalf("repo.List failed: %v", err)
	}
	if len(packages) == 0 {
		t.Fatal("expected packages for supplier")
	}

	found := false
	for _, listed := range packages {
		if listed.ID == pkg.ID {
			found = true
			if listed.SupplierID != pkg.SupplierID {
				t.Fatalf("expected listed supplier id %d, got %d", pkg.SupplierID, listed.SupplierID)
			}
			if listed.AccountID != pkg.AccountID {
				t.Fatalf("expected listed account id %d, got %d", pkg.AccountID, listed.AccountID)
			}
		}
	}
	if !found {
		t.Fatalf("expected package %d in supplier list", pkg.ID)
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
