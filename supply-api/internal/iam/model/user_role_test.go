package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestUserRoleMapping_AssignRole 测试分配角色
func TestUserRoleMapping_AssignRole(t *testing.T) {
	// arrange
	userID := int64(100)
	roleID := int64(1)
	tenantID := int64(1)

	// act
	userRole := NewUserRoleMapping(userID, roleID, tenantID)

	// assert
	assert.Equal(t, userID, userRole.UserID)
	assert.Equal(t, roleID, userRole.RoleID)
	assert.Equal(t, tenantID, userRole.TenantID)
	assert.True(t, userRole.IsActive)
	assert.NotEmpty(t, userRole.RequestID)
	assert.Equal(t, 1, userRole.Version)
}

// TestUserRoleMapping_HasRole 测试用户是否拥有角色
func TestUserRoleMapping_HasRole(t *testing.T) {
	// arrange
	userID := int64(100)
	role := NewRole("org_admin", "组织管理员", RoleTypePlatform, 50)
	role.ID = 1

	// act
	userRole := NewUserRoleMapping(userID, role.ID, 0) // 0 表示全局角色

	// assert
	assert.True(t, userRole.HasRole(role.ID))
	assert.False(t, userRole.HasRole(999)) // 不存在的角色ID
}

// TestUserRoleMapping_GlobalRole 测试全局角色（tenantID为0）
func TestUserRoleMapping_GlobalRole(t *testing.T) {
	// arrange
	userID := int64(100)
	roleID := int64(1)

	// act - 全局角色
	userRole := NewUserRoleMapping(userID, roleID, 0)

	// assert
	assert.Equal(t, int64(0), userRole.TenantID)
	assert.True(t, userRole.IsGlobalRole())
}

// TestUserRoleMapping_TenantRole 测试租户角色
func TestUserRoleMapping_TenantRole(t *testing.T) {
	// arrange
	userID := int64(100)
	roleID := int64(1)
	tenantID := int64(123)

	// act
	userRole := NewUserRoleMapping(userID, roleID, tenantID)

	// assert
	assert.Equal(t, tenantID, userRole.TenantID)
	assert.False(t, userRole.IsGlobalRole())
}

// TestUserRoleMapping_WithGrantInfo 测试带授权信息的分配
func TestUserRoleMapping_WithGrantInfo(t *testing.T) {
	// arrange
	userID := int64(100)
	roleID := int64(1)
	tenantID := int64(1)
	grantedBy := int64(1)
	expiresAt := time.Now().Add(24 * time.Hour)

	// act
	userRole := NewUserRoleMappingWithGrant(userID, roleID, tenantID, grantedBy, &expiresAt)

	// assert
	assert.Equal(t, userID, userRole.UserID)
	assert.Equal(t, roleID, userRole.RoleID)
	assert.Equal(t, grantedBy, userRole.GrantedBy)
	assert.NotNil(t, userRole.ExpiresAt)
	assert.NotNil(t, userRole.GrantedAt)
}

// TestUserRoleMapping_Expired 测试过期角色
func TestUserRoleMapping_Expired(t *testing.T) {
	// arrange
	userID := int64(100)
	roleID := int64(1)
	expiresAt := time.Now().Add(-1 * time.Hour) // 已过期

	// act
	userRole := NewUserRoleMappingWithGrant(userID, roleID, 0, 1, &expiresAt)

	// assert
	assert.True(t, userRole.IsExpired())
}

// TestUserRoleMapping_NotExpired 测试未过期角色
func TestUserRoleMapping_NotExpired(t *testing.T) {
	// arrange
	userID := int64(100)
	roleID := int64(1)
	expiresAt := time.Now().Add(24 * time.Hour) // 未过期

	// act
	userRole := NewUserRoleMappingWithGrant(userID, roleID, 0, 1, &expiresAt)

	// assert
	assert.False(t, userRole.IsExpired())
}

// TestUserRoleMapping_NoExpiration 测试永不过期角色
func TestUserRoleMapping_NoExpiration(t *testing.T) {
	// arrange
	userID := int64(100)
	roleID := int64(1)

	// act
	userRole := NewUserRoleMapping(userID, roleID, 0)

	// assert
	assert.Nil(t, userRole.ExpiresAt)
	assert.False(t, userRole.IsExpired())
}

