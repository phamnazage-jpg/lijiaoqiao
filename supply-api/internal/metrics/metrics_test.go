package metrics

import (
	"strings"
	"testing"
)

func TestExport_ContainsUptime(t *testing.T) {
	output := Export()
	if !strings.Contains(output, "supply_api_uptime_seconds") {
		t.Fatal("missing uptime metric")
	}
}

func TestExport_ContainsHTTPMetrics(t *testing.T) {
	output := Export()
	for _, m := range []string{
		"supply_api_http_requests_total",
		"supply_api_http_requests_ok_total",
		"supply_api_http_requests_error_total",
		"supply_api_http_latency_ms_avg",
	} {
		if !strings.Contains(output, m) {
			t.Errorf("missing metric: %s", m)
		}
	}
}

func TestExport_ContainsTokenPublishMetrics(t *testing.T) {
	output := Export()
	for _, m := range []string{
		"supply_api_token_publishes_total",
		"supply_api_token_publish_fail_total",
	} {
		if !strings.Contains(output, m) {
			t.Errorf("missing metric: %s", m)
		}
	}
}

func TestExport_PrometheusFormat(t *testing.T) {
	output := Export()
	if !strings.Contains(output, "# HELP supply_api_uptime_seconds") {
		t.Error("missing HELP line")
	}
	if !strings.Contains(output, "# TYPE supply_api_uptime_seconds gauge") {
		t.Error("missing TYPE line")
	}
}

func TestIncTokenPublish_IncrementsCounter(t *testing.T) {
	before := global.tokenPublishes.Load()
	IncTokenPublish()
	after := global.tokenPublishes.Load()
	if after != before+1 {
		t.Errorf("expected %d, got %d", before+1, after)
	}
}

func TestSetQueueSize_SetsValue(t *testing.T) {
	SetQueueSize(42)
	if got := global.queueSize.Load(); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
}
