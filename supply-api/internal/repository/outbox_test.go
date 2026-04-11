package repository

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type stubOutboxDB struct {
	querySQL string
	rows     pgx.Rows
}

func (s *stubOutboxDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	s.querySQL = sql
	return s.rows, nil
}

func (s *stubOutboxDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	panic("unexpected QueryRow call")
}

func (s *stubOutboxDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	panic("unexpected Exec call")
}

func (s *stubOutboxDB) Begin(ctx context.Context) (pgx.Tx, error) {
	panic("unexpected Begin call")
}

type stubOutboxRows struct {
	events []*OutboxEvent
	index  int
}

func (r *stubOutboxRows) Close()                                       {}
func (r *stubOutboxRows) Err() error                                   { return nil }
func (r *stubOutboxRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *stubOutboxRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *stubOutboxRows) RawValues() [][]byte                          { return nil }
func (r *stubOutboxRows) Values() ([]any, error)                       { return nil, nil }
func (r *stubOutboxRows) Conn() *pgx.Conn                              { return nil }

func (r *stubOutboxRows) Next() bool {
	if r.index >= len(r.events) {
		return false
	}
	r.index++
	return true
}

func (r *stubOutboxRows) Scan(dest ...any) error {
	event := r.events[r.index-1]
	*(dest[0].(*int64)) = event.ID
	*(dest[1].(*string)) = event.AggregateType
	*(dest[2].(*string)) = event.AggregateID
	*(dest[3].(*string)) = event.EventType
	*(dest[4].(*string)) = event.EventID
	*(dest[5].(*json.RawMessage)) = event.Payload
	*(dest[6].(*OutboxStatus)) = event.Status
	*(dest[7].(*int)) = event.RetryCount
	*(dest[8].(*int)) = event.MaxRetries
	*(dest[9].(*string)) = event.ErrorMessage
	*(dest[10].(*time.Time)) = event.CreatedAt
	*(dest[11].(**time.Time)) = event.ProcessedAt
	*(dest[12].(**time.Time)) = event.NextRetryAt
	*(dest[13].(*int64)) = event.Version
	return nil
}

func TestFetchAndLock_ClaimsEventsAsProcessingInDatabase(t *testing.T) {
	now := time.Now()
	payload := json.RawMessage(`{"event":"created"}`)
	db := &stubOutboxDB{
		rows: &stubOutboxRows{
			events: []*OutboxEvent{
				{
					ID:            1,
					AggregateType: "account",
					AggregateID:   "acc-1",
					EventType:     "created",
					EventID:       "evt-1",
					Payload:       payload,
					Status:        OutboxStatusProcessing,
					RetryCount:    0,
					MaxRetries:    5,
					CreatedAt:     now,
					Version:       2,
				},
			},
		},
	}
	repo := &OutboxRepository{db: db}

	events, err := repo.FetchAndLock(context.Background(), 1)
	if err != nil {
		t.Fatalf("FetchAndLock returned error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Status != OutboxStatusProcessing {
		t.Fatalf("expected event status processing, got %s", events[0].Status)
	}
	if !strings.Contains(db.querySQL, "SET status = 'processing'") {
		t.Fatalf("expected claim query to persist processing status, got SQL: %s", db.querySQL)
	}
}
