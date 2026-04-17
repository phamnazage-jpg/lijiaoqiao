package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type fakeRuntimeRow struct {
	values []any
	err    error
}

func (r fakeRuntimeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = r.values[i].(string)
		case *[]byte:
			*d = append((*d)[:0], r.values[i].([]byte)...)
		case *time.Time:
			*d = r.values[i].(time.Time)
		default:
			tpanic("unsupported scan destination")
		}
	}
	return nil
}

type fakeRuntimeDB struct {
	lastExecSQL  string
	lastExecArgs []any
	lastQuerySQL string
	lastQueryArgs []any
	queryRowFunc func(query string, args ...any) runtimeStoreRow
	execErr      error
}

func (db *fakeRuntimeDB) Exec(_ context.Context, query string, args ...any) error {
	db.lastExecSQL = query
	db.lastExecArgs = append([]any(nil), args...)
	return db.execErr
}

func (db *fakeRuntimeDB) QueryRow(_ context.Context, query string, args ...any) runtimeStoreRow {
	db.lastQuerySQL = query
	db.lastQueryArgs = append([]any(nil), args...)
	if db.queryRowFunc == nil {
		return fakeRuntimeRow{err: pgx.ErrNoRows}
	}
	return db.queryRowFunc(query, args...)
}

func TestPostgresRuntimeStore_SavePersistsTokenRecord(t *testing.T) {
	now := time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC)
	db := &fakeRuntimeDB{}
	store := newPostgresRuntimeStoreWithDB(db)

	err := store.Save(context.Background(), TokenRecord{
		TokenID:       "tok_123",
		AccessToken:   "ptk_secret",
		SubjectID:     "2001",
		Role:          "owner",
		Scope:         []string{"supply:*"},
		IssuedAt:      now,
		ExpiresAt:     now.Add(15 * time.Minute),
		Status:        TokenStatusActive,
		RequestID:     "req-1",
		RevokedReason: "",
	}, "idem-1", "hash-1")
	if err != nil {
		t.Fatalf("save record: %v", err)
	}
	if db.lastExecSQL == "" {
		t.Fatal("expected save query to be executed")
	}
	if got := db.lastExecArgs[1].(string); got == "" {
		t.Fatal("expected token fingerprint to be persisted")
	}
	if got := db.lastExecArgs[10].(string); got != "req-1" {
		t.Fatalf("unexpected request id: got=%s want=req-1", got)
	}
	if got := db.lastExecArgs[11].(string); got != "idem-1" {
		t.Fatalf("unexpected idempotency key: got=%s want=idem-1", got)
	}
	if got := db.lastExecArgs[12].(string); got != "hash-1" {
		t.Fatalf("unexpected request hash: got=%s want=hash-1", got)
	}
}

func TestPostgresRuntimeStore_LookupsUseFingerprintAndIdempotencyHash(t *testing.T) {
	now := time.Date(2026, 4, 17, 9, 5, 0, 0, time.UTC)
	db := &fakeRuntimeDB{
		queryRowFunc: func(query string, args ...any) runtimeStoreRow {
			switch {
			case strings.Contains(query, "token_fingerprint"):
				return fakeRuntimeRow{values: []any{
					"tok_123",
					"2001",
					"owner",
					[]byte(`["supply:*"]`),
					string(TokenStatusActive),
					now,
					now.Add(10 * time.Minute),
					"req-1",
					"",
				}}
			case strings.Contains(query, "issue_idempotency_key"):
				return fakeRuntimeRow{values: []any{"hash-1", "tok_123"}}
			default:
				return fakeRuntimeRow{err: pgx.ErrNoRows}
			}
		},
	}
	store := newPostgresRuntimeStoreWithDB(db)

	record, ok, err := store.GetByAccessToken(context.Background(), "ptk_secret")
	if err != nil {
		t.Fatalf("get by access token: %v", err)
	}
	if !ok {
		t.Fatal("expected access token lookup to succeed")
	}
	if record.TokenID != "tok_123" {
		t.Fatalf("unexpected token id: got=%s want=tok_123", record.TokenID)
	}
	entry, ok, err := store.LookupIdempotency(context.Background(), "idem-1")
	if err != nil {
		t.Fatalf("lookup idempotency: %v", err)
	}
	if !ok {
		t.Fatal("expected idempotency lookup to succeed")
	}
	if entry.RequestHash != "hash-1" {
		t.Fatalf("unexpected request hash: got=%s want=hash-1", entry.RequestHash)
	}
}

func tpanic(msg string) {
	panic(msg)
}
