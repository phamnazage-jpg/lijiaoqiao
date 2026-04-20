//go:build integration
// +build integration

package adapter

import (
	"context"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/repository"
)

func createAdapterIntegrationPackage(t *testing.T, repo *repository.PackageRepository, supplierID, accountID int64, status domain.PackageStatus) *domain.Package {
	t.Helper()

	pkg := &domain.Package{
		SupplierID:       supplierID,
		AccountID:        accountID,
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
		Status:           status,
		MaxConcurrent:    5,
		RateLimitRPM:     60,
		Version:          1,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(context.Background(), pkg, "req-pkg-lifecycle-int", "trace-pkg-lifecycle-int"); err != nil {
		t.Fatalf("创建测试套餐失败: %v", err)
	}

	return pkg
}

func TestDBPackageStore_Lifecycle_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getIntegrationDB(t)
	if pool == nil {
		return
	}

	repo := repository.NewPackageRepository(pool)
	store := NewDBPackageStore(repo)
	service := domain.NewPackageService(store, NewInMemoryAccountStoreAdapter(), audit.NewMemoryAuditStore())
	supplierID := time.Now().UnixNano()
	accountID := supplierID + 1000
	pkg := createAdapterIntegrationPackage(t, repo, supplierID, accountID, domain.PackageStatusDraft)

	fetched, err := store.GetByID(context.Background(), supplierID, pkg.ID)
	if err != nil {
		t.Fatalf("读取套餐失败: %v", err)
	}
	if fetched.SupplierID != supplierID {
		t.Fatalf("expected fetched supplier id %d, got %d", supplierID, fetched.SupplierID)
	}
	if fetched.AccountID != accountID {
		t.Fatalf("expected fetched account id %d, got %d", accountID, fetched.AccountID)
	}

	published, err := service.Publish(context.Background(), supplierID, pkg.ID)
	if err != nil {
		t.Fatalf("发布套餐失败: %v", err)
	}
	if published.Status != domain.PackageStatusActive {
		t.Fatalf("expected published status %q, got %q", domain.PackageStatusActive, published.Status)
	}

	paused, err := service.Pause(context.Background(), supplierID, pkg.ID)
	if err != nil {
		t.Fatalf("暂停套餐失败: %v", err)
	}
	if paused.Status != domain.PackageStatusPaused {
		t.Fatalf("expected paused status %q, got %q", domain.PackageStatusPaused, paused.Status)
	}

	unlisted, err := service.Unlist(context.Background(), supplierID, pkg.ID)
	if err != nil {
		t.Fatalf("下架套餐失败: %v", err)
	}
	if unlisted.Status != domain.PackageStatusExpired {
		t.Fatalf("expected unlisted status %q, got %q", domain.PackageStatusExpired, unlisted.Status)
	}

	after, err := repo.GetByID(context.Background(), supplierID, pkg.ID)
	if err != nil {
		t.Fatalf("生命周期完成后读取套餐失败: %v", err)
	}
	if after.Status != domain.PackageStatusExpired {
		t.Fatalf("expected persisted status %q, got %q", domain.PackageStatusExpired, after.Status)
	}
	if after.SupplierID != supplierID {
		t.Fatalf("expected persisted supplier id %d, got %d", supplierID, after.SupplierID)
	}
	if after.AccountID != accountID {
		t.Fatalf("expected persisted account id %d, got %d", accountID, after.AccountID)
	}
}
