package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestScopeModel_NewScope_ValidInput 测试创建Scope - 有效输入
func TestScopeModel_NewScope_ValidInput(t *testing.T) {
	// arrange
	scopeCode := "platform:read"
	scopeName := "读取平台配置"
	scopeType := "platform"

	// act
	scope := NewScope(scopeCode, scopeName, scopeType)

	// assert
	assert.Equal(t, scopeCode, scope.Code)
	assert.Equal(t, scopeName, scope.Name)
	assert.Equal(t, scopeType, scope.Type)
	assert.True(t, scope.IsActive)
	assert.NotEmpty(t, scope.RequestID)
	assert.Equal(t, 1, scope.Version)
}

// TestScopeModel_ScopeCategories 测试Scope分类
func TestScopeModel_ScopeCategories(t *testing.T) {
	// arrange & act
	testCases := []struct {
		scopeCode string
		expectedType string
	}{
		// platform:* 分类
		{"platform:read", ScopeTypePlatform},
		{"platform:write", ScopeTypePlatform},
		{"platform:admin", ScopeTypePlatform},
		{"platform:audit:read", ScopeTypePlatform},
		{"platform:audit:export", ScopeTypePlatform},

		// tenant:* 分类
		{"tenant:read", ScopeTypePlatform},
		{"tenant:write", ScopeTypePlatform},
		{"tenant:member:manage", ScopeTypePlatform},

		// supply:* 分类
		{"supply:account:read", ScopeTypeSupply},
		{"supply:account:write", ScopeTypeSupply},
		{"supply:package:read", ScopeTypeSupply},
		{"supply:package:write", ScopeTypeSupply},

		// consumer:* 分类
		{"consumer:account:read", ScopeTypeConsumer},
		{"consumer:apikey:create", ScopeTypeConsumer},

		// billing:* 分类
		{"billing:read", ScopeTypePlatform},

		// router:* 分类
		{"router:invoke", ScopeTypeRouter},
		{"router:model:list", ScopeTypeRouter},
	}

	// assert
	for _, tc := range testCases {
		scope := NewScope(tc.scopeCode, tc.scopeCode, tc.expectedType)
		assert.Equal(t, tc.expectedType, scope.Type, "scope %s should be type %s", tc.scopeCode, tc.expectedType)
	}
}

// TestScopeModel_NewScope_DefaultFields 测试创建Scope - 默认字段
func TestScopeModel_NewScope_DefaultFields(t *testing.T) {
	// arrange
	scopeCode := "tenant:read"
	scopeName := "读取租户信息"
	scopeType := ScopeTypePlatform

	// act
	scope := NewScope(scopeCode, scopeName, scopeType)

	// assert - 验证默认字段
	assert.Equal(t, 1, scope.Version, "version should default to 1")
	assert.NotEmpty(t, scope.RequestID, "request_id should be auto-generated")
	assert.True(t, scope.IsActive, "is_active should default to true")
}

// TestScopeModel_NewScope_WithRequestID 测试创建Scope - 指定RequestID
func TestScopeModel_NewScope_WithRequestID(t *testing.T) {
	// arrange
	requestID := "req-54321"

	// act
	scope := NewScopeWithRequestID("platform:read", "读取平台配置", ScopeTypePlatform, requestID)

	// assert
	assert.Equal(t, requestID, scope.RequestID)
}

// TestScopeModel_NewScope_AuditFields 测试创建Scope - 审计字段
func TestScopeModel_NewScope_AuditFields(t *testing.T) {
	// arrange
	createdIP := "10.0.0.1"
	updatedIP := "10.0.0.2"

	// act
	scope := NewScopeWithAudit("billing:read", "读取账单", ScopeTypePlatform, "req-789", createdIP, updatedIP)

	// assert
	assert.Equal(t, createdIP, scope.CreatedIP)
	assert.Equal(t, updatedIP, scope.UpdatedIP)
	assert.Equal(t, 1, scope.Version)
}

// TestScopeModel_Activate 测试激活Scope
func TestScopeModel_Activate(t *testing.T) {
	// arrange
	scope := NewScope("test:scope", "测试Scope", ScopeTypePlatform)
	scope.IsActive = false

	// act
	scope.Activate()

	// assert
	assert.True(t, scope.IsActive)
}

// TestScopeModel_Deactivate 测试停用Scope
func TestScopeModel_Deactivate(t *testing.T) {
	// arrange
	scope := NewScope("test:scope", "测试Scope", ScopeTypePlatform)

	// act
	scope.Deactivate()

	// assert
	assert.False(t, scope.IsActive)
}

