package service

import (
	"context"
	"time"

	"lijiaoqiao/supply-api/internal/audit/model"
)

// Metric 指标结构
type Metric struct {
	MetricID    string                 `json:"metric_id"`
	MetricName  string                 `json:"metric_name"`
	Period      *MetricPeriod          `json:"period"`
	Value       float64                `json:"value"`
	Unit        string                 `json:"unit"`
	Status      string                 `json:"status"` // PASS/FAIL
	Details     map[string]interface{} `json:"details"`
}

// MetricPeriod 指标周期
type MetricPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// MetricsService 指标服务
type MetricsService struct {
	auditSvc *AuditService
}

// NewMetricsService 创建指标服务
func NewMetricsService(auditSvc *AuditService) *MetricsService {
	return &MetricsService{
		auditSvc: auditSvc,
	}
}

// CalculateM013 计算M-013指标：凭证泄露事件数 = 0
func (s *MetricsService) CalculateM013(ctx context.Context, start, end time.Time) (*Metric, error) {
	filter := &EventFilter{
		StartTime: start,
		EndTime:   end,
		Limit:     10000,
	}

	events, _, err := s.auditSvc.ListEventsWithFilter(ctx, filter)
	if err != nil {
		return nil, err
	}

	// 统计CRED-EXPOSE事件数
	exposureCount := 0
	unresolvedCount := 0
	for _, e := range events {
		if model.IsM013Event(e.EventName) {
			exposureCount++
			// 检查是否已解决（通过扩展字段或标记判断）
			if s.isEventUnresolved(e) {
				unresolvedCount++
			}
		}
	}

	metric := &Metric{
		MetricID:   "M-013",
		MetricName: "supplier_credential_exposure_events",
		Period: &MetricPeriod{
			Start: start,
			End:   end,
		},
		Value:  float64(exposureCount),
		Unit:   "count",
		Status: "PASS",
		Details: map[string]interface{}{
			"total_exposure_events": exposureCount,
			"unresolved_events":      unresolvedCount,
		},
	}

	// 判断状态：M-013要求暴露事件数为0
	if exposureCount > 0 {
		metric.Status = "FAIL"
	}

	return metric, nil
}

// CalculateM014 计算M-014指标：平台凭证入站覆盖率 = 100%
// 分母定义：经平台凭证校验的入站请求（credential_type = 'platform_token'），不含被拒绝的无效请求
func (s *MetricsService) CalculateM014(ctx context.Context, start, end time.Time) (*Metric, error) {
	filter := &EventFilter{
		StartTime: start,
		EndTime:   end,
		Limit:     10000,
	}

	events, _, err := s.auditSvc.ListEventsWithFilter(ctx, filter)
	if err != nil {
		return nil, err
	}

	// 统计CRED-INGRESS事件（使用分类字段过滤，符合设计文档）
	var platformCount, totalIngressCount int
	for _, e := range events {
		// M-014使用event_category + event_sub_category过滤（设计文档8.2节）
		if model.IsM014EventByCategory(e) {
			totalIngressCount++
			// M-014分母：platform_token请求
			if e.CredentialType == model.CredentialTypePlatformToken {
				platformCount++
			}
		}
	}

	// 计算覆盖率
	var coveragePct float64
	if totalIngressCount > 0 {
		coveragePct = float64(platformCount) / float64(totalIngressCount) * 100
	} else {
		coveragePct = 100.0 // 没有入站请求时，默认为100%
	}

	metric := &Metric{
		MetricID:   "M-014",
		MetricName: "platform_credential_ingress_coverage_pct",
		Period: &MetricPeriod{
			Start: start,
			End:   end,
		},
		Value:  coveragePct,
		Unit:   "percentage",
		Status: "PASS",
		Details: map[string]interface{}{
			"platform_token_requests": platformCount,
			"total_requests":          totalIngressCount,
			"non_compliant_requests":  totalIngressCount - platformCount,
		},
	}

	// 判断状态：M-014要求覆盖率为100%
	if coveragePct < 100.0 {
		metric.Status = "FAIL"
	}

	return metric, nil
}

