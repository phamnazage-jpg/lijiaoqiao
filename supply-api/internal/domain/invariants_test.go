package domain

import (
	"testing"
)

func TestValidateAccountStateTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     AccountStatus
		to       AccountStatus
		expected bool
	}{
		{"pending to active", AccountStatusPending, AccountStatusActive, true},
		{"pending to disabled", AccountStatusPending, AccountStatusDisabled, true},
		{"active to suspended", AccountStatusActive, AccountStatusSuspended, true},
		{"active to disabled", AccountStatusActive, AccountStatusDisabled, true},
		{"suspended to active", AccountStatusSuspended, AccountStatusActive, true},
		{"suspended to disabled", AccountStatusSuspended, AccountStatusDisabled, true},
		{"disabled to active", AccountStatusDisabled, AccountStatusActive, true},
		{"active to pending", AccountStatusActive, AccountStatusPending, false},
		{"suspended to pending", AccountStatusSuspended, AccountStatusPending, false},
		{"disabled to suspended", AccountStatusDisabled, AccountStatusSuspended, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateStateTransition(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("ValidateStateTransition(%s, %s) = %v, want %v", tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestValidatePackageStateTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     PackageStatus
		to       PackageStatus
		expected bool
	}{
		{"draft to active", PackageStatusDraft, PackageStatusActive, true},
		{"active to paused", PackageStatusActive, PackageStatusPaused, true},
		{"active to sold_out", PackageStatusActive, PackageStatusSoldOut, true},
		{"active to expired", PackageStatusActive, PackageStatusExpired, true},
		{"paused to active", PackageStatusPaused, PackageStatusActive, true},
		{"paused to expired", PackageStatusPaused, PackageStatusExpired, true},
		{"draft to paused", PackageStatusDraft, PackageStatusPaused, false},
		{"sold_out to active", PackageStatusSoldOut, PackageStatusActive, false},
		{"expired to active", PackageStatusExpired, PackageStatusActive, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePackageStateTransition(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("ValidatePackageStateTransition(%s, %s) = %v, want %v", tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestInvariantErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{"account cannot delete active", ErrAccountCannotDeleteActive, "cannot delete active"},
		{"account disabled requires admin", ErrAccountDisabledRequiresAdmin, "disabled account requires admin"},
		{"package sold out system only", ErrPackageSoldOutSystemOnly, "sold_out status"},
		{"package expired cannot restore", ErrPackageExpiredCannotRestore, "expired package cannot"},
		{"settlement cannot cancel", ErrSettlementCannotCancel, "cannot cancel"},
		{"withdraw exceeds balance", ErrWithdrawExceedsBalance, "exceeds available balance"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("expected error but got nil")
			}
			if tt.contains != "" && !containsString(tt.err.Error(), tt.contains) {
				t.Errorf("error = %v, want contains %v", tt.err, tt.contains)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
