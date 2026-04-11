package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"lijiaoqiao/supply-api/internal/iam/model"
	"lijiaoqiao/supply-api/internal/iam/repository"
)

// MockIAMRepository 是 IAMRepository 的 mock 实现
type MockIAMRepository struct {
	roles       map[string]*model.Role
	roleByID    map[int64]*model.Role
	scopes      map[string]*model.Scope
	userRoles   []*model.UserRoleMapping
	roleScopes  map[string][]string // roleCode -> scopeCodes
	nextRoleID  int64
	nextScopeID int64
}

func NewMockIAMRepository() *MockIAMRepository {
	return &MockIAMRepository{
		roles:      make(map[string]*model.Role),
		roleByID:   make(map[int64]*model.Role),
		scopes:     make(map[string]*model.Scope),
		userRoles:  make([]*model.UserRoleMapping, 0),
		roleScopes: make(map[string][]string),
		nextRoleID: 1,
		nextScopeID: 1,
	}
}

func (m *MockIAMRepository) CreateRole(ctx context.Context, role *model.Role) error {
	if _, exists := m.roles[role.Code]; exists {
		return repository.ErrDuplicateRoleCode
	}
	role.ID = m.nextRoleID
	m.nextRoleID++
	m.roles[role.Code] = role
	m.roleByID[role.ID] = role
	return nil
}

func (m *MockIAMRepository) GetRoleByCode(ctx context.Context, code string) (*model.Role, error) {
	if role, ok := m.roles[code]; ok {
		return role, nil
	}
	return nil, errors.New("role not found")
}

func (m *MockIAMRepository) UpdateRole(ctx context.Context, role *model.Role) error {
	if _, ok := m.roles[role.Code]; !ok {
		return errors.New("role not found")
	}
	m.roles[role.Code] = role
	return nil
}

func (m *MockIAMRepository) DeleteRole(ctx context.Context, code string) error {
	if _, ok := m.roles[code]; !ok {
		return errors.New("role not found")
	}
	delete(m.roles, code)
	return nil
}

func (m *MockIAMRepository) ListRoles(ctx context.Context, roleType string) ([]*model.Role, error) {
	var result []*model.Role
	for _, role := range m.roles {
		if roleType == "" || role.Type == roleType {
			result = append(result, role)
		}
	}
	return result, nil
}

func (m *MockIAMRepository) CreateScope(ctx context.Context, scope *model.Scope) error {
	scope.ID = m.nextScopeID
	m.nextScopeID++
	m.scopes[scope.Code] = scope
	return nil
}

func (m *MockIAMRepository) GetScopeByCode(ctx context.Context, code string) (*model.Scope, error) {
	if scope, ok := m.scopes[code]; ok {
		return scope, nil
	}
	return nil, errors.New("scope not found")
}

func (m *MockIAMRepository) ListScopes(ctx context.Context) ([]*model.Scope, error) {
	var result []*model.Scope
	for _, scope := range m.scopes {
		result = append(result, scope)
	}
	return result, nil
}

func (m *MockIAMRepository) AddScopeToRole(ctx context.Context, roleCode, scopeCode string) error {
	if _, ok := m.roles[roleCode]; !ok {
		return errors.New("role not found")
	}
	if _, ok := m.scopes[scopeCode]; !ok {
		return repository.ErrScopeNotFound
	}
	m.roleScopes[roleCode] = append(m.roleScopes[roleCode], scopeCode)
	return nil
}

