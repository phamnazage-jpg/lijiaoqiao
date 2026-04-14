package service

import "testing"

func TestInMemoryRuntimeStore_SaveAndLookup(t *testing.T) {
	store := NewInMemoryRuntimeStore()
	record := TokenRecord{
		TokenID:     "tok_123",
		AccessToken: "ptk_123",
		SubjectID:   "2001",
		Role:        "owner",
		Scope:       []string{"supply:*"},
	}

	store.Save(record, "idem-1", "hash-1")

	byID, ok := store.GetByTokenID("tok_123")
	if !ok {
		t.Fatal("expected record by token id")
	}
	if byID.TokenID != "tok_123" {
		t.Fatalf("unexpected token id: %s", byID.TokenID)
	}

	byToken, ok := store.GetByAccessToken("ptk_123")
	if !ok {
		t.Fatal("expected record by access token")
	}
	if byToken.SubjectID != "2001" {
		t.Fatalf("unexpected subject id: %s", byToken.SubjectID)
	}

	entry, ok := store.LookupIdempotency("idem-1")
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
