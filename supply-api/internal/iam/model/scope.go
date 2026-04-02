package model

import (
	"errors"
	"strings"
	"time"
)

// Scope类型常量
const (
	ScopeTypePlatform = "platform"
	ScopeTypeSupply   = "supply"
	ScopeTypeConsumer = "consumer"
	ScopeTypeRouter   = "router"
	ScopeTypeBilling  = "billing"
)

// Scope错误定义
var (
	ErrInvalidScopeCode = errors.New("invalid scope code: cannot be empty")
	ErrInvalidScopeType = errors.New("invalid scope type: must be platform, supply, consumer, router, or billing")
)

// Scope Scope模型
// 对应数据库 iam_scopes 表
type Scope struct {
	ID          int64   // 主键ID
	Code        string  // Scope代码 (unique): platform:read, supply:account:write
	Name        string  // Scope名称
	Type        string  // Scope类型: platform, supply, consumer, router, billing
	Description string  // 描述
	IsActive    bool    // 是否激活

	// 审计字段
	RequestID string // 请求追踪ID
	CreatedIP string // 创建者IP
	UpdatedIP string // 更新者IP
	Version   int    // 乐观锁版本号

	// 时间戳
	CreatedAt *time.Time // 创建时间
	UpdatedAt *time.Time // 更新时间
}

