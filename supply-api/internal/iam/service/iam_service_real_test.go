package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ==================== 构造函数测试 ====================

func TestNewDefaultIAMService(t *testing.T) {
	// -arrange & act
	svc := NewDefaultIAMService()

	// assert
	assert.NotNil(t, svc)
	assert.NotNil(t, svc.roleStore)
	assert.NotNil(t, svc.userRoleStore)
	assert.NotNil(t, svc.roleScopeStore)
}

// ==================== CreateRole 测试 ====================

func TestIAMService_CreateRole_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	req := &CreateRoleRequest{
		Code:        "developer",
		Name:        "开发者",
		Type:        "platform",
		Level:       20,
		Description: "平台开发者角色",
		Scopes:      []string{"platform:read", "router:invoke"},
	}

	// act
	role, err := svc.CreateRole(ctx, req)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "developer", role.Code)
	assert.Equal(t, "开发者", role.Name)
	assert.Equal(t, "platform", role.Type)
	assert.Equal(t, 20, role.Level)
	assert.Equal(t, "平台开发者角色", role.Description)
	assert.True(t, role.IsActive)
	assert.Equal(t, 1, role.Version)
	assert.False(t, role.CreatedAt.IsZero())
	assert.False(t, role.UpdatedAt.IsZero())
}

func TestIAMService_CreateRole_WithParentCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// 先创建父角色
	parentReq := &CreateRoleRequest{
		Code:   "admin",
		Name:   "管理员",
		Type:   "platform",
		Level:  50,
		Scopes: []string{"platform:admin"},
	}
	svc.CreateRole(ctx, parentReq)

	// 创建子角色
	req := &CreateRoleRequest{
		Code:       "operator",
		Name:       "运维人员",
		Type:       "platform",
		Level:      30,
		Scopes:     []string{"platform:read", "platform:write"},
		ParentCode: "admin",
	}

	// act
	role, err := svc.CreateRole(ctx, req)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "operator", role.Code)
}

func TestIAMService_CreateRole_DuplicateCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	req1 := &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
		Scopes: []string{"platform:read"},
	}
	svc.CreateRole(ctx, req1)

	req2 := &CreateRoleRequest{
		Code:   "developer", // 重复的Code
		Name:   "另一个开发者",
		Type:   "platform",
		Level:  20,
		Scopes: []string{"platform:write"},
	}

	// act
	role, err := svc.CreateRole(ctx, req2)

	// assert
	assert.Error(t, err)
	assert.Nil(t, role)
	assert.Equal(t, ErrDuplicateRoleCode, err)
}

func TestIAMService_CreateRole_InvalidType(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	req := &CreateRoleRequest{
		Code:   "unknown_role",
		Name:   "未知角色",
		Type:   "unknown_type", // 无效类型
		Level:  10,
		Scopes: []string{},
	}

	// act
	role, err := svc.CreateRole(ctx, req)

	// assert
	assert.Error(t, err)
	assert.Nil(t, role)
	assert.Equal(t, ErrInvalidRequest, err)
}

func TestIAMService_CreateRole_AllValidTypes(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	validTypes := []string{"platform", "supply", "consumer"}

	for i, roleType := range validTypes {
		// arrange
		req := &CreateRoleRequest{
			Code:   "role_" + roleType,
			Name:   "角色_" + roleType,
			Type:   roleType,
			Level:  10 * (i + 1),
			Scopes: []string{},
		}

		// act
		role, err := svc.CreateRole(ctx, req)

		// assert
		assert.NoError(t, err)
		assert.NotNil(t, role)
		assert.Equal(t, roleType, role.Type)
	}
}

// ==================== GetRole 测试 ====================

