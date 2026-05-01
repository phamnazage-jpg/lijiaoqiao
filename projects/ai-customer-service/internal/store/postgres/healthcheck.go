package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bridge/ai-customer-service/internal/platform/health"
)

type DBChecker struct {
	db *sql.DB
}

func NewDBChecker(db *sql.DB) health.Checker {
	return &DBChecker{db: db}
}

func (c *DBChecker) Name() string {
	return "postgres"
}

func (c *DBChecker) Check(ctx context.Context) error {
	if c == nil || c.db == nil {
		return fmt.Errorf("postgres db is nil")
	}
	return c.db.PingContext(ctx)
}
