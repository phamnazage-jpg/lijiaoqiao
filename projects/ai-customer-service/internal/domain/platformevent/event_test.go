package platformevent

import (
	"strings"
	"testing"
	"time"
)

func TestEvent_Validate(t *testing.T) {
	now := time.Now()
	event := Event{
		ID:             "evt-1",
		Platform:       "sub2api",
		EventType:      TypeReplyGenerated,
		CallbackTarget: "default",
		Status:         StatusPending,
		AttemptCount:   0,
		NextAttemptAt:  now,
		OccurredAt:     now,
	}

	if err := event.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestEvent_ValidateRejectsInvalidStatus(t *testing.T) {
	event := Event{
		ID:             "evt-1",
		Platform:       "sub2api",
		EventType:      TypeReplyGenerated,
		CallbackTarget: "default",
		Status:         Status("invalid"),
		NextAttemptAt:  time.Now(),
		OccurredAt:     time.Now(),
	}

	err := event.Validate()
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	if !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("unexpected error: %v", err)
	}
}
