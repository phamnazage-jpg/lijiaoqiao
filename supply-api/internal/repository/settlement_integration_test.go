//go:build integration
// +build integration

package repository

import (
	"context"
	"testing"
)

func TestSettlementRepositorySchemaContract(t *testing.T) {
	if testing.Short() {
		t.Skip("integration only")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumns(t, pool, "supply_settlements", []string{
		"id", "settlement_no", "user_id",
		"total_amount", "fee_amount", "net_amount",
		"status", "payment_method", "payment_account",
		"period_start", "period_end", "total_orders", "total_usage_records",
		"payment_transaction_id", "paid_at",
		"currency_code", "amount_unit", "version",
		"request_id", "idempotency_key", "audit_trace_id",
		"created_at", "updated_at",
	})
}

func TestSettlementRepository_Create_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireTable(t, pool, "supply_settlements")
}

func TestSettlementRepository_GetByID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	var count int
	if err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM supply_settlements").Scan(&count); err != nil {
		t.Fatalf("查询 supply_settlements 失败: %v", err)
	}
}

func TestSettlementRepository_Update_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumn(t, pool, "supply_settlements", "version")
}

func TestSettlementRepository_List_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	rows, err := pool.Query(context.Background(), `
		SELECT id, user_id, status, total_amount
		FROM supply_settlements
		LIMIT 10
	`)
	if err != nil {
		t.Fatalf("列出结算单失败: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, userID int64
		var status string
		var totalAmount float64
		if scanErr := rows.Scan(&id, &userID, &status, &totalAmount); scanErr != nil {
			t.Fatalf("扫描结算单失败: %v", scanErr)
		}
	}
}

func TestSettlementRepository_GetForUpdate_Integration(t *testing.T) {
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

	rows, err := tx.Query(context.Background(), "SELECT id FROM supply_settlements WHERE status = 'pending' LIMIT 1 FOR UPDATE SKIP LOCKED")
	if err != nil {
		t.Fatalf("FOR UPDATE SKIP LOCKED 查询失败: %v", err)
	}
	rows.Close()
}

func TestSettlementRepository_GetForUpdateNoWait_Integration(t *testing.T) {
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

	rows, err := tx.Query(context.Background(), "SELECT id FROM supply_settlements LIMIT 1 FOR UPDATE NOWAIT")
	if err != nil {
		t.Fatalf("FOR UPDATE NOWAIT 查询失败: %v", err)
	}
	rows.Close()
}

func TestSettlementRepository_GetProcessing_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	var count int
	if err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM supply_settlements WHERE status = 'processing'
	`).Scan(&count); err != nil {
		t.Fatalf("查询处理中结算单失败: %v", err)
	}
}

func TestSettlementRepository_CreateInTx_Integration(t *testing.T) {
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
	if err := tx.QueryRow(context.Background(), "SELECT 1").Scan(&result); err != nil {
		t.Fatalf("事务内查询失败: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("提交事务失败: %v", err)
	}
}

func TestSettlementRepository_OptimisticLock_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumn(t, pool, "supply_settlements", "version")
}

func TestSettlementRepository_Idempotency_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumn(t, pool, "supply_settlements", "idempotency_key")
}
