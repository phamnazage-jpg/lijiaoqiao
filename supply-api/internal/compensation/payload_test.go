package compensation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseAccountCreatePayload(t *testing.T) {
	payload, err := parseAccountCreatePayload(json.RawMessage(`{"account_id":101,"supplier_id":201}`))
	if err != nil {
		t.Fatalf("expected valid payload, got %v", err)
	}
	if payload.AccountID != 101 {
		t.Fatalf("expected account id 101, got %d", payload.AccountID)
	}
	if payload.SupplierID != 201 {
		t.Fatalf("expected supplier id 201, got %d", payload.SupplierID)
	}
}

func TestParsePackagePublishPayload_RejectsMissingPackageID(t *testing.T) {
	_, err := parsePackagePublishPayload(json.RawMessage(`{"supplier_id":202}`))
	if err == nil {
		t.Fatal("expected missing package id to fail")
	}
	if !strings.Contains(err.Error(), "package_id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseSettlementWithdrawPayload_RejectsInvalidJSON(t *testing.T) {
	_, err := parseSettlementWithdrawPayload(json.RawMessage(`{invalid}`))
	if err == nil {
		t.Fatal("expected invalid json to fail")
	}
	if !strings.Contains(err.Error(), "invalid compensation payload") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseQuotaDeductPayload_RequiresPositiveUsedQuota(t *testing.T) {
	_, err := parseQuotaDeductPayload(json.RawMessage(`{"package_id":104,"supplier_id":204,"used_quota":0}`))
	if err == nil {
		t.Fatal("expected non-positive used quota to fail")
	}
	if !strings.Contains(err.Error(), "used_quota must be greater than 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}