// TestUserRoleMapping_Revoke 测试撤销角色
func TestUserRoleMapping_Revoke(t *testing.T) {
	// arrange
	userRole := NewUserRoleMapping(100, 1, 0)

	// act
	userRole.Revoke()

	// assert
	assert.False(t, userRole.IsActive)
}

// TestUserRoleMapping_Grant 测试重新授予角色
func TestUserRoleMapping_Grant(t *testing.T) {
	// arrange
	userRole := NewUserRoleMapping(100, 1, 0)
	userRole.Revoke()

	// act
	userRole.Grant()

	// assert
	assert.True(t, userRole.IsActive)
}

// TestUserRoleMapping_IncrementVersion 测试版本号递增
func TestUserRoleMapping_IncrementVersion(t *testing.T) {
	// arrange
	userRole := NewUserRoleMapping(100, 1, 0)
	originalVersion := userRole.Version

	// act
	userRole.IncrementVersion()

	// assert
	assert.Equal(t, originalVersion+1, userRole.Version)
}

// TestUserRoleMapping_Valid 测试有效角色
func TestUserRoleMapping_Valid(t *testing.T) {
	// arrange - 活跃且未过期的角色
	userRole := NewUserRoleMapping(100, 1, 0)
	expiresAt := time.Now().Add(24 * time.Hour)
	userRole.ExpiresAt = &expiresAt

	// act & assert
	assert.True(t, userRole.IsValid())
}

// TestUserRoleMapping_InvalidInactive 测试无效角色 - 未激活
func TestUserRoleMapping_InvalidInactive(t *testing.T) {
	// arrange
	userRole := NewUserRoleMapping(100, 1, 0)
	userRole.Revoke()

	// assert
	assert.False(t, userRole.IsValid())
}

// TestUserRoleMapping_Valid_ExpiredButActive 测试过期但激活的角色
func TestUserRoleMapping_Valid_ExpiredButActive(t *testing.T) {
	// arrange - 已过期但仍然激活的角色（应该无效）
	userRole := NewUserRoleMapping(100, 1, 0)
	expiresAt := time.Now().Add(-1 * time.Hour)
	userRole.ExpiresAt = &expiresAt

	// assert - 即使IsActive为true，过期角色也应该无效
	assert.False(t, userRole.IsValid())
}

// TestUserRoleMapping_UniqueConstraint 测试唯一性约束
func TestUserRoleMapping_UniqueConstraint(t *testing.T) {
	// arrange
	userID := int64(100)
	roleID := int64(1)
	tenantID := int64(0) // 全局角色

	// act
	userRole1 := NewUserRoleMapping(userID, roleID, tenantID)
	userRole2 := NewUserRoleMapping(userID, roleID, tenantID)

	// assert - 同一个用户、角色、租户组合应该唯一
	assert.Equal(t, userRole1.UserID, userRole2.UserID)
	assert.Equal(t, userRole1.RoleID, userRole2.RoleID)
	assert.Equal(t, userRole1.TenantID, userRole2.TenantID)
}

// TestUserRoleMapping_DifferentTenants 测试不同租户可以有相同角色
func TestUserRoleMapping_DifferentTenants(t *testing.T) {
	// arrange
	userID := int64(100)
	roleID := int64(1)
	tenantID1 := int64(1)
	tenantID2 := int64(2)

	// act
	userRole1 := NewUserRoleMapping(userID, roleID, tenantID1)
	userRole2 := NewUserRoleMapping(userID, roleID, tenantID2)

	// assert - 不同租户的角色分配互不影响
	assert.Equal(t, tenantID1, userRole1.TenantID)
	assert.Equal(t, tenantID2, userRole2.TenantID)
	assert.NotEqual(t, userRole1.TenantID, userRole2.TenantID)
}

// TestUserRoleMappingInfo_ToInfo 测试转换为UserRoleMappingInfo
func TestUserRoleMappingInfo_ToInfo(t *testing.T) {
	// arrange
	userRole := NewUserRoleMapping(100, 1, 0)
	userRole.ID = 1

	// act
	info := userRole.ToInfo()

	// assert
	assert.Equal(t, int64(100), info.UserID)
	assert.Equal(t, int64(1), info.RoleID)
	assert.Equal(t, int64(0), info.TenantID)
	assert.True(t, info.IsActive)
}
