package compensation

import (
	"context"
	"encoding/json"
	"fmt"

	"lijiaoqiao/supply-api/internal/audit/sanitizer"
	"lijiaoqiao/supply-api/internal/pkg/logging"
)

type operationHandler func(ctx context.Context, payload json.RawMessage) error

type accountCreateRollbackFunc func(ctx context.Context, payload AccountCreatePayload) error
type packagePublishRollbackFunc func(ctx context.Context, payload PackagePublishPayload) error
type settlementWithdrawRollbackFunc func(ctx context.Context, payload SettlementWithdrawPayload) error
type quotaDeductRollbackFunc func(ctx context.Context, payload QuotaDeductPayload) error

// ExecutorDependencies 定义补偿执行器在生产回滚中需要的外部依赖。
type ExecutorDependencies struct {
	AccountCreateRollback      accountCreateRollbackFunc
	PackagePublishRollback     packagePublishRollbackFunc
	SettlementWithdrawRollback settlementWithdrawRollbackFunc
	QuotaDeductRollback        quotaDeductRollbackFunc
}

// AccountCreatePayload 定义 account.create 补偿所需的最小 payload。
type AccountCreatePayload struct {
	AccountID  int64 `json:"account_id"`
	SupplierID int64 `json:"supplier_id"`
}

// PackagePublishPayload 定义 package.publish 补偿所需的最小 payload。
type PackagePublishPayload struct {
	PackageID  int64 `json:"package_id"`
	SupplierID int64 `json:"supplier_id"`
}

// SettlementWithdrawPayload 定义 settlement.withdraw 补偿所需的最小 payload。
type SettlementWithdrawPayload struct {
	SettlementID int64 `json:"settlement_id"`
	SupplierID   int64 `json:"supplier_id"`
}

// QuotaDeductPayload 定义 quota.deduct 补偿所需的最小 payload。
type QuotaDeductPayload struct {
	PackageID  int64   `json:"package_id"`
	SupplierID int64   `json:"supplier_id"`
	UsedQuota  float64 `json:"used_quota"`
}

// DefaultCompensationExecutor 默认补偿执行器
type DefaultCompensationExecutor struct {
	sanitizer    *sanitizer.Sanitizer // 用于脱敏日志输出
	dependencies ExecutorDependencies
	handlers     map[string]operationHandler
}

// NewDefaultCompensationExecutor 创建默认补偿执行器
func NewDefaultCompensationExecutor(deps ExecutorDependencies) *DefaultCompensationExecutor {
	executor := &DefaultCompensationExecutor{
		sanitizer:    sanitizer.NewSanitizer(),
		dependencies: deps,
	}
	executor.handlers = map[string]operationHandler{
		"account.create":      executor.CompensateAccountCreate,
		"package.publish":     executor.CompensatePackagePublish,
		"settlement.withdraw": executor.CompensateSettlementWithdraw,
		"quota.deduct":        executor.CompensateQuotaDeduct,
	}
	return executor
}

// Execute 执行补偿操作
func (e *DefaultCompensationExecutor) Execute(ctx context.Context, operationType string, payload json.RawMessage) error {
	handler, ok := e.handlers[operationType]
	if !ok {
		logger := logging.NewLogger("supply-api", logging.LogLevelWarn)
		logger.Warn("compensation executor: unknown operation type", map[string]interface{}{
			"operation_type": operationType,
			"masked_payload": e.maskPayload(payload),
		})
		return fmt.Errorf("unknown operation type: %s", operationType)
	}

	return handler(ctx, payload)
}

// maskPayload 对payload进行脱敏处理
func (e *DefaultCompensationExecutor) maskPayload(payload json.RawMessage) string {
	if len(payload) == 0 {
		return "<empty>"
	}
	// 尝试解析为JSON map进行字段级脱敏
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		// 如果不是JSON，直接脱敏整个字符串
		return e.sanitizer.Mask(string(payload))
	}
	// 对map进行脱敏
	masked := e.sanitizer.MaskMap(data)
	// 转换回JSON字符串
	maskedJSON, err := json.Marshal(masked)
	if err != nil {
		return e.sanitizer.Mask(string(payload))
	}
	return string(maskedJSON)
}

// CompensateAccountCreate 补偿账号创建
func (e *DefaultCompensationExecutor) CompensateAccountCreate(ctx context.Context, payload json.RawMessage) error {
	parsed, err := parseAccountCreatePayload(payload)
	if err != nil {
		return err
	}
	if e.dependencies.AccountCreateRollback == nil {
		return e.rollbackNotImplemented("account.create", payload)
	}

	logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
	logger.Info("compensation executor: executing account create compensation", map[string]interface{}{
		"masked_payload": e.maskPayload(payload),
		"account_id":     parsed.AccountID,
		"supplier_id":    parsed.SupplierID,
	})
	return e.dependencies.AccountCreateRollback(ctx, parsed)
}

