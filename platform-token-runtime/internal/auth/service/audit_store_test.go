package service

import (
	"context"
	"testing"
	"time"
)

func TestMemoryAuditStore_QueryByTokenID(t *testing.T) {
	store := NewMemoryAuditStore()
	now := time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC)

	if err := store.Emit(context.Background(), AuditEvent{
		EventName:  EventTokenIssueSuccess,
		RequestID:  "req-1",
		TokenID:    "tok-a",
		SubjectID:  "2001",
		Route:      "/issue",
		ResultCode: "OK",
		CreatedAt:  now,
	}); err != nil {
		t.Fatalf("emit first event: %v", err)
	}
	if err := store.Emit(context.Background(), AuditEvent{
		EventName:  EventTokenRevokeSuccess,
		RequestID:  "req-2",
		TokenID:    "tok-b",
		SubjectID:  "2002",
		Route:      "/revoke",
		ResultCode: "OK",
		CreatedAt:  now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("emit second event: %v", err)
	}

	events, err := store.QueryEvents(context.Background(), AuditEventFilter{
		TokenID: "tok-b",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].TokenID != "tok-b" {
		t.Fatalf("unexpected token id: %s", events[0].TokenID)
	}
	if events[0].EventName != EventTokenRevokeSuccess {
		t.Fatalf("unexpected event name: %s", events[0].EventName)
	}
}
