package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
)

type AuditRecorder interface {
	Add(ctx context.Context, event audit.Event) error
}

func newAuditID(prefix string, now time.Time) string {
	return fmt.Sprintf("%s-%d", prefix, now.UnixNano())
}
