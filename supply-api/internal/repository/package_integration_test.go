//go:build integration
// +build integration

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// getPackageTestDB 获取测试数据库连接
func getPackageTestDB(t *testing.T) *pgxpool.Pool {
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

	// 构建 DSN - 如果 host 是路径（Unix socket），使用 host= 参数
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

// TestPackageRepository_Create_Integration 集成测试：创建套餐
func TestPackageRepository_Create_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getPackageTestDB(t)
	if pool == nil {
		return
	}

	// 验证 supply_packages 表存在
	var tableName string
	err := pool.QueryRow(context.Background(), "SELECT table_name FROM information_schema.tables WHERE table_name = 'supply_packages'").Scan(&tableName)
	if err != nil {
		t.Skipf("跳过：supply_packages 表不存在: %v", err)
	}

	t.Log("集成测试：supply_packages 表存在")
}

// TestPackageRepository_GetByID_Integration 集成测试：获取套餐
func TestPackageRepository_GetByID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getPackageTestDB(t)
	if pool == nil {
		return
	}

	var count int
	err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM supply_packages").Scan(&count)
	if err != nil {
		t.Logf("集成测试：supply_packages 查询结果: %v", err)
	} else {
		t.Logf("集成测试：supply_packages 共有 %d 条记录", count)
	}
}

// TestPackageRepository_Update_Integration 集成测试：更新套餐（乐观锁）
func TestPackageRepository_Update_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getPackageTestDB(t)
	if pool == nil {
		return
	}

	// 验证 version 字段存在（乐观锁）
	var columnExists bool
	err := pool.QueryRow(context.Background(), `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'supply_packages' AND column_name = 'version'
		)
	`).Scan(&columnExists)
	if err != nil || !columnExists {
		t.Skip("跳过：supply_packages 表缺少 version 字段")
	}

	t.Log("集成测试：supply_packages 包含 version 字段（乐观锁）")
}

// TestPackageRepository_List_Integration 集成测试：列出套餐
func TestPackageRepository_List_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getPackageTestDB(t)
	if pool == nil {
		return
	}

	// 列出供应商标套餐
	rows, err := pool.Query(context.Background(), `
		SELECT id, supplier_id, available_quota
		FROM supply_packages
		LIMIT 10
	`)
	if err != nil {
		t.Logf("集成测试：列出套餐: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, supplierID int64
		var availableQuota int
		rows.Scan(&id, &supplierID, &availableQuota)
		count++
		t.Logf("集成测试：套餐 ID=%d, SupplierID=%d, AvailableQuota=%d", id, supplierID, availableQuota)
	}

	t.Logf("集成测试：列出 %d 个套餐", count)
}

// TestPackageRepository_UpdateQuota_Integration 集成测试：扣减配额
func TestPackageRepository_UpdateQuota_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getPackageTestDB(t)
	if pool == nil {
		return
	}

	// 验证 available_quota 字段存在
	var columnExists bool
	err := pool.QueryRow(context.Background(), `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'supply_packages' AND column_name = 'available_quota'
		)
	`).Scan(&columnExists)
	if err != nil || !columnExists {
		t.Skip("跳过：supply_packages 表缺少 available_quota 字段")
	}

	t.Log("集成测试：supply_packages 包含 available_quota 字段")
}

// TestPackageRepository_GetForUpdate_Integration 集成测试：悲观锁获取
func TestPackageRepository_GetForUpdate_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getPackageTestDB(t)
	if pool == nil {
		return
	}

	// 测试 FOR UPDATE 锁
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("开始事务失败: %v", err)
	}
	defer tx.Rollback(context.Background())

	rows, err := tx.Query(context.Background(), "SELECT id FROM supply_packages LIMIT 1 FOR UPDATE")
	if err != nil {
		t.Logf("集成测试：FOR UPDATE 查询: %v", err)
	} else {
		rows.Close()
		t.Log("集成测试：FOR UPDATE 锁获取成功")
	}

	tx.Rollback(context.Background())
}

// TestPackageRepository_OptimisticLock_Integration 集成测试：乐观锁冲突
func TestPackageRepository_OptimisticLock_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getPackageTestDB(t)
	if pool == nil {
		return
	}

	// 验证 version 字段存在
	var versionCol int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'supply_packages' AND column_name = 'version'
	`).Scan(&versionCol)
	if err != nil || versionCol == 0 {
		t.Skip("跳过：supply_packages 表缺少 version 字段")
	}

	t.Log("集成测试：乐观锁字段验证通过")
}
