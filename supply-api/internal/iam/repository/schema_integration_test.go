//go:build integration
// +build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func getIAMTestDB(t *testing.T) *pgxpool.Pool {
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
		t.Skipf("跳过 IAM 集成测试：无法连接数据库: %v", err)
		return nil
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("跳过 IAM 集成测试：无法 ping 数据库: %v", err)
		return nil
	}

	t.Cleanup(func() {
		pool.Close()
	})
	return pool
}

func TestIAMSchemaV1_AppliesOnCleanSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("integration only")
	}

	pool := getIAMTestDB(t)
	if pool == nil {
		return
	}

	prereqSQL := mustReadIAMSQLFile(t, filepath.Join("..", "..", "..", "..", "scripts", "devtest", "sql", "supply_iam_prereqs.sql"))
	iamSchemaSQL := mustReadIAMSQLFile(t, filepath.Join("..", "..", "..", "..", "sql", "postgresql", "iam_schema_v1.sql"))

	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection failed: %v", err)
	}
	defer conn.Release()

	schemaName := fmt.Sprintf("iam_schema_contract_%d", time.Now().UnixNano())
	quotedSchema := quoteIAMIdentifier(schemaName)

	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatalf("create schema failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE")
	})

	if _, err := conn.Exec(ctx, "SET search_path TO "+quotedSchema); err != nil {
		t.Fatalf("set search_path failed: %v", err)
	}
	if _, err := conn.Exec(ctx, prereqSQL); err != nil {
		t.Fatalf("apply IAM prereqs failed: %v", err)
	}
	if _, err := conn.Exec(ctx, iamSchemaSQL); err != nil {
		t.Fatalf("apply IAM schema failed: %v", err)
	}

	var allScopeCount int
	if err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM iam_scopes WHERE code = '*'").Scan(&allScopeCount); err != nil {
		t.Fatalf("query all scope failed: %v", err)
	}
	if allScopeCount != 1 {
		t.Fatalf("unexpected all-scope seed count: got=%d want=1", allScopeCount)
	}

	var superAdminWildcardCount int
	if err := conn.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM iam_role_scopes rs
		JOIN iam_roles r ON r.id = rs.role_id
		JOIN iam_scopes s ON s.id = rs.scope_id
		WHERE r.code = 'super_admin' AND s.code = '*'
	`).Scan(&superAdminWildcardCount); err != nil {
		t.Fatalf("query super_admin wildcard scope mapping failed: %v", err)
	}
	if superAdminWildcardCount != 1 {
		t.Fatalf("unexpected super_admin wildcard mapping count: got=%d want=1", superAdminWildcardCount)
	}
}

func mustReadIAMSQLFile(t *testing.T, path string) string {
	t.Helper()

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sql file failed: %s: %v", path, err)
	}
	return string(payload)
}

func quoteIAMIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
