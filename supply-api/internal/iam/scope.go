package iam

import (
	"strings"
)

// ==================== P1-02 Token Scope授权模型 ====================

// Scope命名空间定义
// 格式: {domain}:{resource}:{action}
// 示例: supply:accounts:read, supply:packages:write

const (
	// Domain 域
	ScopeDomainSupply   = "supply"   // 供应域
	ScopeDomainBilling  = "billing"  // 结算域
	ScopeDomainIAM     = "iam"      // 身份域
	ScopeDomainAudit   = "audit"    // 审计域
	ScopeDomainAdmin   = "admin"    // 管理域

	// Action 动作
	ScopeActionRead    = "read"    // 读取
	ScopeActionWrite   = "write"   // 写入
	ScopeActionDelete  = "delete"  // 删除
	ScopeActionManage  = "manage"  // 管理（read+write+delete）
	ScopeActionExecute = "execute" // 执行
)

// Scope定义
type Scope struct {
	Domain   string // 域: supply, billing, iam, audit, admin
	Resource string // 资源: accounts, packages, orders, etc.
	Action   string // 动作: read, write, delete, manage, execute
}

// ParseScope 解析scope字符串
func ParseScope(scopeStr string) (*Scope, error) {
	parts := strings.Split(scopeStr, ":")
	if len(parts) != 3 {
		return nil, &InvalidScopeError{Scope: scopeStr, Reason: "must be in format domain:resource:action"}
	}

	domain := parts[0]
	resource := parts[1]
	action := parts[2]

	// 验证域
	if !isValidDomain(domain) {
		return nil, &InvalidScopeError{Scope: scopeStr, Reason: "invalid domain: " + domain}
	}

	// 验证动作
	if !isValidAction(action) {
		return nil, &InvalidScopeError{Scope: scopeStr, Reason: "invalid action: " + action}
	}

	return &Scope{
		Domain:   domain,
		Resource: resource,
		Action:   action,
	}, nil
}

// isValidDomain 验证域
func isValidDomain(domain string) bool {
	validDomains := []string{
		ScopeDomainSupply,
		ScopeDomainBilling,
		ScopeDomainIAM,
		ScopeDomainAudit,
		ScopeDomainAdmin,
	}
	for _, d := range validDomains {
		if d == domain {
			return true
		}
	}
	return false
}

// isValidAction 验证动作
func isValidAction(action string) bool {
	validActions := []string{
		ScopeActionRead,
		ScopeActionWrite,
		ScopeActionDelete,
		ScopeActionManage,
		ScopeActionExecute,
	}
	for _, a := range validActions {
		if a == action {
			return true
		}
	}
	return false
}

// HasScope 检查是否包含指定scope
func HasScope(userScopes []string, requiredScope *Scope) bool {
	for _, userScopeStr := range userScopes {
		userScope, err := ParseScope(userScopeStr)
		if err != nil {
			continue
		}

		// 完全匹配
		if userScope.Domain == requiredScope.Domain &&
			userScope.Resource == requiredScope.Resource &&
			userScope.Action == requiredScope.Action {
			return true
		}

		// manage权限包含所有其他权限
		if userScope.Domain == requiredScope.Domain &&
			userScope.Resource == requiredScope.Resource &&
			userScope.Action == ScopeActionManage {
			return true
		}

		// admin:admin:manage 包含所有
		if userScope.Domain == ScopeDomainAdmin &&
			userScope.Resource == "admin" &&
			userScope.Action == ScopeActionManage {
			return true
		}
	}
	return false
}

// HasAnyScope 检查是否包含任一指定scope
func HasAnyScope(userScopes []string, requiredScopes []*Scope) bool {
	for _, rs := range requiredScopes {
		if HasScope(userScopes, rs) {
			return true
		}
	}
	return false
}

// HasAllScopes 检查是否包含所有指定scope
func HasAllScopes(userScopes []string, requiredScopes []*Scope) bool {
	for _, rs := range requiredScopes {
		if !HasScope(userScopes, rs) {
			return false
		}
	}
	return true
}

