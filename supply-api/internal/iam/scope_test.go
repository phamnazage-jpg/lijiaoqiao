package iam

import (
	"testing"
)

// TestP102_ParseScope 解析scope字符串
func TestP102_ParseScope(t *testing.T) {
	testCases := []struct {
		name      string
		scopeStr  string
		wantErr   bool
		domain    string
		resource  string
		action    string
	}{
		{
			name:     "valid supply scope",
			scopeStr: "supply:accounts:read",
			wantErr:  false,
			domain:   "supply",
			resource: "accounts",
			action:   "read",
		},
		{
			name:     "valid billing scope",
			scopeStr: "billing:settlements:write",
			wantErr:  false,
			domain:   "billing",
			resource: "settlements",
			action:   "write",
		},
		{
			name:     "invalid format",
			scopeStr: "supply:accounts",
			wantErr:  true,
		},
		{
			name:     "invalid domain",
			scopeStr: "unknown:accounts:read",
			wantErr:  true,
		},
		{
			name:     "invalid action",
			scopeStr: "supply:accounts:unknown",
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scope, err := ParseScope(tc.scopeStr)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if scope.Domain != tc.domain {
					t.Errorf("domain: expected %s, got %s", tc.domain, scope.Domain)
				}
				if scope.Resource != tc.resource {
					t.Errorf("resource: expected %s, got %s", tc.resource, scope.Resource)
				}
				if scope.Action != tc.action {
					t.Errorf("action: expected %s, got %s", tc.action, scope.Action)
				}
			}
		})
	}

	t.Log("P1-02: scope解析验证通过")
}

