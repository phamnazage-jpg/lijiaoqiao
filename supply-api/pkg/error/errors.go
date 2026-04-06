package error

import (
	"fmt"
	"strings"
)

// ErrorCode 错误码格式：{DOMAIN}_{CODE}
// 错误码命名规范：{模块}_{问题类型}_{序号}
//
// 示例：
//   - SUP_ACC_4001 (Supplier Account - 业务错误 - 4001)
//   - AUDIT_EVT_4041 (Audit Event - 资源不存在 - 4041)
//
// 错误码分类：
//   - 4xxx: 业务逻辑错误
//   - 5xxx: 系统/服务器错误
//   - 9xxx: 内部/未知错误

// 预定义的错误码前缀
const (
	PrefixSUP   = "SUP"   // Supplier 模块
	PrefixIAM   = "IAM"   // Identity & Access Management 模块
	PrefixAudit = "AUDIT" // Audit 模块
	PrefixRepo = "REPO"  // Repository 模块
	PrefixSys  = "SYS"   // 系统级错误
)

// CodeError 带错误码的错误
type CodeError struct {
	Code    string // 错误码，如 "SUP_ACC_4001"
	Message string // 错误消息
	Err     error  // 底层错误（可选）
}

// Error 实现 error 接口
func (e *CodeError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 获取底层错误
func (e *CodeError) Unwrap() error {
	return e.Err
}

// NewCodeError 创建带错误码的错误
func NewCodeError(code, message string) *CodeError {
	return &CodeError{
		Code:    code,
		Message: message,
	}
}

// WrapCodeError 包装已有错误
func WrapCodeError(err error, code, message string) *CodeError {
	return &CodeError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// IsCodeError 检查错误是否为 CodeError
func IsCodeError(err error) bool {
	_, ok := err.(*CodeError)
	return ok
}

// GetErrorCode 从错误中提取错误码
func GetErrorCode(err error) string {
	var codeErr *CodeError
	if As(err, &codeErr) {
		return codeErr.Code
	}
	return ""
}

// As 类型断言辅助函数
func As(err error, target **CodeError) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*CodeError); ok {
		*target = e
		return true
	}
	if e, ok := err.(interface{ Unwrap() error }); ok {
		return As(e.Unwrap(), target)
	}
	return false
}

// Common errors - 可以被各模块引用的通用错误
var (
	// ErrNotFound 资源不存在
	ErrNotFound = NewCodeError("SYS_4040", "resource not found")

	// ErrInvalidInput 输入参数无效
	ErrInvalidInput = NewCodeError("SYS_4000", "invalid input parameter")

	// ErrUnauthorized 未授权
	ErrUnauthorized = NewCodeError("SYS_4010", "unauthorized")

	// ErrForbidden 禁止访问
	ErrForbidden = NewCodeError("SYS_4030", "forbidden")

	// ErrInternalServer 服务器内部错误
	ErrInternalServer = NewCodeError("SYS_5000", "internal server error")

	// ErrConcurrencyConflict 并发冲突
	ErrConcurrencyConflict = NewCodeError("SYS_4090", "concurrency conflict")
)

// ValidateErrorCode 验证错误码格式是否合法
func ValidateErrorCode(code string) bool {
	parts := strings.Split(code, "_")
	if len(parts) < 2 {
		return false
	}
	// 检查前缀是否为有效值
	prefix := parts[0]
	validPrefixes := []string{PrefixSUP, PrefixIAM, PrefixAudit, PrefixRepo, PrefixSys}
	for _, p := range validPrefixes {
		if prefix == p {
			return true
		}
	}
	return false
}
