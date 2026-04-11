package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"lijiaoqiao/supply-api/internal/middleware"
)

// TestScopeAuth_CheckScope_SuperAdminHasAllScopes 测试超级管理员拥有所有Scope
func TestScopeAuth_CheckScope_SuperAdminHasAllScopes(t *testing.T) {
	// arrange
	// 创建超级管理员token claims
	claims := &IAMTokenClaims{
		SubjectID: "user:1",
		Role:      "super_admin",
		Scope:     []string{"*"}, // 通配符Scope代表所有权限
		TenantID:  0,
	}

	ctx := WithIAMClaims(context.Background(), claims)

	// act
	hasScope := CheckScope(ctx, "platform:read")
	hasScope2 := CheckScope(ctx, "supply:account:write")
	hasScope3 := CheckScope(ctx, "consumer:apikey:create")

	// assert
	assert.True(t, hasScope, "super_admin should have platform:read")
	assert.True(t, hasScope2, "super_admin should have supply:account:write")
	assert.True(t, hasScope3, "super_admin should have consumer:apikey:create")
}

// TestScopeAuth_CheckScope_ViewerHasReadOnly 测试Viewer只有只读权限
func TestScopeAuth_CheckScope_ViewerHasReadOnly(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:2",
		Role:      "viewer",
		Scope:     []string{"platform:read", "tenant:read", "billing:read"},
		TenantID:  1,
	}

	ctx := WithIAMClaims(context.Background(), claims)

	// act & assert
	assert.True(t, CheckScope(ctx, "platform:read"), "viewer should have platform:read")
	assert.True(t, CheckScope(ctx, "tenant:read"), "viewer should have tenant:read")
	assert.True(t, CheckScope(ctx, "billing:read"), "viewer should have billing:read")

	assert.False(t, CheckScope(ctx, "platform:write"), "viewer should NOT have platform:write")
	assert.False(t, CheckScope(ctx, "tenant:write"), "viewer should NOT have tenant:write")
	assert.False(t, CheckScope(ctx, "supply:account:write"), "viewer should NOT have supply:account:write")
}

// TestScopeAuth_CheckScope_Denied 测试Scope被拒绝
func TestScopeAuth_CheckScope_Denied(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:3",
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}

	ctx := WithIAMClaims(context.Background(), claims)

	// act & assert
	assert.False(t, CheckScope(ctx, "platform:write"), "viewer should NOT have platform:write")
	assert.False(t, CheckScope(ctx, "supply:account:write"), "viewer should NOT have supply:account:write")
}

// TestScopeAuth_CheckScope_MissingTokenClaims 测试缺少Token Claims
func TestScopeAuth_CheckScope_MissingTokenClaims(t *testing.T) {
	// arrange
	ctx := context.Background() // 没有token claims

	// act
	hasScope := CheckScope(ctx, "platform:read")

	// assert
	assert.False(t, hasScope, "should return false when token claims are missing")
}

// TestScopeAuth_CheckScope_EmptyScope 测试空Scope要求
func TestScopeAuth_CheckScope_EmptyScope(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:4",
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}

	ctx := WithIAMClaims(context.Background(), claims)

	// act
	hasEmptyScope := CheckScope(ctx, "")

	// assert - 空scope应该拒绝访问（安全修复）
	assert.False(t, hasEmptyScope, "empty scope should DENY access (security fix)")
}

// TestScopeAuth_CheckMultipleScopes 测试检查多个Scope（需要全部满足）
func TestScopeAuth_CheckMultipleScopes(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:5",
		Role:      "operator",
		Scope:     []string{"platform:read", "platform:write", "tenant:read", "tenant:write"},
		TenantID:  1,
	}

	ctx := WithIAMClaims(context.Background(), claims)

	// act & assert
	assert.True(t, CheckAllScopes(ctx, []string{"platform:read", "platform:write"}), "operator should have both read and write")
	assert.True(t, CheckAllScopes(ctx, []string{"tenant:read", "tenant:write"}), "operator should have both tenant scopes")
	assert.False(t, CheckAllScopes(ctx, []string{"platform:read", "platform:admin"}), "operator should NOT have platform:admin")
}