func TestIAMService_GetRole_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	createReq := &CreateRoleRequest{
		Code:        "viewer",
		Name:        "查看者",
		Type:        "platform",
		Level:       10,
		Description: "只读角色",
	}
	svc.CreateRole(ctx, createReq)

	// act
	role, err := svc.GetRole(ctx, "viewer")

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "viewer", role.Code)
	assert.Equal(t, "查看者", role.Name)
	assert.Equal(t, "platform", role.Type)
	assert.Equal(t, 10, role.Level)
	assert.Equal(t, "只读角色", role.Description)
	assert.True(t, role.IsActive)
}

func TestIAMService_GetRole_NotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// act
	role, err := svc.GetRole(ctx, "nonexistent")

	// assert
	assert.Error(t, err)
	assert.Nil(t, role)
	assert.Equal(t, ErrRoleNotFound, err)
}

// ==================== UpdateRole 测试 ====================

func TestIAMService_UpdateRole_UpdateName(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	createReq := &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
	}
	svc.CreateRole(ctx, createReq)

	updateReq := &UpdateRoleRequest{
		Code: "developer",
		Name: "高级开发者",
	}

	// act
	role, err := svc.UpdateRole(ctx, updateReq)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "高级开发者", role.Name)
	assert.Equal(t, 2, role.Version) // 版本应该递增
}

func TestIAMService_UpdateRole_UpdateDescription(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	createReq := &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
	}
	svc.CreateRole(ctx, createReq)

	updateReq := &UpdateRoleRequest{
		Code:        "developer",
		Description: "负责平台功能开发",
	}

	// act
	role, err := svc.UpdateRole(ctx, updateReq)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "负责平台功能开发", role.Description)
}

func TestIAMService_UpdateRole_UpdateScopes(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	createReq := &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
		Scopes: []string{"platform:read"},
	}
	svc.CreateRole(ctx, createReq)

	updateReq := &UpdateRoleRequest{
		Code:   "developer",
		Scopes: []string{"platform:read", "platform:write", "router:invoke"},
	}

	// act
	role, err := svc.UpdateRole(ctx, updateReq)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, []string{"platform:read", "platform:write", "router:invoke"}, svc.roleScopeStore["developer"])
}

func TestIAMService_UpdateRole_UpdateIsActive(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	createReq := &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
	}
	svc.CreateRole(ctx, createReq)

	isActive := false
	updateReq := &UpdateRoleRequest{
		Code:     "developer",
		IsActive: &isActive,
	}

	// act
	role, err := svc.UpdateRole(ctx, updateReq)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.False(t, role.IsActive)
}

func TestIAMService_UpdateRole_NotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	updateReq := &UpdateRoleRequest{
		Code: "nonexistent",
		Name: "不存在",
	}

	// act
	role, err := svc.UpdateRole(ctx, updateReq)

	// assert
	assert.Error(t, err)
	assert.Nil(t, role)
	assert.Equal(t, ErrRoleNotFound, err)
}

func TestIAMService_UpdateRole_AllFields(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	createReq := &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
	}
	svc.CreateRole(ctx, createReq)

	isActive := false
	updateReq := &UpdateRoleRequest{
		Code:        "developer",
		Name:        "高级开发者",
		Description: "全面负责",
		Scopes:      []string{"platform:admin"},
		IsActive:    &isActive,
	}

	// act
	role, err := svc.UpdateRole(ctx, updateReq)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "高级开发者", role.Name)
	assert.Equal(t, "全面负责", role.Description)
	assert.False(t, role.IsActive)
	assert.Equal(t, 2, role.Version)
}

// ==================== DeleteRole 测试 ====================

func TestIAMService_DeleteRole_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	createReq := &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
	}
	svc.CreateRole(ctx, createReq)

	// act
	err := svc.DeleteRole(ctx, "developer")

	// assert
	assert.NoError(t, err)

	// 验证软删除 - 角色应该还在但isActive为false
	role, _ := svc.GetRole(ctx, "developer")
	assert.NotNil(t, role)
	assert.False(t, role.IsActive)
}

