package service

import (
	"testing"
)

// TestP104_SamplingConfig 验证采样配置
func TestP104_SamplingConfig(t *testing.T) {
	config := DefaultAuditSamplingConfig()

	if config.SuccessSampleRate != 0.01 {
		t.Errorf("expected 0.01 sample rate, got %f", config.SuccessSampleRate)
	}

	if !config.Enabled {
		t.Error("sampling should be enabled by default")
	}

	if len(config.ForceRecordEventTypes) == 0 {
		t.Error("force record event types should not be empty")
	}

	t.Log("P1-04: 采样配置验证通过")
}

// TestP104_FailureEventsAlwaysRecorded 验证失败事件总是被记录
func TestP104_FailureEventsAlwaysRecorded(t *testing.T) {
	sampler := NewAuditSampler(DefaultAuditSamplingConfig())

	// 各种失败事件都应该被记录
	failureEvents := []string{
		"token.authn.fail",
		"token.authz.fail",
		"account.create.fail",
		"api.request.error",
	}

	for _, event := range failureEvents {
		if !sampler.ShouldRecord(event, false) {
			t.Errorf("failure event %s should always be recorded", event)
		}
	}

	t.Log("P1-04: 失败事件总是记录验证通过")
}

// TestP104_SuccessEventsSampled 验证成功事件按采样率记录
func TestP104_SuccessEventsSampled(t *testing.T) {
	config := &AuditSamplingConfig{
		Enabled:           true,
		SuccessSampleRate: 0.01, // 1%
		ForceRecordEventTypes: []string{},
	}

	sampler := NewAuditSampler(config)

	// 运行多次采样测试
	successEvent := "token.authn.success"
	recorded := 0
	total := 10000

	for i := 0; i < total; i++ {
		if sampler.ShouldRecord(successEvent, true) {
			recorded++
		}
	}

	// 采样率应该在合理范围内 (0.5% - 2%)
	sampleRate := float64(recorded) / float64(total)
	if sampleRate < 0.005 || sampleRate > 0.02 {
		t.Errorf("sample rate %f outside expected range [0.005, 0.02]", sampleRate)
	}

	t.Logf("P1-04: 成功事件采样验证通过 (sample rate: %.2f%%)", sampleRate*100)
}

// TestP104_ForceRecordEvents 验证强制记录事件不受采样影响
func TestP104_ForceRecordEvents(t *testing.T) {
	config := &AuditSamplingConfig{
		Enabled:           true,
		SuccessSampleRate: 0.001, // 0.1% - 极低采样率
		ForceRecordEventTypes: []string{
			"token.revoked",
			"account.",      // 前缀匹配
			"settlement.",    // 前缀匹配
		},
	}

	sampler := NewAuditSampler(config)

	// 强制记录事件即使成功也应该100%记录
	forceEvents := []string{
		"token.revoked",
		"account.deleted",
		"settlement.completed",
	}

	for _, event := range forceEvents {
		if !sampler.ShouldRecord(event, true) {
			t.Errorf("force record event %s should always be recorded even if success", event)
		}
	}

	t.Log("P1-04: 强制记录事件验证通过")
}

// TestP104_DisabledSampling 验证禁用采样时全部记录
func TestP104_DisabledSampling(t *testing.T) {
	config := &AuditSamplingConfig{
		Enabled:           false,
		SuccessSampleRate: 0.01,
		ForceRecordEventTypes: []string{},
	}

	sampler := NewAuditSampler(config)

	// 禁用采样后所有事件都应该被记录
	if !sampler.ShouldRecord("token.authn.success", true) {
		t.Error("when disabled, all events should be recorded")
	}

	if !sampler.ShouldRecord("token.authn.fail", false) {
		t.Error("when disabled, all events should be recorded")
	}

	t.Log("P1-04: 禁用采样验证通过")
}

// TestP104_EventClassifier 验证事件分类器
func TestP104_EventClassifier(t *testing.T) {
	classifier := NewAuditEventClassifier()

	testCases := []struct {
		eventName string
		success   bool
		highPriority bool
	}{
		{"token.authn.fail", false, true},
		{"token.authn.success", true, false},
		{"account.created", true, true},
		{"account.deleted", true, true},
		{"settlement.completed", true, true},
		{"payment.processed", true, true},
		{"api.request.start", true, false},
	}

	for _, tc := range testCases {
		result := classifier.IsHighPriorityEvent(tc.eventName, tc.success)
		if result != tc.highPriority {
			t.Errorf("event %s (success=%v): expected highPriority=%v, got %v",
				tc.eventName, tc.success, tc.highPriority, result)
		}
	}

	t.Log("P1-04: 事件分类器验证通过")
}

// TestP104_SamplingRecommendation 验证采样建议
func TestP104_SamplingRecommendation(t *testing.T) {
	classifier := NewAuditEventClassifier()

	testCases := []struct {
		eventName string
		expected string
	}{
		{"token.authn.success", "1% sampling recommended"},
		{"token.authn.fail", "100% record required"},
		{"account.created", "100% record required"},
		{"settlement.completed", "100% record required"},
		{"api.request.start", "1% sampling recommended"},
	}

	for _, tc := range testCases {
		rec := classifier.GetSamplingRecommendation(tc.eventName)
		if rec != tc.expected {
			t.Errorf("event %s: expected '%s', got '%s'", tc.eventName, tc.expected, rec)
		}
	}

	t.Log("P1-04: 采样建议验证通过")
}

