package service

import "testing"

func TestInMemoryStoresImplementContracts(t *testing.T) {
	t.Helper()

	var runtimeStore RuntimeStore = NewInMemoryRuntimeStore()
	var auditStore AuditStore = NewMemoryAuditStore()

	if runtimeStore == nil {
		t.Fatal("expected runtime store contract implementation")
	}
	if auditStore == nil {
		t.Fatal("expected audit store contract implementation")
	}
}