// TestP102_HasScope 检查权限
func TestP102_HasScope(t *testing.T) {
	userScopes := []string{
		"supply:accounts:read",
		"supply:accounts:write",
		"supply:packages:read",
	}

	testCases := []struct {
		name     string
		checkScope *Scope
		expected bool
	}{
		{
			name: "has exact scope",
			checkScope: &Scope{
				Domain:   "supply",
				Resource: "accounts",
				Action:   "read",
			},
			expected: true,
		},
		{
			name: "has exact scope write",
			checkScope: &Scope{
				Domain:   "supply",
				Resource: "accounts",
				Action:   "write",
			},
			expected: true,
		},
		{
			name: "has manage scope",
			checkScope: &Scope{
				Domain:   "supply",
				Resource: "accounts",
				Action:   "manage",
			},
			expected: false, // 用户没有manage但有read/write
		},
		{
			name: "missing scope",
			checkScope: &Scope{
				Domain:   "supply",
				Resource: "orders",
				Action:   "read",
			},
			expected: false,
		},
		{
			name: "admin has all",
			checkScope: &Scope{
				Domain:   "billing",
				Resource: "ledgers",
				Action:   "read",
			},
			expected: false, // 没有admin scope
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := HasScope(userScopes, tc.checkScope)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}

	t.Log("P1-02: HasScope验证通过")
}

// TestP102_HasScopeWithManage 验证manage权限
func TestP102_HasScopeWithManage(t *testing.T) {
	userScopes := []string{
		"supply:accounts:manage", // manage包含所有account操作
		"supply:packages:read",
	}

	// 检查manage是否包含read
	readScope := &Scope{
		Domain:   "supply",
		Resource: "accounts",
		Action:   "read",
	}

	if !HasScope(userScopes, readScope) {
		t.Error("manage should include read")
	}

	// 检查manage是否包含write
	writeScope := &Scope{
		Domain:   "supply",
		Resource: "accounts",
		Action:   "write",
	}

	if !HasScope(userScopes, writeScope) {
		t.Error("manage should include write")
	}

	// 检查manage是否包含delete
	deleteScope := &Scope{
		Domain:   "supply",
		Resource: "accounts",
		Action:   "delete",
	}

	if !HasScope(userScopes, deleteScope) {
		t.Error("manage should include delete")
	}

	t.Log("P1-02: manage权限验证通过")
}

// TestP102_HasAnyScope 任一权限检查
func TestP102_HasAnyScope(t *testing.T) {
	userScopes := []string{
		"supply:accounts:read",
	}

	requiredScopes := []*Scope{
		{Domain: "supply", Resource: "accounts", Action: "read"},
		{Domain: "supply", Resource: "packages", Action: "read"},
		{Domain: "billing", Resource: "ledgers", Action: "read"},
	}

	if !HasAnyScope(userScopes, requiredScopes) {
		t.Error("should return true when user has at least one scope")
	}

	t.Log("P1-02: HasAnyScope验证通过")
}

// TestP102_CommonScopes 常用scope定义
func TestP102_CommonScopes(t *testing.T) {
	// 验证常用scope格式正确
	scopes := []string{
		CommonScopes.SupplyAccountsRead,
		CommonScopes.SupplyAccountsWrite,
		CommonScopes.SupplyAccountsDelete,
		CommonScopes.SupplyAccountsManage,
		CommonScopes.SupplyPackagesRead,
		CommonScopes.BillingAccountsRead,
		CommonScopes.BillingSettlementsWrite,
		CommonScopes.IAMUsersRead,
		CommonScopes.AuditEventsRead,
		CommonScopes.AdminAll,
	}

	for _, scopeStr := range scopes {
		scope, err := ParseScope(scopeStr)
		if err != nil {
			t.Errorf("invalid common scope %s: %v", scopeStr, err)
		}
		if scope == nil {
			t.Errorf("failed to parse common scope %s", scopeStr)
		}
	}

	t.Log("P1-02: 常用scope定义验证通过")
}

// TestP102_RoleScopes 角色默认scope
func TestP102_RoleScopes(t *testing.T) {
	roles := []string{"viewer", "operator", "admin", "owner"}

	for _, role := range roles {
		scopes := GetScopesForRole(role)
		if len(scopes) == 0 {
			t.Errorf("role %s should have scopes", role)
		}

		// 验证每个scope都能正确解析
		for _, scopeStr := range scopes {
			_, err := ParseScope(scopeStr)
			if err != nil {
				t.Errorf("role %s has invalid scope %s: %v", role, scopeStr, err)
			}
		}
	}

	t.Log("P1-02: 角色默认scope验证通过")
}

// TestP102_Summary 测试总结
func TestP102_Summary(t *testing.T) {
	t.Log("=== P1-02 Token Scope授权模型测试总结 ===")
	t.Log("问题: Token runtime定义scope为string[]，但未定义scope命名空间和授权规则")
	t.Log("")
	t.Log("修复方案:")
	t.Log("  - scope格式: {domain}:{resource}:{action}")
	t.Log("  - 域: supply, billing, iam, audit, admin")
	t.Log("  - 动作: read, write, delete, manage, execute")
	t.Log("  - manage权限包含所有其他权限")
	t.Log("  - admin:admin:manage 包含所有权限")
	t.Log("")
	t.Log("示例scope:")
	t.Log("  supply:accounts:read")
	t.Log("  billing:settlements:write")
	t.Log("  iam:users:manage")
}

// TestHasAllScopes_True tests HasAllScopes when user has all required scopes
func TestHasAllScopes_True(t *testing.T) {
	userScopes := []string{
		"supply:accounts:read",
		"supply:accounts:write",
		"supply:accounts:delete",
	}

	requiredScopes := []*Scope{
		{Domain: "supply", Resource: "accounts", Action: "read"},
		{Domain: "supply", Resource: "accounts", Action: "write"},
	}

	result := HasAllScopes(userScopes, requiredScopes)
	if !result {
		t.Error("expected true, user has all required scopes")
	}
}

// TestHasAllScopes_False tests HasAllScopes when user is missing some scopes
func TestHasAllScopes_False(t *testing.T) {
	userScopes := []string{
		"supply:accounts:read",
	}

	requiredScopes := []*Scope{
		{Domain: "supply", Resource: "accounts", Action: "read"},
		{Domain: "supply", Resource: "accounts", Action: "write"},
	}

	result := HasAllScopes(userScopes, requiredScopes)
	if result {
		t.Error("expected false, user is missing write scope")
	}
}

// TestHasAllScopes_EmptyRequired tests HasAllScopes with empty required scopes
func TestHasAllScopes_EmptyRequired(t *testing.T) {
	userScopes := []string{"supply:accounts:read"}

	result := HasAllScopes(userScopes, []*Scope{})
	if !result {
		t.Error("expected true, empty required scopes should return true")
	}
}

// TestHasAllScopes_EmptyUser tests HasAllScopes with empty user scopes
func TestHasAllScopes_EmptyUser(t *testing.T) {
	requiredScopes := []*Scope{
		{Domain: "supply", Resource: "accounts", Action: "read"},
	}

	result := HasAllScopes([]string{}, requiredScopes)
	if result {
		t.Error("expected false, user has no scopes")
	}
}

// TestInvalidScopeError tests the InvalidScopeError type
func TestInvalidScopeError(t *testing.T) {
	err := &InvalidScopeError{
		Scope:  "invalid:scope:format",
		Reason: "invalid format",
	}

	result := err.Error()
	expected := "invalid scope 'invalid:scope:format': invalid format"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

// TestHasAnyScope_False tests HasAnyScope when user has no matching scopes
func TestHasAnyScope_False(t *testing.T) {
	userScopes := []string{
		"supply:accounts:read",
	}

	requiredScopes := []*Scope{
		{Domain: "billing", Resource: "ledgers", Action: "read"},
		{Domain: "iam", Resource: "users", Action: "write"},
	}

	result := HasAnyScope(userScopes, requiredScopes)
	if result {
		t.Error("expected false, user has no matching scopes")
	}
}

// TestHasAnyScope_EmptyRequired tests HasAnyScope with empty required scopes
func TestHasAnyScope_EmptyRequired(t *testing.T) {
	userScopes := []string{"supply:accounts:read"}

	result := HasAnyScope(userScopes, []*Scope{})
	if result {
		t.Error("expected false, empty required scopes should return false")
	}
}

// TestHasAnyScope_EmptyUser tests HasAnyScope with empty user scopes
func TestHasAnyScope_EmptyUser(t *testing.T) {
	requiredScopes := []*Scope{
		{Domain: "supply", Resource: "accounts", Action: "read"},
	}

	result := HasAnyScope([]string{}, requiredScopes)
	if result {
		t.Error("expected false, user has no scopes")
	}
}
