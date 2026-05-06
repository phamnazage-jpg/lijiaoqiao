package platformdelivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/platformevent"
)

type EventStore interface {
	ListDue(ctx context.Context, platform string, dueBefore time.Time, limit int) ([]platformevent.Event, error)
	MarkDelivered(ctx context.Context, eventID string, deliveredAt time.Time) error
	RecordDeliveryAttempt(ctx context.Context, eventID string, attemptNo int, responseStatus int, responseBody string, errorMessage string) error
	MarkRetry(ctx context.Context, eventID string, attemptCount int, nextAttemptAt time.Time, lastError string) error
	MarkDeadLetter(ctx context.Context, eventID string, attemptCount int, finalError string) error
}

type Worker struct {
	Platform      string
	CallbackURL   string
	Store         EventStore
	Client        *http.Client
	Signer        Signer
	MaxRetries    int
	BatchSize     int
	PollInterval  time.Duration
	RetrySchedule []time.Duration
	Now           func() time.Time
	Logger        *slog.Logger
}

func NewWorker(platform, callbackURL string, store EventStore, client *http.Client, signer Signer, maxRetries int) *Worker {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	if maxRetries <= 0 {
		maxRetries = 5
	}
	return &Worker{
		Platform:      strings.TrimSpace(platform),
		CallbackURL:   strings.TrimSpace(callbackURL),
		Store:         store,
		Client:        client,
		Signer:        signer,
		MaxRetries:    maxRetries,
		BatchSize:     20,
		PollInterval:  5 * time.Second,
		RetrySchedule: []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second, 5 * time.Minute, 15 * time.Minute},
		Now:           time.Now,
	}
}

func (w *Worker) Start(ctx context.Context) {
	if ctx == nil {
		return
	}
	ticker := time.NewTicker(w.pollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := w.RunOnce(ctx); err != nil && w.Logger != nil {
			w.Logger.Error("platform callback delivery run failed", "platform", w.Platform, "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	if w.Store == nil {
		return fmt.Errorf("event store is required")
	}
	if w.Platform == "" {
		return fmt.Errorf("platform is required")
	}
	if w.CallbackURL == "" {
		return fmt.Errorf("callback url is required")
	}
	now := w.now()
	events, err := w.Store.ListDue(ctx, w.Platform, now, w.batchSize())
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := w.deliver(ctx, event, now); err != nil && w.Logger != nil {
			w.Logger.Warn("platform callback event delivery failed", "platform", w.Platform, "event_id", event.ID, "error", err.Error())
		}
	}
	return nil
}

func (w *Worker) deliver(ctx context.Context, event platformevent.Event, now time.Time) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.CallbackURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	headers, err := w.Signer.Headers(body, now)
	if err != nil {
		return err
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	resp, err := w.Client.Do(req)
	if err != nil {
		_ = w.Store.RecordDeliveryAttempt(ctx, event.ID, event.AttemptCount+1, 0, "", err.Error())
		return w.retryOrDeadLetter(ctx, event, fmt.Sprintf("callback request failed: %v", err), now)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(resp.Body)
	_ = w.Store.RecordDeliveryAttempt(ctx, event.ID, event.AttemptCount+1, resp.StatusCode, string(responseBody), "")
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return w.Store.MarkDelivered(ctx, event.ID, now)
	}
	return w.retryOrDeadLetter(ctx, event, fmt.Sprintf("callback returned status %d", resp.StatusCode), now)
}

func (w *Worker) retryOrDeadLetter(ctx context.Context, event platformevent.Event, lastError string, now time.Time) error {
	attemptCount := event.AttemptCount + 1
	if attemptCount >= w.MaxRetries {
		return w.Store.MarkDeadLetter(ctx, event.ID, attemptCount, lastError)
	}
	return w.Store.MarkRetry(ctx, event.ID, attemptCount, now.Add(w.backoffForAttempt(attemptCount)), lastError)
}

func (w *Worker) backoffForAttempt(attempt int) time.Duration {
	if attempt <= 0 || len(w.RetrySchedule) == 0 {
		return 10 * time.Second
	}
	index := attempt - 1
	if index >= len(w.RetrySchedule) {
		return w.RetrySchedule[len(w.RetrySchedule)-1]
	}
	return w.RetrySchedule[index]
}

func (w *Worker) batchSize() int {
	if w.BatchSize <= 0 {
		return 20
	}
	return w.BatchSize
}

func (w *Worker) pollInterval() time.Duration {
	if w.PollInterval <= 0 {
		return 5 * time.Second
	}
	return w.PollInterval
}

func (w *Worker) now() time.Time {
	if w.Now == nil {
		return time.Now()
	}
	return w.Now()
}
