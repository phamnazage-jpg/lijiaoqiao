//go:build integration
// +build integration

package adapter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/repository"
)

func getIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	host := os.Getenv("SUPPLY_API_DB_HOST")
	if host == "" {
		host = "/var/run/postgresql"
	}
	port := os.Getenv("SUPPLY_API_DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("SUPPLY_API_DB_USER")
	if user == "" {
		user = "long"
	}
	password := os.Getenv("SUPPLY_API_DB_PASSWORD")
	dbName := os.Getenv("SUPPLY_API_DB_NAME")
	if dbName == "" {
		dbName = "supply_test"
	}

	var dsn string
	if host[0] == '/' {
		dsn = "postgres://" + user + ":" + password + "@/" + dbName + "?host=" + host + "&sslmode=disable"
	} else {
		dsn = "postgres://" + user + ":" + password + "@" + host + ":" + port + "/" + dbName + "?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("跳过集成测试：无法连接数据库: %v", err)
		return nil
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("跳过集成测试：无法 ping 数据库: %v", err)
		return nil
	}

	t.Cleanup(func() {
		pool.Close()
	})
	return pool
}

func createAdapterIntegrationAccount(t *testing.T, repo *repository.AccountRepository, supplierID int64, status domain.AccountStatus) *domain.Account {
	t.Helper()

	account := &domain.Account{
		SupplierID:     supplierID,
		Provider:       domain.ProviderOpenAI,
		AccountType:    domain.AccountTypeAPIKey,
		Alias:          t.Name(),
		Status:         status,
		RiskLevel:      "low",
		TotalQuota:     1000,
		AvailableQuota: 1000,
		Version:        1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := repo.Create(context.Background(), account, "", "", ""); err != nil {
		t.Fatalf("创建测试账号失败: %v", err)
	}

	return account
}

func TestDBAccountStore_Activate_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getIntegrationDB(t)
	if pool == nil {
		return
	}

	repo := repository.NewAccountRepository(pool)
	store := NewDBAccountStore(repo)
	service := domain.NewAccountService(store, audit.NewMemoryAuditStore())
	supplierID := time.Now().UnixNano()
	account := createAdapterIntegrationAccount(t, repo, supplierID, domain.AccountStatusPending)

	before, err := repo.GetByID(context.Background(), supplierID, account.ID)
	if err != nil {
		t.Fatalf("激活前读取账号失败: %v", err)
	}

	activated, err := service.Activate(context.Background(), supplierID, account.ID)
	if err != nil {
		t.Fatalf("激活账号失败: %v", err)
	}

	if activated.Status != domain.AccountStatusActive {
		t.Fatalf("expected activated status %q, got %q", domain.AccountStatusActive, activated.Status)
	}
	if activated.Version != before.Version+1 {
		t.Fatalf("expected activated version %d, got %d", before.Version+1, activated.Version)
	}

	after, err := repo.GetByID(context.Background(), supplierID, account.ID)
	if err != nil {
		t.Fatalf("激活后读取账号失败: %v", err)
	}
	if after.Status != domain.AccountStatusActive {
		t.Fatalf("expected persisted status %q, got %q", domain.AccountStatusActive, after.Status)
	}
	if after.Version != before.Version+1 {
		t.Fatalf("expected persisted version %d, got %d", before.Version+1, after.Version)
	}
}

func TestDBAccountStore_Suspend_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getIntegrationDB(t)
	if pool == nil {
		return
	}

	repo := repository.NewAccountRepository(pool)
	store := NewDBAccountStore(repo)
	service := domain.NewAccountService(store, audit.NewMemoryAuditStore())
	supplierID := time.Now().UnixNano()
	account := createAdapterIntegrationAccount(t, repo, supplierID, domain.AccountStatusActive)

	before, err := repo.GetByID(context.Background(), supplierID, account.ID)
	if err != nil {
		t.Fatalf("暂停前读取账号失败: %v", err)
	}

	suspended, err := service.Suspend(context.Background(), supplierID, account.ID)
	if err != nil {
		t.Fatalf("暂停账号失败: %v", err)
	}

	if suspended.Status != domain.AccountStatusSuspended {
		t.Fatalf("expected suspended status %q, got %q", domain.AccountStatusSuspended, suspended.Status)
	}
	if suspended.Version != before.Version+1 {
		t.Fatalf("expected suspended version %d, got %d", before.Version+1, suspended.Version)
	}

	after, err := repo.GetByID(context.Background(), supplierID, account.ID)
	if err != nil {
		t.Fatalf("暂停后读取账号失败: %v", err)
	}
	if after.Status != domain.AccountStatusSuspended {
		t.Fatalf("expected persisted status %q, got %q", domain.AccountStatusSuspended, after.Status)
	}
	if after.Version != before.Version+1 {
		t.Fatalf("expected persisted version %d, got %d", before.Version+1, after.Version)
	}
}
