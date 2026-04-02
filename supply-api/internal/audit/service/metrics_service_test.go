package service

import (
	"context"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/audit/model"

	"github.com/stretchr/testify/assert"
)

func TestAuditMetrics_M013_CredentialExposure(t *testing.T) {
	// M-013: supplier_credential_exposure_events = 0
	ctx := context.Background()
	svc := NewAuditService(NewInMemoryAuditStore())
	metricsSvc := NewMetricsService(svc)

	// 创建一些事件，包括CRED-EXPOSE事件
	events := []*model.AuditEvent{
		{
			EventName:       "CRED-EXPOSE-RESPONSE",
			EventCategory:   "CRED",
			OperatorID:      1001,
			TenantID:        2001,
			ObjectType:      "account",
			ObjectID:        12345,
			Action:          "create",
			CredentialType: "platform_token",
			SourceType:      "api",
			SourceIP:        "192.168.1.1",
			Success:         true,
			ResultCode:     "SEC_CRED_EXPOSED",
		},
		{
			EventName:       "AUTH-TOKEN-OK",
			EventCategory:   "AUTH",
			OperatorID:      1001,
			TenantID:        2001,
			ObjectType:      "token",
			ObjectID:        12345,
			Action:          "verify",
			CredentialType: "platform_token",
			SourceType:      "api",
			SourceIP:        "192.168.1.1",
			Success:         true,
			ResultCode:     "AUTH_TOKEN_OK",
		},
	}

	for _, e := range events {
		svc.CreateEvent(ctx, e)
	}

	// 计算M-013指标
	now := time.Now()
	metric, err := metricsSvc.CalculateM013(ctx, now.Add(-24*time.Hour), now)

	assert.NoError(t, err)
	assert.NotNil(t, metric)
	assert.Equal(t, "M-013", metric.MetricID)
	assert.Equal(t, "supplier_credential_exposure_events", metric.MetricName)
	assert.Equal(t, float64(1), metric.Value) // 有1个暴露事件
	assert.Equal(t, "FAIL", metric.Status) // 暴露事件数 > 0，应该是FAIL
}

func TestAuditMetrics_M014_IngressCoverage(t *testing.T) {
	// M-014: platform_credential_ingress_coverage_pct = 100%
	// 分母定义：经平台凭证校验的入站请求（credential_type = 'platform_token'），不含被拒绝的无效请求
	ctx := context.Background()
	svc := NewAuditService(NewInMemoryAuditStore())
	metricsSvc := NewMetricsService(svc)

	// 创建入站凭证事件
	events := []*model.AuditEvent{
		// 合规的platform_token请求
		{
			EventName:         "CRED-INGRESS-PLATFORM",
			EventCategory:     "CRED",
			EventSubCategory: "INGRESS",
			OperatorID:       1001,
			TenantID:         2001,
			ObjectType:       "account",
			ObjectID:         12345,
			Action:           "query",
			CredentialType:   "platform_token",
			SourceType:       "api",
			SourceIP:         "192.168.1.1",
			Success:          true,
			ResultCode:       "CRED_INGRESS_OK",
		},
		{
			EventName:         "CRED-INGRESS-PLATFORM",
			EventCategory:     "CRED",
			EventSubCategory: "INGRESS",
			OperatorID:       1002,
			TenantID:         2001,
			ObjectType:       "account",
			ObjectID:         12346,
			Action:           "query",
			CredentialType:   "platform_token",
			SourceType:       "api",
			SourceIP:         "192.168.1.2",
			Success:          true,
			ResultCode:       "CRED_INGRESS_OK",
		},
		// 非合规的query_key请求 - 不应该计入M-014的分母
		{
			EventName:         "CRED-INGRESS-SUPPLIER",
			EventCategory:     "CRED",
			EventSubCategory: "INGRESS",
			OperatorID:       1003,
			TenantID:         2001,
			ObjectType:       "account",
			ObjectID:         12347,
			Action:           "query",
			CredentialType:   "query_key",
			SourceType:       "api",
			SourceIP:         "192.168.1.3",
			Success:          false,
			ResultCode:       "AUTH_QUERY_REJECT",
		},
	}

	for _, e := range events {
		svc.CreateEvent(ctx, e)
	}

	// 计算M-014指标
	now := time.Now()
	metric, err := metricsSvc.CalculateM014(ctx, now.Add(-24*time.Hour), now)

	assert.NoError(t, err)
	assert.NotNil(t, metric)
	assert.Equal(t, "M-014", metric.MetricID)
	assert.Equal(t, "platform_credential_ingress_coverage_pct", metric.MetricName)
	// 2个platform_token / 2个总入站请求 = 100%
	assert.Equal(t, 100.0, metric.Value)
	assert.Equal(t, "PASS", metric.Status)
}

