package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 测试辅助函数

// testRoleResponse 用于测试的角色响应
type testRoleResponse struct {
	Code     string `json:"role_code"`
	Name     string `json:"role_name"`
	Type     string `json:"role_type"`
	Level    int    `json:"level"`
	IsActive bool   `json:"is_active"`
}

// testIAMService 模拟IAM服务
type testIAMService struct {
	roles       map[string]*testRoleResponse
	userScopes  map[int64][]string
}

type testRoleResponse2 struct {
	Code     string
	Name     string
	Type     string
	Level    int
	IsActive bool
}

func newTestIAMService() *testIAMService {
	return &testIAMService{
		roles: map[string]*testRoleResponse{
			"viewer":   {Code: "viewer", Name: "查看者", Type: "platform", Level: 10, IsActive: true},
			"operator": {Code: "operator", Name: "运维", Type: "platform", Level: 30, IsActive: true},
		},
		userScopes: map[int64][]string{
			1: {"platform:read", "platform:write"},
		},
	}
}

func (s *testIAMService) CreateRole(req *CreateRoleHTTPRequest) (*testRoleResponse, error) {
	if _, exists := s.roles[req.Code]; exists {
		return nil, errDuplicateRole
	}
	return &testRoleResponse{
		Code:     req.Code,
		Name:     req.Name,
		Type:     req.Type,
		Level:    req.Level,
		IsActive: true,
	}, nil
}

func (s *testIAMService) GetRole(roleCode string) (*testRoleResponse, error) {
	if role, exists := s.roles[roleCode]; exists {
		return role, nil
	}
	return nil, errNotFound
}

func (s *testIAMService) ListRoles(roleType string) ([]*testRoleResponse, error) {
	var result []*testRoleResponse
	for _, role := range s.roles {
		if roleType == "" || role.Type == roleType {
			result = append(result, role)
		}
	}
	return result, nil
}

func (s *testIAMService) CheckScope(userID int64, scope string) bool {
	scopes, ok := s.userScopes[userID]
	if !ok {
		return false
	}
	for _, s := range scopes {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}

// HTTP请求/响应类型
type CreateRoleHTTPRequest struct {
	Code   string   `json:"code"`
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Level  int      `json:"level"`
	Scopes []string `json:"scopes"`
}

// 错误
var (
	errNotFound      = &HTTPErrorResponse{Code: "NOT_FOUND", Message: "not found"}
	errDuplicateRole = &HTTPErrorResponse{Code: "DUPLICATE", Message: "duplicate"}
)

// HTTPErrorResponse HTTP错误响应
type HTTPErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *HTTPErrorResponse) Error() string {
	return e.Message
}

// HTTPHandler 测试用的HTTP处理器
type HTTPHandler struct {
	iam *testIAMService
}

func newHTTPHandler() *HTTPHandler {
	return &HTTPHandler{iam: newTestIAMService()}
}

