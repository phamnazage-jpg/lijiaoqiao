package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"lijiaoqiao/supply-api/internal/iam/model"
	"lijiaoqiao/supply-api/internal/middleware"
	"lijiaoqiao/supply-api/internal/pkg/logging"
)

// IAM token claims context key
type iamContextKey string

const (
	// IAMTokenClaimsKey 用于在context中存储token claims
	IAMTokenClaimsKey iamContextKey = "iam_token_claims"
)

// ClaimsVersion Token Claims版本号，用于迁移追踪
const ClaimsVersion = 1

// IAMTokenClaims IAM扩展Token Claims
// 版本: v1
// 迁移路径: 见 MigrateClaims 函数
type IAMTokenClaims struct {
	SubjectID   string   `json:"subject_id"`
	Role        string   `json:"role"`
	Scope       []string `json:"scope"`
	TenantID    int64    `json:"tenant_id"`
	UserType    string   `json:"user_type"`     // 用户类型: platform/supply/consumer
	Permissions []string `json:"permissions"`    // 细粒度权限列表

	// 版本控制字段（未来迁移用）
	Version int `json:"version,omitempty"`
}

// MigrateClaims 将旧版本Claims迁移到当前版本
// 迁移路径：
//   v0 -> v1: 初始版本，添加 Version 字段
//
// 使用示例：
//   claims := &IAMTokenClaims{}
//   if err := json.Unmarshal(data, claims); err != nil {
//       return err
//   }
//   migrated := MigrateClaims(claims)
//   // 使用 migrated
func MigrateClaims(claims *IAMTokenClaims) *IAMTokenClaims {
	if claims == nil {
		return nil
	}

	// 当前版本是v1，无需迁移
	// 未来版本迁移：
	// case 0:
	//     claims = migrateV0ToV1(claims)
	// case 1:
	//     claims = migrateV1ToV2(claims)
	claims.Version = ClaimsVersion
	return claims
}

// ValidateClaims 验证Claims完整性
func ValidateClaims(claims *IAMTokenClaims) error {
	if claims == nil {
		return ErrInvalidClaims
	}
	if claims.SubjectID == "" {
		return ErrInvalidSubjectID
	}
	return nil
}

// 迁移相关错误
var (
	ErrInvalidClaims   = &ClaimsError{Code: "IAM_CLAIMS_4001", Message: "invalid claims structure"}
	ErrInvalidSubjectID = &ClaimsError{Code: "IAM_CLAIMS_4002", Message: "subject_id is required"}
)

// ClaimsError Claims相关错误
type ClaimsError struct {
	Code    string
	Message string
}

func (e *ClaimsError) Error() string {
	return e.Code + ": " + e.Message
}

// 角色层级定义（已废弃，请使用 model.RoleHierarchyLevels）
// @deprecated 使用 model.RoleHierarchyLevels 获取角色层级
var roleHierarchyLevels = model.RoleHierarchyLevels

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
		roleHierarchy:      model.RoleHierarchyLevels, // 使用统一的角色层级定义
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

	// 空scope应该拒绝访问
	if requiredScope == "" {
		return false
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
// @deprecated 请使用 model.GetRoleLevelByCode
func GetRoleLevel(role string) int {
	return model.GetRoleLevelByCode(role)
}

// GetIAMTokenClaims 获取IAM Token Claims
func GetIAMTokenClaims(ctx context.Context) *IAMTokenClaims {
	if claims, ok := ctx.Value(IAMTokenClaimsKey).(*IAMTokenClaims); ok {
		return claims
	}
	return nil
}

// getIAMTokenClaims 内部获取IAM Token Claims
func getIAMTokenClaims(ctx context.Context) *IAMTokenClaims {
	if claims, ok := ctx.Value(IAMTokenClaimsKey).(*IAMTokenClaims); ok {
		return claims
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

// hasWildcardScope 检查scope列表是否包含通配符scope
func hasWildcardScope(scopes []string) bool {
	for _, scope := range scopes {
		if scope == "*" {
			return true
		}
	}
	return false
}

// logWildcardScopeAccess 记录通配符scope访问的审计日志
// P2-01: 通配符scope是安全风险，应记录审计日志
func logWildcardScopeAccess(ctx context.Context, claims *IAMTokenClaims, requiredScope string) {
	if claims == nil {
		return
	}

	// 检查是否使用了通配符scope
	if hasWildcardScope(claims.Scope) {
		// 记录审计日志
		logger := logging.NewLogger("supply-api", logging.LogLevelWarn)
		logger.Warn("P2-01 WILDCARD_SCOPE_ACCESS", map[string]interface{}{
			"subject_id":    claims.SubjectID,
			"role":          claims.Role,
			"required_scope": requiredScope,
			"tenant_id":     claims.TenantID,
			"user_type":     claims.UserType,
		})
	}
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

			// P2-01: 记录通配符scope访问的审计日志
			if hasWildcardScope(claims.Scope) {
				logWildcardScopeAccess(r.Context(), claims, requiredScope)
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

			// P2-01: 记录通配符scope访问的审计日志
			if hasWildcardScope(claims.Scope) {
				logWildcardScopeAccess(r.Context(), claims, "")
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

			// 空列表应该拒绝访问
			if len(requiredScopes) == 0 || !hasAnyScope(claims.Scope, requiredScopes) {
				writeAuthError(w, http.StatusForbidden, "AUTH_SCOPE_DENIED",
					"none of the required scopes are granted")
				return
			}

			// P2-01: 记录通配符scope访问的审计日志
			if hasWildcardScope(claims.Scope) {
				logWildcardScopeAccess(r.Context(), claims, "")
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
	json.NewEncoder(w).Encode(resp)
}

// WithIAMClaims 设置IAM Claims到Context
func WithIAMClaims(ctx context.Context, claims *IAMTokenClaims) context.Context {
	return context.WithValue(ctx, IAMTokenClaimsKey, claims)
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
