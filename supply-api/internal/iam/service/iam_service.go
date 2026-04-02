package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

// 错误定义
var (
	ErrRoleNotFound        = errors.New("role not found")
	ErrDuplicateRoleCode   = errors.New("role code already exists")
	ErrDuplicateAssignment = errors.New("user already has this role")
	ErrInvalidRequest      = errors.New("invalid request")
)

// Role 角色（简化的服务层模型）
type Role struct {
	Code        string
	Name        string
	Type        string
	Level       int
	Description string
	IsActive    bool
	Version     int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UserRole 用户角色（简化的服务层模型）
type UserRole struct {
	UserID    int64
	RoleCode  string
	TenantID  int64
	IsActive  bool
	ExpiresAt *time.Time
}

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	Code        string
	Name        string
	Type        string
	Level       int
	Description string
	Scopes      []string
	ParentCode  string
}

// UpdateRoleRequest 更新角色请求
type UpdateRoleRequest struct {
	Code        string
	Name        string
	Description string
	Scopes      []string
	IsActive    *bool
}

// AssignRoleRequest 分配角色请求
type AssignRoleRequest struct {
	UserID    int64
	RoleCode  string
	TenantID  int64
	GrantedBy int64
	ExpiresAt *time.Time
}

// IAMServiceInterface IAM服务接口
type IAMServiceInterface interface {
	CreateRole(ctx context.Context, req *CreateRoleRequest) (*Role, error)
	GetRole(ctx context.Context, roleCode string) (*Role, error)
	UpdateRole(ctx context.Context, req *UpdateRoleRequest) (*Role, error)
	DeleteRole(ctx context.Context, roleCode string) error
	ListRoles(ctx context.Context, roleType string) ([]*Role, error)

	AssignRole(ctx context.Context, req *AssignRoleRequest) (*UserRole, error)
	RevokeRole(ctx context.Context, userID int64, roleCode string, tenantID int64) error
	GetUserRoles(ctx context.Context, userID int64) ([]*UserRole, error)

	CheckScope(ctx context.Context, userID int64, requiredScope string) (bool, error)
	GetUserScopes(ctx context.Context, userID int64) ([]string, error)
}

// DefaultIAMService 默认IAM服务实现
type DefaultIAMService struct {
	// 角色存储
	roleStore map[string]*Role
	// 用户角色存储: userID -> []*UserRole
	userRoleStore map[int64][]*UserRole
	// 角色Scope存储: roleCode -> []scopeCode
	roleScopeStore map[string][]string
	// 并发控制
	mu sync.RWMutex
}

// NewDefaultIAMService 创建默认IAM服务
func NewDefaultIAMService() *DefaultIAMService {
	return &DefaultIAMService{
		roleStore:      make(map[string]*Role),
		userRoleStore:  make(map[int64][]*UserRole),
		roleScopeStore: make(map[string][]string),
	}
}

