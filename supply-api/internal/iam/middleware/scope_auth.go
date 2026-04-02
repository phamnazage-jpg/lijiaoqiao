package middleware

import (
	"context"
	"net/http"

	"lijiaoqiao/supply-api/internal/middleware"
)

// IAM token claims context key
type iamContextKey string

const (
	// IAMTokenClaimsKey 用于在context中存储token claims
	IAMTokenClaimsKey iamContextKey = "iam_token_claims"
)

// IAMTokenClaims IAM扩展Token Claims
type IAMTokenClaims struct {
	SubjectID   string   `json:"subject_id"`
	Role        string   `json:"role"`
	Scope       []string `json:"scope"`
	TenantID    int64    `json:"tenant_id"`
	UserType    string   `json:"user_type"`     // 用户类型: platform/supply/consumer
	Permissions []string `json:"permissions"`    // 细粒度权限列表
}

// ScopeAuthMiddleware Scope权限验证中间件
type ScopeAuthMiddleware struct {
	// 路由-Scope映射
	routeScopePolicies map[string][]string
	// 角色层级
	roleHierarchy map[string]int
}

// NewScopeAuthMiddleware 创建Scope权限验证中间件
func NewScopeAuthMiddleware() *ScopeAuthMiddleware {
	return &ScopeAuthMiddleware{
		routeScopePolicies: make(map[string][]string),
		roleHierarchy: map[string]int{
			"super_admin":     100,
			"org_admin":        50,
			"supply_admin":     40,
			"consumer_admin":   40,
			"operator":         30,
			"developer":         20,
			"finops":           20,
			"supply_operator":  30,
			"supply_finops":    20,
			"supply_viewer":    10,
			"consumer_operator": 30,
			"consumer_viewer":  10,
			"viewer":           10,
		},
	}
}

// SetRouteScopePolicy 设置路由的Scope要求
func (m *ScopeAuthMiddleware) SetRouteScopePolicy(route string, scopes []string) {
	m.routeScopePolicies[route] = scopes
}

// CheckScope 检查是否拥有指定Scope
func CheckScope(ctx context.Context, requiredScope string) bool {
	claims := getIAMTokenClaims(ctx)
	if claims == nil {
		return false
	}

	// 空scope直接通过
	if requiredScope == "" {
		return true
	}

	return hasScope(claims.Scope, requiredScope)
}

// CheckAllScopes 检查是否拥有所有指定Scope
func CheckAllScopes(ctx context.Context, requiredScopes []string) bool {
	claims := getIAMTokenClaims(ctx)
	if claims == nil {
		return false
	}

	// 空列表直接通过
	if len(requiredScopes) == 0 {
		return true
	}

	for _, scope := range requiredScopes {
		if !hasScope(claims.Scope, scope) {
			return false
		}
	}
	return true
}

// CheckAnyScope 检查是否拥有任一指定Scope
func CheckAnyScope(ctx context.Context, requiredScopes []string) bool {
	claims := getIAMTokenClaims(ctx)
	if claims == nil {
		return false
	}

	// 空列表直接通过
	if len(requiredScopes) == 0 {
		return true
	}

	for _, scope := range requiredScopes {
		if hasScope(claims.Scope, scope) {
			return true
		}
	}
	return false
}

// HasRole 检查是否拥有指定角色
func HasRole(ctx context.Context, requiredRole string) bool {
	claims := getIAMTokenClaims(ctx)
	if claims == nil {
		return false
	}

	return claims.Role == requiredRole
}

// HasRoleLevel 检查角色层级是否满足要求
func HasRoleLevel(ctx context.Context, minLevel int) bool {
	claims := getIAMTokenClaims(ctx)
	if claims == nil {
		return false
	}

	level := GetRoleLevel(claims.Role)
	return level >= minLevel
}

// GetRoleLevel 获取角色层级数值
func GetRoleLevel(role string) int {
	hierarchy := map[string]int{
		"super_admin":     100,
		"org_admin":        50,
		"supply_admin":     40,
		"consumer_admin":   40,
		"operator":         30,
		"developer":        20,
		"finops":           20,
		"supply_operator":  30,
		"supply_finops":    20,
		"supply_viewer":    10,
		"consumer_operator": 30,
		"consumer_viewer":  10,
		"viewer":           10,
	}

	if level, ok := hierarchy[role]; ok {
		return level
	}
	return 0
}

