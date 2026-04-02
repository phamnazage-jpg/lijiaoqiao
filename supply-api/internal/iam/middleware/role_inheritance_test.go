package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRoleInheritance_OperatorInheritsViewer 测试运维人员继承查看者
func TestRoleInheritance_OperatorInheritsViewer(t *testing.T) {
	// arrange
	// operator 显式配置拥有 viewer 所有 scope + platform:write 等
	operatorScopes := []string{"platform:read", "platform:write", "tenant:read", "tenant:write", "billing:read"}
	viewerScopes := []string{"platform:read", "tenant:read", "billing:read"}

	operatorClaims := &IAMTokenClaims{
		SubjectID: "user:1",
		Role:      "operator",
		Scope:     operatorScopes,
		TenantID:  1,
	}

	ctx := context.WithValue(context.Background(), IAMTokenClaimsKey, *operatorClaims)

	// act & assert - operator 应该拥有 viewer 的所有 scope
	for _, viewerScope := range viewerScopes {
		assert.True(t, CheckScope(ctx, viewerScope),
			"operator should inherit viewer scope: %s", viewerScope)
	}

	// operator 还有额外的 scope
	assert.True(t, CheckScope(ctx, "platform:write"))
	assert.False(t, CheckScope(ctx, "platform:admin")) // viewer 没有 platform:admin
}

// TestRoleInheritance_ExplicitOverride 测试显式配置的Scope优先
func TestRoleInheritance_ExplicitOverride(t *testing.T) {
	// arrange
	// org_admin 显式配置拥有 operator + finops + developer + viewer 所有 scope
	orgAdminScopes := []string{
		// viewer scopes
		"platform:read", "tenant:read", "billing:read",
		// operator scopes
		"platform:write", "tenant:write",
		// finops scopes
		"billing:write",
		// developer scopes
		"router:model:list",
		// org_admin 自身 scope
		"platform:admin", "tenant:member:manage",
	}

	orgAdminClaims := &IAMTokenClaims{
		SubjectID: "user:2",
		Role:      "org_admin",
		Scope:     orgAdminScopes,
		TenantID:  1,
	}

	ctx := context.WithValue(context.Background(), IAMTokenClaimsKey, *orgAdminClaims)

	// act & assert - org_admin 应该拥有所有子角色的 scope
	assert.True(t, CheckScope(ctx, "platform:read"))   // viewer
	assert.True(t, CheckScope(ctx, "tenant:read"))    // viewer
	assert.True(t, CheckScope(ctx, "billing:read"))   // viewer/finops
	assert.True(t, CheckScope(ctx, "platform:write")) // operator
	assert.True(t, CheckScope(ctx, "tenant:write"))  // operator
	assert.True(t, CheckScope(ctx, "billing:write"))  // finops
	assert.True(t, CheckScope(ctx, "router:model:list")) // developer
	assert.True(t, CheckScope(ctx, "platform:admin")) // org_admin 自身
}

// TestRoleInheritance_ViewerDoesNotInherit 测试查看者不继承任何角色
func TestRoleInheritance_ViewerDoesNotInherit(t *testing.T) {
	// arrange
	viewerScopes := []string{"platform:read", "tenant:read", "billing:read"}

	viewerClaims := &IAMTokenClaims{
		SubjectID: "user:3",
		Role:      "viewer",
		Scope:     viewerScopes,
		TenantID:  1,
	}

	ctx := context.WithValue(context.Background(), IAMTokenClaimsKey, *viewerClaims)

	// act & assert - viewer 是基础角色，不继承任何角色
	assert.True(t, CheckScope(ctx, "platform:read"))
	assert.False(t, CheckScope(ctx, "platform:write"))  // viewer 没有 write
	assert.False(t, CheckScope(ctx, "platform:admin")) // viewer 没有 admin
}

