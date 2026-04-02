package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRoleModel_NewRole_ValidInput 测试创建角色 - 有效输入
func TestRoleModel_NewRole_ValidInput(t *testing.T) {
	// arrange
	roleCode := "org_admin"
	roleName := "组织管理员"
	roleType := "platform"
	level := 50

	// act
	role := NewRole(roleCode, roleName, roleType, level)

	// assert
	assert.Equal(t, roleCode, role.Code)
	assert.Equal(t, roleName, role.Name)
	assert.Equal(t, roleType, role.Type)
	assert.Equal(t, level, role.Level)
	assert.True(t, role.IsActive)
	assert.NotEmpty(t, role.RequestID)
	assert.Equal(t, 1, role.Version)
}

// TestRoleModel_NewRole_DefaultFields 测试创建角色 - 验证默认字段
func TestRoleModel_NewRole_DefaultFields(t *testing.T) {
	// arrange
	roleCode := "viewer"
	roleName := "查看者"
	roleType := "platform"
	level := 10

	// act
	role := NewRole(roleCode, roleName, roleType, level)

	// assert - 验证默认字段
	assert.Equal(t, 1, role.Version, "version should default to 1")
	assert.NotEmpty(t, role.RequestID, "request_id should be auto-generated")
	assert.True(t, role.IsActive, "is_active should default to true")
	assert.Nil(t, role.ParentRoleID, "parent_role_id should be nil for root roles")
}

// TestRoleModel_NewRole_WithParent 测试创建角色 - 带父角色
func TestRoleModel_NewRole_WithParent(t *testing.T) {
	// arrange
	parentRole := NewRole("viewer", "查看者", "platform", 10)
	parentRole.ID = 1

	// act
	childRole := NewRoleWithParent("developer", "开发者", "platform", 20, parentRole.ID)

	// assert
	assert.Equal(t, "developer", childRole.Code)
	assert.Equal(t, 20, childRole.Level)
	assert.NotNil(t, childRole.ParentRoleID)
	assert.Equal(t, parentRole.ID, *childRole.ParentRoleID)
}

// TestRoleModel_NewRole_WithRequestID 测试创建角色 - 指定RequestID
func TestRoleModel_NewRole_WithRequestID(t *testing.T) {
	// arrange
	requestID := "req-12345"

	// act
	role := NewRoleWithRequestID("org_admin", "组织管理员", "platform", 50, requestID)

	// assert
	assert.Equal(t, requestID, role.RequestID)
}

// TestRoleModel_NewRole_AuditFields 测试创建角色 - 审计字段
func TestRoleModel_NewRole_AuditFields(t *testing.T) {
	// arrange
	createdIP := "192.168.1.1"
	updatedIP := "192.168.1.2"

	// act
	role := NewRoleWithAudit("supply_admin", "供应方管理员", "supply", 40, "req-123", createdIP, updatedIP)

	// assert
	assert.Equal(t, createdIP, role.CreatedIP)
	assert.Equal(t, updatedIP, role.UpdatedIP)
	assert.Equal(t, 1, role.Version)
}

// TestRoleModel_NewRole_Timestamps 测试创建角色 - 时间戳
func TestRoleModel_NewRole_Timestamps(t *testing.T) {
	// arrange
	beforeCreate := time.Now()

	// act
	role := NewRole("test_role", "测试角色", "platform", 10)
	_ = time.Now() // afterCreate not needed

	// assert
	assert.NotNil(t, role.CreatedAt)
	assert.NotNil(t, role.UpdatedAt)
	assert.True(t, role.CreatedAt.After(beforeCreate) || role.CreatedAt.Equal(beforeCreate))
	assert.True(t, role.UpdatedAt.After(beforeCreate) || role.UpdatedAt.Equal(beforeCreate))
}

// TestRoleModel_Activate 测试激活角色
func TestRoleModel_Activate(t *testing.T) {
	// arrange
	role := NewRole("inactive_role", "非活跃角色", "platform", 10)
	role.IsActive = false

	// act
	role.Activate()

	// assert
	assert.True(t, role.IsActive)
}

// TestRoleModel_Deactivate 测试停用角色
func TestRoleModel_Deactivate(t *testing.T) {
	// arrange
	role := NewRole("active_role", "活跃角色", "platform", 10)

	// act
	role.Deactivate()

	// assert
	assert.False(t, role.IsActive)
}

// TestRoleModel_IncrementVersion 测试版本号递增
func TestRoleModel_IncrementVersion(t *testing.T) {
	// arrange
	role := NewRole("test_role", "测试角色", "platform", 10)
	originalVersion := role.Version

	// act
	role.IncrementVersion()

	// assert
	assert.Equal(t, originalVersion+1, role.Version)
}

