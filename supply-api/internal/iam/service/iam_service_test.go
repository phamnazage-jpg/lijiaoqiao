package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockIAMService 模拟IAM服务（用于测试）
type MockIAMService struct {
	roles      map[string]*Role
	userRoles  map[int64][]*UserRole
	roleScopes map[string][]string
}

func NewMockIAMService() *MockIAMService {
	return &MockIAMService{
		roles:      make(map[string]*Role),
		userRoles:  make(map[int64][]*UserRole),
		roleScopes: make(map[string][]string),
	}
}

func (m *MockIAMService) CreateRole(ctx context.Context, req *CreateRoleRequest) (*Role, error) {
	if _, exists := m.roles[req.Code]; exists {
		return nil, ErrDuplicateRoleCode
	}
	role := &Role{
		Code:   req.Code,
		Name:   req.Name,
		Type:   req.Type,
		Level:  req.Level,
		IsActive: true,
		Version: 1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.roles[req.Code] = role
	if len(req.Scopes) > 0 {
		m.roleScopes[req.Code] = req.Scopes
	}
	return role, nil
}

func (m *MockIAMService) GetRole(ctx context.Context, roleCode string) (*Role, error) {
	if role, exists := m.roles[roleCode]; exists {
		return role, nil
	}
	return nil, ErrRoleNotFound
}

func (m *MockIAMService) UpdateRole(ctx context.Context, req *UpdateRoleRequest) (*Role, error) {
	role, exists := m.roles[req.Code]
	if !exists {
		return nil, ErrRoleNotFound
	}
	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Description != "" {
		role.Description = req.Description
	}
	if req.Scopes != nil {
		m.roleScopes[req.Code] = req.Scopes
	}
	role.Version++
	role.UpdatedAt = time.Now()
	return role, nil
}

func (m *MockIAMService) DeleteRole(ctx context.Context, roleCode string) error {
	role, exists := m.roles[roleCode]
	if !exists {
		return ErrRoleNotFound
	}
	role.IsActive = false
	role.UpdatedAt = time.Now()
	return nil
}

func (m *MockIAMService) ListRoles(ctx context.Context, roleType string) ([]*Role, error) {
	var roles []*Role
	for _, role := range m.roles {
		if roleType == "" || role.Type == roleType {
			roles = append(roles, role)
		}
	}
	return roles, nil
}

func (m *MockIAMService) AssignRole(ctx context.Context, req *AssignRoleRequest) (*modelUserRoleMapping, error) {
	for _, ur := range m.userRoles[req.UserID] {
		if ur.RoleCode == req.RoleCode && ur.TenantID == req.TenantID && ur.IsActive {
			return nil, ErrDuplicateAssignment
		}
	}
	mapping := &modelUserRoleMapping{
		UserID:   req.UserID,
		RoleCode: req.RoleCode,
		TenantID: req.TenantID,
		IsActive: true,
	}
	m.userRoles[req.UserID] = append(m.userRoles[req.UserID], &UserRole{
		UserID:   req.UserID,
		RoleCode: req.RoleCode,
		TenantID: req.TenantID,
		IsActive: true,
	})
	return mapping, nil
}

func (m *MockIAMService) RevokeRole(ctx context.Context, userID int64, roleCode string, tenantID int64) error {
	for _, ur := range m.userRoles[userID] {
		if ur.RoleCode == roleCode && ur.TenantID == tenantID {
			ur.IsActive = false
			return nil
		}
	}
	return ErrRoleNotFound
}

func (m *MockIAMService) GetUserRoles(ctx context.Context, userID int64) ([]*UserRole, error) {
	var userRoles []*UserRole
	for _, ur := range m.userRoles[userID] {
		if ur.IsActive {
			userRoles = append(userRoles, ur)
		}
	}
	return userRoles, nil
}

func (m *MockIAMService) CheckScope(ctx context.Context, userID int64, requiredScope string) (bool, error) {
	scopes, err := m.GetUserScopes(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, scope := range scopes {
		if scope == requiredScope || scope == "*" {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockIAMService) GetUserScopes(ctx context.Context, userID int64) ([]string, error) {
	var allScopes []string
	seen := make(map[string]bool)
	for _, ur := range m.userRoles[userID] {
		if ur.IsActive {
			if scopes, exists := m.roleScopes[ur.RoleCode]; exists {
				for _, scope := range scopes {
					if !seen[scope] {
						seen[scope] = true
						allScopes = append(allScopes, scope)
					}
				}
			}
		}
	}
	return allScopes, nil
}

// modelUserRoleMapping 简化的用户角色映射（用于测试）
type modelUserRoleMapping struct {
	UserID   int64
	RoleCode string
	TenantID int64
	IsActive bool
}

// TestIAMService_CreateRole_Success 测试创建角色成功
func TestIAMService_CreateRole_Success(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()
	req := &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
		Scopes: []string{"platform:read", "router:invoke"},
	}

	// act
	role, err := mockService.CreateRole(context.Background(), req)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "developer", role.Code)
	assert.Equal(t, "开发者", role.Name)
	assert.Equal(t, "platform", role.Type)
	assert.Equal(t, 20, role.Level)
	assert.True(t, role.IsActive)
}

// TestIAMService_CreateRole_DuplicateName 测试创建重复角色
func TestIAMService_CreateRole_DuplicateName(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()
	mockService.roles["developer"] = &Role{Code: "developer", Name: "开发者", Type: "platform", Level: 20}

	req := &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
	}

	// act
	role, err := mockService.CreateRole(context.Background(), req)

	// assert
	assert.Error(t, err)
	assert.Nil(t, role)
	assert.Equal(t, ErrDuplicateRoleCode, err)
}

// TestIAMService_UpdateRole_Success 测试更新角色成功
func TestIAMService_UpdateRole_Success(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()
	existingRole := &Role{
		Code:    "developer",
		Name:    "开发者",
		Type:    "platform",
		Level:   20,
		IsActive: true,
		Version: 1,
	}
	mockService.roles["developer"] = existingRole

	req := &UpdateRoleRequest{
		Code:        "developer",
		Name:        "AI开发者",
		Description: "AI应用开发者",
	}

	// act
	updatedRole, err := mockService.UpdateRole(context.Background(), req)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, updatedRole)
	assert.Equal(t, "AI开发者", updatedRole.Name)
	assert.Equal(t, "AI应用开发者", updatedRole.Description)
	assert.Equal(t, 2, updatedRole.Version) // version 应该递增
}

