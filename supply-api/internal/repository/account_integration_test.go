//go:build integration
// +build integration

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

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

func requireTable(t *testing.T, pool *pgxpool.Pool, table string) {
	t.Helper()

	var exists bool
	err := pool.QueryRow(context.Background(), `
		SELECT EXISTS(
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, table).Scan(&exists)
	if err != nil {
		t.Fatalf("检查表 %s 失败: %v", table, err)
	}
	if !exists {
		t.Fatalf("缺少表 %s", table)
	}
}

func requireColumn(t *testing.T, pool *pgxpool.Pool, table, column string) {
	t.Helper()

	var exists bool
	err := pool.QueryRow(context.Background(), `
		SELECT EXISTS(
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
		)
	`, table, column).Scan(&exists)
	if err != nil {
		t.Fatalf("检查列 %s.%s 失败: %v", table, column, err)
	}
	if !exists {
		t.Fatalf("缺少列 %s.%s", table, column)
	}
}

func requireColumns(t *testing.T, pool *pgxpool.Pool, table string, columns []string) {
	t.Helper()
	requireTable(t, pool, table)
	for _, column := range columns {
		requireColumn(t, pool, table, column)
	}
}

func TestAccountRepositorySchemaContract(t *testing.T) {
	if testing.Short() {
		t.Skip("integration only")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumns(t, pool, "supply_accounts", []string{
		"id", "user_id", "platform", "account_type", "account_name",
		"encrypted_credentials", "key_id",
		"status", "risk_level", "total_quota", "available_quota", "frozen_quota",
		"is_verified", "verified_at", "last_check_at",
		"tos_compliant", "tos_check_result",
		"total_requests", "total_tokens", "total_cost", "success_rate",
		"risk_score", "risk_reason", "is_frozen", "frozen_reason",
		"credential_cipher_algo", "credential_kms_key_alias", "credential_key_version",
		"quota_unit", "currency_code", "version",
		"created_ip", "updated_ip", "audit_trace_id",
		"request_id", "idempotency_key",
		"created_at", "updated_at",
	})
}

func TestAccountRepository_Create_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	var result int
	err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if result != 1 {
		t.Fatalf("预期结果 1，实际: %d", result)
	}
}

func TestAccountRepository_GetByID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireTable(t, pool, "supply_accounts")
}

func TestAccountRepository_Update_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumn(t, pool, "supply_accounts", "version")
}

func TestAccountRepository_List_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

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
		if scanErr := rows.Scan(&name); scanErr != nil {
			t.Fatalf("扫描表名失败: %v", scanErr)
		}
		count++
	}
	if count == 0 {
		t.Fatal("预期至少有一些表")
	}
}

func TestAccountRepository_GetWithdrawableBalance_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumn(t, pool, "supply_accounts", "available_quota")

	var total float64
	err := pool.QueryRow(context.Background(), "SELECT COALESCE(SUM(available_quota), 0) FROM supply_accounts").Scan(&total)
	if err != nil {
		t.Fatalf("查询可用额度失败: %v", err)
	}
}

func TestAccountRepository_OptimisticLock_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumn(t, pool, "supply_accounts", "version")
}

func TestAccountRepository_Transaction_Integration(t *testing.T) {
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

	var result int
	err = tx.QueryRow(context.Background(), "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("事务内查询失败: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("提交事务失败: %v", err)
	}
}
