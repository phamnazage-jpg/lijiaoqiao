//go:build integration
// +build integration

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"lijiaoqiao/supply-api/internal/audit/model"
)

func getAuditRepositoryTestDB(t *testing.T) *pgxpool.Pool {
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

func TestPostgresAuditRepository_EmitQueryAndGetByEventID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（short mode）")
	}

	pool := getAuditRepositoryTestDB(t)
	if pool == nil {
		return
	}

	repo := NewPostgresAuditRepository(pool)
	now := time.Now().UTC().Truncate(time.Millisecond)
	event := &model.AuditEvent{
		EventName:             "AUD-REPO-INTEGRATION",
		EventCategory:         model.CategorySECURITY,
		EventSubCategory:      model.SubCategoryCredIngress,
		Timestamp:             now,
		RequestID:             "req-audit-repo-int",
		TraceID:               "trace-audit-repo-int",
		SpanID:                "span-audit-repo-int",
		IdempotencyKey:        "idem-audit-repo-int",
		OperatorID:            42,
		OperatorType:          model.OperatorTypeUser,
		OperatorRole:          "supplier_admin",
		TenantID:              1001,
		TenantType:            model.TenantTypeSupplier,
		ObjectType:            "supply_account",
		ObjectID:              2002,
		Action:                "activate",
		ActionDetail:          "activate account audit trail",
		CredentialType:        model.CredentialTypePlatformToken,
		CredentialID:          "ptok-001",
		CredentialFingerprint: "fp-001",
		SourceType:            "api",
		SourceIP:              "127.0.0.1",
		SourceRegion:          "cn-hz",
		UserAgent:             "audit-repo-integration",
		TargetType:            "http",
		TargetEndpoint:        "/api/v1/audit/events",
		TargetDirect:          false,
		ResultCode:            "OK",
		ResultMessage:         "emitted",
		Success:               true,
		BeforeState:           map[string]any{"status": "pending"},
		AfterState:            map[string]any{"status": "active"},
		SecurityFlags: model.SecurityFlags{
			HasCredential:     true,
			CredentialExposed: false,
			Desensitized:      true,
			Scanned:           true,
			ScanPassed:        true,
			ViolationTypes:    []string{},
		},
		RiskScore:      12,
		ComplianceTags: []string{"SOC2", "GDPR"},
		InvariantRule:  "AUD-001",
		Extensions:     map[string]any{"source": "integration"},
		Version:        1,
		CreatedAt:      now,
	}

	if err := repo.Emit(context.Background(), event); err != nil {
		t.Fatalf("emit audit event failed: %v", err)
	}
	if event.EventID == "" {
		t.Fatal("expected event id after emit")
	}

	stored, err := repo.GetByEventID(context.Background(), event.EventID)
	if err != nil {
		t.Fatalf("get by event id failed: %v", err)
	}
	if stored.TraceID != event.TraceID {
		t.Fatalf("expected trace id %q, got %q", event.TraceID, stored.TraceID)
	}
	if stored.SpanID != event.SpanID {
		t.Fatalf("expected span id %q, got %q", event.SpanID, stored.SpanID)
	}
	if stored.BeforeState["status"] != event.BeforeState["status"] {
		t.Fatalf("expected before status %v, got %v", event.BeforeState["status"], stored.BeforeState["status"])
	}
	if stored.AfterState["status"] != event.AfterState["status"] {
		t.Fatalf("expected after status %v, got %v", event.AfterState["status"], stored.AfterState["status"])
	}
	if len(stored.ComplianceTags) != len(event.ComplianceTags) {
		t.Fatalf("expected compliance tags %v, got %v", event.ComplianceTags, stored.ComplianceTags)
	}

	events, total, err := repo.Query(context.Background(), &EventFilter{
		TenantID:  1001,
		EventName: event.EventName,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("query audit events failed: %v", err)
	}
	if total == 0 || len(events) == 0 {
		t.Fatalf("expected queried events, total=%d len=%d", total, len(events))
	}
}
