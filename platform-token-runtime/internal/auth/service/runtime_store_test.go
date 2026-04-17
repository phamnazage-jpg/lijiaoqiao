package service

import (
	"context"
	"testing"
)

func TestInMemoryRuntimeStore_SaveAndLookup(t *testing.T) {
	store := NewInMemoryRuntimeStore()
	record := TokenRecord{
		TokenID:     "tok_123",
		AccessToken: "ptk_123",
		SubjectID:   "2001",
		Role:        "owner",
		Scope:       []string{"supply:*"},
	}

	if err := store.Save(context.Background(), record, "idem-1", "hash-1"); err != nil {
		t.Fatalf("save record: %v", err)
	}

	byID, ok, err := store.GetByTokenID(context.Background(), "tok_123")
	if err != nil {
		t.Fatalf("get by token id: %v", err)
	}
	if !ok {
		t.Fatal("expected record by token id")
	}
	if byID.TokenID != "tok_123" {
		t.Fatalf("unexpected token id: %s", byID.TokenID)
	}

	byToken, ok, err := store.GetByAccessToken(context.Background(), "ptk_123")
	if err != nil {
		t.Fatalf("get by access token: %v", err)
	}
	if !ok {
		t.Fatal("expected record by access token")
	}
	if byToken.SubjectID != "2001" {
		t.Fatalf("unexpected subject id: %s", byToken.SubjectID)
	}

	entry, ok, err := store.LookupIdempotency(context.Background(), "idem-1")
	if err != nil {
		t.Fatalf("lookup idempotency: %v", err)
	}
	if !ok {
		t.Fatal("expected idempotency entry")
	}
	if entry.RequestHash != "hash-1" {
		t.Fatalf("unexpected request hash: %s", entry.RequestHash)
	}

	if store.TokenCount() != 1 {
		t.Fatalf("unexpected token count: %d", store.TokenCount())
	}
}