// TestScopeModel_IncrementVersion 测试版本号递增
func TestScopeModel_IncrementVersion(t *testing.T) {
	// arrange
	scope := NewScope("test:scope", "测试Scope", ScopeTypePlatform)
	originalVersion := scope.Version

	// act
	scope.IncrementVersion()

	// assert
	assert.Equal(t, originalVersion+1, scope.Version)
}

// TestScopeModel_ScopeType_Platform 测试平台Scope类型
func TestScopeModel_ScopeType_Platform(t *testing.T) {
	// arrange & act
	scope := NewScope("platform:admin", "平台管理", ScopeTypePlatform)

	// assert
	assert.Equal(t, ScopeTypePlatform, scope.Type)
}

// TestScopeModel_ScopeType_Supply 测试供应方Scope类型
func TestScopeModel_ScopeType_Supply(t *testing.T) {
	// arrange & act
	scope := NewScope("supply:account:write", "管理供应账号", ScopeTypeSupply)

	// assert
	assert.Equal(t, ScopeTypeSupply, scope.Type)
}

// TestScopeModel_ScopeType_Consumer 测试需求方Scope类型
func TestScopeModel_ScopeType_Consumer(t *testing.T) {
	// arrange & act
	scope := NewScope("consumer:apikey:create", "创建API Key", ScopeTypeConsumer)

	// assert
	assert.Equal(t, ScopeTypeConsumer, scope.Type)
}

// TestScopeModel_ScopeType_Router 测试路由Scope类型
func TestScopeModel_ScopeType_Router(t *testing.T) {
	// arrange & act
	scope := NewScope("router:invoke", "调用模型", ScopeTypeRouter)

	// assert
	assert.Equal(t, ScopeTypeRouter, scope.Type)
}

// TestScopeModel_NewScope_EmptyCode 测试创建Scope - 空Scope代码（应返回错误）
func TestScopeModel_NewScope_EmptyCode(t *testing.T) {
	// arrange & act
	scope, err := NewScopeWithValidation("", "测试Scope", ScopeTypePlatform)

	// assert
	assert.Error(t, err)
	assert.Nil(t, scope)
	assert.Equal(t, ErrInvalidScopeCode, err)
}

// TestScopeModel_NewScope_InvalidScopeType 测试创建Scope - 无效Scope类型
func TestScopeModel_NewScope_InvalidScopeType(t *testing.T) {
	// arrange & act
	scope, err := NewScopeWithValidation("test:scope", "测试Scope", "invalid_type")

	// assert
	assert.Error(t, err)
	assert.Nil(t, scope)
	assert.Equal(t, ErrInvalidScopeType, err)
}

// TestScopeModel_ToScopeInfo 测试Scope转换为ScopeInfo
func TestScopeModel_ToScopeInfo(t *testing.T) {
	// arrange
	scope := NewScope("platform:read", "读取平台配置", ScopeTypePlatform)
	scope.ID = 1

	// act
	scopeInfo := scope.ToScopeInfo()

	// assert
	assert.Equal(t, "platform:read", scopeInfo.ScopeCode)
	assert.Equal(t, "读取平台配置", scopeInfo.ScopeName)
	assert.Equal(t, ScopeTypePlatform, scopeInfo.ScopeType)
	assert.True(t, scopeInfo.IsActive)
}

// TestScopeModel_GetScopeTypeFromCode 测试从Scope Code推断类型
func TestScopeModel_GetScopeTypeFromCode(t *testing.T) {
	// arrange & act & assert
	assert.Equal(t, ScopeTypePlatform, GetScopeTypeFromCode("platform:read"))
	assert.Equal(t, ScopeTypePlatform, GetScopeTypeFromCode("tenant:read"))
	assert.Equal(t, ScopeTypeSupply, GetScopeTypeFromCode("supply:account:read"))
	assert.Equal(t, ScopeTypeConsumer, GetScopeTypeFromCode("consumer:apikey:read"))
	assert.Equal(t, ScopeTypeRouter, GetScopeTypeFromCode("router:invoke"))
	assert.Equal(t, ScopeTypePlatform, GetScopeTypeFromCode("billing:read"))
}

// TestScopeModel_IsWildcardScope 测试通配符Scope
func TestScopeModel_IsWildcardScope(t *testing.T) {
	// arrange
	wildcardScope := NewScope("*", "通配符", ScopeTypePlatform)
	normalScope := NewScope("platform:read", "读取平台配置", ScopeTypePlatform)

	// assert
	assert.True(t, wildcardScope.IsWildcard())
	assert.False(t, normalScope.IsWildcard())
}
