package repository

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lijiaoqiao/supply-api/internal/audit/model"
)

func TestNewPostgresAlertRepository_ImplementsAlertStore(t *testing.T) {
	repo := NewPostgresAlertRepository(nil)
	if repo == nil {
		t.Fatal("expected repository")
	}

	var store interface {
		Create(context.Context, *model.Alert) error
		GetByID(context.Context, string) (*model.Alert, error)
		Update(context.Context, *model.Alert) error
		Delete(context.Context, string) error
		List(context.Context, *model.AlertFilter) ([]*model.Alert, int64, error)
	} = repo
	if store == nil {
		t.Fatal("expected alert store implementation")
	}
}

func TestPostgresAlertRepository_RequiresPool(t *testing.T) {
	repo := NewPostgresAlertRepository(nil)
	err := repo.Create(context.Background(), &model.Alert{Title: "pool guard"})
	if err == nil {
		t.Fatal("expected nil pool error")
	}
	if !strings.Contains(err.Error(), "pool") {
		t.Fatalf("expected error to mention pool, got %v", err)
	}
}

func TestAuditAlertsDDL_DefinesAlertTable(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "sql", "postgresql", "audit_alerts_v1.sql"))
	if err != nil {
		t.Fatalf("failed to read ddl: %v", err)
	}

	sql := string(content)
	checks := []string{
		"create table if not exists audit_alerts",
		"alert_id text primary key",
		"metadata jsonb not null default '{}'::jsonb",
		"tags jsonb not null default '[]'::jsonb",
		"notify_channels jsonb not null default '[]'::jsonb",
		"event_ids jsonb not null default '[]'::jsonb",
	}
	for _, check := range checks {
		if !strings.Contains(strings.ToLower(sql), check) {
			t.Fatalf("expected ddl to contain %q", check)
		}
	}
}