// TestScopeAuth_CheckAnyScope 测试检查多个Scope（只需满足其一）
func TestScopeAuth_CheckAnyScope(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:6",
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}

	ctx := WithIAMClaims(context.Background(), claims)

	// act & assert
	assert.True(t, CheckAnyScope(ctx, []string{"platform:read", "platform:write"}), "should pass with one matching scope")
	assert.False(t, CheckAnyScope(ctx, []string{"platform:write", "platform:admin"}), "should fail when no scopes match")
	assert.True(t, CheckAnyScope(ctx, []string{}), "empty scope list should pass")
}

// TestScopeAuth_GetIAMTokenClaims 测试从Context获取IAMTokenClaims
func TestScopeAuth_GetIAMTokenClaims(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:7",
		Role:      "org_admin",
		Scope:     []string{"platform:read", "platform:write"},
		TenantID:  1,
	}

	ctx := WithIAMClaims(context.Background(), claims)

	// act
	retrievedClaims := GetIAMTokenClaims(ctx)

	// assert
	assert.NotNil(t, retrievedClaims)
	assert.Equal(t, claims.SubjectID, retrievedClaims.SubjectID)
	assert.Equal(t, claims.Role, retrievedClaims.Role)
	assert.Equal(t, claims.Scope, retrievedClaims.Scope)
}

// TestScopeAuth_GetIAMTokenClaims_Missing 测试获取不存在的IAMTokenClaims
func TestScopeAuth_GetIAMTokenClaims_Missing(t *testing.T) {
	// arrange
	ctx := context.Background()

	// act
	retrievedClaims := GetIAMTokenClaims(ctx)

	// assert
	assert.Nil(t, retrievedClaims)
}

// TestScopeAuth_HasRole 测试用户角色检查
func TestScopeAuth_HasRole(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:8",
		Role:      "operator",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}

	ctx := WithIAMClaims(context.Background(), claims)

	// act & assert
	assert.True(t, HasRole(ctx, "operator"))
	assert.False(t, HasRole(ctx, "viewer"))
	assert.False(t, HasRole(ctx, "admin"))
}

// TestScopeAuth_HasRole_MissingClaims 测试缺少Claims时的角色检查
func TestScopeAuth_HasRole_MissingClaims(t *testing.T) {
	// arrange
	ctx := context.Background()

	// act & assert
	assert.False(t, HasRole(ctx, "operator"))
}

