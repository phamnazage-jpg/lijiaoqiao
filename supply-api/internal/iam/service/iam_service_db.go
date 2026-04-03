package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lijiaoqiao/supply-api/internal/iam/model"
	"lijiaoqiao/supply-api/internal/iam/repository"
)

// DatabaseIAMService 数据库-backed IAM服务
type DatabaseIAMService struct {
	repo repository.IAMRepository
}

// NewDatabaseIAMService 创建数据库-backed IAM服务
func NewDatabaseIAMService(repo repository.IAMRepository) *DatabaseIAMService {
	return &DatabaseIAMService{repo: repo}
}

// Ensure interface
var _ IAMServiceInterface = (*DatabaseIAMService)(nil)

// ============ Role Operations ============

// CreateRole 创建角色
func (s *DatabaseIAMService) CreateRole(ctx context.Context, req *CreateRoleRequest) (*Role, error) {
	// 验证角色类型
	if req.Type != model.RoleTypePlatform && req.Type != model.RoleTypeSupply && req.Type != model.RoleTypeConsumer {
		return nil, ErrInvalidRequest
	}

	now := time.Now()
	role := &model.Role{
		Code:        req.Code,
		Name:        req.Name,
		Type:        req.Type,
		Level:       req.Level,
		Description: req.Description,
		IsActive:    true,
		Version:     1,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}

	// 处理父角色
	if req.ParentCode != "" {
		parent, err := s.repo.GetRoleByCode(ctx, req.ParentCode)
		if err != nil {
			return nil, fmt.Errorf("parent role not found: %w", err)
		}
		role.ParentRoleID = &parent.ID
	}

	// 创建角色
	if err := s.repo.CreateRole(ctx, role); err != nil {
		if errors.Is(err, repository.ErrDuplicateRoleCode) {
			return nil, ErrDuplicateRoleCode
		}
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	// 添加权限关联
	for _, scopeCode := range req.Scopes {
		if err := s.repo.AddScopeToRole(ctx, req.Code, scopeCode); err != nil {
			if !errors.Is(err, repository.ErrScopeNotFound) {
				return nil, fmt.Errorf("failed to add scope %s: %w", scopeCode, err)
			}
		}
	}

	// 重新获取完整角色信息
	createdRole, err := s.repo.GetRoleByCode(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to get created role: %w", err)
	}

	return modelRoleToServiceRole(createdRole), nil
}

// GetRole 获取角色
func (s *DatabaseIAMService) GetRole(ctx context.Context, roleCode string) (*Role, error) {
	role, err := s.repo.GetRoleByCode(ctx, roleCode)
	if err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	// 获取角色关联的权限
	scopes, err := s.repo.GetScopesByRoleCode(ctx, roleCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get role scopes: %w", err)
	}
	role.Scopes = scopes

	return modelRoleToServiceRole(role), nil
}

// UpdateRole 更新角色
func (s *DatabaseIAMService) UpdateRole(ctx context.Context, req *UpdateRoleRequest) (*Role, error) {
	// 获取现有角色
	existing, err := s.repo.GetRoleByCode(ctx, req.Code)
	if err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	// 更新字段
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}

	// 更新权限关联
	if req.Scopes != nil {
		// 移除所有现有权限
		currentScopes, _ := s.repo.GetScopesByRoleCode(ctx, req.Code)
		for _, scope := range currentScopes {
			s.repo.RemoveScopeFromRole(ctx, req.Code, scope)
		}
		// 添加新权限
		for _, scope := range req.Scopes {
			s.repo.AddScopeToRole(ctx, req.Code, scope)
		}
	}

	// 保存更新
	if err := s.repo.UpdateRole(ctx, existing); err != nil {
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	return s.GetRole(ctx, req.Code)
}

// DeleteRole 删除角色（软删除）
func (s *DatabaseIAMService) DeleteRole(ctx context.Context, roleCode string) error {
	if err := s.repo.DeleteRole(ctx, roleCode); err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("failed to delete role: %w", err)
	}
	return nil
}

