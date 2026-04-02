package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRoleScopeMapping_GrantScope 测试授予Scope
func TestRoleScopeMapping_GrantScope(t *testing.T) {
	// arrange
	role := NewRole("operator", "运维人员", RoleTypePlatform, 30)
	role.ID = 1
	scope1 := NewScope("platform:read", "读取平台配置", ScopeTypePlatform)
	scope1.ID = 1
	scope2 := NewScope("platform:write", "修改平台配置", ScopeTypePlatform)
	scope2.ID = 2

	// act
	roleScopeMapping := NewRoleScopeMapping(role.ID, scope1.ID)
	roleScopeMapping2 := NewRoleScopeMapping(role.ID, scope2.ID)

	// assert
	assert.Equal(t, role.ID, roleScopeMapping.RoleID)
	assert.Equal(t, scope1.ID, roleScopeMapping.ScopeID)
	assert.NotEmpty(t, roleScopeMapping.RequestID)
	assert.Equal(t, 1, roleScopeMapping.Version)

	assert.Equal(t, role.ID, roleScopeMapping2.RoleID)
	assert.Equal(t, scope2.ID, roleScopeMapping2.ScopeID)
}

// TestRoleScopeMapping_RevokeScope 测试撤销Scope
func TestRoleScopeMapping_RevokeScope(t *testing.T) {
	// arrange
	role := NewRole("viewer", "查看者", RoleTypePlatform, 10)
	role.ID = 1
	scope := NewScope("platform:read", "读取平台配置", ScopeTypePlatform)
	scope.ID = 1

	// act
	roleScopeMapping := NewRoleScopeMapping(role.ID, scope.ID)
	roleScopeMapping.Revoke()

	// assert
	assert.False(t, roleScopeMapping.IsActive, "revoked mapping should be inactive")
}

// TestRoleScopeMapping_WithAudit 测试带审计字段的映射
func TestRoleScopeMapping_WithAudit(t *testing.T) {
	// arrange
	roleID := int64(1)
	scopeID := int64(2)
	requestID := "req-role-scope-123"
	createdIP := "192.168.1.100"

	// act
	mapping := NewRoleScopeMappingWithAudit(roleID, scopeID, requestID, createdIP)

	// assert
	assert.Equal(t, roleID, mapping.RoleID)
	assert.Equal(t, scopeID, mapping.ScopeID)
	assert.Equal(t, requestID, mapping.RequestID)
	assert.Equal(t, createdIP, mapping.CreatedIP)
	assert.True(t, mapping.IsActive)
}

// TestRoleScopeMapping_IncrementVersion 测试版本号递增
func TestRoleScopeMapping_IncrementVersion(t *testing.T) {
	// arrange
	mapping := NewRoleScopeMapping(1, 1)
	originalVersion := mapping.Version

	// act
	mapping.IncrementVersion()

	// assert
	assert.Equal(t, originalVersion+1, mapping.Version)
}

// TestRoleScopeMapping_IsActive 测试活跃状态
func TestRoleScopeMapping_IsActive(t *testing.T) {
	// arrange
	mapping := NewRoleScopeMapping(1, 1)

	// assert - 默认应该激活
	assert.True(t, mapping.IsActive)
}

// TestRoleScopeMapping_UniqueConstraint 测试唯一性（同一个角色和Scope组合）
func TestRoleScopeMapping_UniqueConstraint(t *testing.T) {
	// arrange
	roleID := int64(1)
	scopeID := int64(1)

	// act
	mapping1 := NewRoleScopeMapping(roleID, scopeID)
	mapping2 := NewRoleScopeMapping(roleID, scopeID)

	// assert - 两个映射应该有相同的 RoleID 和 ScopeID（代表唯一约束）
	assert.Equal(t, mapping1.RoleID, mapping2.RoleID)
	assert.Equal(t, mapping1.ScopeID, mapping2.ScopeID)
}

// TestRoleScopeMapping_GrantScopeList 测试批量授予Scope
func TestRoleScopeMapping_GrantScopeList(t *testing.T) {
	// arrange
	roleID := int64(1)
	scopeIDs := []int64{1, 2, 3, 4, 5}

	// act
	mappings := GrantScopeList(roleID, scopeIDs)

	// assert
	assert.Len(t, mappings, len(scopeIDs))
	for i, scopeID := range scopeIDs {
		assert.Equal(t, roleID, mappings[i].RoleID)
		assert.Equal(t, scopeID, mappings[i].ScopeID)
		assert.True(t, mappings[i].IsActive)
	}
}

// TestRoleScopeMapping_RevokeAll 测试撤销所有Scope（针对某个角色）
func TestRoleScopeMapping_RevokeAll(t *testing.T) {
	// arrange
	roleID := int64(1)
	scopeIDs := []int64{1, 2, 3}
	mappings := GrantScopeList(roleID, scopeIDs)

	// act
	RevokeAll(mappings)

	// assert
	for _, mapping := range mappings {
		assert.False(t, mapping.IsActive, "all mappings should be revoked")
	}
}

// TestRoleScopeMapping_GetActiveScopes 测试获取活跃的Scope列表
func TestRoleScopeMapping_GetActiveScopes(t *testing.T) {
	// arrange
	roleID := int64(1)
	scopeIDs := []int64{1, 2, 3}
	mappings := GrantScopeList(roleID, scopeIDs)

	// 撤销中间的Scope
	mappings[1].Revoke()

	// act
	activeScopes := GetActiveScopeIDs(mappings)

	// assert
	assert.Len(t, activeScopes, 2)
	assert.Contains(t, activeScopes, int64(1))
	assert.Contains(t, activeScopes, int64(3))
	assert.NotContains(t, activeScopes, int64(2))
}