// handleCreateRole 创建角色
func (h *HTTPHandler) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var req CreateRoleHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorHTTPTest(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	role, err := h.iam.CreateRole(&req)
	if err != nil {
		writeErrorHTTPTest(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSONHTTPTest(w, http.StatusCreated, map[string]interface{}{
		"role": role,
	})
}

// handleListRoles 列出角色
func (h *HTTPHandler) handleListRoles(w http.ResponseWriter, r *http.Request) {
	roleType := r.URL.Query().Get("type")

	roles, err := h.iam.ListRoles(roleType)
	if err != nil {
		writeErrorHTTPTest(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSONHTTPTest(w, http.StatusOK, map[string]interface{}{
		"roles": roles,
	})
}

// handleGetRole 获取角色
func (h *HTTPHandler) handleGetRole(w http.ResponseWriter, r *http.Request) {
	roleCode := r.URL.Query().Get("code")
	if roleCode == "" {
		writeErrorHTTPTest(w, http.StatusBadRequest, "MISSING_CODE", "role code is required")
		return
	}

	role, err := h.iam.GetRole(roleCode)
	if err != nil {
		if err == errNotFound {
			writeErrorHTTPTest(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		writeErrorHTTPTest(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSONHTTPTest(w, http.StatusOK, map[string]interface{}{
		"role": role,
	})
}

// handleCheckScope 检查Scope
func (h *HTTPHandler) handleCheckScope(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		writeErrorHTTPTest(w, http.StatusBadRequest, "MISSING_SCOPE", "scope is required")
		return
	}

	userID := int64(1)
	hasScope := h.iam.CheckScope(userID, scope)

	writeJSONHTTPTest(w, http.StatusOK, map[string]interface{}{
		"has_scope": hasScope,
		"scope":     scope,
	})
}

func writeJSONHTTPTest(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeErrorHTTPTest(w http.ResponseWriter, status int, code, message string) {
	writeJSONHTTPTest(w, status, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// ==================== 测试用例 ====================

// TestHTTPHandler_CreateRole_Success 测试创建角色成功
func TestHTTPHandler_CreateRole_Success(t *testing.T) {
	// arrange
	handler := newHTTPHandler()

	body := `{"code":"developer","name":"开发者","type":"platform","level":20}`
	req := httptest.NewRequest("POST", "/api/v1/iam/roles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// act
	rec := httptest.NewRecorder()
	handler.handleCreateRole(rec, req)

	// assert
	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	role := resp["role"].(map[string]interface{})
	assert.Equal(t, "developer", role["role_code"])
	assert.Equal(t, "开发者", role["role_name"])
}

// TestHTTPHandler_ListRoles_Success 测试列出角色成功
func TestHTTPHandler_ListRoles_Success(t *testing.T) {
	// arrange
	handler := newHTTPHandler()

	req := httptest.NewRequest("GET", "/api/v1/iam/roles", nil)

	// act
	rec := httptest.NewRecorder()
	handler.handleListRoles(rec, req)

	// assert
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	roles := resp["roles"].([]interface{})
	assert.Len(t, roles, 2)
}

// TestHTTPHandler_ListRoles_WithType 测试按类型列出角色
func TestHTTPHandler_ListRoles_WithType(t *testing.T) {
	// arrange
	handler := newHTTPHandler()

	req := httptest.NewRequest("GET", "/api/v1/iam/roles?type=platform", nil)

	// act
	rec := httptest.NewRecorder()
	handler.handleListRoles(rec, req)

	// assert
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestHTTPHandler_GetRole_Success 测试获取角色成功
func TestHTTPHandler_GetRole_Success(t *testing.T) {
	// arrange
	handler := newHTTPHandler()

	req := httptest.NewRequest("GET", "/api/v1/iam/roles?code=viewer", nil)

	// act
	rec := httptest.NewRecorder()
	handler.handleGetRole(rec, req)

	// assert
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	role := resp["role"].(map[string]interface{})
	assert.Equal(t, "viewer", role["role_code"])
}

// TestHTTPHandler_GetRole_NotFound 测试获取不存在的角色
func TestHTTPHandler_GetRole_NotFound(t *testing.T) {
	// arrange
	handler := newHTTPHandler()

	req := httptest.NewRequest("GET", "/api/v1/iam/roles?code=nonexistent", nil)

	// act
	rec := httptest.NewRecorder()
	handler.handleGetRole(rec, req)

	// assert
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestHTTPHandler_CheckScope_HasScope 测试检查Scope存在
func TestHTTPHandler_CheckScope_HasScope(t *testing.T) {
	// arrange
	handler := newHTTPHandler()

	req := httptest.NewRequest("GET", "/api/v1/iam/check-scope?scope=platform:read", nil)

	// act
	rec := httptest.NewRecorder()
	handler.handleCheckScope(rec, req)

	// assert
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	assert.Equal(t, true, resp["has_scope"])
	assert.Equal(t, "platform:read", resp["scope"])
}

// TestHTTPHandler_CheckScope_NoScope 测试检查Scope不存在
func TestHTTPHandler_CheckScope_NoScope(t *testing.T) {
	// arrange
	handler := newHTTPHandler()

	req := httptest.NewRequest("GET", "/api/v1/iam/check-scope?scope=platform:admin", nil)

	// act
	rec := httptest.NewRecorder()
	handler.handleCheckScope(rec, req)

	// assert
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	assert.Equal(t, false, resp["has_scope"])
}

// TestHTTPHandler_CheckScope_MissingScope 测试缺少Scope参数
func TestHTTPHandler_CheckScope_MissingScope(t *testing.T) {
	// arrange
	handler := newHTTPHandler()

	req := httptest.NewRequest("GET", "/api/v1/iam/check-scope", nil)

	// act
	rec := httptest.NewRecorder()
	handler.handleCheckScope(rec, req)

	// assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestHTTPHandler_CreateRole_InvalidJSON 测试无效JSON
func TestHTTPHandler_CreateRole_InvalidJSON(t *testing.T) {
	// arrange
	handler := newHTTPHandler()

	body := `invalid json`
	req := httptest.NewRequest("POST", "/api/v1/iam/roles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// act
	rec := httptest.NewRecorder()
	handler.handleCreateRole(rec, req)

	// assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestHTTPHandler_GetRole_MissingCode 测试缺少角色代码
func TestHTTPHandler_GetRole_MissingCode(t *testing.T) {
	// arrange
	handler := newHTTPHandler()

	req := httptest.NewRequest("GET", "/api/v1/iam/roles", nil) // 没有code参数

	// act
	rec := httptest.NewRecorder()
	handler.handleGetRole(rec, req)

	// assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// 确保函数被使用（避免编译错误）
var _ = context.Background
