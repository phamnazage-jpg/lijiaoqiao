package model

import (
	"time"
)

// UserRoleMapping 用户-角色关联模型
// 对应数据库 iam_user_roles 表
type UserRoleMapping struct {
	ID        int64  // 主键ID
	UserID    int64  // 用户ID
	RoleID    int64  // 角色ID (FK -> iam_roles.id)
	TenantID  int64  // 租户范围（NULL表示全局，0也代表全局）
	GrantedBy int64  // 授权人ID
	ExpiresAt *time.Time // 角色过期时间（nil表示永不过期）
	IsActive  bool   // 是否激活

	// 审计字段
	RequestID string // 请求追踪ID
	CreatedIP string // 创建者IP
	UpdatedIP string // 更新者IP
	Version   int    // 乐观锁版本号

	// 时间戳
	CreatedAt *time.Time // 创建时间
	UpdatedAt *time.Time // 更新时间
	GrantedAt *time.Time // 授权时间
}

// NewUserRoleMapping 创建新的用户-角色映射
func NewUserRoleMapping(userID, roleID, tenantID int64) *UserRoleMapping {
	now := time.Now()
	return &UserRoleMapping{
		UserID:    userID,
		RoleID:    roleID,
		TenantID:  tenantID,
		IsActive:  true,
		RequestID: generateRequestID(),
		Version:   1,
		CreatedAt: &now,
		UpdatedAt: &now,
	}
}

// NewUserRoleMappingWithGrant 创建带授权信息的用户-角色映射
func NewUserRoleMappingWithGrant(userID, roleID, tenantID, grantedBy int64, expiresAt *time.Time) *UserRoleMapping {
	now := time.Now()
	return &UserRoleMapping{
		UserID:    userID,
		RoleID:    roleID,
		TenantID:  tenantID,
		GrantedBy: grantedBy,
		ExpiresAt: expiresAt,
		GrantedAt: &now,
		IsActive:  true,
		RequestID: generateRequestID(),
		Version:   1,
		CreatedAt: &now,
		UpdatedAt: &now,
	}
}

// HasRole 检查用户是否拥有指定角色
func (m *UserRoleMapping) HasRole(roleID int64) bool {
	return m.RoleID == roleID && m.IsActive
}

// IsGlobalRole 检查是否为全局角色（租户ID为0或nil）
func (m *UserRoleMapping) IsGlobalRole() bool {
	return m.TenantID == 0
}

// IsExpired 检查角色是否已过期
func (m *UserRoleMapping) IsExpired() bool {
	if m.ExpiresAt == nil {
		return false // 永不过期
	}
	return time.Now().After(*m.ExpiresAt)
}

// IsValid 检查角色分配是否有效（激活且未过期）
func (m *UserRoleMapping) IsValid() bool {
	return m.IsActive && !m.IsExpired()
}

// Revoke 撤销角色分配
func (m *UserRoleMapping) Revoke() {
	m.IsActive = false
	m.UpdatedAt = nowPtr()
}

// Grant 重新授予角色
func (m *UserRoleMapping) Grant() {
	m.IsActive = true
	m.UpdatedAt = nowPtr()
}

// IncrementVersion 递增版本号
func (m *UserRoleMapping) IncrementVersion() {
	m.Version++
	m.UpdatedAt = nowPtr()
}

// ExtendExpiration 延长过期时间
func (m *UserRoleMapping) ExtendExpiration(newExpiresAt *time.Time) {
	m.ExpiresAt = newExpiresAt
	m.UpdatedAt = nowPtr()
}

// UserRoleMappingInfo 用户-角色映射信息（用于API响应）
type UserRoleMappingInfo struct {
	UserID    int64   `json:"user_id"`
	RoleID    int64   `json:"role_id"`
	TenantID  int64   `json:"tenant_id"`
	IsActive  bool    `json:"is_active"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

// ToInfo 转换为映射信息
func (m *UserRoleMapping) ToInfo() *UserRoleMappingInfo {
	info := &UserRoleMappingInfo{
		UserID:   m.UserID,
		RoleID:   m.RoleID,
		TenantID: m.TenantID,
		IsActive: m.IsActive,
	}
	if m.ExpiresAt != nil {
		expStr := m.ExpiresAt.Format(time.RFC3339)
		info.ExpiresAt = &expStr
	}
	return info
}

// UserRoleAssignmentInfo 用户角色分配详情（用于API响应）
type UserRoleAssignmentInfo struct {
	UserID     int64  `json:"user_id"`
	RoleCode   string `json:"role_code"`
	RoleName   string `json:"role_name"`
	TenantID   int64  `json:"tenant_id"`
	GrantedBy  int64  `json:"granted_by"`
	GrantedAt  string `json:"granted_at"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	IsActive   bool   `json:"is_active"`
	IsExpired  bool   `json:"is_expired"`
}

// UserRoleWithDetails 用户角色分配（含角色详情）
type UserRoleWithDetails struct {
	*UserRoleMapping
	RoleCode string
	RoleName string
}

// ToAssignmentInfo 转换为分配详情
func (m *UserRoleWithDetails) ToAssignmentInfo() *UserRoleAssignmentInfo {
	info := &UserRoleAssignmentInfo{
		UserID:    m.UserID,
		RoleCode:  m.RoleCode,
		RoleName:  m.RoleName,
		TenantID:  m.TenantID,
		GrantedBy: m.GrantedBy,
		IsActive:  m.IsActive,
		IsExpired: m.IsExpired(),
	}
	if m.GrantedAt != nil {
		info.GrantedAt = m.GrantedAt.Format(time.RFC3339)
	}
	if m.ExpiresAt != nil {
		info.ExpiresAt = m.ExpiresAt.Format(time.RFC3339)
	}
	return info
}