// CalculateM015 计算M-015指标：直连绕过事件数 = 0
func (s *MetricsService) CalculateM015(ctx context.Context, start, end time.Time) (*Metric, error) {
	filter := &EventFilter{
		StartTime: start,
		EndTime:   end,
		Limit:     10000,
	}

	events, _, err := s.auditSvc.ListEventsWithFilter(ctx, filter)
	if err != nil {
		return nil, err
	}

	// 统计直连绕过事件（使用target_direct字段过滤，符合设计文档8.3节）
	directCallCount := 0
	blockedCount := 0
	for _, e := range events {
		// M-015使用target_direct字段过滤（设计文档8.3.3节）
		if model.IsM015EventByTargetDirect(e) {
			directCallCount++
			// 检查是否被阻断
			if s.isEventBlocked(e) {
				blockedCount++
			}
		}
	}

	metric := &Metric{
		MetricID:   "M-015",
		MetricName: "direct_supplier_call_by_consumer_events",
		Period: &MetricPeriod{
			Start: start,
			End:   end,
		},
		Value:  float64(directCallCount),
		Unit:   "count",
		Status: "PASS",
		Details: map[string]interface{}{
			"total_direct_call_events": directCallCount,
			"blocked_events":           blockedCount,
		},
	}

	// 判断状态：M-015要求直连事件数为0
	if directCallCount > 0 {
		metric.Status = "FAIL"
	}

	return metric, nil
}

// CalculateM016 计算M-016指标：query key外部拒绝率 = 100%
// 分母定义：检测到的所有query key请求，含被拒绝的请求
func (s *MetricsService) CalculateM016(ctx context.Context, start, end time.Time) (*Metric, error) {
	filter := &EventFilter{
		StartTime: start,
		EndTime:   end,
		Limit:     10000,
	}

	events, _, err := s.auditSvc.ListEventsWithFilter(ctx, filter)
	if err != nil {
		return nil, err
	}

	// 统计token.query_key.*事件
	// M-016分母：所有query key请求事件（含rejected）
	// M-016分子：被拒绝的query key请求
	var totalQueryKey, rejectedCount int
	rejectBreakdown := make(map[string]int)
	for _, e := range events {
		if model.IsM016Event(e.EventName) {
			// 分母：所有 query key 请求（包含 rejected）
			totalQueryKey++
			// 分子：只计算 token.query_key.rejected
			if model.IsM016QueryKeyRejectEvent(e.EventName) {
				rejectedCount++
				rejectBreakdown[e.ResultCode]++
			}
		}
	}

	// 计算拒绝率
	var rejectRate float64
	if totalQueryKey > 0 {
		rejectRate = float64(rejectedCount) / float64(totalQueryKey) * 100
	} else {
		rejectRate = 100.0 // 没有query key请求时，默认为100%
	}

	metric := &Metric{
		MetricID:   "M-016",
		MetricName: "query_key_external_reject_rate_pct",
		Period: &MetricPeriod{
			Start: start,
			End:   end,
		},
		Value:  rejectRate,
		Unit:   "percentage",
		Status: "PASS",
		Details: map[string]interface{}{
			"rejected_requests":                  rejectedCount,
			"total_external_query_key_requests":   totalQueryKey,
			"reject_breakdown":                    rejectBreakdown,
		},
	}

	// 判断状态：M-016要求拒绝率为100%（所有外部query key请求都被拒绝）
	if rejectRate < 100.0 {
		metric.Status = "FAIL"
	}

	return metric, nil
}

// isEventUnresolved 检查事件是否未解决
func (s *MetricsService) isEventUnresolved(e *model.AuditEvent) bool {
	// 如果事件成功，表示已处理/已解决
	// 如果事件失败，表示有问题/未解决
	return !e.Success
}

// isEventBlocked 检查直连事件是否被阻断
func (s *MetricsService) isEventBlocked(e *model.AuditEvent) bool {
	// 通过检查扩展字段或Success标志来判断是否被阻断
	if e.Success {
		return false // 成功表示未被阻断
	}

	// 检查扩展字段中的blocked标记
	if e.Extensions != nil {
		if blocked, ok := e.Extensions["blocked"].(bool); ok {
			return blocked
		}
	}

	// 通过结果码判断
	switch e.ResultCode {
	case "SEC_DIRECT_BYPASS", "SEC_DIRECT_BYPASS_BLOCKED":
		return true
	default:
		return false
	}
}

// GetAllMetrics 获取所有M-013~M-016指标
func (s *MetricsService) GetAllMetrics(ctx context.Context, start, end time.Time) ([]*Metric, error) {
	m013, err := s.CalculateM013(ctx, start, end)
	if err != nil {
		return nil, err
	}

	m014, err := s.CalculateM014(ctx, start, end)
	if err != nil {
		return nil, err
	}

	m015, err := s.CalculateM015(ctx, start, end)
	if err != nil {
		return nil, err
	}

	m016, err := s.CalculateM016(ctx, start, end)
	if err != nil {
		return nil, err
	}

	return []*Metric{m013, m014, m015, m016}, nil
}