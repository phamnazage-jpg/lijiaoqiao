package error

import (
	"errors"
	"testing"
)

func TestErrorCodes(t *testing.T) {
	// 验证所有错误码常量
	tests := []struct {
		code     ErrorCode
		expected string
	}{
		{AUTH_INVALID_TOKEN, "AUTH_001"},
		{AUTH_INSUFFICIENT_PERMISSION, "AUTH_002"},
		{AUTH_MFA_REQUIRED, "AUTH_003"},
		{BILLING_INSUFFICIENT_BALANCE, "BILLING_001"},
		{BILLING_CHARGE_FAILED, "BILLING_002"},
		{BILLING_REFUND_FAILED, "BILLING_003"},
		{BILLING_DISCREPANCY, "BILLING_004"},
		{ROUTER_NO_PROVIDER_AVAILABLE, "ROUTER_001"},
		{ROUTER_ALL_PROVIDERS_FAILED, "ROUTER_002"},
		{ROUTER_TIMEOUT, "ROUTER_003"},
		{PROVIDER_INVALID_KEY, "PROVIDER_001"},
		{PROVIDER_RATE_LIMIT, "PROVIDER_002"},
		{PROVIDER_QUOTA_EXCEEDED, "PROVIDER_003"},
		{PROVIDER_MODEL_NOT_FOUND, "PROVIDER_004"},
		{PROVIDER_ERROR, "PROVIDER_005"},
		{RATE_LIMIT_EXCEEDED, "RATE_LIMIT_001"},
		{RATE_LIMIT_TOKEN_EXCEEDED, "RATE_LIMIT_002"},
		{RATE_LIMIT_BURST_EXCEEDED, "RATE_LIMIT_003"},
		{COMMON_INVALID_REQUEST, "COMMON_001"},
		{COMMON_RESOURCE_NOT_FOUND, "COMMON_002"},
		{COMMON_INTERNAL_ERROR, "COMMON_003"},
		{COMMON_SERVICE_UNAVAILABLE, "COMMON_004"},
	}

	for _, tt := range tests {
		if string(tt.code) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.code)
		}
	}
}

func TestNewGatewayError(t *testing.T) {
	err := NewGatewayError(COMMON_INVALID_REQUEST, "test message")

	if err.Code != COMMON_INVALID_REQUEST {
		t.Errorf("expected code COMMON_INVALID_REQUEST, got %s", err.Code)
	}
	if err.Message != "test message" {
		t.Errorf("expected message 'test message', got %s", err.Message)
	}
	if err.Details == nil {
		t.Error("expected Details to be initialized")
	}
}

