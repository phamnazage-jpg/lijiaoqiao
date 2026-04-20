package repository

import "testing"

func TestNewPartitionManager_ExcludesIdempotencyTable(t *testing.T) {
	manager := NewPartitionManager(nil)

	if _, ok := manager.config["supply_idempotency_records"]; ok {
		t.Fatal("expected idempotency table to be excluded from partition manager config")
	}
}
