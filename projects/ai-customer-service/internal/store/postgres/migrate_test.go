package postgres

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestRunMigrationsRequiresDir(t *testing.T) {
	if err := RunMigrations(&sql.DB{}, filepath.Join("nonexistent")); err == nil {
		t.Fatalf("expected error for missing dir")
	}
}
