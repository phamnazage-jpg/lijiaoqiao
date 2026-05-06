package handlers

import (
	"context"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/google/uuid"
)

type AuditRecorder interface {
	Add(ctx context.Context, event audit.Event) error
}

func newAuditID(prefix string, now time.Time) string {
	return uuid.NewString()
}
