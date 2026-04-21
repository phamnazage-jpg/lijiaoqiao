// Package sharedauth — 三服务共享 auth 工具函数
//
// 可共享（无服务特有依赖）：
//   - extractBearerToken: 从 Authorization header 提取 Bearer token
//   - hasExternalQueryKey: 检查 query string 中是否含敏感 key
//   - writeAuthError: 统一 JSON 错误响应格式
//   - AuditEvent: 统一审计事件结构
//
// 不适合共享（各服务必须自行实现）：
//   - JWT 验证逻辑
//   - Token 状态查询
//   - 授权策略
package sharedauth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// AuthErrorCode 统一错误码（三服务共享）
type AuthErrorCode string

const (
	CodeMissingBearer      AuthErrorCode = "AUTH_MISSING_BEARER"
	CodeInvalidToken       AuthErrorCode = "AUTH_INVALID_TOKEN"
	CodeTokenInactive      AuthErrorCode = "AUTH_TOKEN_INACTIVE"
	CodeQueryKeyNotAllowed AuthErrorCode = "QUERY_KEY_NOT_ALLOWED"
	CodeAuthzDenied        AuthErrorCode = "AUTH_SCOPE_DENIED"
	CodeAuthzRoleDenied    AuthErrorCode = "AUTH_ROLE_DENIED"
	CodeAuthNotReady       AuthErrorCode = "AUTH_NOT_READY"
)

// AuditEvent 统一审计事件结构（三服务共享）
type AuditEvent struct {
	EventName  string `json:"event_name"`
	RequestID  string `json:"request_id,omitempty"`
	TokenID    string `json:"token_id,omitempty"`
	SubjectID  string `json:"subject_id,omitempty"`
	Route      string `json:"route,omitempty"`
	ResultCode string `json:"result_code"`
	ClientIP   string `json:"client_ip,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// errorResponse 统一错误响应格式（三服务共享）
type errorResponse struct {
	RequestID string       `json:"request_id"`
	Error     errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// extractBearerToken 从 Authorization header 提取 Bearer token
// 返回 (token, ok)
// 行为：
//   - 必须以 "Bearer " 为前缀
//   - token 字符串 TrimSpace 后非空
//   - 否则返回 "", false
func ExtractBearerToken(authHeader string) (string, bool) {
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
	return token, token != ""
}

// hasExternalQueryKey 检查 query string 是否含敏感参数
// 敏感参数名（大小写不敏感）：key, api_key, token, access_token
func HasExternalQueryKey(queryVals map[string][]string) bool {
	for key := range queryVals {
		lowerKey := strings.ToLower(key)
		if lowerKey == "key" || lowerKey == "api_key" || lowerKey == "token" || lowerKey == "access_token" {
			return true
		}
	}
	return false
}

// QueryParamsFromRequest 从 *http.Request 提取 query 参数 map
func QueryParamsFromRequest(r *http.Request) map[string][]string {
	if r.URL == nil {
		return nil
	}
	return r.URL.Query()
}

// writeAuthError 写入统一 JSON 错误响应
func WriteAuthError(w http.ResponseWriter, status int, requestID string, code AuthErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	payload := errorResponse{
		RequestID: requestID,
		Error: errorPayload{
			Code:    string(code),
			Message: message,
		},
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// AuditEventFromMap 从 map 构建 AuditEvent（用于测试）
func AuditEventFromMap(m map[string]string) AuditEvent {
	return AuditEvent{
		EventName:  m["event_name"],
		RequestID:  m["request_id"],
		TokenID:    m["token_id"],
		SubjectID:  m["subject_id"],
		Route:      m["route"],
		ResultCode: m["result_code"],
		ClientIP:   m["client_ip"],
	}
}
