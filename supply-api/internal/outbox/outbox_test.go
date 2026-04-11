package outbox

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/messaging"
	"lijiaoqiao/supply-api/internal/repository"
)

type stubRunnerRepo struct {
	events []*repository.OutboxEvent
}

func (r *stubRunnerRepo) FetchAndLock(ctx context.Context, limit int) ([]*repository.OutboxEvent, error) {
	return r.events, nil
}

func (r *stubRunnerRepo) MarkCompleted(ctx context.Context, eventID string) error {
	return nil
}

func (r *stubRunnerRepo) MarkFailed(ctx context.Context, eventID string, errorMsg string, nextRetryAt *time.Time) error {
	return nil
}

func (r *stubRunnerRepo) MoveToDeadLetter(ctx context.Context, event *repository.OutboxEvent, errorMsg string) error {
	return nil
}

func TestOutboxProcessorRunner_ProcessRejectsNilMessageBroker(t *testing.T) {
	payload := json.RawMessage(`{"event":"created"}`)
	runner := NewOutboxProcessorRunner(&stubRunnerRepo{
		events: []*repository.OutboxEvent{
			{
				ID:            1,
				AggregateType: "account",
				AggregateID:   "acc-1",
				EventType:     "created",
				EventID:       "evt-1",
				Payload:       payload,
				Status:        repository.OutboxStatusProcessing,
				MaxRetries:    5,
			},
		},
	}, nil, &messaging.NoOpOutboxStats{})

	err := runner.process(context.Background())
	if err == nil {
		t.Fatal("expected nil message broker to return error")
	}
	if !strings.Contains(err.Error(), "message broker") {
		t.Fatalf("expected error to mention message broker, got %v", err)
	}
}