func (m *MockIAMRepository) RemoveScopeFromRole(ctx context.Context, roleCode, scopeCode string) error {
	if _, ok := m.roles[roleCode]; !ok {
		return errors.New("role not found")
	}
	scopes := m.roleScopes[roleCode]
	for i, s := range scopes {
		if s == scopeCode {
			m.roleScopes[roleCode] = append(scopes[:i], scopes[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockIAMRepository) GetScopesByRoleCode(ctx context.Context, roleCode string) ([]string, error) {
	return m.roleScopes[roleCode], nil
}

func (m *MockIAMRepository) AssignRole(ctx context.Context, userRole *model.UserRoleMapping) error {
	m.userRoles = append(m.userRoles, userRole)
	return nil
}

func (m *MockIAMRepository) RevokeRole(ctx context.Context, userID int64, roleCode string, tenantID int64) error {
	role := m.roles[roleCode]
	if role == nil {
		return errors.New("role not found")
	}
	for i, ur := range m.userRoles {
		if ur.UserID == userID && ur.RoleID == role.ID && ur.TenantID == tenantID {
			m.userRoles = append(m.userRoles[:i], m.userRoles[i+1:]...)
			return nil
		}
	}
	return errors.New("user role not found")
}

func (m *MockIAMRepository) GetUserRoles(ctx context.Context, userID int64) ([]*model.UserRoleMapping, error) {
	var result []*model.UserRoleMapping
	for _, ur := range m.userRoles {
		if ur.UserID == userID {
			result = append(result, ur)
		}
	}
	return result, nil
}

func (m *MockIAMRepository) GetUserRolesWithCode(ctx context.Context, userID int64) ([]*repository.UserRoleWithCode, error) {
	var result []*repository.UserRoleWithCode
	for _, ur := range m.userRoles {
		if ur.UserID == userID {
			role := m.roleByID[ur.RoleID]
			if role != nil {
				result = append(result, &repository.UserRoleWithCode{
					UserRoleMapping: ur,
					RoleCode:        role.Code,
				})
			}
		}
	}
	return result, nil
}

func (m *MockIAMRepository) GetUserScopes(ctx context.Context, userID int64) ([]string, error) {
	var result []string
	for _, ur := range m.userRoles {
		if ur.UserID == userID {
			role := m.roleByID[ur.RoleID]
			if role != nil {
				if scopes, ok := m.roleScopes[role.Code]; ok {
					result = append(result, scopes...)
				}
			}
		}
	}
	return result, nil
}

// 辅助函数：添加 scope 到 mock
func (m *MockIAMRepository) AddScope(code, name, scopeType string) {
	m.scopes[code] = &model.Scope{
		Code:  code,
		Name:  name,
		Type:  scopeType,
	}
}

// 辅助函数：添加角色到 mock
func (m *MockIAMRepository) AddRole(code, name, roleType string, level int) {
	role := &model.Role{
		Code:  code,
		Name:  name,
		Type:  roleType,
		Level: level,
	}
	role.ID = m.nextRoleID
	m.nextRoleID++
	m.roles[code] = role
	m.roleByID[role.ID] = role
}

// ============ DatabaseIAMService 测试 ============

func TestNewDatabaseIAMService(t *testing.T) {
	repo := NewMockIAMRepository()
	svc := NewDatabaseIAMService(repo)
	assert.NotNil(t, svc)
	assert.Equal(t, repo, svc.repo)
}

func TestDatabaseIAMService_CreateRole_Success(t *testing.T) {
	repo := NewMockIAMRepository()
	repo.AddScope("read", "读取", "platform")
	repo.AddScope("write", "写入", "platform")

	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	req := &CreateRoleRequest{
		Code:        "developer",
		Name:        "开发者",
		Type:        "platform",
		Level:       20,
		Description: "平台开发者角色",
		Scopes:      []string{"read", "write"},
	}

	role, err := svc.CreateRole(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "developer", role.Code)
	assert.Equal(t, "开发者", role.Name)
	assert.Equal(t, "platform", role.Type)
	assert.Equal(t, 20, role.Level)
}

func TestDatabaseIAMService_CreateRole_InvalidType(t *testing.T) {
	repo := NewMockIAMRepository()
	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	req := &CreateRoleRequest{
		Code: "developer",
		Name: "开发者",
		Type: "invalid_type", // 无效类型
		Level: 20,
	}

	role, err := svc.CreateRole(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, role)
	assert.Equal(t, ErrInvalidRequest, err)
}

func TestDatabaseIAMService_CreateRole_DuplicateCode(t *testing.T) {
	repo := NewMockIAMRepository()
	repo.AddRole("developer", "开发者", "platform", 20)

	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	req := &CreateRoleRequest{
		Code: "developer", // 已存在
		Name: "另一个开发者",
		Type: "platform",
		Level: 20,
	}

	role, err := svc.CreateRole(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, role)
	assert.Equal(t, ErrDuplicateRoleCode, err)
}

func TestDatabaseIAMService_GetRole_Success(t *testing.T) {
	repo := NewMockIAMRepository()
	repo.AddRole("developer", "开发者", "platform", 20)

	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	role, err := svc.GetRole(ctx, "developer")

	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "developer", role.Code)
}

func TestDatabaseIAMService_GetRole_NotFound(t *testing.T) {
	repo := NewMockIAMRepository()
	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	role, err := svc.GetRole(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, role)
}

func TestDatabaseIAMService_UpdateRole_Success(t *testing.T) {
	repo := NewMockIAMRepository()
	repo.AddRole("developer", "开发者", "platform", 20)

	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	req := &UpdateRoleRequest{
		Code:        "developer",
		Name:        "高级开发者",
		Description: "更新后的描述",
	}

	role, err := svc.UpdateRole(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, role)
	assert.Equal(t, "高级开发者", role.Name)
}

func TestDatabaseIAMService_DeleteRole_Success(t *testing.T) {
	repo := NewMockIAMRepository()
	repo.AddRole("developer", "开发者", "platform", 20)

	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	err := svc.DeleteRole(ctx, "developer")

	assert.NoError(t, err)
}

func TestDatabaseIAMService_ListRoles_Success(t *testing.T) {
	repo := NewMockIAMRepository()
	repo.AddRole("admin", "管理员", "platform", 10)
	repo.AddRole("developer", "开发者", "platform", 20)
	repo.AddRole("user", "普通用户", "consumer", 30)

	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	// 列出所有角色
	roles, err := svc.ListRoles(ctx, "")
	assert.NoError(t, err)
	assert.Len(t, roles, 3)

	// 只列出 platform 类型
	roles, err = svc.ListRoles(ctx, "platform")
	assert.NoError(t, err)
	assert.Len(t, roles, 2)
}

func TestDatabaseIAMService_AssignRole_Success(t *testing.T) {
	repo := NewMockIAMRepository()
	repo.AddRole("developer", "开发者", "platform", 20)

	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	req := &AssignRoleRequest{
		UserID:   1001,
		RoleCode: "developer",
		TenantID: 1,
	}

	userRole, err := svc.AssignRole(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, userRole)
	assert.Equal(t, "developer", userRole.RoleCode)
}

func TestDatabaseIAMService_RevokeRole_Success(t *testing.T) {
	repo := NewMockIAMRepository()
	repo.AddRole("developer", "开发者", "platform", 20)

	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	// 先分配角色
	assignReq := &AssignRoleRequest{
		UserID:   1001,
		RoleCode: "developer",
		TenantID: 1,
	}
	_, err := svc.AssignRole(ctx, assignReq)
	assert.NoError(t, err)

	// 然后撤销
	err = svc.RevokeRole(ctx, 1001, "developer", 1)
	assert.NoError(t, err)
}

func TestDatabaseIAMService_GetUserRoles_Success(t *testing.T) {
	repo := NewMockIAMRepository()
	repo.AddRole("developer", "开发者", "platform", 20)
	repo.AddRole("admin", "管理员", "platform", 10)

	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	// 分配两个角色
	_, _ = svc.AssignRole(ctx, &AssignRoleRequest{UserID: 1001, RoleCode: "developer", TenantID: 1})
	_, _ = svc.AssignRole(ctx, &AssignRoleRequest{UserID: 1001, RoleCode: "admin", TenantID: 1})

	roles, err := svc.GetUserRoles(ctx, 1001)

	assert.NoError(t, err)
	assert.Len(t, roles, 2)
}

func TestDatabaseIAMService_CheckScope_Success(t *testing.T) {
	repo := NewMockIAMRepository()
	repo.AddScope("read", "读取", "platform")
	repo.AddScope("write", "写入", "platform")

	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	// 先创建角色（会分配 scopes）
	_, _ = svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
		Scopes: []string{"read", "write"},
	})

	// 分配角色
	_, _ = svc.AssignRole(ctx, &AssignRoleRequest{UserID: 1001, RoleCode: "developer", TenantID: 1})

	// 检查权限
	hasScope, err := svc.CheckScope(ctx, 1001, "read")
	assert.NoError(t, err)
	assert.True(t, hasScope)

	hasScope, err = svc.CheckScope(ctx, 1001, "write")
	assert.NoError(t, err)
	assert.True(t, hasScope)

	hasScope, err = svc.CheckScope(ctx, 1001, "delete")
	assert.NoError(t, err)
	assert.False(t, hasScope)
}

func TestDatabaseIAMService_GetUserScopes_Success(t *testing.T) {
	repo := NewMockIAMRepository()
	repo.AddScope("read", "读取", "platform")
	repo.AddScope("write", "写入", "platform")

	svc := NewDatabaseIAMService(repo)
	ctx := context.Background()

	// 先创建角色（会分配 scopes）
	_, _ = svc.CreateRole(ctx, &CreateRoleRequest{
		Code:   "developer",
		Name:   "开发者",
		Type:   "platform",
		Level:  20,
		Scopes: []string{"read", "write"},
	})

	// 分配角色
	_, _ = svc.AssignRole(ctx, &AssignRoleRequest{UserID: 1001, RoleCode: "developer", TenantID: 1})

	scopes, err := svc.GetUserScopes(ctx, 1001)

	assert.NoError(t, err)
	assert.Len(t, scopes, 2)
	assert.Contains(t, scopes, "read")
	assert.Contains(t, scopes, "write")
}

// Note: IsExpired is a method on DefaultIAMService, not DatabaseIAMService
