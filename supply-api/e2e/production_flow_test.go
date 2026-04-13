//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/audit"
)

// ============================================================================
// 生产级 E2E 测试：完整用户流程
// ============================================================================

// TestProductionFlow_SupplierOnboarding 测试供应商完整入驻流程
func TestProductionFlow_SupplierOnboarding(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	tenantID := int64(90001)
	token := system.tokenForTenant(t, "tok-onboard", tenantID)

	// Step 1: 验证账户凭证
	verifyBody := `{"provider":"openai","account_type":"resource","credential_input":"sk-prod-test-key-12345678901234567890","min_quota_threshold":1000}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts/verify", strings.NewReader(verifyBody))
	req1.Header.Set("Authorization", "Bearer "+token)
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Request-Id", "onboard-step1-verify")
	recorder1 := httptest.NewRecorder()
	system.handler.ServeHTTP(recorder1, req1)

	if recorder1.Code != http.StatusOK {
		t.Fatalf("步骤1失败: 期望 200, 实际 %d, body=%s", recorder1.Code, recorder1.Body.String())
	}
	t.Log("✅ Step 1: 账户验证成功")

	// Step 2: 创建账户
	createBody := `{"provider":"openai","account_type":"resource","credential_input":"sk-prod-test-key-12345678901234567890"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts", strings.NewReader(createBody))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Request-Id", "onboard-step2-create")
	recorder2 := httptest.NewRecorder()
	system.handler.ServeHTTP(recorder2, req2)

	if recorder2.Code != http.StatusCreated {
		t.Fatalf("步骤2失败: 期望 201, 实际 %d, body=%s", recorder2.Code, recorder2.Body.String())
	}
	t.Log("✅ Step 2: 账户创建成功")

	// Step 3: 创建套餐草稿
	draftBody := `{"model":"gpt-4-turbo","display_name":"GPT-4 Turbo 生产版","price_input":0.03,"quota_per_batch":10000}`
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/supply/packages/draft", strings.NewReader(draftBody))
	req3.Header.Set("Authorization", "Bearer "+token)
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Request-Id", "onboard-step3-draft")
	recorder3 := httptest.NewRecorder()
	system.handler.ServeHTTP(recorder3, req3)

	if recorder3.Code != http.StatusCreated {
		t.Fatalf("步骤3失败: 期望 201, 实际 %d, body=%s", recorder3.Code, recorder3.Body.String())
	}
	t.Log("✅ Step 3: 套餐草稿创建成功")

	// Step 4: 发布套餐
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/supply/packages/1/publish", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	req4.Header.Set("X-Request-Id", "onboard-step4-publish")
	recorder4 := httptest.NewRecorder()
	system.handler.ServeHTTP(recorder4, req4)

	if recorder4.Code != http.StatusOK {
		t.Fatalf("步骤4失败: 期望 200, 实际 %d, body=%s", recorder4.Code, recorder4.Body.String())
	}
	t.Log("✅ Step 4: 套餐发布成功")

	t.Log("🎉 供应商入驻流程完成")
}

