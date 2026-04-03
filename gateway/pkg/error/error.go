package error

import "fmt"

// ErrorCode 错误码枚举
type ErrorCode string

const (
	// 认证授权 (AUTH_*)
	AUTH_INVALID_TOKEN          ErrorCode = "AUTH_001"
	AUTH_INSUFFICIENT_PERMISSION ErrorCode = "AUTH_002"
	AUTH_MFA_REQUIRED           ErrorCode = "AUTH_003"

	// 计费 (BILLING_*)
	BILLING_INSUFFICIENT_BALANCE ErrorCode = "BILLING_001"
	BILLING_CHARGE_FAILED         ErrorCode = "BILLING_002"
	BILLING_REFUND_FAILED         ErrorCode = "BILLING_003"
	BILLING_DISCREPANCY          ErrorCode = "BILLING_004"

	// 路由 (ROUTER_*)
	ROUTER_NO_PROVIDER_AVAILABLE ErrorCode = "ROUTER_001"
	ROUTER_ALL_PROVIDERS_FAILED  ErrorCode = "ROUTER_002"
	ROUTER_TIMEOUT               ErrorCode = "ROUTER_003"

	// 供应商 (PROVIDER_*)
	PROVIDER_INVALID_KEY       ErrorCode = "PROVIDER_001"
	PROVIDER_RATE_LIMIT        ErrorCode = "PROVIDER_002"
	PROVIDER_QUOTA_EXCEEDED   ErrorCode = "PROVIDER_003"
	PROVIDER_MODEL_NOT_FOUND   ErrorCode = "PROVIDER_004"
	PROVIDER_ERROR             ErrorCode = "PROVIDER_005"

	// 限流 (RATE_LIMIT_*)
	RATE_LIMIT_EXCEEDED       ErrorCode = "RATE_LIMIT_001"
	RATE_LIMIT_TOKEN_EXCEEDED ErrorCode = "RATE_LIMIT_002"
	RATE_LIMIT_BURST_EXCEEDED ErrorCode = "RATE_LIMIT_003"

	// 通用 (COMMON_*)
	COMMON_INVALID_REQUEST      ErrorCode = "COMMON_001"
	COMMON_RESOURCE_NOT_FOUND   ErrorCode = "COMMON_002"
	COMMON_INTERNAL_ERROR       ErrorCode = "COMMON_003"
	COMMON_SERVICE_UNAVAILABLE  ErrorCode = "COMMON_004"
	COMMON_REQUEST_TOO_LARGE    ErrorCode = "COMMON_005"
)

// ErrorInfo 错误信息
type ErrorInfo struct {
	Code        ErrorCode
	Message     string
	HTTPStatus  int
	Retryable   bool
}

// GatewayError 网关错误
type GatewayError struct {
	Code      ErrorCode
	Message   string
	Details   map[string]interface{}
	RequestID string
	Internal  error
}

func (e *GatewayError) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Internal)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *GatewayError) Unwrap() error {
	return e.Internal
}

