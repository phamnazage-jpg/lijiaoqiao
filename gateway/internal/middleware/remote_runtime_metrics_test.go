package middleware

import "testing"

var remoteRuntimeHardeningScenarios = []string{
	"timeout",
	"cache_hit",
	"cache_evict",
	"upstream_fail",
}

var remoteRuntimeMetricDraft = []string{
	"cache_hit",
	"cache_miss",
	"cache_evict",
	"upstream_latency_ms",
}

var remoteRuntimeConfigDraft = []string{
	"GATEWAY_TOKEN_RUNTIME_HTTP_TIMEOUT",
	"GATEWAY_TOKEN_RUNTIME_DIAL_TIMEOUT",
	"GATEWAY_TOKEN_RUNTIME_IDLE_CONN_TIMEOUT",
	"GATEWAY_TOKEN_RUNTIME_MAX_IDLE_CONNS_PER_HOST",
	"GATEWAY_TOKEN_RUNTIME_CACHE_ACTIVE_TTL",
	"GATEWAY_TOKEN_RUNTIME_CACHE_EXPIRED_TTL",
	"GATEWAY_TOKEN_RUNTIME_CACHE_REVOKED_TTL",
	"GATEWAY_TOKEN_RUNTIME_CACHE_MAX_ENTRIES",
}

func TestRemoteTokenRuntimeHardeningMatrix_HasRequiredScenarios(t *testing.T) {
	assertStringSetContains(t, remoteRuntimeHardeningScenarios, []string{
		"timeout",
		"cache_hit",
		"cache_evict",
		"upstream_fail",
	})
}

func TestRemoteTokenRuntimeMetricDraft_HasRequiredMetrics(t *testing.T) {
	assertStringSetContains(t, remoteRuntimeMetricDraft, []string{
		"cache_hit",
		"cache_miss",
		"cache_evict",
		"upstream_latency_ms",
	})
}

func TestRemoteTokenRuntimeConfigDraft_HasRequiredEnvNames(t *testing.T) {
	assertStringSetContains(t, remoteRuntimeConfigDraft, []string{
		"GATEWAY_TOKEN_RUNTIME_HTTP_TIMEOUT",
		"GATEWAY_TOKEN_RUNTIME_DIAL_TIMEOUT",
		"GATEWAY_TOKEN_RUNTIME_IDLE_CONN_TIMEOUT",
		"GATEWAY_TOKEN_RUNTIME_MAX_IDLE_CONNS_PER_HOST",
		"GATEWAY_TOKEN_RUNTIME_CACHE_ACTIVE_TTL",
		"GATEWAY_TOKEN_RUNTIME_CACHE_EXPIRED_TTL",
		"GATEWAY_TOKEN_RUNTIME_CACHE_REVOKED_TTL",
		"GATEWAY_TOKEN_RUNTIME_CACHE_MAX_ENTRIES",
	})
}

func assertStringSetContains(t *testing.T, actual []string, required []string) {
	t.Helper()

	seen := make(map[string]struct{}, len(actual))
	for _, item := range actual {
		seen[item] = struct{}{}
	}

	for _, item := range required {
		if _, ok := seen[item]; !ok {
			t.Fatalf("expected %q to be present, got %v", item, actual)
		}
	}
}
