package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"lijiaoqiao/supply-api/internal/iam/service"
)

// IAMHandler IAM HTTP处理器
type IAMHandler struct {
	iamService service.IAMServiceInterface
}

// NewIAMHandler 创建IAM处理器
func NewIAMHandler(iamService service.IAMServiceInterface) *IAMHandler {
	return &IAMHandler{
		iamService: iamService,
	}
}

// RoleResponse HTTP响应中的角色信息
type RoleResponse struct {
	Code     string   `json:"role_code"`
	Name     string   `json:"role_name"`
	Type     string   `json:"role_type"`
	Level    int      `json:"level"`
	Scopes   []string `json:"scopes,omitempty"`
	IsActive bool     `json:"is_active"`
}

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	Code   string   `json:"code"`
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Level  int      `json:"level"`
	Scopes []string `json:"scopes"`
}

// UpdateRoleRequest 更新角色请求
type UpdateRoleRequest struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
	IsActive    *bool    `json:"is_active"`
}

// AssignRoleRequest 分配角色请求
type AssignRoleRequest struct {
	RoleCode  string `json:"role_code"`
	TenantID  int64  `json:"tenant_id"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// HTTPError HTTP错误响应
type HTTPError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Error HTTPError `json:"error"`
}

// RegisterRoutes 注册IAM路由
func (h *IAMHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/iam/roles", h.handleRoles)
	mux.HandleFunc("/api/v1/iam/roles/", h.handleRoleByCode)
	mux.HandleFunc("/api/v1/iam/scopes", h.handleScopes)
	mux.HandleFunc("/api/v1/iam/users/", h.handleUserRoles)
	mux.HandleFunc("/api/v1/iam/check-scope", h.handleCheckScope)
}

// handleRoles 处理角色相关路由
func (h *IAMHandler) handleRoles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListRoles(w, r)
	case http.MethodPost:
		h.CreateRole(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

// handleRoleByCode 处理单个角色路由
func (h *IAMHandler) handleRoleByCode(w http.ResponseWriter, r *http.Request) {
	roleCode := extractRoleCode(r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		h.GetRole(w, r, roleCode)
	case http.MethodPut:
		h.UpdateRole(w, r, roleCode)
	case http.MethodDelete:
		h.DeleteRole(w, r, roleCode)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

// handleScopes 处理Scope列表路由
func (h *IAMHandler) handleScopes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	h.ListScopes(w, r)
}

// handleUserRoles 处理用户角色路由
func (h *IAMHandler) handleUserRoles(w http.ResponseWriter, r *http.Request) {
	// 解析用户ID
	path := r.URL.Path
	userIDStr := extractUserID(path)
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_USER_ID", "invalid user id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.GetUserRoles(w, r, userID)
	case http.MethodPost:
		h.AssignRole(w, r, userID)
	case http.MethodDelete:
		roleCode := extractRoleCodeFromUserPath(path)
		tenantID := int64(0) // 从请求或context获取
		h.RevokeRole(w, r, userID, roleCode, tenantID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

// handleCheckScope 处理检查Scope路由
func (h *IAMHandler) handleCheckScope(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	h.CheckScope(w, r)
}

// CreateRole 处理创建角色请求
func (h *IAMHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// 验证必填字段
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "MISSING_CODE", "role code is required")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "MISSING_NAME", "role name is required")
		return
	}
	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "MISSING_TYPE", "role type is required")
		return
	}

	serviceReq := &service.CreateRoleRequest{
		Code:   req.Code,
		Name:   req.Name,
		Type:   req.Type,
		Level:  req.Level,
		Scopes: req.Scopes,
	}

	role, err := h.iamService.CreateRole(r.Context(), serviceReq)
	if err != nil {
		if err == service.ErrDuplicateRoleCode {
			writeError(w, http.StatusConflict, "DUPLICATE_ROLE_CODE", err.Error())
			return
		}
		if err == service.ErrInvalidRequest {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"role": toRoleResponse(role),
	})
}

// GetRole 处理获取单个角色请求
func (h *IAMHandler) GetRole(w http.ResponseWriter, r *http.Request, roleCode string) {
	role, err := h.iamService.GetRole(r.Context(), roleCode)
	if err != nil {
		if err == service.ErrRoleNotFound {
			writeError(w, http.StatusNotFound, "ROLE_NOT_FOUND", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"role": toRoleResponse(role),
	})
}

// ListRoles 处理列出角色请求
func (h *IAMHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roleType := r.URL.Query().Get("type")

	roles, err := h.iamService.ListRoles(r.Context(), roleType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	roleResponses := make([]*RoleResponse, len(roles))
	for i, role := range roles {
		roleResponses[i] = toRoleResponse(role)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"roles": roleResponses,
	})
}

// UpdateRole 处理更新角色请求
func (h *IAMHandler) UpdateRole(w http.ResponseWriter, r *http.Request, roleCode string) {
	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	req.Code = roleCode // 确保使用URL中的roleCode

	serviceReq := &service.UpdateRoleRequest{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Scopes:      req.Scopes,
		IsActive:    req.IsActive,
	}

	role, err := h.iamService.UpdateRole(r.Context(), serviceReq)
	if err != nil {
		if err == service.ErrRoleNotFound {
			writeError(w, http.StatusNotFound, "ROLE_NOT_FOUND", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"role": toRoleResponse(role),
	})
}

// DeleteRole 处理删除角色请求
func (h *IAMHandler) DeleteRole(w http.ResponseWriter, r *http.Request, roleCode string) {
	err := h.iamService.DeleteRole(r.Context(), roleCode)
	if err != nil {
		if err == service.ErrRoleNotFound {
			writeError(w, http.StatusNotFound, "ROLE_NOT_FOUND", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "role deleted successfully",
	})
}

// ListScopes 处理列出所有Scope请求
func (h *IAMHandler) ListScopes(w http.ResponseWriter, r *http.Request) {
	// 从预定义Scope列表获取
	scopes := []map[string]interface{}{
		{"scope_code": "platform:read", "scope_name": "读取平台配置", "scope_type": "platform"},
		{"scope_code": "platform:write", "scope_name": "修改平台配置", "scope_type": "platform"},
		{"scope_code": "platform:admin", "scope_name": "平台级管理", "scope_type": "platform"},
		{"scope_code": "tenant:read", "scope_name": "读取租户信息", "scope_type": "platform"},
		{"scope_code": "supply:account:read", "scope_name": "读取供应账号", "scope_type": "supply"},
		{"scope_code": "consumer:apikey:create", "scope_name": "创建API Key", "scope_type": "consumer"},
		{"scope_code": "router:invoke", "scope_name": "调用模型", "scope_type": "router"},
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"scopes": scopes,
	})
}

// GetUserRoles 处理获取用户角色请求
func (h *IAMHandler) GetUserRoles(w http.ResponseWriter, r *http.Request, userID int64) {
	roles, err := h.iamService.GetUserRoles(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": userID,
		"roles":   roles,
	})
}

// AssignRole 处理分配角色请求
func (h *IAMHandler) AssignRole(w http.ResponseWriter, r *http.Request, userID int64) {
	var req AssignRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	serviceReq := &service.AssignRoleRequest{
		UserID:   userID,
		RoleCode: req.RoleCode,
		TenantID: req.TenantID,
	}

	mapping, err := h.iamService.AssignRole(r.Context(), serviceReq)
	if err != nil {
		if err == service.ErrRoleNotFound {
			writeError(w, http.StatusNotFound, "ROLE_NOT_FOUND", err.Error())
			return
		}
		if err == service.ErrDuplicateAssignment {
			writeError(w, http.StatusConflict, "DUPLICATE_ASSIGNMENT", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "role assigned successfully",
		"mapping": mapping,
	})
}

// RevokeRole 处理撤销角色请求
func (h *IAMHandler) RevokeRole(w http.ResponseWriter, r *http.Request, userID int64, roleCode string, tenantID int64) {
	err := h.iamService.RevokeRole(r.Context(), userID, roleCode, tenantID)
	if err != nil {
		if err == service.ErrRoleNotFound {
			writeError(w, http.StatusNotFound, "ROLE_NOT_FOUND", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "role revoked successfully",
	})
}

// CheckScope 处理检查Scope请求
func (h *IAMHandler) CheckScope(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		writeError(w, http.StatusBadRequest, "MISSING_SCOPE", "scope parameter is required")
		return
	}

	// 从context获取userID（实际应用中应从认证中间件获取）
	userID := int64(1) // 模拟

	hasScope, err := h.iamService.CheckScope(r.Context(), userID, scope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"has_scope": hasScope,
		"scope":     scope,
		"user_id":   userID,
	})
}

// toRoleResponse 转换为RoleResponse
func toRoleResponse(role *service.Role) *RoleResponse {
	return &RoleResponse{
		Code:     role.Code,
		Name:     role.Name,
		Type:     role.Type,
		Level:    role.Level,
		IsActive: role.IsActive,
	}
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Error: HTTPError{
			Code:    code,
			Message: message,
		},
	})
}

// extractRoleCode 从URL路径提取角色代码
func extractRoleCode(path string) string {
	// /api/v1/iam/roles/developer -> developer
	parts := splitPath(path)
	if len(parts) >= 5 {
		return parts[4]
	}
	return ""
}

// extractUserID 从URL路径提取用户ID
func extractUserID(path string) string {
	// /api/v1/iam/users/123/roles -> 123
	parts := splitPath(path)
	if len(parts) >= 4 {
		return parts[3]
	}
 if len(parts) >= 6 {
		return parts[3]
	}
	return ""
}

// extractRoleCodeFromUserPath 从用户路径提取角色代码
func extractRoleCodeFromUserPath(path string) string {
	// /api/v1/iam/users/123/roles/developer -> developer
	parts := splitPath(path)
	if len(parts) >= 6 {
		return parts[5]
	}
	return ""
}

// splitPath 分割URL路径
func splitPath(path string) []string {
	var parts []string
	var current string
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// RequireScope 返回一个要求特定Scope的中间件函数
func RequireScope(scope string, iamService service.IAMServiceInterface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 从context获取userID
			userID := getUserIDFromContext(r.Context())
			if userID == 0 {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "user not authenticated")
				return
			}

			hasScope, err := iamService.CheckScope(r.Context(), userID, scope)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
				return
			}

			if !hasScope {
				writeError(w, http.StatusForbidden, "SCOPE_DENIED", "insufficient scope")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getUserIDFromContext 从context获取userID（实际应用中应从认证中间件获取）
func getUserIDFromContext(ctx context.Context) int64 {
	// TODO: 从认证中间件获取真实的userID
	return 1
}
