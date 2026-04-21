package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ==================== P4-C-07: Scope-UserType 匹配校验测试 ====================

func TestValidateScopeCodeMatch(t *testing.T) {
	tests := []struct {
		name      string
		claims    *IAMTokenClaims
		scopeCode string
		want      bool
	}{
		// platform 用户可使用所有 scope 类型
		{
			name: "platform_user_can_use_supply_scope",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "super_admin",
				UserType:  "platform",
				Scope:     []string{"supply:account:read"},
			},
			scopeCode: "supply:account:read",
			want:      true,
		},
		{
			name: "platform_user_can_use_consumer_scope",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "super_admin",
				UserType:  "platform",
				Scope:     []string{"consumer:account:read"},
			},
			scopeCode: "consumer:account:read",
			want:      true,
		},
		{
			name: "platform_user_can_use_platform_scope",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "super_admin",
				UserType:  "platform",
				Scope:     []string{"platform:read"},
			},
			scopeCode: "platform:read",
			want:      true,
		},

		// supply 用户只能使用 supply 和 platform scope
		{
			name: "supply_user_can_use_supply_scope",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "supply_admin",
				UserType:  "supply",
				Scope:     []string{"supply:account:read"},
			},
			scopeCode: "supply:account:read",
			want:      true,
		},
		{
			name: "supply_user_can_use_platform_scope",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "supply_admin",
				UserType:  "supply",
				Scope:     []string{"platform:read"},
			},
			scopeCode: "platform:read",
			want:      true,
		},
		{
			name: "supply_user_cannot_use_consumer_scope",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "supply_admin",
				UserType:  "supply",
				Scope:     []string{"consumer:account:read"},
			},
			scopeCode: "consumer:account:read",
			want:      false, // P4-C-07 核心闭环：supply 用户不能操作 consumer 资源
		},

		// consumer 用户只能使用 consumer 和 platform scope
		{
			name: "consumer_user_can_use_consumer_scope",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "consumer_admin",
				UserType:  "consumer",
				Scope:     []string{"consumer:account:read"},
			},
			scopeCode: "consumer:account:read",
			want:      true,
		},
		{
			name: "consumer_user_can_use_platform_scope",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "consumer_admin",
				UserType:  "consumer",
				Scope:     []string{"platform:read"},
			},
			scopeCode: "platform:read",
			want:      true,
		},
		{
			name: "consumer_user_cannot_use_supply_scope",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "consumer_admin",
				UserType:  "consumer",
				Scope:     []string{"supply:account:read"},
			},
			scopeCode: "supply:account:read",
			want:      false, // P4-C-07 核心闭环：consumer 用户不能操作 supply 资源
		},

		// 边界情况
		{
			name:      "nil_claims_returns_false",
			claims:    nil,
			scopeCode: "supply:account:read",
			want:      false,
		},
		{
			name: "empty_scope_passes",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "supply_admin",
				UserType:  "supply",
				Scope:     []string{},
			},
			scopeCode: "",
			want:      true,
		},
		{
			name: "wildcard_scope_still_checks_user_type",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "supply_admin",
				UserType:  "supply",
				Scope:     []string{"*", "supply:account:read"},
			},
			scopeCode: "consumer:account:read",
			want:      false, // wildcard跳过scope持有但仍校验UserType，supply用户不能访问consumer scope
		},
		{
			name: "unknown_scope_type_rejected",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "supply_admin",
				UserType:  "supply",
				Scope:     []string{"unknown:read"},
			},
			scopeCode: "unknown:read",
			want:      false, // 未知类型保守拒绝
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateScopeCodeMatch(tt.claims, tt.scopeCode)
			if got != tt.want {
				t.Errorf("ValidateScopeCodeMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRequireScopeWithUserType_Middleware 集成测试 RequireScopeWithUserType 中间件
func TestRequireScopeWithUserType_Middleware(t *testing.T) {
	m := NewScopeAuthMiddleware()
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name           string
		claims         *IAMTokenClaims
		requiredScope  string
		wantStatus     int
		wantNextCalled bool
	}{
		{
			name: "supply_user_access_supply_scope_allowed",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "supply_admin",
				UserType:  "supply",
				Scope:     []string{"supply:account:read"},
			},
			requiredScope:  "supply:account:read",
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		{
			name: "supply_user_access_consumer_scope_rejected",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "supply_admin",
				UserType:  "supply",
				Scope:     []string{"consumer:account:read"},
			},
			requiredScope:  "consumer:account:read",
			wantStatus:     http.StatusForbidden,
			wantNextCalled: false,
		},
		{
			name: "platform_user_access_consumer_scope_allowed",
			claims: &IAMTokenClaims{
				SubjectID: "user1",
				Role:      "super_admin",
				UserType:  "platform",
				Scope:     []string{"consumer:account:read"},
			},
			requiredScope:  "consumer:account:read",
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		{
			name:           "missing_claims_returns_401",
			claims:         nil,
			requiredScope:  "supply:account:read",
			wantStatus:     http.StatusUnauthorized,
			wantNextCalled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled = false
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.claims != nil {
				req = req.WithContext(WithIAMClaims(context.Background(), tt.claims))
			}

			handler := m.RequireScopeWithUserType(tt.requiredScope)(nextHandler)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if nextCalled != tt.wantNextCalled {
				t.Errorf("nextCalled = %v, want %v", nextCalled, tt.wantNextCalled)
			}
		})
	}
}