// InvalidScopeError 无效scope错误
type InvalidScopeError struct {
	Scope  string
	Reason string
}

func (e *InvalidScopeError) Error() string {
	return "invalid scope '" + e.Scope + "': " + e.Reason
}

// CommonScopes 常用scope定义
var CommonScopes = struct {
	// Supply域
	SupplyAccountsRead    string
	SupplyAccountsWrite  string
	SupplyAccountsDelete string
	SupplyAccountsManage string

	SupplyPackagesRead    string
	SupplyPackagesWrite   string
	SupplyPackagesDelete string
	SupplyPackagesManage string

	SupplyOrdersRead    string
	SupplyOrdersWrite   string
	SupplyOrdersManage string

	SupplyUsageRead string

	// Billing域
	BillingAccountsRead  string
	BillingLedgersRead  string
	BillingSettlementsRead string
	BillingSettlementsWrite string

	// IAM域
	IAMUsersRead  string
	IAMUsersWrite string
	IAMUsersManage string

	// Audit域
	AuditEventsRead string

	// Admin域
	AdminAll string
}{
	// Supply域
	SupplyAccountsRead:    "supply:accounts:read",
	SupplyAccountsWrite:   "supply:accounts:write",
	SupplyAccountsDelete:  "supply:accounts:delete",
	SupplyAccountsManage:  "supply:accounts:manage",

	SupplyPackagesRead:    "supply:packages:read",
	SupplyPackagesWrite:   "supply:packages:write",
	SupplyPackagesDelete:  "supply:packages:delete",
	SupplyPackagesManage:  "supply:packages:manage",

	SupplyOrdersRead:    "supply:orders:read",
	SupplyOrdersWrite:   "supply:orders:write",
	SupplyOrdersManage:  "supply:orders:manage",

	SupplyUsageRead: "supply:usage:read",

	// Billing域
	BillingAccountsRead:     "billing:accounts:read",
	BillingLedgersRead:      "billing:ledgers:read",
	BillingSettlementsRead:  "billing:settlements:read",
	BillingSettlementsWrite: "billing:settlements:write",

	// IAM域
	IAMUsersRead:  "iam:users:read",
	IAMUsersWrite: "iam:users:write",
	IAMUsersManage: "iam:users:manage",

	// Audit域
	AuditEventsRead: "audit:events:read",

	// Admin域
	AdminAll: "admin:admin:manage",
}

// RoleScopes 角色默认scope映射
var RoleScopes = map[string][]string{
	"viewer": {
		CommonScopes.SupplyAccountsRead,
		CommonScopes.SupplyPackagesRead,
		CommonScopes.SupplyOrdersRead,
		CommonScopes.SupplyUsageRead,
		CommonScopes.BillingAccountsRead,
		CommonScopes.BillingLedgersRead,
		CommonScopes.AuditEventsRead,
	},
	"operator": {
		CommonScopes.SupplyAccountsRead,
		CommonScopes.SupplyAccountsWrite,
		CommonScopes.SupplyPackagesRead,
		CommonScopes.SupplyPackagesWrite,
		CommonScopes.SupplyOrdersRead,
		CommonScopes.SupplyOrdersWrite,
		CommonScopes.SupplyUsageRead,
		CommonScopes.BillingAccountsRead,
		CommonScopes.BillingSettlementsRead,
	},
	"admin": {
		CommonScopes.SupplyAccountsManage,
		CommonScopes.SupplyPackagesManage,
		CommonScopes.SupplyOrdersManage,
		CommonScopes.BillingAccountsRead,
		CommonScopes.BillingSettlementsRead,
		CommonScopes.BillingSettlementsWrite,
		CommonScopes.IAMUsersRead,
		CommonScopes.IAMUsersWrite,
		CommonScopes.AuditEventsRead,
	},
	"owner": {
		// Owner拥有所有权限
		CommonScopes.AdminAll,
	},
}

// GetScopesForRole 获取角色默认scope列表
func GetScopesForRole(role string) []string {
	if scopes, ok := RoleScopes[role]; ok {
		return scopes
	}
	return []string{}
}