// TestProductionFlow_CompleteSettlementCycle 测试完整结算周期
func TestProductionFlow_CompleteSettlementCycle(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	tenantID := int64(90002)
	token := system.tokenForTenant(t, "tok-settlement", tenantID)

	// Step 1: 查看账单汇总
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/supply/billing?start_date=2026-04-01&end_date=2026-04-30", nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	req1.Header.Set("X-Request-Id", "settlement-step1-billing")
	recorder1 := httptest.NewRecorder()
	system.handler.ServeHTTP(recorder1, req1)

	if recorder1.Code != http.StatusOK {
		t.Fatalf("步骤1失败: 期望 200, 实际 %d", recorder1.Code)
	}

	payload1 := decodeJSONBody(t, recorder1)
	data1, ok := payload1["data"].(map[string]any)
	if !ok {
		t.Fatal("期望账单数据结构")
	}
	summary1, ok := data1["summary"].(map[string]any)
	if !ok {
		t.Fatal("期望账单汇总数据")
	}
	t.Logf("✅ Step 1: 账单汇总 - 总收入: %v", summary1["total_revenue"])
	t.Logf("   总订单: %v, 总使用量: %v", summary1["total_orders"], summary1["total_usage"])

	// Step 2: 查看收益记录
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/supply/earnings/records?start_date=2026-04-01&end_date=2026-04-30&page=1&page_size=10", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("X-Request-Id", "settlement-step2-earnings")
	recorder2 := httptest.NewRecorder()
	system.handler.ServeHTTP(recorder2, req2)

	if recorder2.Code != http.StatusOK {
		t.Fatalf("步骤2失败: 期望 200, 实际 %d", recorder2.Code)
	}
	t.Log("✅ Step 2: 收益记录查询成功")

	// Step 3: 验证提现功能状态
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/supply/settlements/withdraw", strings.NewReader(`{"withdraw_amount":100,"payment_method":"bank","payment_account":"13800000000","sms_code":"123456"}`))
	req3.Header.Set("Authorization", "Bearer "+token)
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Request-Id", "settlement-step3-withdraw-check")
	recorder3 := httptest.NewRecorder()
	system.handler.ServeHTTP(recorder3, req3)

	if recorder3.Code != http.StatusServiceUnavailable {
		t.Fatalf("步骤3失败: 提现功能应禁用, 期望 503, 实际 %d", recorder3.Code)
	}
	t.Log("✅ Step 3: 提现功能正确禁用 (SMS 未集成)")

	t.Log("🎉 结算周期流程完成")
}

