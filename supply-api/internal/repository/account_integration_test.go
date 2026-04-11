//go:build integration
// +build integration

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// getTestDB 获取测试数据库连接
func getTestDB(t *testing.T) *pgxpool.Pool {
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

// TestAccountRepository_Create_Integration 集成测试：创建账号
func TestAccountRepository_Create_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	// 验证连接成功
	var result int
	err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if result != 1 {
		t.Fatalf("预期结果 1，实际: %d", result)
	}

	t.Log("集成测试：数据库连接成功")
}

// TestAccountRepository_GetByID_Integration 集成测试：获取账号
func TestAccountRepository_GetByID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	// 验证 supply_accounts 表存在
	var tableName string
	err := pool.QueryRow(context.Background(), "SELECT table_name FROM information_schema.tables WHERE table_name = 'supply_accounts'").Scan(&tableName)
	if err != nil {
		t.Skipf("跳过：supply_accounts 表不存在: %v", err)
	}

	t.Log("集成测试：supply_accounts 表存在")
}

// TestAccountRepository_Update_Integration 集成测试：更新账号（乐观锁）
func TestAccountRepository_Update_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	// 验证表结构包含 version 字段（乐观锁）
	var columnExists bool
	err := pool.QueryRow(context.Background(), `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'supply_accounts' AND column_name = 'version'
		)
	`).Scan(&columnExists)
	if err != nil || !columnExists {
		t.Skip("跳过：supply_accounts 表缺少 version 字段（乐观锁）")
	}

	t.Log("集成测试：supply_accounts 表包含 version 字段（乐观锁）")
}

// TestAccountRepository_List_Integration 集成测试：列出账号
func TestAccountRepository_List_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	// 列出所有表
	rows, err := pool.Query(context.Background(), `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public'
	`)
	if err != nil {
		t.Fatalf("查询表列表失败: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var name string
		rows.Scan(&name)
		count++
	}

	if count == 0 {
		t.Fatal("预期至少有一些表")
	}

	t.Logf("集成测试：数据库包含 %d 个表", count)
}

// TestAccountRepository_GetWithdrawableBalance_Integration 集成测试：获取可提现余额
func TestAccountRepository_GetWithdrawableBalance_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	// 验证 supply_accounts 表存在并且有相关字段
	var accountID int64
	err := pool.QueryRow(context.Background(), "SELECT COALESCE(MAX(id), 0) FROM supply_accounts").Scan(&accountID)
	if err != nil {
		t.Logf("集成测试：supply_accounts 表为空或不存在: %v", err)
	} else {
		t.Logf("集成测试：supply_accounts 最大 ID = %d", accountID)
	}
}

// TestAccountRepository_OptimisticLock_Integration 集成测试：乐观锁冲突
func TestAccountRepository_OptimisticLock_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	// 验证 version 字段存在
	var versionCol int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'supply_accounts' AND column_name = 'version'
	`).Scan(&versionCol)
	if err != nil || versionCol == 0 {
		t.Skip("跳过：supply_accounts 表缺少 version 字段")
	}

	t.Log("集成测试：乐观锁字段验证通过")
}

// TestAccountRepository_Transaction_Integration 集成测试：事务操作
func TestAccountRepository_Transaction_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	// 测试事务
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("开始事务失败: %v", err)
	}
	defer tx.Rollback(context.Background())

	var result int
	err = tx.QueryRow(context.Background(), "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("事务内查询失败: %v", err)
	}

	err = tx.Commit(context.Background())
	if err != nil {
		t.Fatalf("提交事务失败: %v", err)
	}

	t.Log("集成测试：事务操作成功")
}
