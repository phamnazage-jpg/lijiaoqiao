package postgres

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/platformevent"
)

func TestPlatformEventStore_ShouldInsertPendingEvent(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewPlatformEventStore(db)
	now := time.Now().UTC().Truncate(time.Second)
	event := &platformevent.Event{
		ID:              uniqueID("evt"),
		Platform:        "sub2api",
		EventType:       platformevent.TypeMessageReceived,
		CallbackTarget:  "default",
		Payload:         map[string]any{"message": "hello"},
		Status:          platformevent.StatusPending,
		AttemptCount:    0,
		NextAttemptAt:   now,
		OccurredAt:      now,
		CreatedAt:       now,
		UpdatedAt:       now,
		SourceMessageID: uniqueID("msg"),
	}

	if err := store.InsertPending(context.Background(), event); err != nil {
		t.Fatalf("InsertPending() error = %v", err)
	}

	var (
		status       string
		callbackName string
	)
	if err := db.QueryRowContext(context.Background(), `
		SELECT status, callback_target
		FROM cs_platform_event_outbox
		WHERE id = $1
	`, event.ID).Scan(&status, &callbackName); err != nil {
		t.Fatalf("query inserted event failed: %v", err)
	}
	if status != string(platformevent.StatusPending) {
		t.Fatalf("status = %s, want %s", status, platformevent.StatusPending)
	}
	if callbackName != "default" {
		t.Fatalf("callback target = %s, want default", callbackName)
	}
}

func TestPlatformEventStore_ShouldListPendingEventsInOrder(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewPlatformEventStore(db)
	now := time.Now().UTC().Truncate(time.Second)
	firstID := uniqueID("evt")
	secondID := uniqueID("evt")
	platformName := "s2a-" + firstID[:8]

	first := &platformevent.Event{
		ID:             firstID,
		Platform:       platformName,
		EventType:      platformevent.TypeMessageProcessing,
		CallbackTarget: "default",
		Payload:        map[string]any{"step": 1},
		Status:         platformevent.StatusPending,
		NextAttemptAt:  now.Add(-2 * time.Minute),
		OccurredAt:     now.Add(-2 * time.Minute),
		CreatedAt:      now.Add(-2 * time.Minute),
		UpdatedAt:      now.Add(-2 * time.Minute),
	}
	second := &platformevent.Event{
		ID:             secondID,
		Platform:       platformName,
		EventType:      platformevent.TypeReplyGenerated,
		CallbackTarget: "default",
		Payload:        map[string]any{"step": 2},
		Status:         platformevent.StatusPending,
		NextAttemptAt:  now.Add(-1 * time.Minute),
		OccurredAt:     now.Add(-1 * time.Minute),
		CreatedAt:      now.Add(-1 * time.Minute),
		UpdatedAt:      now.Add(-1 * time.Minute),
	}

	if err := store.InsertPending(context.Background(), second); err != nil {
		t.Fatalf("InsertPending(second) error = %v", err)
	}
	if err := store.InsertPending(context.Background(), first); err != nil {
		t.Fatalf("InsertPending(first) error = %v", err)
	}

	events, err := store.ListDue(context.Background(), platformName, now, 10)
	if err != nil {
		t.Fatalf("ListDue() error = %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("due events count = %d, want at least 2", len(events))
	}

	firstPos := -1
	secondPos := -1
	for i, event := range events {
		if event.ID == firstID {
			firstPos = i
		}
		if event.ID == secondID {
			secondPos = i
		}
	}
	if firstPos == -1 || secondPos == -1 {
		t.Fatalf("did not find inserted events in due list: first=%d second=%d", firstPos, secondPos)
	}
	if firstPos >= secondPos {
		t.Fatalf("event order invalid: firstPos=%d secondPos=%d", firstPos, secondPos)
	}
}

func TestPlatformEventStore_ShouldPersistDeliveryAttemptAudit(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewPlatformEventStore(db)
	now := time.Now().UTC().Truncate(time.Second)
	event := &platformevent.Event{
		ID:             uniqueID("evt"),
		Platform:       "s2a-" + uniqueID("plt")[:8],
		EventType:      platformevent.TypeReplyGenerated,
		CallbackTarget: "default",
		Payload:        map[string]any{"reply": "好的"},
		Status:         platformevent.StatusPending,
		NextAttemptAt:  now,
		OccurredAt:     now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := store.InsertPending(context.Background(), event); err != nil {
		t.Fatalf("InsertPending() error = %v", err)
	}
	if err := store.RecordDeliveryAttempt(context.Background(), event.ID, 1, http.StatusBadGateway, `{"error":"upstream"}`, ""); err != nil {
		t.Fatalf("RecordDeliveryAttempt() error = %v", err)
	}

	var (
		attemptNo      int
		responseStatus int
	)
	if err := db.QueryRowContext(context.Background(), `
		SELECT attempt_no, response_status
		FROM cs_platform_event_delivery_attempts
		WHERE event_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, event.ID).Scan(&attemptNo, &responseStatus); err != nil {
		t.Fatalf("query delivery attempt failed: %v", err)
	}
	if attemptNo != 1 {
		t.Fatalf("attempt no = %d, want 1", attemptNo)
	}
	if responseStatus != http.StatusBadGateway {
		t.Fatalf("response status = %d, want %d", responseStatus, http.StatusBadGateway)
	}
}

func TestPlatformEventStore_ShouldMoveToDeadLetter(t *testing.T) {
	db := openDBForTest(t)
	defer db.Close()

	store := NewPlatformEventStore(db)
	now := time.Now().UTC().Truncate(time.Second)
	event := &platformevent.Event{
		ID:             uniqueID("evt"),
		Platform:       "s2a-" + uniqueID("plt")[:8],
		EventType:      platformevent.TypeReplyGenerated,
		CallbackTarget: "default",
		Payload:        map[string]any{"reply": "失败"},
		Status:         platformevent.StatusPending,
		NextAttemptAt:  now,
		OccurredAt:     now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := store.InsertPending(context.Background(), event); err != nil {
		t.Fatalf("InsertPending() error = %v", err)
	}
	if err := store.MarkDeadLetter(context.Background(), event.ID, 5, "callback failed"); err != nil {
		t.Fatalf("MarkDeadLetter() error = %v", err)
	}

	var status string
	if err := db.QueryRowContext(context.Background(), `SELECT status FROM cs_platform_event_outbox WHERE id = $1`, event.ID).Scan(&status); err != nil {
		t.Fatalf("query outbox status failed: %v", err)
	}
	if status != string(platformevent.StatusDeadLetter) {
		t.Fatalf("status = %s, want %s", status, platformevent.StatusDeadLetter)
	}

	var finalError string
	if err := db.QueryRowContext(context.Background(), `SELECT final_error FROM cs_platform_event_dead_letters WHERE event_id = $1`, event.ID).Scan(&finalError); err != nil {
		t.Fatalf("query dead letter failed: %v", err)
	}
	if finalError != "callback failed" {
		t.Fatalf("final error = %s, want callback failed", finalError)
	}
}
