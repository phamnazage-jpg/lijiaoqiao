package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"lijiaoqiao/supply-api/internal/iam/model"
)

// errors
var (
	ErrRoleNotFound        = errors.New("role not found")
	ErrDuplicateRoleCode   = errors.New("role code already exists")
	ErrDuplicateAssignment = errors.New("user already has this role")
	ErrScopeNotFound       = errors.New("scope not found")
	ErrUserRoleNotFound    = errors.New("user role not found")
)

// IAMRepository IAM数据仓储接口
type IAMRepository interface {
	// Role operations
	CreateRole(ctx context.Context, role *model.Role) error
	GetRoleByCode(ctx context.Context, code string) (*model.Role, error)
	UpdateRole(ctx context.Context, role *model.Role) error
	DeleteRole(ctx context.Context, code string) error
	ListRoles(ctx context.Context, roleType string) ([]*model.Role, error)

	// Scope operations
	CreateScope(ctx context.Context, scope *model.Scope) error
	GetScopeByCode(ctx context.Context, code string) (*model.Scope, error)
	ListScopes(ctx context.Context) ([]*model.Scope, error)

	// Role-Scope operations
	AddScopeToRole(ctx context.Context, roleCode, scopeCode string) error
	RemoveScopeFromRole(ctx context.Context, roleCode, scopeCode string) error
	GetScopesByRoleCode(ctx context.Context, roleCode string) ([]string, error)

	// User-Role operations
	AssignRole(ctx context.Context, userRole *model.UserRoleMapping) error
	RevokeRole(ctx context.Context, userID int64, roleCode string, tenantID int64) error
	GetUserRoles(ctx context.Context, userID int64) ([]*model.UserRoleMapping, error)
	GetUserRolesWithCode(ctx context.Context, userID int64) ([]*UserRoleWithCode, error)
	GetUserScopes(ctx context.Context, userID int64) ([]string, error)
}

type iamDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PostgresIAMRepository PostgreSQL实现的IAM仓储
type PostgresIAMRepository struct {
	pool iamDB
}

// NewPostgresIAMRepository 创建PostgreSQL IAM仓储
func NewPostgresIAMRepository(pool *pgxpool.Pool) *PostgresIAMRepository {
	return &PostgresIAMRepository{pool: pool}
}

func newPostgresIAMRepositoryWithDB(db iamDB) *PostgresIAMRepository {
	return &PostgresIAMRepository{pool: db}
}

func iamStringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func iamInt64OrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func iamNullableIP(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

// Ensure interfaces
var _ IAMRepository = (*PostgresIAMRepository)(nil)

// ============ Role Operations ============

// CreateRole 创建角色
func (r *PostgresIAMRepository) CreateRole(ctx context.Context, role *model.Role) error {
	query := `
		INSERT INTO iam_roles (code, name, type, parent_role_id, level, description, is_active,
		                       request_id, created_ip, updated_ip, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	var parentID *int64
	if role.ParentRoleID != nil {
		parentID = role.ParentRoleID
	}

	now := time.Now()
	if role.CreatedAt == nil {
		role.CreatedAt = &now
	}
	if role.UpdatedAt == nil {
		role.UpdatedAt = &now
	}

	_, err := r.pool.Exec(ctx, query,
		role.Code, role.Name, role.Type, parentID, role.Level, role.Description, role.IsActive,
		role.RequestID, iamNullableIP(role.CreatedIP), iamNullableIP(role.UpdatedIP), role.Version, role.CreatedAt, role.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return ErrDuplicateRoleCode
		}
		return fmt.Errorf("failed to create role: %w", err)
	}
	return nil
}

// GetRoleByCode 根据角色代码获取角色
func (r *PostgresIAMRepository) GetRoleByCode(ctx context.Context, code string) (*model.Role, error) {
	query := `
		SELECT id, code, name, type, parent_role_id, level, description, is_active,
		       request_id, created_ip, updated_ip, version, created_at, updated_at
		FROM iam_roles WHERE code = $1 AND is_active = true
	`

	var role model.Role
	var parentID *int64
	var requestID, createdIP, updatedIP *string

	err := r.pool.QueryRow(ctx, query, code).Scan(
		&role.ID, &role.Code, &role.Name, &role.Type, &parentID, &role.Level,
		&role.Description, &role.IsActive, &requestID, &createdIP, &updatedIP,
		&role.Version, &role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	role.ParentRoleID = parentID
	role.RequestID = iamStringOrEmpty(requestID)
	role.CreatedIP = iamStringOrEmpty(createdIP)
	role.UpdatedIP = iamStringOrEmpty(updatedIP)

	return &role, nil
}

// UpdateRole 更新角色
func (r *PostgresIAMRepository) UpdateRole(ctx context.Context, role *model.Role) error {
	query := `
		UPDATE iam_roles
		SET name = $2, description = $3, is_active = $4, updated_ip = $5, version = version + 1, updated_at = NOW()
		WHERE code = $1 AND is_active = true
	`

	result, err := r.pool.Exec(ctx, query, role.Code, role.Name, role.Description, role.IsActive, iamNullableIP(role.UpdatedIP))
	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrRoleNotFound
	}

	return nil
}

// DeleteRole 删除角色（软删除）
func (r *PostgresIAMRepository) DeleteRole(ctx context.Context, code string) error {
	query := `UPDATE iam_roles SET is_active = false, updated_at = NOW() WHERE code = $1`

	result, err := r.pool.Exec(ctx, query, code)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrRoleNotFound
	}

	return nil
}

// ListRoles 列出角色
func (r *PostgresIAMRepository) ListRoles(ctx context.Context, roleType string) ([]*model.Role, error) {
	var query string
	var args []interface{}

	if roleType != "" {
		query = `
			SELECT id, code, name, type, parent_role_id, level, description, is_active,
			       request_id, created_ip, updated_ip, version, created_at, updated_at
			FROM iam_roles WHERE type = $1 AND is_active = true
		`
		args = []interface{}{roleType}
	} else {
		query = `
			SELECT id, code, name, type, parent_role_id, level, description, is_active,
			       request_id, created_ip, updated_ip, version, created_at, updated_at
			FROM iam_roles WHERE is_active = true
		`
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	defer rows.Close()

	var roles []*model.Role
	for rows.Next() {
		var role model.Role
		var parentID *int64
		var requestID, createdIP, updatedIP *string

		err := rows.Scan(
			&role.ID, &role.Code, &role.Name, &role.Type, &parentID, &role.Level,
			&role.Description, &role.IsActive, &requestID, &createdIP, &updatedIP,
			&role.Version, &role.CreatedAt, &role.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}

		role.ParentRoleID = parentID
		role.RequestID = iamStringOrEmpty(requestID)
		role.CreatedIP = iamStringOrEmpty(createdIP)
		role.UpdatedIP = iamStringOrEmpty(updatedIP)

		roles = append(roles, &role)
	}

	return roles, nil
}

// ============ Scope Operations ============

// CreateScope 创建权限范围
func (r *PostgresIAMRepository) CreateScope(ctx context.Context, scope *model.Scope) error {
	query := `
		INSERT INTO iam_scopes (code, name, description, category, is_active, request_id, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(ctx, query, scope.Code, scope.Name, scope.Description, scope.Type, scope.IsActive, scope.RequestID, scope.Version)
	if err != nil {
		return fmt.Errorf("failed to create scope: %w", err)
	}
	return nil
}

// GetScopeByCode 根据代码获取权限范围
func (r *PostgresIAMRepository) GetScopeByCode(ctx context.Context, code string) (*model.Scope, error) {
	query := `
		SELECT id, code, name, description, category, is_active, request_id, version, created_at, updated_at
		FROM iam_scopes WHERE code = $1 AND is_active = true
	`

	var scope model.Scope
	var requestID *string
	err := r.pool.QueryRow(ctx, query, code).Scan(
		&scope.ID, &scope.Code, &scope.Name, &scope.Description, &scope.Type,
		&scope.IsActive, &requestID, &scope.Version, &scope.CreatedAt, &scope.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrScopeNotFound
		}
		return nil, fmt.Errorf("failed to get scope: %w", err)
	}

	scope.RequestID = iamStringOrEmpty(requestID)
	return &scope, nil
}

// ListScopes 列出所有权限范围
func (r *PostgresIAMRepository) ListScopes(ctx context.Context) ([]*model.Scope, error) {
	query := `
		SELECT id, code, name, description, category, is_active, request_id, version, created_at, updated_at
		FROM iam_scopes WHERE is_active = true
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list scopes: %w", err)
	}
	defer rows.Close()

	var scopes []*model.Scope
	for rows.Next() {
		var scope model.Scope
		var requestID *string
		err := rows.Scan(
			&scope.ID, &scope.Code, &scope.Name, &scope.Description, &scope.Type,
			&scope.IsActive, &requestID, &scope.Version, &scope.CreatedAt, &scope.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan scope: %w", err)
		}
		scope.RequestID = iamStringOrEmpty(requestID)
		scopes = append(scopes, &scope)
	}

	return scopes, nil
}

// ============ Role-Scope Operations ============

// AddScopeToRole 为角色添加权限
func (r *PostgresIAMRepository) AddScopeToRole(ctx context.Context, roleCode, scopeCode string) error {
	// 获取role_id和scope_id
	var roleID, scopeID int64

	err := r.pool.QueryRow(ctx, "SELECT id FROM iam_roles WHERE code = $1 AND is_active = true", roleCode).Scan(&roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("failed to get role: %w", err)
	}

	err = r.pool.QueryRow(ctx, "SELECT id FROM iam_scopes WHERE code = $1 AND is_active = true", scopeCode).Scan(&scopeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrScopeNotFound
		}
		return fmt.Errorf("failed to get scope: %w", err)
	}

	_, err = r.pool.Exec(ctx, "INSERT INTO iam_role_scopes (role_id, scope_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", roleID, scopeID)
	if err != nil {
		return fmt.Errorf("failed to add scope to role: %w", err)
	}

	return nil
}

// RemoveScopeFromRole 移除角色的权限
func (r *PostgresIAMRepository) RemoveScopeFromRole(ctx context.Context, roleCode, scopeCode string) error {
	var roleID, scopeID int64

	err := r.pool.QueryRow(ctx, "SELECT id FROM iam_roles WHERE code = $1 AND is_active = true", roleCode).Scan(&roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("failed to get role: %w", err)
	}

	err = r.pool.QueryRow(ctx, "SELECT id FROM iam_scopes WHERE code = $1 AND is_active = true", scopeCode).Scan(&scopeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrScopeNotFound
		}
		return fmt.Errorf("failed to get scope: %w", err)
	}

	_, err = r.pool.Exec(ctx, "DELETE FROM iam_role_scopes WHERE role_id = $1 AND scope_id = $2", roleID, scopeID)
	if err != nil {
		return fmt.Errorf("failed to remove scope from role: %w", err)
	}

	return nil
}

// GetScopesByRoleCode 获取角色的所有权限
func (r *PostgresIAMRepository) GetScopesByRoleCode(ctx context.Context, roleCode string) ([]string, error) {
	query := `
		SELECT s.code FROM iam_scopes s
		JOIN iam_role_scopes rs ON s.id = rs.scope_id
		JOIN iam_roles r ON r.id = rs.role_id
		WHERE r.code = $1 AND r.is_active = true AND s.is_active = true
	`

	rows, err := r.pool.Query(ctx, query, roleCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get scopes by role: %w", err)
	}
	defer rows.Close()

	var scopes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("failed to scan scope code: %w", err)
		}
		scopes = append(scopes, code)
	}

	return scopes, nil
}

// ============ User-Role Operations ============

// AssignRole 分配角色给用户
func (r *PostgresIAMRepository) AssignRole(ctx context.Context, userRole *model.UserRoleMapping) error {
	// 检查是否已分配
	var existingID int64
	err := r.pool.QueryRow(ctx,
		"SELECT id FROM iam_user_roles WHERE user_id = $1 AND role_id = $2 AND tenant_id = $3 AND is_active = true",
		userRole.UserID, userRole.RoleID, userRole.TenantID,
	).Scan(&existingID)

	if err == nil {
		return ErrDuplicateAssignment // 已存在
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to check existing assignment: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO iam_user_roles (user_id, role_id, tenant_id, is_active, granted_by, expires_at, request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, userRole.UserID, userRole.RoleID, userRole.TenantID, true, userRole.GrantedBy, userRole.ExpiresAt, userRole.RequestID)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return ErrDuplicateAssignment
		}
		return fmt.Errorf("failed to assign role: %w", err)
	}

	return nil
}

// RevokeRole 撤销用户的角色
func (r *PostgresIAMRepository) RevokeRole(ctx context.Context, userID int64, roleCode string, tenantID int64) error {
	var roleID int64
	err := r.pool.QueryRow(ctx, "SELECT id FROM iam_roles WHERE code = $1 AND is_active = true", roleCode).Scan(&roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("failed to get role: %w", err)
	}

	result, err := r.pool.Exec(ctx,
		"UPDATE iam_user_roles SET is_active = false WHERE user_id = $1 AND role_id = $2 AND tenant_id = $3 AND is_active = true",
		userID, roleID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("failed to revoke role: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserRoleNotFound
	}

	return nil
}

// UserRoleWithCode 用户角色（含角色代码）
type UserRoleWithCode struct {
	*model.UserRoleMapping
	RoleCode string
}

// GetUserRoles 获取用户的角色
func (r *PostgresIAMRepository) GetUserRoles(ctx context.Context, userID int64) ([]*model.UserRoleMapping, error) {
	query := `
		SELECT ur.id, ur.user_id, r.code, ur.tenant_id, ur.is_active, ur.granted_by, ur.expires_at, ur.request_id, ur.created_at, ur.updated_at
		FROM iam_user_roles ur
		JOIN iam_roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND ur.is_active = true AND r.is_active = true
		AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	defer rows.Close()

	var userRoles []*model.UserRoleMapping
	for rows.Next() {
		var ur model.UserRoleMapping
		var roleCode string
		var tenantID, grantedBy *int64
		var requestID *string
		err := rows.Scan(&ur.ID, &ur.UserID, &roleCode, &tenantID, &ur.IsActive, &grantedBy, &ur.ExpiresAt, &requestID, &ur.CreatedAt, &ur.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user role: %w", err)
		}
		ur.TenantID = iamInt64OrZero(tenantID)
		ur.GrantedBy = iamInt64OrZero(grantedBy)
		ur.RequestID = iamStringOrEmpty(requestID)
		userRoles = append(userRoles, &ur)
	}

	return userRoles, nil
}

// GetUserRolesWithCode 获取用户的角色（含角色代码）
func (r *PostgresIAMRepository) GetUserRolesWithCode(ctx context.Context, userID int64) ([]*UserRoleWithCode, error) {
	query := `
		SELECT ur.id, ur.user_id, r.code, ur.tenant_id, ur.is_active, ur.granted_by, ur.expires_at, ur.request_id, ur.created_at, ur.updated_at
		FROM iam_user_roles ur
		JOIN iam_roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND ur.is_active = true AND r.is_active = true
		AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	defer rows.Close()

	var userRoles []*UserRoleWithCode
	for rows.Next() {
		var ur model.UserRoleMapping
		var roleCode string
		var tenantID, grantedBy *int64
		var requestID *string
		err := rows.Scan(&ur.ID, &ur.UserID, &roleCode, &tenantID, &ur.IsActive, &grantedBy, &ur.ExpiresAt, &requestID, &ur.CreatedAt, &ur.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user role: %w", err)
		}
		ur.TenantID = iamInt64OrZero(tenantID)
		ur.GrantedBy = iamInt64OrZero(grantedBy)
		ur.RequestID = iamStringOrEmpty(requestID)
		userRoles = append(userRoles, &UserRoleWithCode{UserRoleMapping: &ur, RoleCode: roleCode})
	}

	return userRoles, nil
}

// GetUserScopes 获取用户的所有权限
func (r *PostgresIAMRepository) GetUserScopes(ctx context.Context, userID int64) ([]string, error) {
	query := `
		SELECT DISTINCT s.code
		FROM iam_user_roles ur
		JOIN iam_roles r ON r.id = ur.role_id
		JOIN iam_role_scopes rs ON rs.role_id = r.id
		JOIN iam_scopes s ON s.id = rs.scope_id
		WHERE ur.user_id = $1
		  AND ur.is_active = true
		  AND r.is_active = true
		  AND s.is_active = true
		  AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user scopes: %w", err)
	}
	defer rows.Close()

	var scopes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("failed to scan scope code: %w", err)
		}
		scopes = append(scopes, code)
	}

	return scopes, nil
}

// ServiceRole is a copy of service.Role for conversion (avoids import cycle)
// Service层角色结构，用于仓储层到服务层的转换
type ServiceRole struct {
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

// ServiceUserRole is a copy of service.UserRole for conversion
type ServiceUserRole struct {
	UserID    int64
	RoleCode  string
	TenantID  int64
	IsActive  bool
	ExpiresAt *time.Time
}

// ModelRoleToServiceRole 将模型角色转换为服务层角色
func ModelRoleToServiceRole(mr *model.Role) *ServiceRole {
	if mr == nil {
		return nil
	}
	return &ServiceRole{
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

// ModelUserRoleToServiceUserRole 将模型用户角色转换为服务层用户角色
// 注意：UserRoleMapping 不包含 RoleCode，需要通过 GetUserRolesWithCode 获取
func ModelUserRoleToServiceUserRole(mur *model.UserRoleMapping, roleCode string) *ServiceUserRole {
	if mur == nil {
		return nil
	}
	return &ServiceUserRole{
		UserID:    mur.UserID,
		RoleCode:  roleCode,
		TenantID:  mur.TenantID,
		IsActive:  mur.IsActive,
		ExpiresAt: mur.ExpiresAt,
	}
}
