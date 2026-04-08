package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ==================== P0-09 外键约束策略 ====================
// 问题：跨域模型缺少外键约束策略声明
// 修复方案：应用层外键 + 定期一致性校验

// ForeignKeyValidator 应用层外键校验器
type ForeignKeyValidator struct {
	pool *pgxpool.Pool
}

// NewForeignKeyValidator 创建外键校验器
func NewForeignKeyValidator(pool *pgxpool.Pool) *ForeignKeyValidator {
	return &ForeignKeyValidator{pool: pool}
}

// ValidateReference 验证引用完整性
func (v *ForeignKeyValidator) ValidateReference(ctx context.Context, ref ReferenceCheck) error {
	var exists bool
	err := v.pool.QueryRow(ctx, ref.CheckSQL, ref.Args...).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check reference: %w", err)
	}
	if !exists {
		return ErrReferencedEntityNotFound
	}
	return nil
}

// ReferenceCheck 引用检查
type ReferenceCheck struct {
	TableName string
	FieldName string
	FieldValue interface{}
	CheckSQL  string
	Args      []interface{}
}

// ErrReferencedEntityNotFound 引用实体不存在
var ErrReferencedEntityNotFound = fmt.Errorf("referenced entity not found")

// ValidateSupplyAccountOwner 校验供应账号所属用户存在
func (v *ForeignKeyValidator) ValidateSupplyAccountOwner(ctx context.Context, userID int64) error {
	return v.ValidateReference(ctx, ReferenceCheck{
		TableName: "iam_users",
		FieldName: "id",
		FieldValue: userID,
		CheckSQL:  "SELECT EXISTS(SELECT 1 FROM iam_users WHERE id = $1)",
		Args:      []interface{}{userID},
	})
}

// ValidatePackageSupplyAccount 校验套餐所属供应账号存在
func (v *ForeignKeyValidator) ValidatePackageSupplyAccount(ctx context.Context, accountID int64) error {
	return v.ValidateReference(ctx, ReferenceCheck{
		TableName: "supply_accounts",
		FieldName: "id",
		FieldValue: accountID,
		CheckSQL:  "SELECT EXISTS(SELECT 1 FROM supply_accounts WHERE id = $1)",
		Args:      []interface{}{accountID},
	})
}

// ValidateOrderSupplyAccount 校验订单所属供应账号存在
func (v *ForeignKeyValidator) ValidateOrderSupplyAccount(ctx context.Context, accountID int64) error {
	return v.ValidateReference(ctx, ReferenceCheck{
		TableName: "supply_accounts",
		FieldName: "id",
		FieldValue: accountID,
		CheckSQL:  "SELECT EXISTS(SELECT 1 FROM supply_accounts WHERE id = $1)",
		Args:      []interface{}{accountID},
	})
}

// ValidateOrderSupplyPackage 校验订单所属套餐存在
func (v *ForeignKeyValidator) ValidateOrderSupplyPackage(ctx context.Context, packageID int64) error {
	return v.ValidateReference(ctx, ReferenceCheck{
		TableName: "supply_packages",
		FieldName: "id",
		FieldValue: packageID,
		CheckSQL:  "SELECT EXISTS(SELECT 1 FROM supply_packages WHERE id = $1)",
		Args:      []interface{}{packageID},
	})
}

// ValidateBillingAccount 校验账户所属租户存在
func (v *ForeignKeyValidator) ValidateBillingAccount(ctx context.Context, tenantID int64) error {
	return v.ValidateReference(ctx, ReferenceCheck{
		TableName: "core_tenants",
		FieldName: "id",
		FieldValue: tenantID,
		CheckSQL:  "SELECT EXISTS(SELECT 1 FROM core_tenants WHERE id = $1)",
		Args:      []interface{}{tenantID},
	})
}

// OrphanRecordCheck 孤立记录检查结果
type OrphanRecordCheck struct {
	TableName  string
	FieldName  string
	Count      int64
}

// orphanCheckSQL 孤立检查SQL
type orphanCheckSQL struct {
	TableName string
	FieldName string
	SQL       string
}

// CheckOrphanRecords 执行孤立记录检查
func (v *ForeignKeyValidator) CheckOrphanRecords(ctx context.Context) ([]OrphanRecordCheck, error) {
	checks := []orphanCheckSQL{
		// 检查孤立的supply_accounts
		{
			TableName: "supply_accounts",
			FieldName: "user_id",
			SQL:       `SELECT COUNT(*) FROM supply_accounts sa WHERE NOT EXISTS (SELECT 1 FROM iam_users WHERE id = sa.user_id)`,
		},
		// 检查孤立的supply_packages
		{
			TableName: "supply_packages",
			FieldName: "supply_account_id",
			SQL:       `SELECT COUNT(*) FROM supply_packages sp WHERE NOT EXISTS (SELECT 1 FROM supply_accounts WHERE id = sp.supply_account_id)`,
		},
		// 检查孤立的supply_orders (supply_account_id)
		{
			TableName: "supply_orders",
			FieldName: "supply_account_id",
			SQL:       `SELECT COUNT(*) FROM supply_orders so WHERE NOT EXISTS (SELECT 1 FROM supply_accounts WHERE id = so.supply_account_id)`,
		},
		// 检查孤立的supply_orders (supply_package_id)
		{
			TableName: "supply_orders",
			FieldName: "supply_package_id",
			SQL:       `SELECT COUNT(*) FROM supply_orders so WHERE NOT EXISTS (SELECT 1 FROM supply_packages WHERE id = so.supply_package_id)`,
		},
	}

	var results []OrphanRecordCheck
	for _, check := range checks {
		var count int64
		err := v.pool.QueryRow(ctx, check.SQL).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("failed to check orphans for %s.%s: %w", check.TableName, check.FieldName, err)
		}
		if count > 0 {
			results = append(results, OrphanRecordCheck{
				TableName: check.TableName,
				FieldName: check.FieldName,
				Count:    count,
			})
		}
	}

	return results, nil
}

// ForeignKeyPolicy 外键策略定义
type ForeignKeyPolicy struct {
	// 保留物理外键的表
	PhysicalFKTables []string
	// 使用应用层外键的表
	ApplicationFKTables []string
	// 无外键的表
	NoFKTables []string
}

// GetDefaultForeignKeyPolicy 获取默认外键策略
func GetDefaultForeignKeyPolicy() *ForeignKeyPolicy {
	return &ForeignKeyPolicy{
		// 核心实体表保留物理外键
		PhysicalFKTables: []string{
			"core_tenants",
			"core_projects",
			"iam_users",
			"billing_accounts",
		},
		// 高频写入表使用应用层外键
		ApplicationFKTables: []string{
			"supply_accounts",
			"supply_packages",
			"supply_orders",
			"supply_usage_records",
			"supply_settlements",
		},
		// 审计/日志表无外键
		NoFKTables: []string{
			"audit_events",
			"outbox_events",
			"outbox_dead_letter",
			"supply_idempotency_record",
			"supply_batch_compensation",
		},
	}
}

// GetPolicyForTable 获取表的外键策略
func (p *ForeignKeyPolicy) GetPolicyForTable(tableName string) string {
	for _, t := range p.PhysicalFKTables {
		if t == tableName {
			return "physical"
		}
	}
	for _, t := range p.ApplicationFKTables {
		if t == tableName {
			return "application"
		}
	}
	for _, t := range p.NoFKTables {
		if t == tableName {
			return "none"
		}
	}
	return "unknown"
}
