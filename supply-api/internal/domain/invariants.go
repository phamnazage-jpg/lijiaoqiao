package domain

import (
	"context"
	"errors"
	"fmt"
)

// 领域不变量错误

var (
	// INV-ACC-001: active账号不可删除
	ErrAccountCannotDeleteActive = errors.New("SUP_ACC_4092: cannot delete active accounts")

	// INV-ACC-002: disabled账号仅管理员可恢复
	ErrAccountDisabledRequiresAdmin = errors.New("SUP_ACC_4031: disabled account requires admin to restore")

	// INV-PKG-001: sold_out只能系统迁移
	ErrPackageSoldOutSystemOnly = errors.New("SUP_PKG_4092: sold_out status can only be changed by system")

	// INV-PKG-002: expired套餐不可直接恢复
	ErrPackageExpiredCannotRestore = errors.New("SUP_PKG_4093: expired package cannot be directly restored")

	// INV-PKG-003: 售价不得低于保护价
	ErrPriceBelowProtection = errors.New("SUP_PKG_4001: price cannot be below protected price")

	// INV-SET-001: processing/completed不可撤销
	ErrSettlementCannotCancel = errors.New("SUP_SET_4092: cannot cancel processing or completed settlements")

	// INV-SET-002: 提现金额不得超过可提现余额
	ErrWithdrawExceedsBalance = errors.New("SUP_SET_4001: withdraw amount exceeds available balance")

	// INV-SET-003: 结算单金额与余额流水必须平衡
	ErrSettlementBalanceMismatch = errors.New("SUP_SET_5002: settlement amount does not match balance ledger")

	// INV-SET-004: 已有处理中的提现时不允许再次提现
	ErrWithdrawAlreadyProcessing = errors.New("SUP_SET_4093: another withdrawal is already processing")
)

// InvariantChecker 领域不变量检查器
type InvariantChecker struct {
	accountStore    AccountStore
	packageStore    PackageStore
	settlementStore SettlementStore
}

// NewInvariantChecker 创建不变量检查器
func NewInvariantChecker(
	accountStore AccountStore,
	packageStore PackageStore,
	settlementStore SettlementStore,
) *InvariantChecker {
	return &InvariantChecker{
		accountStore:    accountStore,
		packageStore:    packageStore,
		settlementStore: settlementStore,
	}
}

// CheckAccountDelete 检查账号删除不变量
func (c *InvariantChecker) CheckAccountDelete(ctx context.Context, accountID, supplierID int64) error {
	account, err := c.accountStore.GetByID(ctx, supplierID, accountID)
	if err != nil {
		return err
	}

	// INV-ACC-001: active账号不可删除
	if account.Status == AccountStatusActive {
		return ErrAccountCannotDeleteActive
	}

	return nil
}

// CheckAccountActivate 检查账号激活不变量
func (c *InvariantChecker) CheckAccountActivate(ctx context.Context, accountID, supplierID int64) error {
	account, err := c.accountStore.GetByID(ctx, supplierID, accountID)
	if err != nil {
		return err
	}

	// INV-ACC-002: disabled账号仅管理员可恢复（简化处理，实际需要检查角色）
	if account.Status == AccountStatusDisabled {
		return ErrAccountDisabledRequiresAdmin
	}

	return nil
}

// CheckPackagePublish 检查套餐发布不变量
func (c *InvariantChecker) CheckPackagePublish(ctx context.Context, packageID, supplierID int64) error {
	pkg, err := c.packageStore.GetByID(ctx, supplierID, packageID)
	if err != nil {
		return err
	}

	// INV-PKG-002: expired套餐不可直接恢复
	if pkg.Status == PackageStatusExpired {
		return ErrPackageExpiredCannotRestore
	}

	return nil
}

// CheckPackagePrice 检查套餐价格不变量
func (c *InvariantChecker) CheckPackagePrice(ctx context.Context, pkg *Package, newPricePer1MInput, newPricePer1MOutput float64) error {
	// INV-PKG-003: 售价不得低于保护价（这里简化处理，实际需要查询保护价配置）
	minPrice := 0.01
	if newPricePer1MInput > 0 && newPricePer1MInput < minPrice {
		return fmt.Errorf("%w: input price %.6f is below minimum %.6f",
			ErrPriceBelowProtection, newPricePer1MInput, minPrice)
	}
	if newPricePer1MOutput > 0 && newPricePer1MOutput < minPrice {
		return fmt.Errorf("%w: output price %.6f is below minimum %.6f",
			ErrPriceBelowProtection, newPricePer1MOutput, minPrice)
	}

	return nil
}

// CheckSettlementCancel 检查结算撤销不变量
func (c *InvariantChecker) CheckSettlementCancel(ctx context.Context, settlementID, supplierID int64) error {
	settlement, err := c.settlementStore.GetByID(ctx, supplierID, settlementID)
	if err != nil {
		return err
	}

	// INV-SET-001: processing/completed不可撤销
	if settlement.Status == SettlementStatusProcessing || settlement.Status == SettlementStatusCompleted {
		return ErrSettlementCannotCancel
	}

	return nil
}

// CheckWithdrawBalance 检查提现余额不变量
func (c *InvariantChecker) CheckWithdrawBalance(ctx context.Context, supplierID int64, amount float64) error {
	balance, err := c.settlementStore.GetWithdrawableBalance(ctx, supplierID)
	if err != nil {
		return err
	}

	// INV-SET-002: 提现金额不得超过可提现余额
	if amount > balance {
		return fmt.Errorf("%w: requested %.2f but available %.2f",
			ErrWithdrawExceedsBalance, amount, balance)
	}

	return nil
}

// InvariantViolation 领域不变量违反事件
type InvariantViolation struct {
	RuleCode   string
	ObjectType string
	ObjectID   int64
	Message    string
	OccurredAt string
}

// EmitInvariantViolation 发射不变量违反事件
func EmitInvariantViolation(ruleCode, objectType string, objectID int64, err error) *InvariantViolation {
	return &InvariantViolation{
		RuleCode:   ruleCode,
		ObjectType: objectType,
		ObjectID:   objectID,
		Message:    err.Error(),
		OccurredAt: "now", // 实际应使用时间戳
	}
}

// ValidateStateTransition 验证状态转换是否合法
func ValidateStateTransition(from, to AccountStatus) bool {
	validTransitions := map[AccountStatus][]AccountStatus{
		AccountStatusPending:   {AccountStatusActive, AccountStatusDisabled},
		AccountStatusActive:    {AccountStatusSuspended, AccountStatusDisabled},
		AccountStatusSuspended: {AccountStatusActive, AccountStatusDisabled},
		AccountStatusDisabled:  {AccountStatusActive}, // 需要管理员权限
	}

	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, status := range allowed {
		if status == to {
			return true
		}
	}
	return false
}

// ValidatePackageStateTransition 验证套餐状态转换
func ValidatePackageStateTransition(from, to PackageStatus) bool {
	validTransitions := map[PackageStatus][]PackageStatus{
		PackageStatusDraft:   {PackageStatusActive},
		PackageStatusActive:  {PackageStatusPaused, PackageStatusSoldOut, PackageStatusExpired},
		PackageStatusPaused:  {PackageStatusActive, PackageStatusExpired},
		PackageStatusSoldOut: {}, // 只能由系统迁移
		PackageStatusExpired: {}, // 不能直接恢复，需要通过克隆
	}

	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, status := range allowed {
		if status == to {
			return true
		}
	}
	return false
}