// GetIAMTokenClaims 获取IAM Token Claims
func GetIAMTokenClaims(ctx context.Context) *IAMTokenClaims {
	if claims, ok := ctx.Value(IAMTokenClaimsKey).(IAMTokenClaims); ok {
		return &claims
	}
	return nil
}

// getIAMTokenClaims 内部获取IAM Token Claims
func getIAMTokenClaims(ctx context.Context) *IAMTokenClaims {
	if claims, ok := ctx.Value(IAMTokenClaimsKey).(IAMTokenClaims); ok {
		return &claims
	}
	return nil
}

// hasScope 检查scope列表是否包含目标scope
func hasScope(scopes []string, target string) bool {
	for _, scope := range scopes {
		if scope == target || scope == "*" {
			return true
		}
	}
	return false
}

// RequireScope 返回一个要求特定Scope的中间件
func (m *ScopeAuthMiddleware) RequireScope(requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := getIAMTokenClaims(r.Context())

			if claims == nil {
				writeAuthError(w, http.StatusUnauthorized, "AUTH_CONTEXT_MISSING",
					"authentication context is missing")
				return
			}

			// 检查scope
			if requiredScope != "" && !hasScope(claims.Scope, requiredScope) {
				writeAuthError(w, http.StatusForbidden, "AUTH_SCOPE_DENIED",
					"required scope is not granted")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAllScopes 返回一个要求所有指定Scope的中间件
func (m *ScopeAuthMiddleware) RequireAllScopes(requiredScopes []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := getIAMTokenClaims(r.Context())

			if claims == nil {
				writeAuthError(w, http.StatusUnauthorized, "AUTH_CONTEXT_MISSING",
					"authentication context is missing")
				return
			}

			for _, scope := range requiredScopes {
				if !hasScope(claims.Scope, scope) {
					writeAuthError(w, http.StatusForbidden, "AUTH_SCOPE_DENIED",
						"required scope is not granted")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyScope 返回一个要求任一指定Scope的中间件
func (m *ScopeAuthMiddleware) RequireAnyScope(requiredScopes []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := getIAMTokenClaims(r.Context())

			if claims == nil {
				writeAuthError(w, http.StatusUnauthorized, "AUTH_CONTEXT_MISSING",
					"authentication context is missing")
				return
			}

			// 空列表直接通过
			if len(requiredScopes) > 0 && !hasAnyScope(claims.Scope, requiredScopes) {
				writeAuthError(w, http.StatusForbidden, "AUTH_SCOPE_DENIED",
					"none of the required scopes are granted")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole 返回一个要求特定角色的中间件
func (m *ScopeAuthMiddleware) RequireRole(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := getIAMTokenClaims(r.Context())

			if claims == nil {
				writeAuthError(w, http.StatusUnauthorized, "AUTH_CONTEXT_MISSING",
					"authentication context is missing")
				return
			}

			if claims.Role != requiredRole {
				writeAuthError(w, http.StatusForbidden, "AUTH_ROLE_DENIED",
					"required role is not granted")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireMinLevel 返回一个要求最小角色层级的中间件
func (m *ScopeAuthMiddleware) RequireMinLevel(minLevel int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := getIAMTokenClaims(r.Context())

			if claims == nil {
				writeAuthError(w, http.StatusUnauthorized, "AUTH_CONTEXT_MISSING",
					"authentication context is missing")
				return
			}

			level := GetRoleLevel(claims.Role)
			if level < minLevel {
				writeAuthError(w, http.StatusForbidden, "AUTH_ROLE_LEVEL_DENIED",
					"insufficient role level")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// hasAnyScope 检查scope列表是否包含任一目标scope
func hasAnyScope(scopes, targets []string) bool {
	for _, scope := range scopes {
		for _, target := range targets {
			if scope == target || scope == "*" {
				return true
			}
		}
	}
	return false
}

// writeAuthError 写入鉴权错误
func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	}
	_ = resp
}

// WithIAMClaims 设置IAM Claims到Context
func WithIAMClaims(ctx context.Context, claims *IAMTokenClaims) context.Context {
	return context.WithValue(ctx, IAMTokenClaimsKey, *claims)
}

// GetClaimsFromLegacy 从原有middleware.TokenClaims转换为IAMTokenClaims
func GetClaimsFromLegacy(legacy *middleware.TokenClaims) *IAMTokenClaims {
	if legacy == nil {
		return nil
	}
	return &IAMTokenClaims{
		SubjectID: legacy.SubjectID,
		Role:      legacy.Role,
		Scope:     legacy.Scope,
		TenantID:  legacy.TenantID,
	}
}