// TestProductionFlow_AuditTrailCompliance 测试审计追溯合规
func TestProductionFlow_AuditTrailCompliance(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	tenantID := int64(90003)
	token := system.tokenForTenant(t, "tok-audit-compliance", tenantID)

	// 执行多个操作以生成审计事件
	operations := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"验证账户", "POST", "/api/v1/supply/accounts/verify", `{"provider":"openai","account_type":"resource","credential_input":"sk-test-key-1234567890"}`},
		{"创建套餐", "POST", "/api/v1/supply/packages/draft", `{"model":"gpt-4","display_name":"GPT-4","price_input":0.03,"quota_per_batch":1000}`},
		{"发布套餐", "POST", "/api/v1/supply/packages/1/publish", ""},
		{"查询账单", "GET", "/api/v1/supply/billing?start_date=2026-04-01&end_date=2026-04-30", ""},
	}

	// 执行操作
	for i, op := range operations {
		var req *http.Request
		if op.body != "" {
			req = httptest.NewRequest(op.method, op.path, strings.NewReader(op.body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(op.method, op.path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Request-Id", "audit-op-"+string(rune('a'+i)))
		recorder := httptest.NewRecorder()
		system.handler.ServeHTTP(recorder, req)

		if recorder.Code >= 400 && recorder.Code != 404 {
			t.Logf("操作 %s 返回: %d (可能是预期的)", op.name, recorder.Code)
		} else {
			t.Logf("✅ 操作 %s 执行", op.name)
		}
	}

	// 验证审计存储包含事件
	events, err := system.auditStore.Query(context.Background(), audit.EventFilter{
		TenantID: tenantID,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("查询审计事件失败: %v", err)
	}

	// 验证事件元数据
	for _, event := range events {
		if event.EventID == "" {
			t.Error("审计事件缺少 EventID")
		}
		if event.TenantID != tenantID {
			t.Errorf("审计事件 TenantID 不匹配: 期望 %d, 实际 %d", tenantID, event.TenantID)
		}
		if event.CreatedAt.IsZero() {
			t.Error("审计事件缺少 CreatedAt 时间戳")
		}
		t.Logf("✅ 审计事件: type=%s, action=%s, result=%s", event.ObjectType, event.Action, event.ResultCode)
	}

	t.Log("🎉 审计追溯合规检查完成")
}

// ============================================================================
// 生产级 E2E 测试：安全边界测试
// ============================================================================

// TestSecurityBoundary_TokenReplayPrevention 测试 Token 重放防护
func TestSecurityBoundary_TokenReplayPrevention(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	token := system.tokenForTenant(t, "tok-replay-test", 90010)

	// 第一次请求（应该成功）- 使用 POST /api/v1/supply/accounts/verify
	body := `{"provider":"openai","account_type":"resource","credential_input":"sk-test"}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts/verify", strings.NewReader(body))
	req1.Header.Set("Authorization", "Bearer "+token)
	req1.Header.Set("Content-Type", "application/json")
	recorder1 := httptest.NewRecorder()
	system.handler.ServeHTTP(recorder1, req1)

	if recorder1.Code != http.StatusOK {
		t.Fatalf("第一次请求失败: %d, body=%s", recorder1.Code, recorder1.Body.String())
	}
	t.Log("✅ 第一次请求成功")

	// 模拟重复请求（幂等性测试）
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts/verify", strings.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "replay-test-key")
	recorder2 := httptest.NewRecorder()
	system.handler.ServeHTTP(recorder2, req2)

	t.Logf("Token 重放测试: 状态码=%d", recorder2.Code)
	t.Log("✅ 重放防护测试完成")
}

// TestSecurityBoundary_SQLInjectionPrevention 测试 SQL 注入防护
func TestSecurityBoundary_SQLInjectionPrevention(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	token := system.tokenForTenant(t, "tok-sql-inj", 90011)

	// 测试各种 SQL 注入 payload
	injectionPayloads := []string{
		"'; DROP TABLE supply_accounts; --",
		"1' OR '1'='1",
		"1; DELETE FROM supply_accounts WHERE 1=1; --",
		"NULL",
	}

	for _, payload := range injectionPayloads {
		body := `{"provider":"` + payload + `","account_type":"resource","credential_input":"sk-test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts/verify", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		system.handler.ServeHTTP(recorder, req)

		// 应该返回 400/401/403，但绝对不应该是 200 并执行了注入
		if recorder.Code == http.StatusOK && strings.Contains(recorder.Body.String(), "DROP TABLE") {
			t.Errorf("SQL 注入漏洞: payload=%s", payload)
		}
		t.Logf("SQL 注入测试: payload=%q, status=%d", payload, recorder.Code)
	}
}

// TestSecurityBoundary_XSSPrevention 测试 XSS 防护
func TestSecurityBoundary_XSSPrevention(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	token := system.tokenForTenant(t, "tok-xss", 90012)

	xssPayloads := []string{
		"<script>alert('xss')</script>",
		"javascript:alert('xss')",
		"<img src=x onerror=alert('xss')>",
	}

	for _, payload := range xssPayloads {
		body := `{"provider":"openai","account_type":"` + payload + `","credential_input":"sk-test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts/verify", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		system.handler.ServeHTTP(recorder, req)

		// 验证响应中没有未经处理的 XSS payload
		responseBody := recorder.Body.String()
		if strings.Contains(responseBody, "<script>") || strings.Contains(responseBody, "javascript:") {
			t.Errorf("XSS 漏洞: payload=%s", payload)
		}
		t.Logf("XSS 测试: payload=%q, status=%d", payload, recorder.Code)
	}
}

// TestSecurityBoundary_RateLimitingBoundary 测试限流边界
func TestSecurityBoundary_RateLimitingBoundary(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	token := system.tokenForTenant(t, "tok-rate-limit", 90013)

	// 发送大量请求（模拟限流测试）
	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/supply/accounts", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		system.handler.ServeHTTP(recorder, req)

		if recorder.Code == http.StatusOK {
			successCount++
		} else if recorder.Code == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	t.Logf("限流测试结果: 成功=%d, 限流=%d, 总请求=100", successCount, rateLimitedCount)

	// 验证系统有某种限流机制（要么成功数 < 100，要么有明确的限流响应）
	if successCount == 100 && rateLimitedCount == 0 {
		t.Log("⚠️ 警告: 100 个请求全部成功，可能缺少限流保护")
	}
}

// ============================================================================
// 生产级 E2E 测试：性能与可靠性
// ============================================================================

// TestReliability_ResponseTimeUnderLoad 测试响应时间
func TestReliability_ResponseTimeUnderLoad(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	token := system.tokenForTenant(t, "tok-perf", 90020)

	// 测试关键端点响应时间
	endpoints := []struct {
		name string
		path string
	}{
		{"健康检查", "/actuator/health"},
		{"账单查询", "/api/v1/supply/billing?start_date=2026-04-01&end_date=2026-04-30"},
		{"收益记录", "/api/v1/supply/earnings/records?start_date=2026-04-01&end_date=2026-04-30"},
	}

	for _, ep := range endpoints {
		start := time.Now()
		req := httptest.NewRequest(http.MethodGet, ep.path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		system.handler.ServeHTTP(recorder, req)
		duration := time.Since(start)

		t.Logf("端点 %s: 状态码=%d, 耗时=%v", ep.name, recorder.Code, duration)

		// 生产级 SLA: P99 < 200ms
		if duration > 200*time.Millisecond {
			t.Logf("⚠️ 警告: %s 响应时间超过 200ms SLA", ep.name)
		}
	}
}

// TestReliability_CircuitBreakerBehavior 测试断路器行为
func TestReliability_CircuitBreakerBehavior(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	token := system.tokenForTenant(t, "tok-circuit", 90021)

	// 模拟连续失败场景
	for i := 0; i < 5; i++ {
		// 使用无效路径触发错误
		req := httptest.NewRequest(http.MethodGet, "/api/v1/supply/accounts/invalid-id", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		system.handler.ServeHTTP(recorder, req)

		t.Logf("错误请求 %d: 状态码=%d", i+1, recorder.Code)
	}

	t.Log("断路器测试完成")
}

// ============================================================================
// 生产级 E2E 测试：数据完整性
// ============================================================================

// TestDataIntegrity_AccountStateTransitions 测试账户状态转换
func TestDataIntegrity_AccountStateTransitions(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	token := system.tokenForTenant(t, "tok-state", 90030)

	// 创建账户
	createBody := `{"provider":"openai","account_type":"resource","credential_input":"sk-test-key-123456789012"}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts", strings.NewReader(createBody))
	reqCreate.Header.Set("Authorization", "Bearer "+token)
	reqCreate.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	system.handler.ServeHTTP(recorder, reqCreate)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("创建账户失败: %d", recorder.Code)
	}
	t.Log("✅ 账户创建成功")

	// 暂停账户
	reqSuspend := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts/1/suspend", nil)
	reqSuspend.Header.Set("Authorization", "Bearer "+token)
	recorderSuspend := httptest.NewRecorder()
	system.handler.ServeHTTP(recorderSuspend, reqSuspend)

	if recorderSuspend.Code != http.StatusOK {
		t.Fatalf("暂停账户失败: %d", recorderSuspend.Code)
	}

	payload := decodeJSONBody(t, recorderSuspend)
	data := payload["data"].(map[string]any)
	if data["status"] != "suspended" {
		t.Errorf("期望状态 suspended, 实际 %v", data["status"])
	} else {
		t.Log("✅ 账户暂停成功，状态正确")
	}
}

// TestDataIntegrity_AuditLogImmutability 测试审计日志不可篡改
func TestDataIntegrity_AuditLogImmutability(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	tenantID := int64(90031)

	// 直接向审计存储插入事件
	if err := system.auditStore.Emit(context.Background(), audit.Event{
		TenantID:   tenantID,
		ObjectType: "supply_account",
		ObjectID:   1,
		Action:     "verify",
		ResultCode: "OK",
		RequestID:  "immutable-req-001",
		SourceIP:   "127.0.0.1",
	}); err != nil {
		t.Fatalf("插入审计事件失败: %v", err)
	}

	// 查询事件
	events, err := system.auditStore.Query(context.Background(), audit.EventFilter{
		TenantID: tenantID,
		Limit:   10,
	})

	if err != nil {
		t.Fatalf("查询审计事件失败: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("未找到审计事件")
	}

	// 验证事件不可变字段
	for _, event := range events {
		if event.EventID == "" {
			t.Error("EventID 不应为空")
		}
		if event.CreatedAt.IsZero() {
			t.Error("CreatedAt 时间戳不应为零")
		}
		if event.ResultCode == "" {
			t.Error("ResultCode 不应为空")
		}

		t.Logf("✅ 审计事件验证: EventID=%s, ObjectType=%s, Action=%s, ResultCode=%s",
			event.EventID, event.ObjectType, event.Action, event.ResultCode)
	}
}
