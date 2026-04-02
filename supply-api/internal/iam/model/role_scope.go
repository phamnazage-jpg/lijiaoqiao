package model

import (
	"time"
)

// RoleScopeMapping 角色-Scope关联模型
// 对应数据库 iam_role_scopes 表
type RoleScopeMapping struct {
	ID        int64  // 主键ID
	RoleID    int64  // 角色ID (FK -> iam_roles.id)
	ScopeID   int64  // ScopeID (FK -> iam_scopes.id)
	IsActive  bool   // 是否激活

	// 审计字段
	RequestID string // 请求追踪ID
	CreatedIP string // 创建者IP
	Version   int    // 乐观锁版本号

	// 时间戳
	CreatedAt *time.Time // 创建时间
}

// NewRoleScopeMapping 创建新的角色-Scope映射
func NewRoleScopeMapping(roleID, scopeID int64) *RoleScopeMapping {
	now := time.Now()
	return &RoleScopeMapping{
		RoleID:    roleID,
		ScopeID:   scopeID,
		IsActive:  true,
		RequestID: generateRequestID(),
		Version:   1,
		CreatedAt: &now,
	}
}

// NewRoleScopeMappingWithAudit 创建带审计信息的角色-Scope映射
func NewRoleScopeMappingWithAudit(roleID, scopeID int64, requestID, createdIP string) *RoleScopeMapping {
	now := time.Now()
	return &RoleScopeMapping{
		RoleID:    roleID,
		ScopeID:   scopeID,
		IsActive:  true,
		RequestID: requestID,
		CreatedIP: createdIP,
		Version:   1,
		CreatedAt: &now,
	}
}

// Revoke 撤销角色-Scope映射
func (m *RoleScopeMapping) Revoke() {
	m.IsActive = false
}

// Grant 授予角色-Scope映射
func (m *RoleScopeMapping) Grant() {
	m.IsActive = true
}

// IncrementVersion 递增版本号
func (m *RoleScopeMapping) IncrementVersion() {
	m.Version++
}

// GrantScopeList 批量授予Scope
func GrantScopeList(roleID int64, scopeIDs []int64) []*RoleScopeMapping {
	mappings := make([]*RoleScopeMapping, 0, len(scopeIDs))
	for _, scopeID := range scopeIDs {
		mapping := NewRoleScopeMapping(roleID, scopeID)
		mappings = append(mappings, mapping)
	}
	return mappings
}

// RevokeAll 撤销所有映射
func RevokeAll(mappings []*RoleScopeMapping) {
	for _, mapping := range mappings {
		mapping.Revoke()
	}
}

// GetActiveScopeIDs 从映射列表中获取活跃的Scope ID列表
func GetActiveScopeIDs(mappings []*RoleScopeMapping) []int64 {
	activeIDs := make([]int64, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.IsActive {
			activeIDs = append(activeIDs, mapping.ScopeID)
		}
	}
	return activeIDs
}

// GetInactiveScopeIDs 从映射列表中获取非活跃的Scope ID列表
func GetInactiveScopeIDs(mappings []*RoleScopeMapping) []int64 {
	inactiveIDs := make([]int64, 0, len(mappings))
	for _, mapping := range mappings {
		if !mapping.IsActive {
			inactiveIDs = append(inactiveIDs, mapping.ScopeID)
		}
	}
	return inactiveIDs
}

// FilterActiveMappings 过滤出活跃的映射
func FilterActiveMappings(mappings []*RoleScopeMapping) []*RoleScopeMapping {
	active := make([]*RoleScopeMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.IsActive {
			active = append(active, mapping)
		}
	}
	return active
}

// FilterMappingsByRole 过滤出指定角色的映射
func FilterMappingsByRole(mappings []*RoleScopeMapping, roleID int64) []*RoleScopeMapping {
	filtered := make([]*RoleScopeMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.RoleID == roleID {
			filtered = append(filtered, mapping)
		}
	}
	return filtered
}

// FilterMappingsByScope 过滤出指定Scope的映射
func FilterMappingsByScope(mappings []*RoleScopeMapping, scopeID int64) []*RoleScopeMapping {
	filtered := make([]*RoleScopeMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.ScopeID == scopeID {
			filtered = append(filtered, mapping)
		}
	}
	return filtered
}

// RoleScopeMappingInfo 角色-Scope映射信息（用于API响应）
type RoleScopeMappingInfo struct {
	RoleID   int64  `json:"role_id"`
	ScopeID  int64  `json:"scope_id"`
	IsActive bool   `json:"is_active"`
}

// ToInfo 转换为映射信息
func (m *RoleScopeMapping) ToInfo() *RoleScopeMappingInfo {
	return &RoleScopeMappingInfo{
		RoleID:   m.RoleID,
		ScopeID:  m.ScopeID,
		IsActive: m.IsActive,
	}
}
