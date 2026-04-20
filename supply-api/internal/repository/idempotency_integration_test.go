//go:build integration
// +build integration

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIdempotencyRepositorySchemaContract(t *testing.T) {
	if testing.Short() {
		t.Skip("integration only")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	requireColumns(t, pool, "supply_idempotency_records", []string{
		"id", "tenant_id", "operator_id", "api_path", "idempotency_key",
		"request_id", "payload_hash", "response_code", "response_body",
		"status", "expires_at", "created_at", "updated_at",
	})
	requireRelationKind(t, pool, "supply_idempotency_records", "r")
	requireUniqueConstraint(t, pool, "supply_idempotency_records", []string{
		"tenant_id", "operator_id", "api_path", "idempotency_key",
	})
}

func TestIdempotencyRepository_AcquireLock_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration only")
	}

	pool := getTestDB(t)
	if pool == nil {
		return
	}

	repo := NewIdempotencyRepository(pool)
	ctx := context.Background()
	apiPath := "/supply/accounts"
	idempotencyKey := "idem-lock-integration-20260420"
	firstPayloadHash := strings.Repeat("a", 64)
	secondPayloadHash := strings.Repeat("b", 64)

	cleanupIdempotencyRecords(t, pool, apiPath, idempotencyKey)

	first, err := repo.AcquireLock(ctx, 1001, 2001, apiPath, idempotencyKey, "req-idem-1", firstPayloadHash, 24*time.Hour)
	if err != nil {
		t.Fatalf("first acquire lock failed: %v", err)
	}
	if first == nil || first.ID == 0 {
		t.Fatalf("expected first lock record with id, got %#v", first)
	}
	if first.RequestID != "req-idem-1" {
		t.Fatalf("unexpected first request id: got=%s want=req-idem-1", first.RequestID)
	}
	if first.PayloadHash != firstPayloadHash {
		t.Fatalf("unexpected first payload hash: got=%s want=%s", first.PayloadHash, firstPayloadHash)
	}
	if first.Status != IdempotencyStatusProcessing {
		t.Fatalf("unexpected first lock status: got=%s want=%s", first.Status, IdempotencyStatusProcessing)
	}

	persisted, err := repo.GetByKey(ctx, 1001, 2001, apiPath, idempotencyKey)
	if err != nil {
		t.Fatalf("get by key after first acquire failed: %v", err)
	}
	if persisted == nil {
		t.Fatal("expected persisted idempotency record after first acquire")
	}
	if persisted.RequestID != "req-idem-1" {
		t.Fatalf("unexpected persisted request id: got=%s want=req-idem-1", persisted.RequestID)
	}
	if persisted.PayloadHash != firstPayloadHash {
		t.Fatalf("unexpected persisted payload hash: got=%s want=%s", persisted.PayloadHash, firstPayloadHash)
	}

	second, err := repo.AcquireLock(ctx, 1001, 2001, apiPath, idempotencyKey, "req-idem-2", secondPayloadHash, 24*time.Hour)
	if err != nil {
		t.Fatalf("second acquire lock failed: %v", err)
	}
	if second == nil {
		t.Fatal("expected second lock record")
	}
	if second.ID != first.ID {
		t.Fatalf("expected second acquire to return existing record: first=%d second=%d", first.ID, second.ID)
	}
	if second.RequestID != "req-idem-1" {
		t.Fatalf("expected active lock replay to preserve original request id: got=%s want=req-idem-1", second.RequestID)
	}
	if second.PayloadHash != firstPayloadHash {
		t.Fatalf("expected active lock replay to preserve original payload hash: got=%s want=%s", second.PayloadHash, firstPayloadHash)
	}
}

func requireRelationKind(t *testing.T, pool *pgxpool.Pool, table, want string) {
	t.Helper()

	var relkind string
	err := pool.QueryRow(context.Background(), `
		SELECT c.relkind::text
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = $1
	`, table).Scan(&relkind)
	if err != nil {
		t.Fatalf("检查表 %s relkind 失败: %v", table, err)
	}
	if relkind != want {
		t.Fatalf("表 %s 类型错误: got=%s want=%s", table, relkind, want)
	}
}

func requireUniqueConstraint(t *testing.T, pool *pgxpool.Pool, table string, columns []string) {
	t.Helper()

	var exists bool
	err := pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint c
			JOIN pg_class tbl ON tbl.oid = c.conrelid
			JOIN pg_namespace n ON n.oid = tbl.relnamespace
			JOIN LATERAL (
				SELECT string_agg(att.attname, ',' ORDER BY u.ordinality) AS cols
				FROM unnest(c.conkey) WITH ORDINALITY AS u(attnum, ordinality)
				JOIN pg_attribute att ON att.attrelid = c.conrelid AND att.attnum = u.attnum
			) cols ON TRUE
			WHERE n.nspname = 'public'
			  AND tbl.relname = $1
			  AND c.contype = 'u'
			  AND cols.cols = $2
		)
	`, table, strings.Join(columns, ",")).Scan(&exists)
	if err != nil {
		t.Fatalf("检查表 %s 唯一约束失败: %v", table, err)
	}
	if !exists {
		t.Fatalf("表 %s 缺少唯一约束: %s", table, strings.Join(columns, ","))
	}
}

func cleanupIdempotencyRecords(t *testing.T, pool *pgxpool.Pool, apiPath, idempotencyKey string) {
	t.Helper()

	_, err := pool.Exec(context.Background(), `
		DELETE FROM supply_idempotency_records
		WHERE api_path = $1 AND idempotency_key = $2
	`, apiPath, idempotencyKey)
	if err != nil {
		t.Fatalf("清理幂等记录失败: %v", err)
	}
}
