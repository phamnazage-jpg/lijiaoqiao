package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/messaging"
	"lijiaoqiao/supply-api/internal/repository"
)

type stubRunnerRepo struct {
	events           []*repository.OutboxEvent
	failedEventID    string
	failedErrorMsg   string
	failedNextRetryAt *time.Time
}

func (r *stubRunnerRepo) FetchAndLock(ctx context.Context, limit int) ([]*repository.OutboxEvent, error) {
	return r.events, nil
}

func (r *stubRunnerRepo) MarkCompleted(ctx context.Context, eventID string) error {
	return nil
}

func (r *stubRunnerRepo) MarkFailed(ctx context.Context, eventID string, errorMsg string, nextRetryAt *time.Time) error {
	r.failedEventID = eventID
	r.failedErrorMsg = errorMsg
	r.failedNextRetryAt = nextRetryAt
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

type failingBroker struct {
	err error
}

func (b *failingBroker) Publish(ctx context.Context, event *repository.OutboxEvent) error {
	return b.err
}

func TestOutboxProcessorRunner_HandleFailureUsesDomainBackoff(t *testing.T) {
	payload := json.RawMessage(`{"event":"created"}`)
	repo := &stubRunnerRepo{
		events: []*repository.OutboxEvent{
			{
				ID:            1,
				AggregateType: "account",
				AggregateID:   "acc-1",
				EventType:     "created",
				EventID:       "evt-1",
				Payload:       payload,
				Status:        repository.OutboxStatusProcessing,
				RetryCount:    0,
				MaxRetries:    5,
			},
		},
	}

	runner := NewOutboxProcessorRunner(repo, &failingBroker{
		err: errors.New("publish failed"),
	}, &messaging.NoOpOutboxStats{})

	start := time.Now()
	if err := runner.process(context.Background()); err != nil {
		t.Fatalf("expected runner to handle publish failure, got %v", err)
	}
	if repo.failedEventID != "evt-1" {
		t.Fatalf("expected failed event evt-1, got %s", repo.failedEventID)
	}
	if repo.failedNextRetryAt == nil {
		t.Fatal("expected failed retry timestamp to be recorded")
	}
	expectedBackoff := time.Duration(domain.CalculateOutboxBackoff(1, 5)) * time.Second
	actualBackoff := repo.failedNextRetryAt.Sub(start)
	if actualBackoff < expectedBackoff-time.Second || actualBackoff > expectedBackoff+time.Second {
		t.Fatalf("expected retry backoff around %s, got %s", expectedBackoff, actualBackoff)
	}
}