func TestAuditMetrics_M015_DirectCall(t *testing.T) {
	// M-015: direct_supplier_call_by_consumer_events = 0
	ctx := context.Background()
	svc := NewAuditService(NewInMemoryAuditStore())
	metricsSvc := NewMetricsService(svc)

	// 创建直连事件
	events := []*model.AuditEvent{
		{
			EventName:       "CRED-DIRECT-SUPPLIER",
			EventCategory:   "CRED",
			EventSubCategory: "DIRECT",
			OperatorID:      1001,
			TenantID:        2001,
			ObjectType:      "api",
			ObjectID:        12345,
			Action:          "call",
			CredentialType: "none",
			SourceType:      "api",
			SourceIP:        "192.168.1.1",
			Success:         false,
			ResultCode:     "SEC_DIRECT_BYPASS",
			TargetDirect:    true,
		},
		{
			EventName:       "AUTH-TOKEN-OK",
			EventCategory:   "AUTH",
			OperatorID:      1001,
			TenantID:        2001,
			ObjectType:      "token",
			ObjectID:        12345,
			Action:          "verify",
			CredentialType: "platform_token",
			SourceType:      "api",
			SourceIP:        "192.168.1.1",
			Success:         true,
			ResultCode:     "AUTH_TOKEN_OK",
		},
	}

	for _, e := range events {
		svc.CreateEvent(ctx, e)
	}

	// 计算M-015指标
	now := time.Now()
	metric, err := metricsSvc.CalculateM015(ctx, now.Add(-24*time.Hour), now)

	assert.NoError(t, err)
	assert.NotNil(t, metric)
	assert.Equal(t, "M-015", metric.MetricID)
	assert.Equal(t, "direct_supplier_call_by_consumer_events", metric.MetricName)
	assert.Equal(t, float64(1), metric.Value) // 有1个直连事件
	assert.Equal(t, "FAIL", metric.Status) // 直连事件数 > 0，应该是FAIL
}

func TestAuditMetrics_M016_QueryKeyRejectRate(t *testing.T) {
	// M-016: query_key_external_reject_rate_pct = 100%
	// 分母：所有query key请求（不含被拒绝的无效请求）
	ctx := context.Background()
	svc := NewAuditService(NewInMemoryAuditStore())
	metricsSvc := NewMetricsService(svc)

	// 创建query key事件
	events := []*model.AuditEvent{
		// 被拒绝的query key请求
		{
			EventName:       "AUTH-QUERY-REJECT",
			EventCategory:   "AUTH",
			OperatorID:      1001,
			TenantID:        2001,
			ObjectType:      "query_key",
			ObjectID:        12345,
			Action:          "query",
			CredentialType: "query_key",
			SourceType:      "api",
			SourceIP:        "192.168.1.1",
			Success:         false,
			ResultCode:       "QUERY_KEY_NOT_ALLOWED",
		},
		{
			EventName:       "AUTH-QUERY-REJECT",
			EventCategory:   "AUTH",
			OperatorID:      1002,
			TenantID:        2001,
			ObjectType:      "query_key",
			ObjectID:        12346,
			Action:          "query",
			CredentialType: "query_key",
			SourceType:      "api",
			SourceIP:        "192.168.1.2",
			Success:         false,
			ResultCode:       "QUERY_KEY_EXPIRED",
		},
		// query key请求
		{
			EventName:       "AUTH-QUERY-KEY",
			EventCategory:   "AUTH",
			OperatorID:      1003,
			TenantID:        2001,
			ObjectType:      "query_key",
			ObjectID:        12347,
			Action:          "query",
			CredentialType: "query_key",
			SourceType:      "api",
			SourceIP:        "192.168.1.3",
			Success:         false,
			ResultCode:       "QUERY_KEY_EXPIRED",
		},
		// 非query key事件
		{
			EventName:       "AUTH-TOKEN-OK",
			EventCategory:   "AUTH",
			OperatorID:      1001,
			TenantID:        2001,
			ObjectType:      "token",
			ObjectID:        12345,
			Action:          "verify",
			CredentialType: "platform_token",
			SourceType:      "api",
			SourceIP:        "192.168.1.1",
			Success:         true,
			ResultCode:     "AUTH_TOKEN_OK",
		},
	}

	for _, e := range events {
		svc.CreateEvent(ctx, e)
	}

	// 计算M-016指标
	now := time.Now()
	metric, err := metricsSvc.CalculateM016(ctx, now.Add(-24*time.Hour), now)

	assert.NoError(t, err)
	assert.NotNil(t, metric)
	assert.Equal(t, "M-016", metric.MetricID)
	assert.Equal(t, "query_key_external_reject_rate_pct", metric.MetricName)
	// 2个拒绝 / 3个query key总请求 = 66.67%
	assert.InDelta(t, 66.67, metric.Value, 0.01)
	assert.Equal(t, "FAIL", metric.Status) // 拒绝率 < 100%，应该是FAIL
}