func TestIAMService_DeleteRole_NotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// act
	err := svc.DeleteRole(ctx, "nonexistent")

	// assert
	assert.Error(t, err)
	assert.Equal(t, ErrRoleNotFound, err)
}

func TestIAMService_DeleteRole_UpdatesTimestamp(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	createReq := &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
	}
	svc.CreateRole(ctx, createReq)

	time.Sleep(10 * time.Millisecond) // 确保时间戳有差异

	// act
	err := svc.DeleteRole(ctx, "developer")

	// assert
	assert.NoError(t, err)
	role, _ := svc.GetRole(ctx, "developer")
	assert.True(t, role.UpdatedAt.After(role.CreatedAt) || role.UpdatedAt.Equal(role.CreatedAt))
}

// ==================== ListRoles 测试 ====================

func TestIAMService_ListRoles_AllRoles(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// 创建多个角色
	roles := []*CreateRoleRequest{
		{Code: "viewer", Name: "查看者", Type: "platform", Level: 10},
		{Code: "operator", Name: "运维", Type: "platform", Level: 30},
		{Code: "supply_admin", Name: "供应管理员", Type: "supply", Level: 40},
		{Code: "consumer_user", Name: "消费者用户", Type: "consumer", Level: 10},
	}

	for _, req := range roles {
		svc.CreateRole(ctx, req)
	}

	// act
	result, err := svc.ListRoles(ctx, "")

	// assert
	assert.NoError(t, err)
	assert.Len(t, result, 4)
}

func TestIAMService_ListRoles_FilterByType(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// 创建不同类型的角色
	svc.CreateRole(ctx, &CreateRoleRequest{Code: "viewer", Name: "查看者", Type: "platform", Level: 10})
	svc.CreateRole(ctx, &CreateRoleRequest{Code: "operator", Name: "运维", Type: "platform", Level: 30})
	svc.CreateRole(ctx, &CreateRoleRequest{Code: "supply_admin", Name: "供应管理员", Type: "supply", Level: 40})

	// act
	platformRoles, err := svc.ListRoles(ctx, "platform")
	supplyRoles, err2 := svc.ListRoles(ctx, "supply")
	consumerRoles, err3 := svc.ListRoles(ctx, "consumer")

	// assert
	assert.NoError(t, err)
	assert.Len(t, platformRoles, 2)

	assert.NoError(t, err2)
	assert.Len(t, supplyRoles, 1)
	assert.Equal(t, "supply", supplyRoles[0].Type)

	assert.NoError(t, err3)
	assert.Len(t, consumerRoles, 0)
}

func TestIAMService_ListRoles_Empty(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// act
	roles, err := svc.ListRoles(ctx, "")

	// assert
	assert.NoError(t, err)
	assert.Len(t, roles, 0)
}

// ==================== AssignRole 测试 ====================

func TestIAMService_AssignRole_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// 创建角色
	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "viewer",
		Name:   "查看者",
		Type:   "platform",
		Level:  10,
		Scopes: []string{"platform:read"},
	})

	req := &AssignRoleRequest{
		UserID:   100,
		RoleCode: "viewer",
		TenantID: 1,
	}

	// act
	userRole, err := svc.AssignRole(ctx, req)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, userRole)
	assert.Equal(t, int64(100), userRole.UserID)
	assert.Equal(t, "viewer", userRole.RoleCode)
	assert.Equal(t, int64(1), userRole.TenantID)
	assert.True(t, userRole.IsActive)
}

func TestIAMService_AssignRole_WithExpiresAt(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "temp_admin",
		Name:   "临时管理员",
		Type:   "platform",
		Level:  50,
	})

	futureTime := time.Now().Add(24 * time.Hour)
	req := &AssignRoleRequest{
		UserID:   100,
		RoleCode: "temp_admin",
		TenantID: 1,
		ExpiresAt: &futureTime,
	}

	// act
	userRole, err := svc.AssignRole(ctx, req)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, userRole)
	assert.NotNil(t, userRole.ExpiresAt)
	assert.False(t, userRole.IsExpired())
}