// TestRoleInheritance_SupplyChain 测试供应方角色链
func TestRoleInheritance_SupplyChain(t *testing.T) {
	// arrange
	// supply_admin > supply_operator > supply_viewer
	supplyViewerScopes := []string{"supply:account:read", "supply:package:read"}
	supplyOperatorScopes := []string{"supply:account:read", "supply:account:write", "supply:package:read", "supply:package:write", "supply:package:publish"}
	supplyAdminScopes := []string{"supply:account:read", "supply:account:write", "supply:package:read", "supply:package:write", "supply:package:publish", "supply:package:offline", "supply:settlement:withdraw"}

	// supply_viewer 测试
	viewerCtx := context.WithValue(context.Background(), IAMTokenClaimsKey, IAMTokenClaims{
		SubjectID: "user:4",
		Role:      "supply_viewer",
		Scope:     supplyViewerScopes,
		TenantID:  1,
	})

	// act & assert
	assert.True(t, CheckScope(viewerCtx, "supply:account:read"))
	assert.False(t, CheckScope(viewerCtx, "supply:account:write"))

	// supply_operator 测试
	operatorCtx := context.WithValue(context.Background(), IAMTokenClaimsKey, IAMTokenClaims{
		SubjectID: "user:5",
		Role:      "supply_operator",
		Scope:     supplyOperatorScopes,
		TenantID:  1,
	})

	// act & assert - operator 继承 viewer
	assert.True(t, CheckScope(operatorCtx, "supply:account:read"))
	assert.True(t, CheckScope(operatorCtx, "supply:account:write"))
	assert.False(t, CheckScope(operatorCtx, "supply:settlement:withdraw")) // operator 没有 withdraw

	// supply_admin 测试
	adminCtx := context.WithValue(context.Background(), IAMTokenClaimsKey, IAMTokenClaims{
		SubjectID: "user:6",
		Role:      "supply_admin",
		Scope:     supplyAdminScopes,
		TenantID:  1,
	})

	// act & assert - admin 继承所有
	assert.True(t, CheckScope(adminCtx, "supply:account:read"))
	assert.True(t, CheckScope(adminCtx, "supply:settlement:withdraw"))
}

// TestRoleInheritance_ConsumerChain 测试需求方角色链
func TestRoleInheritance_ConsumerChain(t *testing.T) {
	// arrange
	// consumer_admin > consumer_operator > consumer_viewer
	consumerViewerScopes := []string{"consumer:account:read", "consumer:apikey:read", "consumer:usage:read"}
	consumerOperatorScopes := []string{"consumer:account:read", "consumer:account:write", "consumer:apikey:read", "consumer:apikey:create", "consumer:apikey:revoke", "consumer:usage:read"}
	consumerAdminScopes := []string{"consumer:account:read", "consumer:account:write", "consumer:apikey:read", "consumer:apikey:create", "consumer:apikey:revoke", "consumer:usage:read"}

	// consumer_viewer 测试
	viewerCtx := context.WithValue(context.Background(), IAMTokenClaimsKey, IAMTokenClaims{
		SubjectID: "user:7",
		Role:      "consumer_viewer",
		Scope:     consumerViewerScopes,
		TenantID:  1,
	})

	// act & assert
	assert.True(t, CheckScope(viewerCtx, "consumer:account:read"))
	assert.True(t, CheckScope(viewerCtx, "consumer:usage:read"))
	assert.False(t, CheckScope(viewerCtx, "consumer:apikey:create"))

	// consumer_operator 测试
	operatorCtx := context.WithValue(context.Background(), IAMTokenClaimsKey, IAMTokenClaims{
		SubjectID: "user:8",
		Role:      "consumer_operator",
		Scope:     consumerOperatorScopes,
		TenantID:  1,
	})

	// act & assert - operator 继承 viewer
	assert.True(t, CheckScope(operatorCtx, "consumer:apikey:create"))
	assert.True(t, CheckScope(operatorCtx, "consumer:apikey:revoke"))

	// consumer_admin 测试
	adminCtx := context.WithValue(context.Background(), IAMTokenClaimsKey, IAMTokenClaims{
		SubjectID: "user:9",
		Role:      "consumer_admin",
		Scope:     consumerAdminScopes,
		TenantID:  1,
	})

	// act & assert - admin 继承所有
	assert.True(t, CheckScope(adminCtx, "consumer:account:read"))
	assert.True(t, CheckScope(adminCtx, "consumer:apikey:revoke"))
}

