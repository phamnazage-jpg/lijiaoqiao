package router

import (
	"context"
	"testing"
)

// TestP2_04_StrategyRoundRobin_NotImplemented 验证RoundRobin策略是否真正实现
// P2-04: StrategyRoundRobin定义了但走default分支
func TestP2_04_StrategyRoundRobin_NotImplemented(t *testing.T) {
	// 创建3个provider，都设置不同的延迟
	// 如果走latency策略，延迟最低的会被持续选中
	// 如果走RoundRobin策略，应该轮询选择
	r := NewRouter(StrategyRoundRobin)
	
	prov1 := &mockProvider{name: "p1", models: []string{"gpt-4"}, healthy: true}
	prov2 := &mockProvider{name: "p2", models: []string{"gpt-4"}, healthy: true}
	prov3 := &mockProvider{name: "p3", models: []string{"gpt-4"}, healthy: true}
	
	r.RegisterProvider("p1", prov1)
	r.RegisterProvider("p2", prov2)
	r.RegisterProvider("p3", prov3)
	
	// 设置不同的延迟 - p1延迟最低
	r.health["p1"].LatencyMs = 10
	r.health["p2"].LatencyMs = 20
	r.health["p3"].LatencyMs = 30
	
	// 选择100次，统计每个provider被选中的次数
	counts := map[string]int{"p1": 0, "p2": 0, "p3": 0}
	const iterations = 99  // 99能被3整除
	
	for i := 0; i < iterations; i++ {
		selected, err := r.SelectProvider(context.Background(), "gpt-4")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[selected.ProviderName()]++
	}
	
	t.Logf("Selection counts with different latencies: p1=%d, p2=%d, p3=%d", counts["p1"], counts["p2"], counts["p3"])
	
	// 如果走latency策略，p1应该几乎100%被选中
	// 如果走RoundRobin，应该约33% each
	
	// 严格检查：如果p1被选中了超过50次，说明走的是latency策略而不是round_robin
	if counts["p1"] > iterations/2 {
		t.Errorf("RoundRobin strategy appears to NOT be implemented. p1 was selected %d/%d times (%.1f%%), which indicates latency-based selection is being used instead.", 
			counts["p1"], iterations, float64(counts["p1"])*100/float64(iterations))
	}
}
