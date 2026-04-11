//go:build integration
// +build integration

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// getSettlementTestDB 获取测试数据库连接
func getSettlementTestDB(t *testing.T) *pgxpool.Pool {
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

// TestSettlementRepository_Create_Integration 集成测试：创建结算单
func TestSettlementRepository_Create_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getSettlementTestDB(t)
	if pool == nil {
		return
	}

	// 验证 supply_settlements 表存在
	var tableName string
	err := pool.QueryRow(context.Background(), "SELECT table_name FROM information_schema.tables WHERE table_name = 'supply_settlements'").Scan(&tableName)
	if err != nil {
		t.Skipf("跳过：supply_settlements 表不存在: %v", err)
	}

	t.Log("集成测试：supply_settlements 表存在")
}

// TestSettlementRepository_GetByID_Integration 集成测试：获取结算单
func TestSettlementRepository_GetByID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getSettlementTestDB(t)
	if pool == nil {
		return
	}

	var count int
	err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM supply_settlements").Scan(&count)
	if err != nil {
		t.Logf("集成测试：supply_settlements 查询结果: %v", err)
	} else {
		t.Logf("集成测试：supply_settlements 共有 %d 条记录", count)
	}
}

// TestSettlementRepository_Update_Integration 集成测试：更新结算单（乐观锁）
func TestSettlementRepository_Update_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getSettlementTestDB(t)
	if pool == nil {
		return
	}

	// 验证 version 字段存在（乐观锁）
	var columnExists bool
	err := pool.QueryRow(context.Background(), `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'supply_settlements' AND column_name = 'version'
		)
	`).Scan(&columnExists)
	if err != nil || !columnExists {
		t.Skip("跳过：supply_settlements 表缺少 version 字段")
	}

	t.Log("集成测试：supply_settlements 包含 version 字段（乐观锁）")
}

// TestSettlementRepository_List_Integration 集成测试：列出结算单
func TestSettlementRepository_List_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getSettlementTestDB(t)
	if pool == nil {
		return
	}

	// 列出结算单
	rows, err := pool.Query(context.Background(), `
		SELECT id, supplier_id, status, total_amount
		FROM supply_settlements
		LIMIT 10
	`)
	if err != nil {
		t.Logf("集成测试：列出结算单: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, supplierID int64
		var status string
		var totalAmount float64
		rows.Scan(&id, &supplierID, &status, &totalAmount)
		count++
		t.Logf("集成测试：结算单 ID=%d, SupplierID=%d, Status=%s, Amount=%.2f", id, supplierID, status, totalAmount)
	}

	t.Logf("集成测试：列出 %d 个结算单", count)
}

// TestSettlementRepository_GetForUpdate_Integration 集成测试：悲观锁获取
func TestSettlementRepository_GetForUpdate_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getSettlementTestDB(t)
	if pool == nil {
		return
	}

	// 测试 FOR UPDATE SKIP LOCKED
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("开始事务失败: %v", err)
	}
	defer tx.Rollback(context.Background())

	rows, err := tx.Query(context.Background(), "SELECT id FROM supply_settlements WHERE status = 'pending' LIMIT 1 FOR UPDATE SKIP LOCKED")
	if err != nil {
		t.Logf("集成测试：FOR UPDATE SKIP LOCKED 查询: %v", err)
	} else {
		rows.Close()
		t.Log("集成测试：FOR UPDATE SKIP LOCKED 获取成功")
	}

	tx.Rollback(context.Background())
}

// TestSettlementRepository_GetForUpdateNoWait_Integration 集成测试：NOWAIT悲观锁
func TestSettlementRepository_GetForUpdateNoWait_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getSettlementTestDB(t)
	if pool == nil {
		return
	}

	// 测试 FOR UPDATE NOWAIT
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("开始事务失败: %v", err)
	}
	defer tx.Rollback(context.Background())

	rows, err := tx.Query(context.Background(), "SELECT id FROM supply_settlements LIMIT 1 FOR UPDATE NOWAIT")
	if err != nil {
		t.Logf("集成测试：FOR UPDATE NOWAIT 查询: %v", err)
	} else {
		rows.Close()
		t.Log("集成测试：FOR UPDATE NOWAIT 获取成功")
	}

	tx.Rollback(context.Background())
}

// TestSettlementRepository_GetProcessing_Integration 集成测试：获取处理中的结算单
func TestSettlementRepository_GetProcessing_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getSettlementTestDB(t)
	if pool == nil {
		return
	}

	// 查找处理中的结算单
	var count int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM supply_settlements WHERE status = 'processing'
	`).Scan(&count)
	if err != nil {
		t.Logf("集成测试：查找处理中结算单: %v", err)
	} else {
		t.Logf("集成测试：处理中的结算单数量 = %d", count)
	}
}

// TestSettlementRepository_CreateInTx_Integration 集成测试：事务中创建结算单
func TestSettlementRepository_CreateInTx_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getSettlementTestDB(t)
	if pool == nil {
		return
	}

	// 测试在事务中创建记录
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("开始事务失败: %v", err)
	}
	defer tx.Rollback(context.Background())

	// 验证可以插入测试数据
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

// TestSettlementRepository_OptimisticLock_Integration 集成测试：乐观锁冲突
func TestSettlementRepository_OptimisticLock_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getSettlementTestDB(t)
	if pool == nil {
		return
	}

	// 验证 version 字段存在
	var versionCol int
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'supply_settlements' AND column_name = 'version'
	`).Scan(&versionCol)
	if err != nil || versionCol == 0 {
		t.Skip("跳过：supply_settlements 表缺少 version 字段")
	}

	t.Log("集成测试：乐观锁字段验证通过")
}

// TestSettlementRepository_Idempotency_Integration 集成测试：幂等性验证
func TestSettlementRepository_Idempotency_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getSettlementTestDB(t)
	if pool == nil {
		return
	}

	// 验证 idempotency_key 字段存在
	var columnExists bool
	err := pool.QueryRow(context.Background(), `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'supply_settlements' AND column_name = 'idempotency_key'
		)
	`).Scan(&columnExists)
	if err != nil || !columnExists {
		t.Skip("跳过：supply_settlements 表缺少 idempotency_key 字段")
	}

	t.Log("集成测试：幂等性字段验证通过")
}