// TestIAMService_UpdateRole_NotFound 测试更新不存在的角色
func TestIAMService_UpdateRole_NotFound(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()

	req := &UpdateRoleRequest{
		Code: "nonexistent",
		Name: "不存在",
	}

	// act
	role, err := mockService.UpdateRole(context.Background(), req)

	// assert
	assert.Error(t, err)
	assert.Nil(t, role)
	assert.Equal(t, ErrRoleNotFound, err)
}

// TestIAMService_DeleteRole_Success 测试删除角色成功
func TestIAMService_DeleteRole_Success(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()
	mockService.roles["developer"] = &Role{Code: "developer", Name: "开发者", IsActive: true}

	// act
	err := mockService.DeleteRole(context.Background(), "developer")

	// assert
	assert.NoError(t, err)
	assert.False(t, mockService.roles["developer"].IsActive) // 应该被停用而不是删除
}

// TestIAMService_ListRoles 测试列出角色
func TestIAMService_ListRoles(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()
	mockService.roles["viewer"] = &Role{Code: "viewer", Type: "platform", Level: 10}
	mockService.roles["operator"] = &Role{Code: "operator", Type: "platform", Level: 30}
	mockService.roles["supply_admin"] = &Role{Code: "supply_admin", Type: "supply", Level: 40}

	// act
	platformRoles, err := mockService.ListRoles(context.Background(), "platform")
	supplyRoles, err2 := mockService.ListRoles(context.Background(), "supply")
	allRoles, err3 := mockService.ListRoles(context.Background(), "")

	// assert
	assert.NoError(t, err)
	assert.Len(t, platformRoles, 2)

	assert.NoError(t, err2)
	assert.Len(t, supplyRoles, 1)

	assert.NoError(t, err3)
	assert.Len(t, allRoles, 3)
}