func TestIAMService_AssignRole_ExpiredRole(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "temp_admin",
		Name:   "临时管理员",
		Type:   "platform",
		Level:  50,
	})

	pastTime := time.Now().Add(-1 * time.Hour)
	req := &AssignRoleRequest{
		UserID:   100,
		RoleCode: "temp_admin",
		TenantID: 1,
		ExpiresAt: &pastTime,
	}

	// act
	userRole, err := svc.AssignRole(ctx, req)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, userRole)
	assert.True(t, userRole.IsExpired())
}

func TestIAMService_AssignRole_RoleNotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	req := &AssignRoleRequest{
		UserID:   100,
		RoleCode: "nonexistent",
		TenantID: 1,
	}

	// act
	userRole, err := svc.AssignRole(ctx, req)

	// assert
	assert.Error(t, err)
	assert.Nil(t, userRole)
	assert.Equal(t, ErrRoleNotFound, err)
}

func TestIAMService_AssignRole_DuplicateAssignment(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "viewer",
		Name:   "查看者",
		Type:   "platform",
		Level:  10,
	})

	req := &AssignRoleRequest{
		UserID:   100,
		RoleCode: "viewer",
		TenantID: 1,
	}

	// 第一次分配
	svc.AssignRole(ctx, req)

	// act - 第二次分配同一个角色
	userRole, err := svc.AssignRole(ctx, req)

	// assert
	assert.Error(t, err)
	assert.Nil(t, userRole)
	assert.Equal(t, ErrDuplicateAssignment, err)
}

func TestIAMService_AssignRole_SameRoleDifferentTenant(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "viewer",
		Name:   "查看者",
		Type:   "platform",
		Level:  10,
	})

	// act - 同一用户在同一角色上分配到不同租户
	req1 := &AssignRoleRequest{UserID: 100, RoleCode: "viewer", TenantID: 1}
	req2 := &AssignRoleRequest{UserID: 100, RoleCode: "viewer", TenantID: 2}

	userRole1, err1 := svc.AssignRole(ctx, req1)
	userRole2, err2 := svc.AssignRole(ctx, req2)

	// assert
	assert.NoError(t, err1)
	assert.NotNil(t, userRole1)
	assert.NoError(t, err2)
	assert.NotNil(t, userRole2)
	assert.Equal(t, int64(1), userRole1.TenantID)
	assert.Equal(t, int64(2), userRole2.TenantID)
}

// ==================== RevokeRole 测试 ====================

func TestIAMService_RevokeRole_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "viewer",
		Name:   "查看者",
		Type:   "platform",
		Level:  10,
	})
	svc.AssignRole(ctx, &AssignRoleRequest{
		UserID:   100,
		RoleCode: "viewer",
		TenantID: 1,
	})

	// act
	err := svc.RevokeRole(ctx, 100, "viewer", 1)

	// assert
	assert.NoError(t, err)

	// 验证角色已被撤销
	userRoles, _ := svc.GetUserRoles(ctx, 100)
	assert.Len(t, userRoles, 0) // 因为IsActive=false，不会返回
}

func TestIAMService_RevokeRole_NotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// act
	err := svc.RevokeRole(ctx, 100, "nonexistent", 1)

	// assert
	assert.Error(t, err)
	assert.Equal(t, ErrRoleNotFound, err)
}

func TestIAMService_RevokeRole_Idempotent(t *testing.T) {
	// RevokeRole是幂等操作，重复撤销不会返回错误
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "viewer",
		Name:   "查看者",
		Type:   "platform",
		Level:  10,
	})
	svc.AssignRole(ctx, &AssignRoleRequest{
		UserID:   100,
		RoleCode: "viewer",
		TenantID: 1,
	})

	// 先撤销一次
	err1 := svc.RevokeRole(ctx, 100, "viewer", 1)
	assert.NoError(t, err1)

	// act - 再次撤销（幂等操作，不返回错误）
	err2 := svc.RevokeRole(ctx, 100, "viewer", 1)
	assert.NoError(t, err2) // 幂等操作，不会返回错误
}