func TestGatewayError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *GatewayError
		expected string
	}{
		{
			name:     "without internal error",
			err:      NewGatewayError(COMMON_INVALID_REQUEST, "test"),
			expected: "COMMON_001: test",
		},
		{
			name:     "with internal error",
			err:      NewGatewayError(COMMON_INTERNAL_ERROR, "outer").WithInternal(errors.New("inner")),
			expected: "COMMON_003: outer (caused by: inner)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGatewayError_Unwrap(t *testing.T) {
	internalErr := errors.New("inner error")
	err := NewGatewayError(COMMON_INTERNAL_ERROR, "outer").WithInternal(internalErr)

	if err.Unwrap() != internalErr {
		t.Error("Unwrap() should return the internal error")
	}
}

func TestGatewayError_WithRequestID(t *testing.T) {
	err := NewGatewayError(COMMON_INVALID_REQUEST, "test")
	result := err.WithRequestID("req-123")

	if err.RequestID != "req-123" {
		t.Errorf("expected RequestID req-123, got %s", err.RequestID)
	}
	if result != err {
		t.Error("WithRequestID should return the same error")
	}
}

func TestGatewayError_WithDetail(t *testing.T) {
	err := NewGatewayError(COMMON_INVALID_REQUEST, "test")
	result := err.WithDetail("key", "value")

	if err.Details["key"] != "value" {
		t.Errorf("expected Details[key] = value, got %v", err.Details["key"])
	}
	if result != err {
		t.Error("WithDetail should return the same error")
	}
}

func TestGatewayError_WithInternal(t *testing.T) {
	internalErr := errors.New("internal error")
	err := NewGatewayError(COMMON_INVALID_REQUEST, "test")
	result := err.WithInternal(internalErr)

	if err.Internal != internalErr {
		t.Error("expected Internal to be set")
	}
	if result != err {
		t.Error("WithInternal should return the same error")
	}
}

func TestGetErrorInfo(t *testing.T) {
	tests := []struct {
		code           ErrorCode
		expectedStatus int
		expectedRetry  bool
	}{
		{AUTH_INVALID_TOKEN, 401, false},
		{AUTH_INSUFFICIENT_PERMISSION, 403, false},
		{AUTH_MFA_REQUIRED, 403, false},
		{BILLING_INSUFFICIENT_BALANCE, 402, false},
		{BILLING_CHARGE_FAILED, 500, true},
		{BILLING_REFUND_FAILED, 500, true},
		{BILLING_DISCREPANCY, 500, true},
		{ROUTER_NO_PROVIDER_AVAILABLE, 503, true},
		{ROUTER_ALL_PROVIDERS_FAILED, 503, true},
		{ROUTER_TIMEOUT, 504, true},
		{PROVIDER_INVALID_KEY, 401, false},
		{PROVIDER_RATE_LIMIT, 429, true},
		{PROVIDER_QUOTA_EXCEEDED, 402, false},
		{PROVIDER_MODEL_NOT_FOUND, 404, false},
		{PROVIDER_ERROR, 502, true},
		{RATE_LIMIT_EXCEEDED, 429, false},
		{RATE_LIMIT_TOKEN_EXCEEDED, 429, false},
		{RATE_LIMIT_BURST_EXCEEDED, 429, false},
		{COMMON_INVALID_REQUEST, 400, false},
		{COMMON_RESOURCE_NOT_FOUND, 404, false},
		{COMMON_INTERNAL_ERROR, 500, true},
		{COMMON_SERVICE_UNAVAILABLE, 503, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			err := NewGatewayError(tt.code, "test")
			info := err.GetErrorInfo()

			if info.HTTPStatus != tt.expectedStatus {
				t.Errorf("code %s: expected status %d, got %d", tt.code, tt.expectedStatus, info.HTTPStatus)
			}
			if info.Retryable != tt.expectedRetry {
				t.Errorf("code %s: expected retryable %v, got %v", tt.code, tt.expectedRetry, info.Retryable)
			}
		})
	}
}

func TestGetErrorInfo_UnknownCode(t *testing.T) {
	err := NewGatewayError("UNKNOWN_CODE", "test")
	info := err.GetErrorInfo()

	// 未知错误码应返回默认值
	if info.HTTPStatus != 500 {
		t.Errorf("expected status 500, got %d", info.HTTPStatus)
	}
	if info.Retryable != true {
		t.Error("expected retryable true for unknown code")
	}
	if info.Code != COMMON_INTERNAL_ERROR {
		t.Errorf("expected code COMMON_INTERNAL_ERROR, got %s", info.Code)
	}
}

func TestErrorInfo_Struct(t *testing.T) {
	info := ErrorInfo{
		Code:       COMMON_INVALID_REQUEST,
		Message:    "test message",
		HTTPStatus: 400,
		Retryable:  false,
	}

	if info.Code != COMMON_INVALID_REQUEST {
		t.Errorf("expected code COMMON_INVALID_REQUEST, got %s", info.Code)
	}
	if info.Message != "test message" {
		t.Errorf("expected message 'test message', got %s", info.Message)
	}
	if info.HTTPStatus != 400 {
		t.Errorf("expected HTTPStatus 400, got %d", info.HTTPStatus)
	}
	if info.Retryable != false {
		t.Error("expected Retryable false")
	}
}