// TestP104_Summary 测试总结
func TestP104_Summary(t *testing.T) {
	t.Log("=== P1-04 审计事件采样策略测试总结 ===")
	t.Log("问题: token.authn.success对每个请求记录，高流量下日均8600万条")
	t.Log("")
	t.Log("修复方案:")
	t.Log("  - 成功事件: 1%采样")
	t.Log("  - 失败事件: 100%记录")
	t.Log("  - 高优先级事件(admin.*, settlement.*, payment.*): 100%记录")
	t.Log("")
	t.Log("配置:")
	t.Log("  - SuccessSampleRate: 0.01 (1%)")
	t.Log("  - ForceRecordEventTypes: token.authn.fail, account.*, settlement.*, payment.*, admin.*")
}

// TestAuditSampler_RecordRate tests the RecordRate function
func TestAuditSampler_RecordRate(t *testing.T) {
	config := &AuditSamplingConfig{
		Enabled:           true,
		SuccessSampleRate: 0.05, // 5%
	}
	sampler := NewAuditSampler(config)

	rate := sampler.RecordRate()
	expected := 0.95 // 1.0 - 0.05

	if rate != expected {
		t.Errorf("expected rate %f, got %f", expected, rate)
	}
}

// TestAuditSampler_GetConfig tests the GetConfig function
func TestAuditSampler_GetConfig(t *testing.T) {
	config := &AuditSamplingConfig{
		Enabled:           true,
		SuccessSampleRate: 0.01,
		ForceRecordEventTypes: []string{"test.event"},
	}
	sampler := NewAuditSampler(config)

	returnedConfig := sampler.GetConfig()

	if returnedConfig == nil {
		t.Fatal("expected non-nil config")
	}
	if returnedConfig.SuccessSampleRate != 0.01 {
		t.Errorf("expected 0.01, got %f", returnedConfig.SuccessSampleRate)
	}
	if !returnedConfig.Enabled {
		t.Error("expected Enabled to be true")
	}
	if len(returnedConfig.ForceRecordEventTypes) != 1 {
		t.Errorf("expected 1 force record type, got %d", len(returnedConfig.ForceRecordEventTypes))
	}
}

// TestAuditSampler_NewAuditSampler_NilConfig tests with nil config
func TestAuditSampler_NewAuditSampler_NilConfig(t *testing.T) {
	sampler := NewAuditSampler(nil)

	if sampler == nil {
		t.Fatal("expected non-nil sampler")
	}

	config := sampler.GetConfig()
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if config.SuccessSampleRate != 0.01 {
		t.Errorf("expected default rate 0.01, got %f", config.SuccessSampleRate)
	}
}

// TestAuditSampler_PrefixMatching tests prefix matching for force record events
func TestAuditSampler_PrefixMatching(t *testing.T) {
	config := &AuditSamplingConfig{
		Enabled:           true,
		SuccessSampleRate: 0.001, // Very low rate
		ForceRecordEventTypes: []string{"account.", "settlement."},
	}
	sampler := NewAuditSampler(config)

	// These should always be recorded due to prefix match
	events := []string{
		"account.created",
		"account.updated",
		"account.deleted",
		"settlement.created",
		"settlement.paid",
	}

	for _, event := range events {
		if !sampler.ShouldRecord(event, true) {
			t.Errorf("event %s should be recorded due to prefix match", event)
		}
	}
}

// TestAuditEventClassifier_GetSamplingRecommendation_Default tests default recommendation
func TestAuditEventClassifier_GetSamplingRecommendation_Default(t *testing.T) {
	classifier := NewAuditEventClassifier()

	rec := classifier.GetSamplingRecommendation("unknown.event")
	expected := "10% sampling recommended"

	if rec != expected {
		t.Errorf("expected '%s', got '%s'", expected, rec)
	}
}

// TestAuditEventClassifier_HighPriorityPrefixes tests high priority prefix detection
func TestAuditEventClassifier_HighPriorityPrefixes(t *testing.T) {
	classifier := NewAuditEventClassifier()

	events := []string{
		"token.authn.fail",
		"token.authz.fail",
		"token.revoked",
		"admin.action",
		"admin.config",
	}

	for _, event := range events {
		if !classifier.IsHighPriorityEvent(event, true) {
			t.Errorf("event %s should be high priority", event)
		}
	}
}

// TestAuditEventClassifier_LowPriorityPrefixes tests low priority prefix detection
func TestAuditEventClassifier_LowPriorityPrefixes(t *testing.T) {
	classifier := NewAuditEventClassifier()

	events := []string{
		"token.authn.success",
		"token.access",
		"api.request",
	}

	for _, event := range events {
		if classifier.IsHighPriorityEvent(event, true) {
			t.Errorf("event %s should be low priority", event)
		}
	}
}