// NewScope 创建新Scope（基础构造函数）
func NewScope(code, name, scopeType string) *Scope {
	now := time.Now()
	return &Scope{
		Code:        code,
		Name:        name,
		Type:        scopeType,
		IsActive:    true,
		RequestID:   generateRequestID(),
		Version:     1,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
}

// NewScopeWithRequestID 创建带指定RequestID的Scope
func NewScopeWithRequestID(code, name, scopeType string, requestID string) *Scope {
	scope := NewScope(code, name, scopeType)
	scope.RequestID = requestID
	return scope
}

// NewScopeWithAudit 创建带审计信息的Scope
func NewScopeWithAudit(code, name, scopeType string, requestID, createdIP, updatedIP string) *Scope {
	scope := NewScope(code, name, scopeType)
	scope.RequestID = requestID
	scope.CreatedIP = createdIP
	scope.UpdatedIP = updatedIP
	return scope
}

// NewScopeWithValidation 创建Scope并进行验证
func NewScopeWithValidation(code, name, scopeType string) (*Scope, error) {
	// 验证Scope代码
	if code == "" {
		return nil, ErrInvalidScopeCode
	}

	// 验证Scope类型
	if !IsValidScopeType(scopeType) {
		return nil, ErrInvalidScopeType
	}

	scope := NewScope(code, name, scopeType)
	return scope, nil
}

// Activate 激活Scope
func (s *Scope) Activate() {
	s.IsActive = true
	s.UpdatedAt = nowPtr()
}

// Deactivate 停用Scope
func (s *Scope) Deactivate() {
	s.IsActive = false
	s.UpdatedAt = nowPtr()
}

// IncrementVersion 递增版本号（用于乐观锁）
func (s *Scope) IncrementVersion() {
	s.Version++
	s.UpdatedAt = nowPtr()
}

// IsWildcard 检查是否为通配符Scope
func (s *Scope) IsWildcard() bool {
	return s.Code == "*"
}

// ToScopeInfo 转换为ScopeInfo结构（用于API响应）
func (s *Scope) ToScopeInfo() *ScopeInfo {
	return &ScopeInfo{
		ScopeCode: s.Code,
		ScopeName: s.Name,
		ScopeType: s.Type,
		IsActive:  s.IsActive,
	}
}

// ScopeInfo Scope信息（用于API响应）
type ScopeInfo struct {
	ScopeCode string `json:"scope_code"`
	ScopeName string `json:"scope_name"`
	ScopeType string `json:"scope_type"`
	IsActive  bool   `json:"is_active"`
}

// IsValidScopeType 验证Scope类型是否有效
func IsValidScopeType(scopeType string) bool {
	switch scopeType {
	case ScopeTypePlatform, ScopeTypeSupply, ScopeTypeConsumer, ScopeTypeRouter, ScopeTypeBilling:
		return true
	default:
		return false
	}
}

// GetScopeTypeFromCode 从Scope Code推断Scope类型
// 例如: platform:read -> platform, supply:account:write -> supply, consumer:apikey:create -> consumer
func GetScopeTypeFromCode(scopeCode string) string {
	parts := strings.SplitN(scopeCode, ":", 2)
	if len(parts) < 1 {
		return ""
	}

	prefix := parts[0]
	switch prefix {
	case "platform", "tenant", "billing":
		return ScopeTypePlatform
	case "supply":
		return ScopeTypeSupply
	case "consumer":
		return ScopeTypeConsumer
	case "router":
		return ScopeTypeRouter
	default:
		return ""
	}
}

// PredefinedScopes 预定义的Scope列表
var PredefinedScopes = []*Scope{
	// Platform Scopes
	{Code: "platform:read", Name: "读取平台配置", Type: ScopeTypePlatform},
	{Code: "platform:write", Name: "修改平台配置", Type: ScopeTypePlatform},
	{Code: "platform:admin", Name: "平台级管理", Type: ScopeTypePlatform},
	{Code: "platform:audit:read", Name: "读取审计日志", Type: ScopeTypePlatform},
	{Code: "platform:audit:export", Name: "导出审计日志", Type: ScopeTypePlatform},

	// Tenant Scopes (属于platform类型)
	{Code: "tenant:read", Name: "读取租户信息", Type: ScopeTypePlatform},
	{Code: "tenant:write", Name: "修改租户配置", Type: ScopeTypePlatform},
	{Code: "tenant:member:manage", Name: "管理租户成员", Type: ScopeTypePlatform},
	{Code: "tenant:billing:write", Name: "修改账单设置", Type: ScopeTypePlatform},

	// Supply Scopes
	{Code: "supply:account:read", Name: "读取供应账号", Type: ScopeTypeSupply},
	{Code: "supply:account:write", Name: "管理供应账号", Type: ScopeTypeSupply},
	{Code: "supply:package:read", Name: "读取套餐信息", Type: ScopeTypeSupply},
	{Code: "supply:package:write", Name: "管理套餐", Type: ScopeTypeSupply},
	{Code: "supply:package:publish", Name: "发布套餐", Type: ScopeTypeSupply},
	{Code: "supply:package:offline", Name: "下架套餐", Type: ScopeTypeSupply},
	{Code: "supply:settlement:withdraw", Name: "提现", Type: ScopeTypeSupply},
	{Code: "supply:credential:manage", Name: "管理凭证", Type: ScopeTypeSupply},

	// Consumer Scopes
	{Code: "consumer:account:read", Name: "读取账户信息", Type: ScopeTypeConsumer},
	{Code: "consumer:account:write", Name: "管理账户", Type: ScopeTypeConsumer},
	{Code: "consumer:apikey:create", Name: "创建API Key", Type: ScopeTypeConsumer},
	{Code: "consumer:apikey:read", Name: "读取API Key", Type: ScopeTypeConsumer},
	{Code: "consumer:apikey:revoke", Name: "吊销API Key", Type: ScopeTypeConsumer},
	{Code: "consumer:usage:read", Name: "读取使用量", Type: ScopeTypeConsumer},

	// Billing Scopes
	{Code: "billing:read", Name: "读取账单", Type: ScopeTypeBilling},
	{Code: "billing:write", Name: "修改账单设置", Type: ScopeTypeBilling},

	// Router Scopes
	{Code: "router:invoke", Name: "调用模型", Type: ScopeTypeRouter},
	{Code: "router:model:list", Name: "列出可用模型", Type: ScopeTypeRouter},
	{Code: "router:model:config", Name: "配置路由策略", Type: ScopeTypeRouter},

	// Wildcard Scope
	{Code: "*", Name: "通配符", Type: ScopeTypePlatform},
}

// GetPredefinedScopeByCode 根据Code获取预定义Scope
func GetPredefinedScopeByCode(code string) *Scope {
	for _, scope := range PredefinedScopes {
		if scope.Code == code {
			return scope
		}
	}
	return nil
}

// IsPredefinedScope 检查是否为预定义Scope
func IsPredefinedScope(code string) bool {
	return GetPredefinedScopeByCode(code) != nil
}