func TestAuditMetrics_M016_DifferentFromM014(t *testing.T) {
	// M-014与M-016边界清晰：分母不同，无重叠
	// M-014 分母：经平台凭证校验的入站请求（platform_token）
	// M-016 分母：检测到的所有query key请求

	ctx := context.Background()
	svc := NewAuditService(NewInMemoryAuditStore())
	metricsSvc := NewMetricsService(svc)

	// 场景：100个请求，80个使用platform_token，20个使用query key（被拒绝）
	// M-014 = 80/80 = 100%（分母只计算platform_token请求）
	// M-016 = 20/20 = 100%（分母计算所有query key请求）

	// 创建80个platform_token请求
	for i := 0; i < 80; i++ {
		svc.CreateEvent(ctx, &model.AuditEvent{
			EventName:         "CRED-INGRESS-PLATFORM",
			EventCategory:     "CRED",
			EventSubCategory: "INGRESS",
			OperatorID:       int64(1000 + i),
			TenantID:         2001,
			ObjectType:       "account",
			ObjectID:        int64(i),
			Action:           "query",
			CredentialType:   "platform_token",
			SourceType:       "api",
			SourceIP:         "192.168.1.1",
			Success:          true,
			ResultCode:       "CRED_INGRESS_OK",
		})
	}

	// 创建20个query key请求（全部被拒绝）
	for i := 0; i < 20; i++ {
		svc.CreateEvent(ctx, &model.AuditEvent{
			EventName:       "AUTH-QUERY-REJECT",
			EventCategory:   "AUTH",
			OperatorID:      int64(2000 + i),
			TenantID:        2001,
			ObjectType:      "query_key",
			ObjectID:        int64(1000 + i),
			Action:          "query",
			CredentialType: "query_key",
			SourceType:      "api",
			SourceIP:        "192.168.1.1",
			Success:         false,
			ResultCode:       "QUERY_KEY_NOT_ALLOWED",
		})
	}

	now := time.Now()

	// 计算M-014
	m014, err := metricsSvc.CalculateM014(ctx, now.Add(-24*time.Hour), now)
	assert.NoError(t, err)
	assert.Equal(t, 100.0, m014.Value) // 80/80 = 100%

	// 计算M-016
	m016, err := metricsSvc.CalculateM016(ctx, now.Add(-24*time.Hour), now)
	assert.NoError(t, err)
	assert.Equal(t, 100.0, m016.Value) // 20/20 = 100%
}

func TestAuditMetrics_M013_ZeroExposure(t *testing.T) {
	// M-013: 当没有凭证暴露事件时，应该为0，状态PASS
	ctx := context.Background()
	svc := NewAuditService(NewInMemoryAuditStore())
	metricsSvc := NewMetricsService(svc)

	// 创建一些正常事件，没有CRED-EXPOSE
	svc.CreateEvent(ctx, &model.AuditEvent{
		EventName:       "AUTH-TOKEN-OK",
		EventCategory:   "AUTH",
		OperatorID:      1001,
		TenantID:        2001,
		ObjectType:      "token",
		ObjectID:        12345,
		Action:          "verify",
		CredentialType: "platform_token",
		SourceType:      "api",
		SourceIP:        "192.168.1.1",
		Success:         true,
		ResultCode:     "AUTH_TOKEN_OK",
	})

	now := time.Now()
	metric, err := metricsSvc.CalculateM013(ctx, now.Add(-24*time.Hour), now)

	assert.NoError(t, err)
	assert.Equal(t, float64(0), metric.Value)
	assert.Equal(t, "PASS", metric.Status)
}