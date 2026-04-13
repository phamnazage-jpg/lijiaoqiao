package compensation

import (
	"context"
	"encoding/json"
	"fmt"

	"lijiaoqiao/supply-api/internal/audit/sanitizer"
	"lijiaoqiao/supply-api/internal/pkg/logging"
)

// DefaultCompensationExecutor 默认补偿执行器
type DefaultCompensationExecutor struct {
	sanitizer *sanitizer.Sanitizer // 用于脱敏日志输出
}

// NewDefaultCompensationExecutor 创建默认补偿执行器
func NewDefaultCompensationExecutor() *DefaultCompensationExecutor {
	return &DefaultCompensationExecutor{
		sanitizer: sanitizer.NewSanitizer(),
	}
}

// Execute 执行补偿操作
func (e *DefaultCompensationExecutor) Execute(ctx context.Context, operationType string, payload json.RawMessage) error {
	// 根据operationType执行相应的补偿操作
	// operationType 示例:
	// - "account.create" - 账号创建失败补偿
	// - "package.publish" - 套餐发布失败补偿
	// - "settlement.withdraw" - 提现失败补偿
	// - "quota.deduct" - 配额扣减失败补偿

	switch operationType {
	case "account.create":
		return e.CompensateAccountCreate(ctx, payload)
	case "package.publish":
		return e.CompensatePackagePublish(ctx, payload)
	case "settlement.withdraw":
		return e.CompensateSettlementWithdraw(ctx, payload)
	case "quota.deduct":
		return e.CompensateQuotaDeduct(ctx, payload)
	default:
		logger := logging.NewLogger("supply-api", logging.LogLevelWarn)
		logger.Warn("compensation executor: unknown operation type", map[string]interface{}{
			"operation_type":   operationType,
			"masked_payload":   e.maskPayload(payload),
		})
		return fmt.Errorf("unknown operation type: %s", operationType)
	}
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
	logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
	logger.Info("compensation executor: executing account create compensation", map[string]interface{}{
		"masked_payload": e.maskPayload(payload),
	})
	// 实际实现：删除已创建的账号资源
	// 1. 调用账号服务的删除接口
	// 2. 释放相关资源
	return nil
}

// CompensatePackagePublish 补偿套餐发布
func (e *DefaultCompensationExecutor) CompensatePackagePublish(ctx context.Context, payload json.RawMessage) error {
	logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
	logger.Info("compensation executor: executing package publish compensation", map[string]interface{}{
		"masked_payload": e.maskPayload(payload),
	})
	// 实际实现：回滚已发布的套餐
	// 1. 将套餐状态改为draft
	// 2. 或直接删除套餐
	return nil
}

// CompensateSettlementWithdraw 补偿提现
func (e *DefaultCompensationExecutor) CompensateSettlementWithdraw(ctx context.Context, payload json.RawMessage) error {
	logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
	logger.Info("compensation executor: executing settlement withdraw compensation", map[string]interface{}{
		"masked_payload": e.maskPayload(payload),
	})
	// 实际实现：回滚提现状态
	// 1. 将结算单状态改为failed
	// 2. 恢复用户余额
	return nil
}

// CompensateQuotaDeduct 补偿配额扣减
func (e *DefaultCompensationExecutor) CompensateQuotaDeduct(ctx context.Context, payload json.RawMessage) error {
	logger := logging.NewLogger("supply-api", logging.LogLevelInfo)
	logger.Info("compensation executor: executing quota deduct compensation", map[string]interface{}{
		"masked_payload": e.maskPayload(payload),
	})
	// 实际实现：回滚配额扣减
	// 1. 恢复套餐可用配额
	// 2. 减少已售配额
	return nil
}