// ErrorDefinitions 错误码定义
var ErrorDefinitions = map[ErrorCode]ErrorInfo{
	AUTH_INVALID_TOKEN: {
		Code:       AUTH_INVALID_TOKEN,
		Message:    "Invalid or expired token",
		HTTPStatus: 401,
		Retryable:  false,
	},
	AUTH_INSUFFICIENT_PERMISSION: {
		Code:       AUTH_INSUFFICIENT_PERMISSION,
		Message:    "Insufficient permissions",
		HTTPStatus: 403,
		Retryable:  false,
	},
	AUTH_MFA_REQUIRED: {
		Code:       AUTH_MFA_REQUIRED,
		Message:    "MFA verification required",
		HTTPStatus: 403,
		Retryable:  false,
	},
	BILLING_INSUFFICIENT_BALANCE: {
		Code:       BILLING_INSUFFICIENT_BALANCE,
		Message:    "Insufficient balance",
		HTTPStatus: 402,
		Retryable:  false,
	},
	BILLING_CHARGE_FAILED: {
		Code:       BILLING_CHARGE_FAILED,
		Message:    "Charge failed",
		HTTPStatus: 500,
		Retryable:  true,
	},
	BILLING_REFUND_FAILED: {
		Code:       BILLING_REFUND_FAILED,
		Message:    "Refund failed",
		HTTPStatus: 500,
		Retryable:  true,
	},
	BILLING_DISCREPANCY: {
		Code:       BILLING_DISCREPANCY,
		Message:    "Billing discrepancy detected",
		HTTPStatus: 500,
		Retryable:  true,
	},
	ROUTER_NO_PROVIDER_AVAILABLE: {
		Code:       ROUTER_NO_PROVIDER_AVAILABLE,
		Message:    "No provider available",
		HTTPStatus: 503,
		Retryable:  true,
	},
	ROUTER_ALL_PROVIDERS_FAILED: {
		Code:       ROUTER_ALL_PROVIDERS_FAILED,
		Message:    "All providers failed",
		HTTPStatus: 503,
		Retryable:  true,
	},
	ROUTER_TIMEOUT: {
		Code:       ROUTER_TIMEOUT,
		Message:    "Request timeout",
		HTTPStatus: 504,
		Retryable:  true,
	},
	PROVIDER_INVALID_KEY: {
		Code:       PROVIDER_INVALID_KEY,
		Message:    "Invalid API key",
		HTTPStatus: 401,
		Retryable:  false,
	},
	PROVIDER_RATE_LIMIT: {
		Code:       PROVIDER_RATE_LIMIT,
		Message:    "Rate limit exceeded",
		HTTPStatus: 429,
		Retryable:  true,
	},
	PROVIDER_QUOTA_EXCEEDED: {
		Code:       PROVIDER_QUOTA_EXCEEDED,
		Message:    "Quota exceeded",
		HTTPStatus: 402,
		Retryable:  false,
	},
	PROVIDER_MODEL_NOT_FOUND: {
		Code:       PROVIDER_MODEL_NOT_FOUND,
		Message:    "Model not found",
		HTTPStatus: 404,
		Retryable:  false,
	},
	PROVIDER_ERROR: {
		Code:       PROVIDER_ERROR,
		Message:    "Provider error",
		HTTPStatus: 502,
		Retryable:  true,
	},
	RATE_LIMIT_EXCEEDED: {
		Code:       RATE_LIMIT_EXCEEDED,
		Message:    "Rate limit exceeded",
		HTTPStatus: 429,
		Retryable:  false,
	},
	RATE_LIMIT_TOKEN_EXCEEDED: {
		Code:       RATE_LIMIT_TOKEN_EXCEEDED,
		Message:    "Token limit exceeded",
		HTTPStatus: 429,
		Retryable:  false,
	},
	RATE_LIMIT_BURST_EXCEEDED: {
		Code:       RATE_LIMIT_BURST_EXCEEDED,
		Message:    "Burst limit exceeded",
		HTTPStatus: 429,
		Retryable:  false,
	},
	COMMON_INVALID_REQUEST: {
		Code:       COMMON_INVALID_REQUEST,
		Message:    "Invalid request",
		HTTPStatus: 400,
		Retryable:  false,
	},
	COMMON_RESOURCE_NOT_FOUND: {
		Code:       COMMON_RESOURCE_NOT_FOUND,
		Message:    "Resource not found",
		HTTPStatus: 404,
		Retryable:  false,
	},
	COMMON_INTERNAL_ERROR: {
		Code:       COMMON_INTERNAL_ERROR,
		Message:    "Internal error",
		HTTPStatus: 500,
		Retryable:  true,
	},
	COMMON_SERVICE_UNAVAILABLE: {
		Code:       COMMON_SERVICE_UNAVAILABLE,
		Message:    "Service unavailable",
		HTTPStatus: 503,
		Retryable:  true,
	},
	COMMON_REQUEST_TOO_LARGE: {
		Code:       COMMON_REQUEST_TOO_LARGE,
		Message:    "Request body too large",
		HTTPStatus: 413,
		Retryable:  false,
	},
}

// NewGatewayError 创建网关错误
func NewGatewayError(code ErrorCode, message string) *GatewayError {
	return &GatewayError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WithRequestID 设置请求ID
func (e *GatewayError) WithRequestID(requestID string) *GatewayError {
	e.RequestID = requestID
	return e
}

// WithDetail 设置详情
func (e *GatewayError) WithDetail(key string, value interface{}) *GatewayError {
	e.Details[key] = value
	return e
}

// WithInternal 设置内部错误
func (e *GatewayError) WithInternal(err error) *GatewayError {
	e.Internal = err
	return e
}

// GetErrorInfo 获取错误信息
func (e *GatewayError) GetErrorInfo() ErrorInfo {
	if info, ok := ErrorDefinitions[e.Code]; ok {
		return info
	}
	return ErrorInfo{
		Code:       COMMON_INTERNAL_ERROR,
		Message:    e.Message,
		HTTPStatus: 500,
		Retryable:  true,
	}
}