// TestIAMService_AssignRole 测试分配角色
func TestIAMService_AssignRole(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()
	mockService.roles["viewer"] = &Role{Code: "viewer", Type: "platform", Level: 10}

	req := &AssignRoleRequest{
		UserID:   100,
		RoleCode: "viewer",
		TenantID: 1,
	}

	// act
	mapping, err := mockService.AssignRole(context.Background(), req)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, mapping)
	assert.Equal(t, int64(100), mapping.UserID)
	assert.Equal(t, "viewer", mapping.RoleCode)
	assert.True(t, mapping.IsActive)
}

// TestIAMService_AssignRole_Duplicate 测试重复分配角色
func TestIAMService_AssignRole_Duplicate(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()
	mockService.roles["viewer"] = &Role{Code: "viewer", Type: "platform", Level: 10}
	mockService.userRoles[100] = []*UserRole{
		{UserID: 100, RoleCode: "viewer", TenantID: 1, IsActive: true},
	}

	req := &AssignRoleRequest{
		UserID:   100,
		RoleCode: "viewer",
		TenantID: 1,
	}

	// act
	mapping, err := mockService.AssignRole(context.Background(), req)

	// assert
	assert.Error(t, err)
	assert.Nil(t, mapping)
	assert.Equal(t, ErrDuplicateAssignment, err)
}

// TestIAMService_RevokeRole 测试撤销角色
func TestIAMService_RevokeRole(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()
	mockService.userRoles[100] = []*UserRole{
		{UserID: 100, RoleCode: "viewer", TenantID: 1, IsActive: true},
	}

	// act
	err := mockService.RevokeRole(context.Background(), 100, "viewer", 1)

	// assert
	assert.NoError(t, err)
	assert.False(t, mockService.userRoles[100][0].IsActive)
}

// TestIAMService_GetUserRoles 测试获取用户角色
func TestIAMService_GetUserRoles(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()
	mockService.userRoles[100] = []*UserRole{
		{UserID: 100, RoleCode: "viewer", TenantID: 0, IsActive: true},
		{UserID: 100, RoleCode: "developer", TenantID: 1, IsActive: true},
	}

	// act
	roles, err := mockService.GetUserRoles(context.Background(), 100)

	// assert
	assert.NoError(t, err)
	assert.Len(t, roles, 2)
}

// TestIAMService_CheckScope 测试检查用户Scope
func TestIAMService_CheckScope(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()
	mockService.roles["viewer"] = &Role{Code: "viewer", Type: "platform", Level: 10}
	mockService.roleScopes["viewer"] = []string{"platform:read", "tenant:read"}
	mockService.userRoles[100] = []*UserRole{
		{UserID: 100, RoleCode: "viewer", TenantID: 0, IsActive: true},
	}

	// act
	hasScope, err := mockService.CheckScope(context.Background(), 100, "platform:read")
	noScope, err2 := mockService.CheckScope(context.Background(), 100, "platform:write")

	// assert
	assert.NoError(t, err)
	assert.True(t, hasScope)

	assert.NoError(t, err2)
	assert.False(t, noScope)
}

// TestIAMService_GetUserScopes 测试获取用户所有Scope
func TestIAMService_GetUserScopes(t *testing.T) {
	// arrange
	mockService := NewMockIAMService()
	mockService.roles["viewer"] = &Role{Code: "viewer", Type: "platform", Level: 10}
	mockService.roles["developer"] = &Role{Code: "developer", Type: "platform", Level: 20}
	mockService.roleScopes["viewer"] = []string{"platform:read", "tenant:read"}
	mockService.roleScopes["developer"] = []string{"router:invoke", "router:model:list"}
	mockService.userRoles[100] = []*UserRole{
		{UserID: 100, RoleCode: "viewer", TenantID: 0, IsActive: true},
		{UserID: 100, RoleCode: "developer", TenantID: 0, IsActive: true},
	}

	// act
	scopes, err := mockService.GetUserScopes(context.Background(), 100)

	// assert
	assert.NoError(t, err)
	assert.Contains(t, scopes, "platform:read")
	assert.Contains(t, scopes, "tenant:read")
	assert.Contains(t, scopes, "router:invoke")
	assert.Contains(t, scopes, "router:model:list")
}