// TestScopeRoleAuthzMiddleware_WithScope 测试带Scope要求的中间件
func TestScopeRoleAuthzMiddleware_WithScope(t *testing.T) {
	// arrange
	scopeAuth := NewScopeAuthMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// 创建一个带scope验证的handler
	wrappedHandler := scopeAuth.RequireScope("platform:write")(handler)

	// 创建一个带有token claims的请求
	claims := &IAMTokenClaims{
		SubjectID: "user:9",
		Role:      "operator",
		Scope:     []string{"platform:read", "platform:write"},
		TenantID:  1,
	}
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(WithIAMClaims(req.Context(), claims))

	// act
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	// assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestScopeRoleAuthzMiddleware_Denied 测试Scope不足时中间件拒绝
func TestScopeRoleAuthzMiddleware_Denied(t *testing.T) {
	// arrange
	scopeAuth := NewScopeAuthMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := scopeAuth.RequireScope("platform:admin")(handler)

	claims := &IAMTokenClaims{
		SubjectID: "user:10",
		Role:      "viewer",
		Scope:     []string{"platform:read"}, // viewer没有platform:admin
		TenantID:  1,
	}
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(WithIAMClaims(req.Context(), claims))

	// act
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	// assert
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestScopeRoleAuthzMiddleware_MissingClaims 测试缺少Claims时中间件拒绝
func TestScopeRoleAuthzMiddleware_MissingClaims(t *testing.T) {
	// arrange
	scopeAuth := NewScopeAuthMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := scopeAuth.RequireScope("platform:read")(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	// 不设置token claims

	// act
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	// assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestScopeRoleAuthzMiddleware_RequireAllScopes 测试要求所有Scope的中间件
func TestScopeRoleAuthzMiddleware_RequireAllScopes(t *testing.T) {
	// arrange
	scopeAuth := NewScopeAuthMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := scopeAuth.RequireAllScopes([]string{"platform:read", "tenant:read"})(handler)

	claims := &IAMTokenClaims{
		SubjectID: "user:11",
		Role:      "operator",
		Scope:     []string{"platform:read", "platform:write", "tenant:read"},
		TenantID:  1,
	}
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(WithIAMClaims(req.Context(), claims))

	// act
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	// assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestScopeRoleAuthzMiddleware_RequireAllScopes_Denied 测试要求所有Scope但不足时拒绝
func TestScopeRoleAuthzMiddleware_RequireAllScopes_Denied(t *testing.T) {
	// arrange
	scopeAuth := NewScopeAuthMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := scopeAuth.RequireAllScopes([]string{"platform:read", "platform:admin"})(handler)

	claims := &IAMTokenClaims{
		SubjectID: "user:12",
		Role:      "viewer",
		Scope:     []string{"platform:read"}, // viewer没有platform:admin
		TenantID:  1,
	}
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(WithIAMClaims(req.Context(), claims))

	// act
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	// assert
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestScopeAuth_HasRoleLevel 测试角色层级检查
func TestScopeAuth_HasRoleLevel(t *testing.T) {
	// arrange
	testCases := []struct {
		role      string
		minLevel  int
		expected  bool
	}{
		{"super_admin", 50, true},
		{"super_admin", 100, true},
		{"org_admin", 50, true},
		{"org_admin", 60, false},
		{"operator", 30, true},
		{"operator", 40, false},
		{"viewer", 10, true},
		{"viewer", 20, false},
	}

	for _, tc := range testCases {
		claims := &IAMTokenClaims{
			SubjectID: "user:test",
			Role:      tc.role,
			Scope:     []string{},
			TenantID:  1,
		}
		ctx := WithIAMClaims(context.Background(), claims)

		// act
		result := HasRoleLevel(ctx, tc.minLevel)

		// assert
		assert.Equal(t, tc.expected, result, "role=%s, minLevel=%d", tc.role, tc.minLevel)
	}
}

// TestGetRoleLevel 测试获取角色层级
func TestGetRoleLevel(t *testing.T) {
	testCases := []struct {
		role     string
		expected int
	}{
		{"super_admin", 100},
		{"org_admin", 50},
		{"supply_admin", 40},
		{"operator", 30},
		{"developer", 20},
		{"viewer", 10},
		{"unknown_role", 0},
	}

	for _, tc := range testCases {
		// act
		level := GetRoleLevel(tc.role)

		// assert
		assert.Equal(t, tc.expected, level, "role=%s", tc.role)
	}
}

// TestScopeAuth_WithIAMClaims 测试设置IAM Claims到Context
func TestScopeAuth_WithIAMClaims(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:13",
		Role:      "org_admin",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}

	// act
	ctx := WithIAMClaims(context.Background(), claims)
	retrievedClaims := GetIAMTokenClaims(ctx)

	// assert
	assert.NotNil(t, retrievedClaims)
	assert.Equal(t, claims.SubjectID, retrievedClaims.SubjectID)
	assert.Equal(t, claims.Role, retrievedClaims.Role)
}

// TestGetClaimsFromLegacy 测试从原有TokenClaims转换
func TestGetClaimsFromLegacy(t *testing.T) {
	// arrange
	legacyClaims := &middleware.TokenClaims{
		SubjectID: "user:14",
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}

	// act
	iamClaims := GetClaimsFromLegacy(legacyClaims)

	// assert
	assert.NotNil(t, iamClaims)
	assert.Equal(t, legacyClaims.SubjectID, iamClaims.SubjectID)
	assert.Equal(t, legacyClaims.Role, iamClaims.Role)
	assert.Equal(t, legacyClaims.Scope, iamClaims.Scope)
	assert.Equal(t, legacyClaims.TenantID, iamClaims.TenantID)
}

// P0-01: 测试WithIAMClaims存储指针，返回有效指针而非悬空指针
// 问题：GetIAMTokenClaims返回指向栈帧的指针，函数返回后指针无效
// 修复：改为存储和获取指针，返回有效堆内存指针
func TestP0_01_WithIAMClaims_ReturnsValidPointer(t *testing.T) {
	// arrange - 创建一个claims并存储到context
	originalClaims := &IAMTokenClaims{
		SubjectID: "user:p0test1",
		Role:      "operator",
		Scope:     []string{"platform:read"},
		TenantID:  100,
	}

	ctx := WithIAMClaims(context.Background(), originalClaims)

	// act - 从context获取claims（获取的应该是有效指针）
	retrievedClaims := GetIAMTokenClaims(ctx)

	// assert - 返回的应该是有效指针，指向与原始claims相同的内存
	assert.NotNil(t, retrievedClaims, "retrieved claims should not be nil")
	assert.Equal(t, originalClaims, retrievedClaims, "should return same pointer as stored")
	assert.Equal(t, "user:p0test1", retrievedClaims.SubjectID, "SubjectID should match")
	assert.Equal(t, "operator", retrievedClaims.Role, "Role should match")

	// 验证修改原始对象后，retrievedClaims能看到变化（因为共享指针）
	originalClaims.Role = "super_admin"
	assert.Equal(t, "super_admin", retrievedClaims.Role, "retrieved claims should see modification")
}

// P0-01: 测试GetIAMTokenClaims在context返回后仍然有效
func TestP0_01_GetIAMTokenClaims_PointerValidAfterReturn(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:ptrtest",
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}

	// act - 存储到context
	ctx := WithIAMClaims(context.Background(), claims)

	// 在函数外获取claims（模拟中间件在请求处理中访问）
	retrievedClaims := GetIAMTokenClaims(ctx)

	// assert - 应该返回有效指针而不是nil或无效指针
	assert.NotNil(t, retrievedClaims)
	assert.Equal(t, claims, retrievedClaims, "should return exact same pointer")
	assert.Equal(t, "user:ptrtest", retrievedClaims.SubjectID)
}

// P0-02: 测试writeAuthError写入响应体
func TestP0_02_writeAuthError_WritesResponseBody(t *testing.T) {
	// arrange
	rec := httptest.NewRecorder()

	// act - 调用writeAuthError
	writeAuthError(rec, http.StatusUnauthorized, "AUTH_CONTEXT_MISSING", "authentication context is missing")

	// assert - 响应体应该包含错误信息
	body := rec.Body.String()
	assert.NotEmpty(t, body, "response body should not be empty")

	// 验证响应体包含错误码和消息
	assert.Contains(t, body, "AUTH_CONTEXT_MISSING", "body should contain error code")
	assert.Contains(t, body, "authentication context is missing", "body should contain error message")
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "status code should match")
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"), "content type should be JSON")
}

// P0-02: 测试writeAuthError在Forbidden状态下也写入响应体
func TestP0_02_writeAuthError_ForbiddenWritesBody(t *testing.T) {
	// arrange
	rec := httptest.NewRecorder()

	// act
	writeAuthError(rec, http.StatusForbidden, "AUTH_SCOPE_DENIED", "required scope is not granted")

	// assert
	body := rec.Body.String()
	assert.NotEmpty(t, body, "response body should not be empty for Forbidden status")
	assert.Contains(t, body, "AUTH_SCOPE_DENIED")
	assert.Contains(t, body, "required scope is not granted")
}

// HIGH-01: CheckScope空scope应该拒绝访问（而不应该绕过权限检查）
func TestHIGH01_CheckScope_EmptyScopeShouldDenyAccess(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:high01",
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}

	ctx := WithIAMClaims(context.Background(), claims)

	// act - 空scope要求应该拒绝访问（安全修复）
	hasEmptyScope := CheckScope(ctx, "")

	// assert - 空scope应该返回false，拒绝访问
	assert.False(t, hasEmptyScope, "empty scope should DENY access (security fix)")
}

// MED-01: RequireAnyScope当requiredScopes为空时应该拒绝访问
func TestMED01_RequireAnyScope_EmptyScopesShouldDenyAccess(t *testing.T) {
	// arrange
	scopeAuth := NewScopeAuthMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 传入空的requiredScopes
	wrappedHandler := scopeAuth.RequireAnyScope([]string{})(handler)

	claims := &IAMTokenClaims{
		SubjectID: "user:med01",
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(WithIAMClaims(req.Context(), claims))

	// act
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	// assert - 空scope列表应该拒绝访问（安全修复）
	assert.Equal(t, http.StatusForbidden, rec.Code, "empty required scopes should DENY access (security fix)")
}

// P2-01: scope=="*"时直接返回true，应记录审计日志
// 由于hasScope是内部函数，我们通过中间件来验证通配符scope的行为
func TestP2_01_WildcardScope_SecurityRisk(t *testing.T) {
	// 创建一个带通配符scope的claims
	claims := &IAMTokenClaims{
		SubjectID: "user:p2-01",
		Role:      "super_admin",
		Scope:     []string{"*"}, // 通配符scope代表所有权限
		TenantID:  1,
	}

	ctx := WithIAMClaims(context.Background(), claims)

	// 通配符scope应该能通过任何scope检查
	assert.True(t, CheckScope(ctx, "platform:read"), "wildcard scope should have platform:read")
	assert.True(t, CheckScope(ctx, "platform:write"), "wildcard scope should have platform:write")
	assert.True(t, CheckScope(ctx, "any:custom:scope"), "wildcard scope should have any:custom:scope")

	// 问题：通配符scope被使用时没有记录审计日志
	// 修复建议：在hasScope返回true时，如果scope是"*"，应该记录审计日志
	// 这是一个安全风险，因为无法追踪何时使用了超级权限

	t.Logf("P2-01: Wildcard scope usage should be audited for security compliance")
}

// TestSetRouteScopePolicy 测试设置路由Scope策略
func TestSetRouteScopePolicy(t *testing.T) {
	// arrange
	m := NewScopeAuthMiddleware()

	// act
	m.SetRouteScopePolicy("/api/v1/admin", []string{"platform:admin"})
	m.SetRouteScopePolicy("/api/v1/user", []string{"platform:read"})

	// assert - 验证路由策略是否正确设置
	_, ok1 := m.routeScopePolicies["/api/v1/admin"]
	_, ok2 := m.routeScopePolicies["/api/v1/user"]
	assert.True(t, ok1, "admin route policy should be set")
	assert.True(t, ok2, "user route policy should be set")
}

// TestRequireRole_HasRole 测试RequireRole中间件 - 有角色
func TestRequireRole_HasRole(t *testing.T) {
	// arrange
	m := NewScopeAuthMiddleware()
	claims := &IAMTokenClaims{
		SubjectID: "user:1",
		Role:      "org_admin",
		Scope:     []string{"platform:admin"},
		TenantID:  1,
	}
	ctx := WithIAMClaims(context.Background(), claims)

	handler := m.RequireRole("org_admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// act
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// assert
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequireRole_NoRole 测试RequireRole中间件 - 无角色
func TestRequireRole_NoRole(t *testing.T) {
	// arrange
	m := NewScopeAuthMiddleware()
	claims := &IAMTokenClaims{
		SubjectID: "user:1",
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}
	ctx := WithIAMClaims(context.Background(), claims)

	handler := m.RequireRole("org_admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// act
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// assert
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestRequireRole_NoClaims 测试RequireRole中间件 - 无Claims
func TestRequireRole_NoClaims(t *testing.T) {
	// arrange
	m := NewScopeAuthMiddleware()
	ctx := context.Background()

	handler := m.RequireRole("org_admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// act
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequireMinLevel_HasLevel 测试RequireMinLevel中间件 - 满足等级
func TestRequireMinLevel_HasLevel(t *testing.T) {
	// arrange
	m := NewScopeAuthMiddleware()
	claims := &IAMTokenClaims{
		SubjectID: "user:1",
		Role:      "org_admin",
		Scope:     []string{"platform:admin"},
		TenantID:  1,
	}
	ctx := WithIAMClaims(context.Background(), claims)

	handler := m.RequireMinLevel(50)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// act
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// assert
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequireMinLevel_InsufficientLevel 测试RequireMinLevel中间件 - 等级不足
func TestRequireMinLevel_InsufficientLevel(t *testing.T) {
	// arrange
	m := NewScopeAuthMiddleware()
	claims := &IAMTokenClaims{
		SubjectID: "user:1",
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}
	ctx := WithIAMClaims(context.Background(), claims)

	handler := m.RequireMinLevel(50)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// act
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// assert
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestHasAnyScope_True 测试hasAnyScope - 有交集
func TestHasAnyScope_True(t *testing.T) {
	// act & assert
	assert.True(t, hasAnyScope([]string{"platform:read", "platform:write"}, []string{"platform:admin", "platform:read"}))
	assert.True(t, hasAnyScope([]string{"*"}, []string{"platform:read"}))
}

// TestHasAnyScope_False 测试hasAnyScope - 无交集
func TestHasAnyScope_False(t *testing.T) {
	// act & assert
	assert.False(t, hasAnyScope([]string{"platform:read"}, []string{"platform:admin", "supply:write"}))
	assert.False(t, hasAnyScope([]string{"tenant:read"}, []string{"platform:admin"}))
}

// TestMigrateClaims_NilInput 测试MigrateClaims处理nil输入
func TestMigrateClaims_NilInput(t *testing.T) {
	// act
	result := MigrateClaims(nil)

	// assert
	assert.Nil(t, result, "MigrateClaims with nil input should return nil")
}

// TestMigrateClaims_ValidClaims 测试MigrateClaims处理有效claims
func TestMigrateClaims_ValidClaims(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:test",
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
		Version:   0,
	}

	// act
	result := MigrateClaims(claims)

	// assert
	assert.NotNil(t, result, "MigrateClaims should return non-nil for valid claims")
	assert.Equal(t, ClaimsVersion, result.Version, "Version should be updated to ClaimsVersion")
	assert.Equal(t, "user:test", result.SubjectID, "SubjectID should be preserved")
}

// TestValidateClaims_NilClaims 测试ValidateClaims处理nil claims
func TestValidateClaims_NilClaims(t *testing.T) {
	// act
	err := ValidateClaims(nil)

	// assert
	assert.Error(t, err, "ValidateClaims should return error for nil claims")
	assert.Equal(t, ErrInvalidClaims, err, "Error should be ErrInvalidClaims")
}

// TestValidateClaims_EmptySubjectID 测试ValidateClaims处理空SubjectID
func TestValidateClaims_EmptySubjectID(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "", // empty
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}

	// act
	err := ValidateClaims(claims)

	// assert
	assert.Error(t, err, "ValidateClaims should return error for empty SubjectID")
	assert.Equal(t, ErrInvalidSubjectID, err, "Error should be ErrInvalidSubjectID")
}

// TestValidateClaims_ValidClaims 测试ValidateClaims处理有效claims
func TestValidateClaims_ValidClaims(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:test",
		Role:      "viewer",
		Scope:     []string{"platform:read"},
		TenantID:  1,
	}

	// act
	err := ValidateClaims(claims)

	// assert
	assert.NoError(t, err, "ValidateClaims should not return error for valid claims")
}

// TestLogWildcardScopeAccess_NilClaims 测试logWildcardScopeAccess处理nil claims
func TestLogWildcardScopeAccess_NilClaims(t *testing.T) {
	// act - should not panic
	logWildcardScopeAccess(context.Background(), nil, "platform:read")
}

// TestLogWildcardScopeAccess_WithWildcard 测试logWildcardScopeAccess记录通配符访问
func TestLogWildcardScopeAccess_WithWildcard(t *testing.T) {
	// arrange
	claims := &IAMTokenClaims{
		SubjectID: "user:admin",
		Role:      "super_admin",
		Scope:     []string{"*"},
		TenantID:  1,
	}

	// act - should not panic and should log
	logWildcardScopeAccess(context.Background(), claims, "platform:read")
}

// TestHasWildcardScope_True 测试hasWildcardScope - 有通配符
func TestHasWildcardScope_True(t *testing.T) {
	// act & assert
	assert.True(t, hasWildcardScope([]string{"*"}), "wildcard scope should return true")
	assert.True(t, hasWildcardScope([]string{"platform:read", "*"}), "mixed with wildcard should return true")
}

// TestHasWildcardScope_False 测试hasWildcardScope - 无通配符
func TestHasWildcardScope_False(t *testing.T) {
	// act & assert
	assert.False(t, hasWildcardScope([]string{"platform:read"}), "regular scope should return false")
	assert.False(t, hasWildcardScope([]string{"platform:read", "platform:write"}), "multiple regular scopes should return false")
	assert.False(t, hasWildcardScope([]string{}), "empty scopes should return false")
}