func TestGatewayError_Chaining(t *testing.T) {
	err := NewGatewayError(COMMON_INVALID_REQUEST, "test").
		WithRequestID("req-123").
		WithDetail("field", "email").
		WithDetail("reason", "invalid format")

	if err.RequestID != "req-123" {
		t.Errorf("expected RequestID req-123, got %s", err.RequestID)
	}
	if err.Details["field"] != "email" {
		t.Errorf("expected field=email, got %v", err.Details["field"])
	}
	if err.Details["reason"] != "invalid format" {
		t.Errorf("expected reason=invalid format, got %v", err.Details["reason"])
	}
}

func TestErrorDefinitions_Completeness(t *testing.T) {
	// 确保所有错误码都在ErrorDefinitions中定义
	codes := []ErrorCode{
		AUTH_INVALID_TOKEN,
		AUTH_INSUFFICIENT_PERMISSION,
		AUTH_MFA_REQUIRED,
		BILLING_INSUFFICIENT_BALANCE,
		BILLING_CHARGE_FAILED,
		BILLING_REFUND_FAILED,
		BILLING_DISCREPANCY,
		ROUTER_NO_PROVIDER_AVAILABLE,
		ROUTER_ALL_PROVIDERS_FAILED,
		ROUTER_TIMEOUT,
		PROVIDER_INVALID_KEY,
		PROVIDER_RATE_LIMIT,
		PROVIDER_QUOTA_EXCEEDED,
		PROVIDER_MODEL_NOT_FOUND,
		PROVIDER_ERROR,
		RATE_LIMIT_EXCEEDED,
		RATE_LIMIT_TOKEN_EXCEEDED,
		RATE_LIMIT_BURST_EXCEEDED,
		COMMON_INVALID_REQUEST,
		COMMON_RESOURCE_NOT_FOUND,
		COMMON_INTERNAL_ERROR,
		COMMON_SERVICE_UNAVAILABLE,
	}

	for _, code := range codes {
		if _, ok := ErrorDefinitions[code]; !ok {
			t.Errorf("code %s not found in ErrorDefinitions", code)
		}
	}
}

func TestErrorDefinitions_Consistency(t *testing.T) {
	for code, info := range ErrorDefinitions {
		if info.Code != code {
			t.Errorf("ErrorDefinitions[%s].Code = %s, expected %s", code, info.Code, code)
		}
	}
}

func TestGatewayError_ImplementsErrorInterface(t *testing.T) {
	err := NewGatewayError(COMMON_INVALID_REQUEST, "test")

	var e error = err
	if e.Error() != "COMMON_001: test" {
		t.Error("GatewayError should implement error interface")
	}
}

func TestGatewayError_ErrorWithWrappedError(t *testing.T) {
	wrapped := errors.New("wrapped error")
	err := NewGatewayError(COMMON_INTERNAL_ERROR, "outer error").WithInternal(wrapped)

	// Error()应该包含wrapped error的信息
	expected := "COMMON_003: outer error (caused by: wrapped error)"
	if err.Error() != expected {
		t.Errorf("expected %s, got %s", expected, err.Error())
	}
}

func TestNewGatewayError_EmptyMessage(t *testing.T) {
	err := NewGatewayError(COMMON_INVALID_REQUEST, "")

	if err.Message != "" {
		t.Errorf("expected empty message, got %s", err.Message)
	}
}

func TestGetErrorInfo_ErrorDefinitions(t *testing.T) {
	info := ErrorDefinitions[AUTH_INVALID_TOKEN]

	if info.Code != AUTH_INVALID_TOKEN {
		t.Errorf("expected AUTH_INVALID_TOKEN, got %s", info.Code)
	}
	if info.Message != "Invalid or expired token" {
		t.Errorf("unexpected message: %s", info.Message)
	}
	if info.HTTPStatus != 401 {
		t.Errorf("expected 401, got %d", info.HTTPStatus)
	}
	if info.Retryable != false {
		t.Error("expected non-retryable")
	}
}

func TestErrorCode_Type(t *testing.T) {
	var code ErrorCode = "TEST_001"
	if string(code) != "TEST_001" {
		t.Errorf("expected TEST_001, got %s", code)
	}
}
