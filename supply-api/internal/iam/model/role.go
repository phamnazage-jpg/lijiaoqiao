package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// 角色类型常量
const (
	RoleTypePlatform = "platform"
	RoleTypeSupply   = "supply"
	RoleTypeConsumer = "consumer"
)

// 角色层级常量（用于权限优先级判断）
// 注意：这些常量值必须与 RoleHierarchyLevels map保持一致
const (
	LevelSuperAdmin = 100
	LevelOrgAdmin   = 50
	LevelSupplyAdmin = 40
	LevelOperator   = 30
	LevelDeveloper  = 20
	LevelFinops     = 20
	LevelViewer     = 10
)

// RoleHierarchyLevels 角色层级映射（用于权限验证）
// 层级越高，权限越大。superset角色可以执行subset角色的操作。
// 注意：此map的值必须与上述常量保持一致！
var RoleHierarchyLevels = map[string]int{
	"super_admin":       LevelSuperAdmin, // 100 - 超级管理员
	"org_admin":         LevelOrgAdmin,   // 50  - 组织管理员
	"supply_admin":      LevelSupplyAdmin, // 40 - 供应商管理员
	"consumer_admin":    LevelSupplyAdmin, // 40 - 消费者管理员(同供应商)
	"operator":          LevelOperator,  // 30  - 操作员
	"developer":          LevelDeveloper, // 20  - 开发者
	"finops":            LevelFinops,    // 20  - 财务运营
	"supply_operator":   LevelOperator, // 30  - 供应商操作员
	"supply_finops":     LevelFinops,   // 20  - 供应商财务
	"supply_viewer":     LevelViewer,   // 10  - 供应商查看者
	"consumer_operator": LevelOperator, // 30  - 消费者操作员
	"consumer_viewer":   LevelViewer,   // 10  - 消费者查看者
	"viewer":            LevelViewer,    // 10  - 通用查看者
}

// GetRoleLevelByCode 根据角色代码获取层级数值
func GetRoleLevelByCode(roleCode string) int {
	if level, ok := RoleHierarchyLevels[roleCode]; ok {
		return level
	}
	return 0 // 默认最低级别
}

// 角色错误定义
var (
	ErrInvalidRoleCode = errors.New("invalid role code: cannot be empty")
	ErrInvalidRoleType = errors.New("invalid role type: must be platform, supply, or consumer")
	ErrInvalidLevel    = errors.New("invalid level: must be non-negative")
)

// Role 角色模型
// 对应数据库 iam_roles 表
type Role struct {
	ID          int64   // 主键ID
	Code        string  // 角色代码 (unique)
	Name        string  // 角色名称
	Type        string  // 角色类型: platform, supply, consumer
	ParentRoleID *int64 // 父角色ID（用于继承关系）
	Level       int     // 权限层级
	Description string  // 描述
	IsActive    bool    // 是否激活

	// 审计字段
	RequestID string // 请求追踪ID
	CreatedIP string // 创建者IP
	UpdatedIP string // 更新者IP
	Version   int    // 乐观锁版本号

	// 时间戳
	CreatedAt *time.Time // 创建时间
	UpdatedAt *time.Time // 更新时间

	// 关联的Scope列表（运行时填充，不存储在iam_roles表）
	Scopes []string `json:"scopes,omitempty"`
}

// NewRole 创建新角色（基础构造函数）
func NewRole(code, name, roleType string, level int) *Role {
	now := time.Now()
	return &Role{
		Code:        code,
		Name:        name,
		Type:        roleType,
		Level:       level,
		IsActive:    true,
		RequestID:   generateRequestID(),
		Version:     1,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
}

// NewRoleWithParent 创建带父角色的角色
func NewRoleWithParent(code, name, roleType string, level int, parentRoleID int64) *Role {
	role := NewRole(code, name, roleType, level)
	role.ParentRoleID = &parentRoleID
	return role
}

// NewRoleWithRequestID 创建带指定RequestID的角色
func NewRoleWithRequestID(code, name, roleType string, level int, requestID string) *Role {
	role := NewRole(code, name, roleType, level)
	role.RequestID = requestID
	return role
}

// NewRoleWithAudit 创建带审计信息的角色
func NewRoleWithAudit(code, name, roleType string, level int, requestID, createdIP, updatedIP string) *Role {
	role := NewRole(code, name, roleType, level)
	role.RequestID = requestID
	role.CreatedIP = createdIP
	role.UpdatedIP = updatedIP
	return role
}

// NewRoleWithValidation 创建角色并进行验证
func NewRoleWithValidation(code, name, roleType string, level int) (*Role, error) {
	// 验证角色代码
	if code == "" {
		return nil, ErrInvalidRoleCode
	}

	// 验证角色类型
	if roleType != RoleTypePlatform && roleType != RoleTypeSupply && roleType != RoleTypeConsumer {
		return nil, ErrInvalidRoleType
	}

	// 验证层级
	if level < 0 {
		return nil, ErrInvalidLevel
	}

	role := NewRole(code, name, roleType, level)
	return role, nil
}

// Activate 激活角色
func (r *Role) Activate() {
	r.IsActive = true
	r.UpdatedAt = nowPtr()
}

// Deactivate 停用角色
func (r *Role) Deactivate() {
	r.IsActive = false
	r.UpdatedAt = nowPtr()
}

// IncrementVersion 递增版本号（用于乐观锁）
func (r *Role) IncrementVersion() {
	r.Version++
	r.UpdatedAt = nowPtr()
}

// SetParentRole 设置父角色
func (r *Role) SetParentRole(parentID int64) {
	r.ParentRoleID = &parentID
}

// SetScopes 设置角色关联的Scope列表
func (r *Role) SetScopes(scopes []string) {
	r.Scopes = scopes
}

// AddScope 添加一个Scope
func (r *Role) AddScope(scope string) {
	for _, s := range r.Scopes {
		if s == scope {
			return
		}
	}
	r.Scopes = append(r.Scopes, scope)
}

// RemoveScope 移除一个Scope
func (r *Role) RemoveScope(scope string) {
	newScopes := make([]string, 0, len(r.Scopes))
	for _, s := range r.Scopes {
		if s != scope {
			newScopes = append(newScopes, s)
		}
	}
	r.Scopes = newScopes
}

// HasScope 检查角色是否拥有指定Scope
func (r *Role) HasScope(scope string) bool {
	for _, s := range r.Scopes {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}

// ToRoleScopeInfo 转换为RoleScopeInfo结构（用于API响应）
func (r *Role) ToRoleScopeInfo() *RoleScopeInfo {
	return &RoleScopeInfo{
		RoleCode: r.Code,
		RoleName: r.Name,
		RoleType: r.Type,
		Level:    r.Level,
		Scopes:   r.Scopes,
	}
}

// RoleScopeInfo 角色的Scope信息（用于API响应）
type RoleScopeInfo struct {
	RoleCode string   `json:"role_code"`
	RoleName string   `json:"role_name"`
	RoleType string   `json:"role_type"`
	Level    int      `json:"level"`
	Scopes   []string `json:"scopes,omitempty"`
}

// generateRequestID 生成请求追踪ID
func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// nowPtr 返回当前时间的指针
func nowPtr() *time.Time {
	t := time.Now()
	return &t
}
