package model

import "testing"

func TestPrincipalHasScope(t *testing.T) {
	principal := Principal{
		Role:  RoleOwner,
		Scope: []string{"token:read", "supply:*"},
	}

	tests := []struct {
		name     string
		required string
		want     bool
	}{
		{name: "empty required scope", required: "", want: true},
		{name: "exact scope match", required: "token:read", want: true},
		{name: "wildcard scope match", required: "supply:write", want: true},
		{name: "wildcard keeps separator boundary", required: "supplychain:write", want: false},
		{name: "missing scope", required: "token:write", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := principal.HasScope(tt.required); got != tt.want {
				t.Fatalf("HasScope(%q) = %v, want %v", tt.required, got, tt.want)
			}
		})
	}
}

func TestRoleConstantsRemainStable(t *testing.T) {
	if RoleOwner != "owner" {
		t.Fatalf("RoleOwner = %q, want %q", RoleOwner, "owner")
	}
	if RoleAdmin != "admin" {
		t.Fatalf("RoleAdmin = %q, want %q", RoleAdmin, "admin")
	}
	if RoleViewer != "viewer" {
		t.Fatalf("RoleViewer = %q, want %q", RoleViewer, "viewer")
	}
}
