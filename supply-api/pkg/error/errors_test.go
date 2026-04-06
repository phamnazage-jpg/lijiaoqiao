package error

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCodeError(t *testing.T) {
	err := NewCodeError("TEST_4001", "test error message")
	assert.Equal(t, "TEST_4001", err.Code)
	assert.Equal(t, "test error message", err.Message)
	assert.Nil(t, err.Err)
	assert.Equal(t, "TEST_4001: test error message", err.Error())
}

func TestWrapCodeError(t *testing.T) {
	originalErr := errors.New("original error")
	err := WrapCodeError(originalErr, "TEST_4001", "wrapped error")
	assert.Equal(t, "TEST_4001", err.Code)
	assert.Equal(t, "wrapped error", err.Message)
	assert.Equal(t, originalErr, err.Err)
	assert.Contains(t, err.Error(), "caused by: original error")
}

func TestIsCodeError(t *testing.T) {
	codeErr := NewCodeError("TEST_4001", "test")
	assert.True(t, IsCodeError(codeErr))

	stdErr := errors.New("standard error")
	assert.False(t, IsCodeError(stdErr))
}

func TestGetErrorCode(t *testing.T) {
	codeErr := NewCodeError("SUP_ACC_4001", "test")
	assert.Equal(t, "SUP_ACC_4001", GetErrorCode(codeErr))

	stdErr := errors.New("standard error")
	assert.Equal(t, "", GetErrorCode(stdErr))
}

func TestUnwrap(t *testing.T) {
	originalErr := errors.New("original")
	wrapped := WrapCodeError(originalErr, "TEST_4001", "wrapped")

	// 通过 Unwrap 获取原始错误
	unwrapped := wrapped.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
}

func TestValidateErrorCode(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
	}{
		{"SUP_ACC_4001", true},
		{"IAM_ROLE_4040", true},
		{"AUDIT_EVT_5000", true},
		{"REPO_NOT_FOUND", true},
		{"SYS_5000", true},
		{"INVALID", false},       // 没有下划线分隔
		{"BAD_CODE", false},      // 前缀不在白名单
		{"X_4001", false},       // 前缀不在白名单
		{"", false},              // 空字符串
		{"TOOLONG_4001", false},  // 前缀太长
	}

	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			result := ValidateErrorCode(tc.code)
			assert.Equal(t, tc.expected, result, "code: %s", tc.code)
		})
	}
}

func TestCommonErrors(t *testing.T) {
	assert.Equal(t, "SYS_4040", ErrNotFound.Code)
	assert.Equal(t, "resource not found", ErrNotFound.Message)

	assert.Equal(t, "SYS_4000", ErrInvalidInput.Code)
	assert.Equal(t, "invalid input parameter", ErrInvalidInput.Message)

	assert.Equal(t, "SYS_4010", ErrUnauthorized.Code)
	assert.Equal(t, "unauthorized", ErrUnauthorized.Message)

	assert.Equal(t, "SYS_4030", ErrForbidden.Code)
	assert.Equal(t, "forbidden", ErrForbidden.Message)

	assert.Equal(t, "SYS_5000", ErrInternalServer.Code)
	assert.Equal(t, "internal server error", ErrInternalServer.Message)

	assert.Equal(t, "SYS_4090", ErrConcurrencyConflict.Code)
	assert.Equal(t, "concurrency conflict", ErrConcurrencyConflict.Message)
}