// CreateRole 创建角色
func (s *DefaultIAMService) CreateRole(ctx context.Context, req *CreateRoleRequest) (*Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否重复
	if _, exists := s.roleStore[req.Code]; exists {
		return nil, ErrDuplicateRoleCode
	}

	// 验证角色类型
	if req.Type != "platform" && req.Type != "supply" && req.Type != "consumer" {
		return nil, ErrInvalidRequest
	}

	now := time.Now()
	role := &Role{
		Code:        req.Code,
		Name:        req.Name,
		Type:        req.Type,
		Level:       req.Level,
		Description: req.Description,
		IsActive:    true,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 存储角色
	s.roleStore[req.Code] = role

	// 存储角色Scope关联
	if len(req.Scopes) > 0 {
		s.roleScopeStore[req.Code] = req.Scopes
	}

	return role, nil
}

// GetRole 获取角色
func (s *DefaultIAMService) GetRole(ctx context.Context, roleCode string) (*Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	role, exists := s.roleStore[roleCode]
	if !exists {
		return nil, ErrRoleNotFound
	}
	return role, nil
}

// UpdateRole 更新角色
func (s *DefaultIAMService) UpdateRole(ctx context.Context, req *UpdateRoleRequest) (*Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, exists := s.roleStore[req.Code]
	if !exists {
		return nil, ErrRoleNotFound
	}

	// 更新字段
	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Description != "" {
		role.Description = req.Description
	}
	if req.Scopes != nil {
		s.roleScopeStore[req.Code] = req.Scopes
	}
	if req.IsActive != nil {
		role.IsActive = *req.IsActive
	}

	// 递增版本
	role.Version++
	role.UpdatedAt = time.Now()

	return role, nil
}

// DeleteRole 删除角色（软删除）
func (s *DefaultIAMService) DeleteRole(ctx context.Context, roleCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, exists := s.roleStore[roleCode]
	if !exists {
		return ErrRoleNotFound
	}

	role.IsActive = false
	role.UpdatedAt = time.Now()
	return nil
}

// ListRoles 列出角色
func (s *DefaultIAMService) ListRoles(ctx context.Context, roleType string) ([]*Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var roles []*Role
	for _, role := range s.roleStore {
		if roleType == "" || role.Type == roleType {
			roles = append(roles, role)
		}
	}
	return roles, nil
}

// AssignRole 分配角色
func (s *DefaultIAMService) AssignRole(ctx context.Context, req *AssignRoleRequest) (*UserRole, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查角色是否存在
	if _, exists := s.roleStore[req.RoleCode]; !exists {
		return nil, ErrRoleNotFound
	}

	// 检查是否已分配
	for _, ur := range s.userRoleStore[req.UserID] {
		if ur.RoleCode == req.RoleCode && ur.TenantID == req.TenantID && ur.IsActive {
			return nil, ErrDuplicateAssignment
		}
	}

	userRole := &UserRole{
		UserID:    req.UserID,
		RoleCode:  req.RoleCode,
		TenantID:  req.TenantID,
		IsActive:  true,
		ExpiresAt: req.ExpiresAt,
	}

	// 存储映射
	s.userRoleStore[req.UserID] = append(s.userRoleStore[req.UserID], userRole)

	return userRole, nil
}

// RevokeRole 撤销角色
func (s *DefaultIAMService) RevokeRole(ctx context.Context, userID int64, roleCode string, tenantID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ur := range s.userRoleStore[userID] {
		if ur.RoleCode == roleCode && ur.TenantID == tenantID {
			ur.IsActive = false
			return nil
		}
	}
	return ErrRoleNotFound
}

// GetUserRoles 获取用户角色
func (s *DefaultIAMService) GetUserRoles(ctx context.Context, userID int64) ([]*UserRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var userRoles []*UserRole
	for _, ur := range s.userRoleStore[userID] {
		if ur.IsActive {
			userRoles = append(userRoles, ur)
		}
	}
	return userRoles, nil
}

// CheckScope 检查用户是否有指定Scope
func (s *DefaultIAMService) CheckScope(ctx context.Context, userID int64, requiredScope string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	scopes, err := s.getUserScopesLocked(userID)
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

// GetUserScopes 获取用户所有Scope
func (s *DefaultIAMService) GetUserScopes(ctx context.Context, userID int64) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getUserScopesLocked(userID)
}

// getUserScopesLocked 获取用户所有Scope（内部使用，需要持有锁）
func (s *DefaultIAMService) getUserScopesLocked(userID int64) ([]string, error) {
	var allScopes []string
	seen := make(map[string]bool)

	for _, ur := range s.userRoleStore[userID] {
		if ur.IsActive && (ur.ExpiresAt == nil || ur.ExpiresAt.After(time.Now())) {
			if scopes, exists := s.roleScopeStore[ur.RoleCode]; exists {
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

// IsExpired 检查用户角色是否过期
func (ur *UserRole) IsExpired() bool {
	if ur.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*ur.ExpiresAt)
}