// ListRoles 列出角色
func (s *DatabaseIAMService) ListRoles(ctx context.Context, roleType string) ([]*Role, error) {
	roles, err := s.repo.ListRoles(ctx, roleType)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	var result []*Role
	for _, role := range roles {
		// 获取每个角色的权限
		scopes, _ := s.repo.GetScopesByRoleCode(ctx, role.Code)
		role.Scopes = scopes
		result = append(result, modelRoleToServiceRole(role))
	}

	return result, nil
}

// ============ User-Role Operations ============

// AssignRole 分配角色给用户
func (s *DatabaseIAMService) AssignRole(ctx context.Context, req *AssignRoleRequest) (*UserRole, error) {
	// 获取角色ID
	role, err := s.repo.GetRoleByCode(ctx, req.RoleCode)
	if err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	userRole := &model.UserRoleMapping{
		UserID:    req.UserID,
		RoleID:    role.ID,
		TenantID:  req.TenantID,
		IsActive:  true,
		GrantedBy: req.GrantedBy,
		ExpiresAt: req.ExpiresAt,
	}

	if err := s.repo.AssignRole(ctx, userRole); err != nil {
		if errors.Is(err, repository.ErrDuplicateAssignment) {
			return nil, ErrDuplicateAssignment
		}
		return nil, fmt.Errorf("failed to assign role: %w", err)
	}

	return &UserRole{
		UserID:    req.UserID,
		RoleCode:  req.RoleCode,
		TenantID:  req.TenantID,
		IsActive:  true,
		ExpiresAt: req.ExpiresAt,
	}, nil
}

// RevokeRole 撤销用户的角色
func (s *DatabaseIAMService) RevokeRole(ctx context.Context, userID int64, roleCode string, tenantID int64) error {
	if err := s.repo.RevokeRole(ctx, userID, roleCode, tenantID); err != nil {
		if errors.Is(err, repository.ErrRoleNotFound) {
			return ErrRoleNotFound
		}
		if errors.Is(err, repository.ErrUserRoleNotFound) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("failed to revoke role: %w", err)
	}
	return nil
}

// GetUserRoles 获取用户角色
func (s *DatabaseIAMService) GetUserRoles(ctx context.Context, userID int64) ([]*UserRole, error) {
	userRoles, err := s.repo.GetUserRolesWithCode(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	var result []*UserRole
	for _, ur := range userRoles {
		result = append(result, &UserRole{
			UserID:    ur.UserID,
			RoleCode:  ur.RoleCode,
			TenantID:  ur.TenantID,
			IsActive:  ur.IsActive,
			ExpiresAt: ur.ExpiresAt,
		})
	}

	return result, nil
}

// ============ Scope Operations ============

// CheckScope 检查用户是否有指定权限
func (s *DatabaseIAMService) CheckScope(ctx context.Context, userID int64, requiredScope string) (bool, error) {
	scopes, err := s.repo.GetUserScopes(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user scopes: %w", err)
	}

	for _, scope := range scopes {
		if scope == requiredScope || scope == "*" {
			return true, nil
		}
	}

	return false, nil
}

// GetUserScopes 获取用户所有权限
func (s *DatabaseIAMService) GetUserScopes(ctx context.Context, userID int64) ([]string, error) {
	scopes, err := s.repo.GetUserScopes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user scopes: %w", err)
	}
	return scopes, nil
}

// ============ Helper Functions ============

// modelRoleToServiceRole 将模型角色转换为服务层角色
func modelRoleToServiceRole(mr *model.Role) *Role {
	return &Role{
		Code:        mr.Code,
		Name:        mr.Name,
		Type:        mr.Type,
		Level:       mr.Level,
		Description: mr.Description,
		IsActive:    mr.IsActive,
		Version:     mr.Version,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