// CompensatePackagePublish 补偿套餐发布
func (e *DefaultCompensationExecutor) CompensatePackagePublish(ctx context.Context, payload json.RawMessage) error {
	parsed, err := parsePackagePublishPayload(payload)
	if err != nil {
		return err
	}
	if e.dependencies.PackagePublishRollback == nil {
		return e.rollbackNotImplemented("package.publish", payload)
	}

	logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
	logger.Info("compensation executor: executing package publish compensation", map[string]interface{}{
		"masked_payload": e.maskPayload(payload),
		"package_id":     parsed.PackageID,
		"supplier_id":    parsed.SupplierID,
	})
	return e.dependencies.PackagePublishRollback(ctx, parsed)
}

// CompensateSettlementWithdraw 补偿提现
func (e *DefaultCompensationExecutor) CompensateSettlementWithdraw(ctx context.Context, payload json.RawMessage) error {
	parsed, err := parseSettlementWithdrawPayload(payload)
	if err != nil {
		return err
	}
	if e.dependencies.SettlementWithdrawRollback == nil {
		return e.rollbackNotImplemented("settlement.withdraw", payload)
	}

	logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
	logger.Info("compensation executor: executing settlement withdraw compensation", map[string]interface{}{
		"masked_payload":  e.maskPayload(payload),
		"settlement_id":   parsed.SettlementID,
		"supplier_id":     parsed.SupplierID,
	})
	return e.dependencies.SettlementWithdrawRollback(ctx, parsed)
}

// CompensateQuotaDeduct 补偿配额扣减
func (e *DefaultCompensationExecutor) CompensateQuotaDeduct(ctx context.Context, payload json.RawMessage) error {
	parsed, err := parseQuotaDeductPayload(payload)
	if err != nil {
		return err
	}
	if e.dependencies.QuotaDeductRollback == nil {
		return e.rollbackNotImplemented("quota.deduct", payload)
	}

	logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
	logger.Info("compensation executor: executing quota deduct compensation", map[string]interface{}{
		"masked_payload": e.maskPayload(payload),
		"package_id":     parsed.PackageID,
		"supplier_id":    parsed.SupplierID,
		"used_quota":     parsed.UsedQuota,
	})
	return e.dependencies.QuotaDeductRollback(ctx, parsed)
}

func (e *DefaultCompensationExecutor) rollbackNotImplemented(operationType string, payload json.RawMessage) error {
	err := fmt.Errorf("%s compensation not implemented for production rollback", operationType)
	logger := logging.NewLogger("supply-api", logging.LogLevelWarn)
	logger.Warn("compensation executor: rollback dependency unavailable", map[string]interface{}{
		"operation_type": operationType,
		"masked_payload": e.maskPayload(payload),
		"error":          err.Error(),
	})
	return err
}

func parseAccountCreatePayload(payload json.RawMessage) (AccountCreatePayload, error) {
	var parsed AccountCreatePayload
	if err := unmarshalPayload(payload, &parsed); err != nil {
		return AccountCreatePayload{}, err
	}
	if parsed.AccountID <= 0 {
		return AccountCreatePayload{}, fmt.Errorf("invalid compensation payload: account_id is required")
	}
	if parsed.SupplierID <= 0 {
		return AccountCreatePayload{}, fmt.Errorf("invalid compensation payload: supplier_id is required")
	}
	return parsed, nil
}

func parsePackagePublishPayload(payload json.RawMessage) (PackagePublishPayload, error) {
	var parsed PackagePublishPayload
	if err := unmarshalPayload(payload, &parsed); err != nil {
		return PackagePublishPayload{}, err
	}
	if parsed.PackageID <= 0 {
		return PackagePublishPayload{}, fmt.Errorf("invalid compensation payload: package_id is required")
	}
	if parsed.SupplierID <= 0 {
		return PackagePublishPayload{}, fmt.Errorf("invalid compensation payload: supplier_id is required")
	}
	return parsed, nil
}

func parseSettlementWithdrawPayload(payload json.RawMessage) (SettlementWithdrawPayload, error) {
	var parsed SettlementWithdrawPayload
	if err := unmarshalPayload(payload, &parsed); err != nil {
		return SettlementWithdrawPayload{}, err
	}
	if parsed.SettlementID <= 0 {
		return SettlementWithdrawPayload{}, fmt.Errorf("invalid compensation payload: settlement_id is required")
	}
	if parsed.SupplierID <= 0 {
		return SettlementWithdrawPayload{}, fmt.Errorf("invalid compensation payload: supplier_id is required")
	}
	return parsed, nil
}

func parseQuotaDeductPayload(payload json.RawMessage) (QuotaDeductPayload, error) {
	var parsed QuotaDeductPayload
	if err := unmarshalPayload(payload, &parsed); err != nil {
		return QuotaDeductPayload{}, err
	}
	if parsed.PackageID <= 0 {
		return QuotaDeductPayload{}, fmt.Errorf("invalid compensation payload: package_id is required")
	}
	if parsed.SupplierID <= 0 {
		return QuotaDeductPayload{}, fmt.Errorf("invalid compensation payload: supplier_id is required")
	}
	if parsed.UsedQuota <= 0 {
		return QuotaDeductPayload{}, fmt.Errorf("invalid compensation payload: used_quota must be greater than 0")
	}
	return parsed, nil
}

func unmarshalPayload(payload json.RawMessage, target any) error {
	if len(payload) == 0 {
		return fmt.Errorf("invalid compensation payload: payload is empty")
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("invalid compensation payload: %w", err)
	}
	return nil
}