// TestRoleInheritance_MultipleRoles 测试多角色继承（显式配置模拟）
func TestRoleInheritance_MultipleRoles(t *testing.T) {
	// arrange
	// 假设用户同时拥有 developer 和 finops 角色（通过 scope 累加）
	combinedScopes := []string{
		// viewer scopes
		"platform:read", "tenant:read", "billing:read",
		// developer scopes
		"router:model:list", "router:invoke",
		// finops scopes
		"billing:write",
	}

	combinedClaims := &IAMTokenClaims{
		SubjectID: "user:10",
		Role:      "developer", // 主角色
		Scope:     combinedScopes,
		TenantID:  1,
	}

	ctx := context.WithValue(context.Background(), IAMTokenClaimsKey, *combinedClaims)

	// act & assert
	assert.True(t, CheckScope(ctx, "platform:read"))     // viewer
	assert.True(t, CheckScope(ctx, "billing:read"))      // viewer
	assert.True(t, CheckScope(ctx, "router:model:list")) // developer
	assert.True(t, CheckScope(ctx, "billing:write"))      // finops
}

// TestRoleInheritance_SuperAdmin 测试超级管理员
func TestRoleInheritance_SuperAdmin(t *testing.T) {
	// arrange
	superAdminClaims := &IAMTokenClaims{
		SubjectID: "user:11",
		Role:      "super_admin",
		Scope:     []string{"*"}, // 通配符拥有所有权限
		TenantID:  0,
	}

	ctx := context.WithValue(context.Background(), IAMTokenClaimsKey, *superAdminClaims)

	// act & assert - super_admin 拥有所有 scope
	assert.True(t, CheckScope(ctx, "platform:read"))
	assert.True(t, CheckScope(ctx, "platform:admin"))
	assert.True(t, CheckScope(ctx, "supply:account:write"))
	assert.True(t, CheckScope(ctx, "consumer:apikey:create"))
	assert.True(t, CheckScope(ctx, "billing:write"))
}

// TestRoleInheritance_DeveloperInheritsViewer 测试开发者继承查看者
func TestRoleInheritance_DeveloperInheritsViewer(t *testing.T) {
	// arrange
	developerScopes := []string{"platform:read", "tenant:read", "billing:read", "router:invoke", "router:model:list"}

	developerClaims := &IAMTokenClaims{
		SubjectID: "user:12",
		Role:      "developer",
		Scope:     developerScopes,
		TenantID:  1,
	}

	ctx := context.WithValue(context.Background(), IAMTokenClaimsKey, *developerClaims)

	// act & assert - developer 继承 viewer 的所有 scope
	assert.True(t, CheckScope(ctx, "platform:read"))
	assert.True(t, CheckScope(ctx, "tenant:read"))
	assert.True(t, CheckScope(ctx, "billing:read"))
	assert.True(t, CheckScope(ctx, "router:invoke")) // developer 自身 scope
	assert.False(t, CheckScope(ctx, "platform:write")) // developer 没有 write
}

// TestRoleInheritance_FinopsInheritsViewer 测试财务人员继承查看者
func TestRoleInheritance_FinopsInheritsViewer(t *testing.T) {
	// arrange
	finopsScopes := []string{"platform:read", "tenant:read", "billing:read", "billing:write"}

	finopsClaims := &IAMTokenClaims{
		SubjectID: "user:13",
		Role:      "finops",
		Scope:     finopsScopes,
		TenantID:  1,
	}

	ctx := context.WithValue(context.Background(), IAMTokenClaimsKey, *finopsClaims)

	// act & assert - finops 继承 viewer 的所有 scope
	assert.True(t, CheckScope(ctx, "platform:read"))
	assert.True(t, CheckScope(ctx, "tenant:read"))
	assert.True(t, CheckScope(ctx, "billing:read"))
	assert.True(t, CheckScope(ctx, "billing:write")) // finops 自身 scope
	assert.False(t, CheckScope(ctx, "platform:write")) // finops 没有 write
}

// TestRoleInheritance_DeveloperDoesNotInheritOperator 测试开发者不继承运维
func TestRoleInheritance_DeveloperDoesNotInheritOperator(t *testing.T) {
	// arrange
	developerScopes := []string{"platform:read", "tenant:read", "billing:read", "router:invoke", "router:model:list"}

	developerClaims := &IAMTokenClaims{
		SubjectID: "user:14",
		Role:      "developer",
		Scope:     developerScopes,
		TenantID:  1,
	}

	ctx := context.WithValue(context.Background(), IAMTokenClaimsKey, *developerClaims)

	// act & assert - developer 不继承 operator 的 scope
	assert.False(t, CheckScope(ctx, "platform:write"))  // operator 有，developer 没有
	assert.False(t, CheckScope(ctx, "tenant:write"))   // operator 有，developer 没有
}
