package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/repository"
)

func TestInMemoryAccountStoreRespectsSupplierIsolation(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAccountStore()
	account := &domain.Account{
		SupplierID: 1,
		Provider:   domain.ProviderOpenAI,
		Status:     domain.AccountStatusPending,
	}

	if err := store.Create(ctx, account); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if account.ID == 0 {
		t.Fatal("Create() should assign an ID")
	}

	got, err := store.GetByID(ctx, 1, account.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.SupplierID != 1 {
		t.Fatalf("supplier_id = %d, want %d", got.SupplierID, 1)
	}

	if _, err := store.GetByID(ctx, 2, account.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID() wrong supplier error = %v, want %v", err, ErrNotFound)
	}
}

func TestInMemorySettlementStoreUsesOptimisticLockAndPendingGuard(t *testing.T) {
	ctx := context.Background()
	store := NewInMemorySettlementStore()
	first := &domain.Settlement{
		SupplierID: 1,
		Status:     domain.SettlementStatusPending,
		Version:    1,
	}

	if err := store.CreateWithdrawTx(ctx, first); err != nil {
		t.Fatalf("CreateWithdrawTx() error = %v", err)
	}
	if first.ID == 0 {
		t.Fatal("CreateWithdrawTx() should assign an ID")
	}

	hasPending, err := store.HasPendingOrProcessingWithdraw(ctx, 1)
	if err != nil {
		t.Fatalf("HasPendingOrProcessingWithdraw() error = %v", err)
	}
	if !hasPending {
		t.Fatal("expected pending withdrawal to be visible")
	}

	second := &domain.Settlement{
		SupplierID: 1,
		Status:     domain.SettlementStatusPending,
		Version:    1,
	}
	if err := store.CreateWithdrawTx(ctx, second); err == nil {
		t.Fatal("expected duplicate pending withdrawal to be rejected")
	}

	updated := &domain.Settlement{
		ID:         first.ID,
		SupplierID: 1,
		Status:     domain.SettlementStatusCompleted,
		Version:    first.Version,
	}
	if err := store.Update(ctx, updated, 0); !errors.Is(err, repository.ErrConcurrencyConflict) {
		t.Fatalf("Update() wrong version error = %v, want %v", err, repository.ErrConcurrencyConflict)
	}

	before := time.Now()
	if err := store.Update(ctx, updated, 1); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("updated version = %d, want %d", updated.Version, 2)
	}
	if updated.UpdatedAt.Before(before) {
		t.Fatalf("updated_at = %s, want >= %s", updated.UpdatedAt, before)
	}
}

func TestInMemoryEarningStorePaginationBoundaries(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryEarningStore()
	store.records[1] = &domain.EarningRecord{ID: 1, SupplierID: 9}
	store.records[2] = &domain.EarningRecord{ID: 2, SupplierID: 9}
	store.records[3] = &domain.EarningRecord{ID: 3, SupplierID: 9}

	page1, total, err := store.ListRecords(ctx, 9, "", "", 1, 2)
	if err != nil {
		t.Fatalf("ListRecords() error = %v", err)
	}
	if total != 3 || len(page1) != 2 {
		t.Fatalf("page1 = len %d total %d, want len 2 total 3", len(page1), total)
	}

	page2, total, err := store.ListRecords(ctx, 9, "", "", 2, 2)
	if err != nil {
		t.Fatalf("ListRecords() error = %v", err)
	}
	if total != 3 || len(page2) != 1 {
		t.Fatalf("page2 = len %d total %d, want len 1 total 3", len(page2), total)
	}

	page3, total, err := store.ListRecords(ctx, 9, "", "", 3, 2)
	if err != nil {
		t.Fatalf("ListRecords() error = %v", err)
	}
	if total != 3 || len(page3) != 0 {
		t.Fatalf("page3 = len %d total %d, want len 0 total 3", len(page3), total)
	}
}

func TestInMemoryIdempotencyStoreExpiresAndCleansExpiredRecords(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	store.SetProcessing("req-processing", time.Hour)
	store.SetSuccess("req-expired", map[string]string{"status": "ok"}, -time.Second)

	record, ok := store.Get("req-processing")
	if !ok || record.Status != "processing" {
		t.Fatalf("processing record = %#v, want active processing record", record)
	}

	if expired, ok := store.Get("req-expired"); ok || expired != nil {
		t.Fatalf("expired record should not be returned, got %#v", expired)
	}

	store.CleanExpired()
	if store.Len() != 1 {
		t.Fatalf("Len() = %d, want %d", store.Len(), 1)
	}
}
