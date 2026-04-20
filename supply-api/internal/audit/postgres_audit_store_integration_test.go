//go:build integration
// +build integration

package audit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	auditrepo "lijiaoqiao/supply-api/internal/audit/repository"
)

func getAuditStoreTestDB(t *testing.T) *pgxpool.Pool {
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

func TestPostgresAuditStore_QueryWithTotalAndGetByID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getAuditStoreTestDB(t)
	if pool == nil {
		return
	}

	store := NewPostgresAuditStore(auditrepo.NewPostgresAuditRepository(pool))
	now := time.Now().UTC().Truncate(time.Millisecond)
	if err := store.Emit(context.Background(), Event{
		TenantID:    3001,
		ObjectType:  "supply_settlement",
		ObjectID:    88,
		Action:      "cancel",
		BeforeState: map[string]any{"status": "pending"},
		AfterState:  map[string]any{"status": "cancelled"},
		RequestID:   "req-audit-store-int",
		ResultCode:  "OK",
		SourceIP:    "127.0.0.1",
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("emit audit store event failed: %v", err)
	}

	events, total, err := store.QueryWithTotal(context.Background(), EventFilter{
		TenantID: 3001,
		Action:   "cancel",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("query with total failed: %v", err)
	}
	if total == 0 || len(events) == 0 {
		t.Fatalf("expected queried audit events, total=%d len=%d", total, len(events))
	}

	found, err := store.GetByID(context.Background(), events[0].EventID)
	if err != nil {
		t.Fatalf("get audit event by id failed: %v", err)
	}
	if found.Action != "cancel" {
		t.Fatalf("expected action %q, got %q", "cancel", found.Action)
	}
	if found.BeforeState["status"] != "pending" {
		t.Fatalf("expected before status pending, got %v", found.BeforeState["status"])
	}
	if found.AfterState["status"] != "cancelled" {
		t.Fatalf("expected after status cancelled, got %v", found.AfterState["status"])
	}
}
