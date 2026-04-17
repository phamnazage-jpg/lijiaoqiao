package service

import (
	"context"
	"testing"
	"time"
)

type fakeAuditRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeAuditRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeAuditRows) Scan(dest ...any) error {
	row := r.rows[r.idx-1]
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *time.Time:
			*d = row[i].(time.Time)
		default:
			panic("unsupported audit scan destination")
		}
	}
	return nil
}

func (r *fakeAuditRows) Err() error { return r.err }
func (r *fakeAuditRows) Close()     {}

type fakeAuditDB struct {
	lastExecSQL   string
	lastExecArgs  []any
	lastQuerySQL  string
	lastQueryArgs []any
	rows          auditStoreRows
	execErr       error
	queryErr      error
}

func (db *fakeAuditDB) Exec(_ context.Context, query string, args ...any) error {
	db.lastExecSQL = query
	db.lastExecArgs = append([]any(nil), args...)
	return db.execErr
}

func (db *fakeAuditDB) Query(_ context.Context, query string, args ...any) (auditStoreRows, error) {
	db.lastQuerySQL = query
	db.lastQueryArgs = append([]any(nil), args...)
	if db.queryErr != nil {
		return nil, db.queryErr
	}
	return db.rows, nil
}

func TestPostgresAuditStore_EmitPersistsAuditEvent(t *testing.T) {
	db := &fakeAuditDB{}
	store := newPostgresAuditStoreWithDB(db)

	err := store.Emit(context.Background(), AuditEvent{
		EventID:    "evt-1",
		EventName:  EventTokenIssueSuccess,
		RequestID:  "req-1",
		TokenID:    "tok-1",
		SubjectID:  "2001",
		Route:      "/issue",
		ResultCode: "OK",
		ClientIP:   "127.0.0.1",
		CreatedAt:  time.Date(2026, 4, 17, 9, 10, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("emit audit event: %v", err)
	}
	if db.lastExecSQL == "" {
		t.Fatal("expected emit query to be executed")
	}
	if got := db.lastExecArgs[0].(string); got != "evt-1" {
		t.Fatalf("unexpected event id: got=%s want=evt-1", got)
	}
	if got := db.lastExecArgs[7].(string); got != "127.0.0.1" {
		t.Fatalf("unexpected client ip: got=%s want=127.0.0.1", got)
	}
}

func TestPostgresAuditStore_QueryEventsReturnsAscendingOrder(t *testing.T) {
	older := time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC)
	newer := older.Add(2 * time.Minute)
	db := &fakeAuditDB{
		rows: &fakeAuditRows{
			rows: [][]any{
				{"evt-2", EventTokenRevokeSuccess, "req-2", "tok-1", "2001", "/revoke", "OK", "127.0.0.1", newer},
				{"evt-1", EventTokenIssueSuccess, "req-1", "tok-1", "2001", "/issue", "OK", "127.0.0.1", older},
			},
		},
	}
	store := newPostgresAuditStoreWithDB(db)

	events, err := store.QueryEvents(context.Background(), AuditEventFilter{
		TokenID: "tok-1",
		Limit:   600,
	})
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("unexpected event count: got=%d want=2", len(events))
	}
	if events[0].EventID != "evt-1" {
		t.Fatalf("expected ascending order, got first event %s", events[0].EventID)
	}
	if got := db.lastQueryArgs[len(db.lastQueryArgs)-1].(int); got != 500 {
		t.Fatalf("unexpected limit: got=%d want=500", got)
	}
}
