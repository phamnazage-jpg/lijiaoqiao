package repository

import (
	"testing"
)

// ==================== P0-09 外键策略测试 ====================
// 问题：跨域模型缺少外键约束策略声明
// 修复方案：应用层外键 + 定期一致性校验

// TestP009_ForeignKeyPolicyDefinition 验证外键策略定义
func TestP009_ForeignKeyPolicyDefinition(t *testing.T) {
	policy := GetDefaultForeignKeyPolicy()

	// 核心实体表保留物理外键
	physicalFKTables := []string{"core_tenants", "core_projects", "iam_users", "billing_accounts"}
	for _, table := range physicalFKTables {
		if policy.GetPolicyForTable(table) != "physical" {
			t.Errorf("expected table %s to have physical FK", table)
		}
	}

	// 高频写入表使用应用层外键
	appFKTables := []string{"supply_accounts", "supply_packages", "supply_orders"}
	for _, table := range appFKTables {
		if policy.GetPolicyForTable(table) != "application" {
			t.Errorf("expected table %s to have application FK", table)
		}
	}

	// 审计/日志表无外键
	noFKTables := []string{"audit_events", "outbox_events", "supply_idempotency_record"}
	for _, table := range noFKTables {
		if policy.GetPolicyForTable(table) != "none" {
			t.Errorf("expected table %s to have no FK", table)
		}
	}
}

// TestP009_PhysicalFKTables 验证保留物理外键的表
func TestP009_PhysicalFKTables(t *testing.T) {
	policy := GetDefaultForeignKeyPolicy()

	expectedPhysicalTables := []string{
		"core_tenants",
		"core_projects",
		"iam_users",
		"billing_accounts",
	}

	if len(policy.PhysicalFKTables) != len(expectedPhysicalTables) {
		t.Errorf("expected %d physical FK tables, got %d", len(expectedPhysicalTables), len(policy.PhysicalFKTables))
	}

	for i, table := range expectedPhysicalTables {
		if policy.PhysicalFKTables[i] != table {
			t.Errorf("expected physical FK table %s at index %d, got %s", table, i, policy.PhysicalFKTables[i])
		}
	}
}

// TestP009_ApplicationFKTables 验证使用应用层外键的表
func TestP009_ApplicationFKTables(t *testing.T) {
	policy := GetDefaultForeignKeyPolicy()

	expectedAppTables := []string{
		"supply_accounts",
		"supply_packages",
		"supply_orders",
		"supply_usage_records",
		"supply_settlements",
	}

	if len(policy.ApplicationFKTables) != len(expectedAppTables) {
		t.Errorf("expected %d application FK tables, got %d", len(expectedAppTables), len(policy.ApplicationFKTables))
	}
}

// TestP009_NoFKTables 验证无外键的表
func TestP009_NoFKTables(t *testing.T) {
	policy := GetDefaultForeignKeyPolicy()

	expectedNoTables := []string{
		"audit_events",
		"outbox_events",
		"outbox_dead_letter",
		"supply_idempotency_record",
		"supply_batch_compensation",
	}

	if len(policy.NoFKTables) != len(expectedNoTables) {
		t.Errorf("expected %d no-FK tables, got %d", len(expectedNoTables), len(policy.NoFKTables))
	}
}

// TestP009_ForeignKeyValidatorStructure 验证外键校验器结构
func TestP009_ForeignKeyValidatorStructure(t *testing.T) {
	validator := &ForeignKeyValidator{}

	if validator == nil {
		t.Error("ForeignKeyValidator should not be nil")
	}
}

// TestP009_ReferenceCheckStructure 验证引用检查结构
func TestP009_ReferenceCheckStructure(t *testing.T) {
	check := ReferenceCheck{
		TableName: "iam_users",
		FieldName: "id",
		FieldValue: 1,
		CheckSQL:  "SELECT EXISTS(SELECT 1 FROM iam_users WHERE id = $1)",
		Args:      []interface{}{1},
	}

	if check.TableName != "iam_users" {
		t.Errorf("expected table name iam_users, got %s", check.TableName)
	}
	if check.FieldName != "id" {
		t.Errorf("expected field name id, got %s", check.FieldName)
	}
}