// TestRoleModel_RoleType_Platform 测试平台角色类型
func TestRoleModel_RoleType_Platform(t *testing.T) {
	// arrange & act
	role := NewRole("super_admin", "超级管理员", RoleTypePlatform, 100)

	// assert
	assert.Equal(t, RoleTypePlatform, role.Type)
}

// TestRoleModel_RoleType_Supply 测试供应方角色类型
func TestRoleModel_RoleType_Supply(t *testing.T) {
	// arrange & act
	role := NewRole("supply_admin", "供应方管理员", RoleTypeSupply, 40)

	// assert
	assert.Equal(t, RoleTypeSupply, role.Type)
}

// TestRoleModel_RoleType_Consumer 测试需求方角色类型
func TestRoleModel_RoleType_Consumer(t *testing.T) {
	// arrange & act
	role := NewRole("consumer_admin", "需求方管理员", RoleTypeConsumer, 40)

	// assert
	assert.Equal(t, RoleTypeConsumer, role.Type)
}

// TestRoleModel_LevelHierarchy 测试角色层级关系
func TestRoleModel_LevelHierarchy(t *testing.T) {
	// 测试设计文档中的层级关系
	// super_admin(100) > org_admin(50) > supply_admin(40) > operator(30) > developer/finops(20) > viewer(10)

	// arrange
	superAdmin := NewRole("super_admin", "超级管理员", RoleTypePlatform, 100)
	orgAdmin := NewRole("org_admin", "组织管理员", RoleTypePlatform, 50)
	supplyAdmin := NewRole("supply_admin", "供应方管理员", RoleTypeSupply, 40)
	operator := NewRole("operator", "运维人员", RoleTypePlatform, 30)
	developer := NewRole("developer", "开发者", RoleTypePlatform, 20)
	viewer := NewRole("viewer", "查看者", RoleTypePlatform, 10)

	// assert - 验证层级数值
	assert.Greater(t, superAdmin.Level, orgAdmin.Level)
	assert.Greater(t, orgAdmin.Level, supplyAdmin.Level)
	assert.Greater(t, supplyAdmin.Level, operator.Level)
	assert.Greater(t, operator.Level, developer.Level)
	assert.Greater(t, developer.Level, viewer.Level)
}

// TestRoleModel_NewRole_EmptyCode 测试创建角色 - 空角色代码（应返回错误）
func TestRoleModel_NewRole_EmptyCode(t *testing.T) {
	// arrange & act
	role, err := NewRoleWithValidation("", "测试角色", "platform", 10)

	// assert
	assert.Error(t, err)
	assert.Nil(t, role)
	assert.Equal(t, ErrInvalidRoleCode, err)
}

// TestRoleModel_NewRole_InvalidRoleType 测试创建角色 - 无效角色类型
func TestRoleModel_NewRole_InvalidRoleType(t *testing.T) {
	// arrange & act
	role, err := NewRoleWithValidation("test_role", "测试角色", "invalid_type", 10)

	// assert
	assert.Error(t, err)
	assert.Nil(t, role)
	assert.Equal(t, ErrInvalidRoleType, err)
}

// TestRoleModel_NewRole_NegativeLevel 测试创建角色 - 负数层级
func TestRoleModel_NewRole_NegativeLevel(t *testing.T) {
	// arrange & act
	role, err := NewRoleWithValidation("test_role", "测试角色", "platform", -1)

	// assert
	assert.Error(t, err)
	assert.Nil(t, role)
	assert.Equal(t, ErrInvalidLevel, err)
}

// TestRoleModel_ToRoleScopeInfo 测试角色转换为RoleScopeInfo
func TestRoleModel_ToRoleScopeInfo(t *testing.T) {
	// arrange
	role := NewRole("org_admin", "组织管理员", RoleTypePlatform, 50)
	role.ID = 1
	role.Scopes = []string{"platform:read", "platform:write"}

	// act
	roleScopeInfo := role.ToRoleScopeInfo()

	// assert
	assert.Equal(t, "org_admin", roleScopeInfo.RoleCode)
	assert.Equal(t, "组织管理员", roleScopeInfo.RoleName)
	assert.Equal(t, 50, roleScopeInfo.Level)
	assert.Len(t, roleScopeInfo.Scopes, 2)
	assert.Contains(t, roleScopeInfo.Scopes, "platform:read")
	assert.Contains(t, roleScopeInfo.Scopes, "platform:write")
}
