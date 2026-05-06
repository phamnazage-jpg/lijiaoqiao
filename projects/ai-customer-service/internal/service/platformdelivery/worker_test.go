package platformdelivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/platformevent"
)

type stubEventStore struct {
	events         []platformevent.Event
	deliveredIDs   []string
	retriedIDs     []string
	deadLetterIDs  []string
	recordedIDs    []string
	recordedStatus []int
	lastRetryAt    time.Time
	lastRetryError string
	lastAttempt    int
}

func (s *stubEventStore) ListDue(_ context.Context, platform string, _ time.Time, _ int) ([]platformevent.Event, error) {
	result := make([]platformevent.Event, 0, len(s.events))
	for _, event := range s.events {
		if event.Platform == platform {
			result = append(result, event)
		}
	}
	return result, nil
}

func (s *stubEventStore) MarkDelivered(_ context.Context, eventID string, _ time.Time) error {
	s.deliveredIDs = append(s.deliveredIDs, eventID)
	return nil
}

func (s *stubEventStore) RecordDeliveryAttempt(_ context.Context, eventID string, attemptNo int, responseStatus int, responseBody string, errorMessage string) error {
	s.recordedIDs = append(s.recordedIDs, eventID)
	s.recordedStatus = append(s.recordedStatus, responseStatus)
	s.lastAttempt = attemptNo
	if errorMessage != "" {
		s.lastRetryError = errorMessage
	}
	_ = responseBody
	return nil
}

func (s *stubEventStore) MarkRetry(_ context.Context, eventID string, attemptCount int, nextAttemptAt time.Time, lastError string) error {
	s.retriedIDs = append(s.retriedIDs, eventID)
	s.lastAttempt = attemptCount
	s.lastRetryAt = nextAttemptAt
	s.lastRetryError = lastError
	return nil
}

func (s *stubEventStore) MarkDeadLetter(_ context.Context, eventID string, attemptCount int, finalError string) error {
	s.deadLetterIDs = append(s.deadLetterIDs, eventID)
	s.lastAttempt = attemptCount
	s.lastRetryError = finalError
	return nil
}

func TestWorker_ShouldDeliverPendingEventToCallbackServer(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store := &stubEventStore{
		events: []platformevent.Event{{
			ID:             "evt-1",
			Platform:       "sub2api",
			EventType:      platformevent.TypeReplyGenerated,
			CallbackTarget: "default",
			Payload:        map[string]any{"reply": "好的"},
			Status:         platformevent.StatusPending,
			NextAttemptAt:  now,
			OccurredAt:     now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(DefaultTimestampHeader) == "" {
			t.Fatal("timestamp header is missing")
		}
		if r.Header.Get(DefaultSignatureHeader) == "" {
			t.Fatal("signature header is missing")
		}
		var event platformevent.Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		if event.ID != "evt-1" {
			t.Fatalf("event id = %s, want evt-1", event.ID)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	worker := NewWorker("sub2api", server.URL, store, server.Client(), Signer{Secret: "callback-secret"}, 5)
	worker.Now = func() time.Time { return now }

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(store.deliveredIDs) != 1 || store.deliveredIDs[0] != "evt-1" {
		t.Fatalf("delivered ids = %v, want [evt-1]", store.deliveredIDs)
	}
}

func TestWorker_ShouldRetryWhenCallbackReturns5xx(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store := &stubEventStore{
		events: []platformevent.Event{{
			ID:             "evt-1",
			Platform:       "sub2api",
			EventType:      platformevent.TypeReplyGenerated,
			CallbackTarget: "default",
			Payload:        map[string]any{"reply": "好的"},
			Status:         platformevent.StatusPending,
			NextAttemptAt:  now,
			OccurredAt:     now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	worker := NewWorker("sub2api", server.URL, store, server.Client(), Signer{Secret: "callback-secret"}, 5)
	worker.Now = func() time.Time { return now }
	worker.RetrySchedule = []time.Duration{15 * time.Second}

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(store.retriedIDs) != 1 || store.retriedIDs[0] != "evt-1" {
		t.Fatalf("retried ids = %v, want [evt-1]", store.retriedIDs)
	}
	if store.lastAttempt != 1 {
		t.Fatalf("attempt count = %d, want 1", store.lastAttempt)
	}
	if !store.lastRetryAt.Equal(now.Add(15 * time.Second)) {
		t.Fatalf("retry at = %s, want %s", store.lastRetryAt, now.Add(15*time.Second))
	}
}

func TestWorker_ShouldMoveEventToDeadLetterAfterMaxRetries(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store := &stubEventStore{
		events: []platformevent.Event{{
			ID:             "evt-1",
			Platform:       "sub2api",
			EventType:      platformevent.TypeReplyGenerated,
			CallbackTarget: "default",
			Payload:        map[string]any{"reply": "失败"},
			Status:         platformevent.StatusRetrying,
			AttemptCount:   1,
			NextAttemptAt:  now,
			OccurredAt:     now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	worker := NewWorker("sub2api", server.URL, store, server.Client(), Signer{Secret: "callback-secret"}, 2)
	worker.Now = func() time.Time { return now }

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(store.deadLetterIDs) != 1 || store.deadLetterIDs[0] != "evt-1" {
		t.Fatalf("dead letter ids = %v, want [evt-1]", store.deadLetterIDs)
	}
}

func TestWorker_ShouldPersistDeliveryAttemptAudit(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store := &stubEventStore{
		events: []platformevent.Event{{
			ID:             "evt-1",
			Platform:       "sub2api",
			EventType:      platformevent.TypeReplyGenerated,
			CallbackTarget: "default",
			Payload:        map[string]any{"reply": "失败"},
			Status:         platformevent.StatusPending,
			NextAttemptAt:  now,
			OccurredAt:     now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream"}`))
	}))
	defer server.Close()

	worker := NewWorker("sub2api", server.URL, store, server.Client(), Signer{Secret: "callback-secret"}, 5)
	worker.Now = func() time.Time { return now }

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(store.recordedIDs) != 1 || store.recordedIDs[0] != "evt-1" {
		t.Fatalf("recorded ids = %v, want [evt-1]", store.recordedIDs)
	}
	if len(store.recordedStatus) != 1 || store.recordedStatus[0] != http.StatusBadGateway {
		t.Fatalf("recorded status = %v, want [%d]", store.recordedStatus, http.StatusBadGateway)
	}
}