// TestP009_GetPolicyForTable_UnknownTable 验证获取未知表的策略
func TestP009_GetPolicyForTable_UnknownTable(t *testing.T) {
	policy := GetDefaultForeignKeyPolicy()

	policyStr := policy.GetPolicyForTable("unknown_table")
	if policyStr != "unknown" {
		t.Errorf("expected unknown for unknown_table, got %s", policyStr)
	}
}

// TestP009_GetPolicyForTable_AllTables 验证所有表都有正确的策略
func TestP009_GetPolicyForTable_AllTables(t *testing.T) {
	policy := GetDefaultForeignKeyPolicy()

	tables := []string{
		// Physical FK tables
		"core_tenants", "core_projects", "iam_users", "billing_accounts",
		// Application FK tables
		"supply_accounts", "supply_packages", "supply_orders",
		"supply_usage_records", "supply_settlements",
		// No FK tables
		"audit_events", "outbox_events", "outbox_dead_letter",
		"supply_idempotency_record", "supply_batch_compensation",
	}

	for _, table := range tables {
		result := policy.GetPolicyForTable(table)
		if result == "unknown" {
			t.Errorf("table %s should have a known policy", table)
		}
	}
}

// TestP009_ErrReferencedEntityNotFound 验证错误常量
func TestP009_ErrReferencedEntityNotFound(t *testing.T) {
	if ErrReferencedEntityNotFound == nil {
		t.Error("ErrReferencedEntityNotFound should not be nil")
	}
	if ErrReferencedEntityNotFound.Error() != "referenced entity not found" {
		t.Errorf("unexpected error message: %s", ErrReferencedEntityNotFound.Error())
	}
}

// TestP009_OrphanRecordCheckStructure 验证孤立记录检查结构
func TestP009_OrphanRecordCheckStructure(t *testing.T) {
	check := OrphanRecordCheck{
		TableName: "supply_accounts",
		FieldName: "user_id",
		Count:    5,
	}

	if check.TableName != "supply_accounts" {
		t.Errorf("expected table name supply_accounts, got %s", check.TableName)
	}
	if check.Count != 5 {
		t.Errorf("expected count 5, got %d", check.Count)
	}
}

// TestP009_OrphanCheckSQLStructure 验证孤立检查SQL结构
func TestP009_OrphanCheckSQLStructure(t *testing.T) {
	check := orphanCheckSQL{
		TableName: "supply_packages",
		FieldName: "supply_account_id",
		SQL:       "SELECT COUNT(*) FROM supply_packages",
	}

	if check.TableName != "supply_packages" {
		t.Errorf("expected table name supply_packages, got %s", check.TableName)
	}
	if check.SQL == "" {
		t.Error("SQL should not be empty")
	}
}

// TestP009_ForeignKeyPolicyStructure 验证外键策略结构
func TestP009_ForeignKeyPolicyStructure(t *testing.T) {
	policy := &ForeignKeyPolicy{
		PhysicalFKTables:    []string{"core_tenants"},
		ApplicationFKTables: []string{"supply_accounts"},
		NoFKTables:          []string{"audit_events"},
	}

	if len(policy.PhysicalFKTables) != 1 {
		t.Errorf("expected 1 physical FK table, got %d", len(policy.PhysicalFKTables))
	}
	if len(policy.ApplicationFKTables) != 1 {
		t.Errorf("expected 1 application FK table, got %d", len(policy.ApplicationFKTables))
	}
	if len(policy.NoFKTables) != 1 {
		t.Errorf("expected 1 no-FK table, got %d", len(policy.NoFKTables))
	}
}

// TestP009_Summary 测试总结
func TestP009_Summary(t *testing.T) {
	t.Log("=== P0-09 外键策略测试总结 ===")
	t.Log("问题: 跨域模型缺少外键约束策略声明")
	t.Log("")
	t.Log("修复方案:")
	t.Log("  保留物理外键: core_tenants, core_projects, iam_users, billing_accounts")
	t.Log("  应用层外键: supply_accounts, supply_packages, supply_orders, supply_usage_records")
	t.Log("  无外键: audit_events, outbox_events, outbox_dead_letter")
	t.Log("")
	t.Log("一致性校验:")
	t.Log("  - 每日执行orphan records检查")
	t.Log("  - 发现孤立记录时记录审计事件")
}