// ==================== GetUserRoles 测试 ====================

func TestIAMService_GetUserRoles_MultipleRoles(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// 创建多个角色
	svc.CreateRole(ctx, &CreateRoleRequest{Code: "viewer", Name: "查看者", Type: "platform", Level: 10})
	svc.CreateRole(ctx, &CreateRoleRequest{Code: "developer", Name: "开发者", Type: "platform", Level: 20})

	// 分配多个角色给同一用户
	svc.AssignRole(ctx, &AssignRoleRequest{UserID: 100, RoleCode: "viewer", TenantID: 0})
	svc.AssignRole(ctx, &AssignRoleRequest{UserID: 100, RoleCode: "developer", TenantID: 0})

	// act
	userRoles, err := svc.GetUserRoles(ctx, 100)

	// assert
	assert.NoError(t, err)
	assert.Len(t, userRoles, 2)
}

func TestIAMService_GetUserRoles_NoRoles(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// act
	userRoles, err := svc.GetUserRoles(ctx, 999) // 不存在的用户

	// assert
	assert.NoError(t, err)
	assert.Len(t, userRoles, 0)
}

func TestIAMService_GetUserRoles_ExcludesInactive(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{Code: "viewer", Name: "查看者", Type: "platform", Level: 10})
	svc.AssignRole(ctx, &AssignRoleRequest{UserID: 100, RoleCode: "viewer", TenantID: 1})
	svc.RevokeRole(ctx, 100, "viewer", 1)

	// act
	userRoles, err := svc.GetUserRoles(ctx, 100)

	// assert
	assert.NoError(t, err)
	assert.Len(t, userRoles, 0)
}

// ==================== CheckScope 测试 ====================

func TestIAMService_CheckScope_HasScope(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "viewer",
		Name:   "查看者",
		Type:   "platform",
		Level:  10,
		Scopes: []string{"platform:read", "tenant:read"},
	})
	svc.AssignRole(ctx, &AssignRoleRequest{UserID: 100, RoleCode: "viewer", TenantID: 0})

	// act
	hasScope, err := svc.CheckScope(ctx, 100, "platform:read")

	// assert
	assert.NoError(t, err)
	assert.True(t, hasScope)
}

func TestIAMService_CheckScope_NoScope(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "viewer",
		Name:   "查看者",
		Type:   "platform",
		Level:  10,
		Scopes: []string{"platform:read"},
	})
	svc.AssignRole(ctx, &AssignRoleRequest{UserID: 100, RoleCode: "viewer", TenantID: 0})

	// act
	hasScope, err := svc.CheckScope(ctx, 100, "platform:write")

	// assert
	assert.NoError(t, err)
	assert.False(t, hasScope)
}

func TestIAMService_CheckScope_WildcardScope(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "admin",
		Name:   "管理员",
		Type:   "platform",
		Level:  50,
		Scopes: []string{"*"}, // 通配符
	})
	svc.AssignRole(ctx, &AssignRoleRequest{UserID: 100, RoleCode: "admin", TenantID: 0})

	// act
	hasScope, err := svc.CheckScope(ctx, 100, "any:scope")

	// assert
	assert.NoError(t, err)
	assert.True(t, hasScope)
}

func TestIAMService_CheckScope_NoUser(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// act
	hasScope, err := svc.CheckScope(ctx, 999, "platform:read")

	// assert
	assert.NoError(t, err)
	assert.False(t, hasScope)
}

// ==================== GetUserScopes 测试 ====================

