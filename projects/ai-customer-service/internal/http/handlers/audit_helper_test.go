package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewAuditID_ReturnsValidUUID(t *testing.T) {
	id := newAuditID("audit", time.Now())
	if _, err := uuid.Parse(id); err != nil {
		t.Fatalf("newAuditID() = %q, want valid UUID: %v", id, err)
	}
}