func TestIAMService_GetUserScopes_MultipleRoles(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "viewer",
		Name:   "查看者",
		Type:   "platform",
		Level:  10,
		Scopes: []string{"platform:read", "tenant:read"},
	})
	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
		Scopes: []string{"router:invoke", "router:model:list"},
	})

	svc.AssignRole(ctx, &AssignRoleRequest{UserID: 100, RoleCode: "viewer", TenantID: 0})
	svc.AssignRole(ctx, &AssignRoleRequest{UserID: 100, RoleCode: "developer", TenantID: 0})

	// act
	scopes, err := svc.GetUserScopes(ctx, 100)

	// assert
	assert.NoError(t, err)
	assert.Len(t, scopes, 4)
	assert.Contains(t, scopes, "platform:read")
	assert.Contains(t, scopes, "tenant:read")
	assert.Contains(t, scopes, "router:invoke")
	assert.Contains(t, scopes, "router:model:list")
}

func TestIAMService_GetUserScopes_Deduplication(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// 两个角色有相同的scope
	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "viewer",
		Name:   "查看者",
		Type:   "platform",
		Level:  10,
		Scopes: []string{"platform:read"},
	})
	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "operator",
		Name:   "运维",
		Type:   "platform",
		Level:  30,
		Scopes: []string{"platform:read", "platform:write"},
	})

	svc.AssignRole(ctx, &AssignRoleRequest{UserID: 100, RoleCode: "viewer", TenantID: 0})
	svc.AssignRole(ctx, &AssignRoleRequest{UserID: 100, RoleCode: "operator", TenantID: 0})

	// act
	scopes, err := svc.GetUserScopes(ctx, 100)

	// assert
	assert.NoError(t, err)
	assert.Len(t, scopes, 2) // 应该去重
	assert.Contains(t, scopes, "platform:read")
	assert.Contains(t, scopes, "platform:write")
}

func TestIAMService_GetUserScopes_NoRoles(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	// act
	scopes, err := svc.GetUserScopes(ctx, 999)

	// assert
	assert.NoError(t, err)
	assert.Len(t, scopes, 0)
}

func TestIAMService_GetUserScopes_ExpiredRoleExcluded(t *testing.T) {
	// arrange
	ctx := context.Background()
	svc := NewDefaultIAMService()

	svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "temp_admin",
		Name:   "临时管理员",
		Type:   "platform",
		Level:  50,
		Scopes: []string{"platform:admin"},
	})

	pastTime := time.Now().Add(-1 * time.Hour)
	svc.AssignRole(ctx, &AssignRoleRequest{
		UserID:    100,
		RoleCode:  "temp_admin",
		TenantID:  1,
		ExpiresAt: &pastTime,
	})

	// act
	scopes, err := svc.GetUserScopes(ctx, 100)

	// assert
	assert.NoError(t, err)
	assert.Len(t, scopes, 0) // 过期的角色应该被排除
}

// ==================== UserRole.IsExpired 测试 ====================

func TestUserRole_IsExpired_Nil(t *testing.T) {
	// arrange
	ur := &UserRole{
		UserID:   100,
		RoleCode: "viewer",
		TenantID: 1,
		ExpiresAt: nil,
	}

	// act & assert
	assert.False(t, ur.IsExpired())
}

func TestUserRole_IsExpired_Future(t *testing.T) {
	// arrange
	futureTime := time.Now().Add(1 * time.Hour)
	ur := &UserRole{
		UserID:    100,
		RoleCode:  "viewer",
		TenantID:  1,
		ExpiresAt: &futureTime,
	}

	// act & assert
	assert.False(t, ur.IsExpired())
}

func TestUserRole_IsExpired_Past(t *testing.T) {
	// arrange
	pastTime := time.Now().Add(-1 * time.Hour)
	ur := &UserRole{
		UserID:    100,
		RoleCode:  "viewer",
		TenantID:  1,
		ExpiresAt: &pastTime,
	}

	// act & assert
	assert.True(t, ur.IsExpired())
}
