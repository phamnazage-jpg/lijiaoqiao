package rules

import (
	"sync"
	"testing"
)

// ==================== P0-05 测试: regexp编译错误被静默忽略 ====================

// TestExtractMatch_InvalidRegex_P0_05 测试无效正则表达式被静默忽略的问题
// 问题: extractMatch在regexp.Compile失败时会panic，因为错误被丢弃
func TestExtractMatch_InvalidRegex_P0_05(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	// 使用无效的正则表达式 - 这会导致panic因为错误被忽略
	invalidPattern := "[invalid" // 无效的正则表达式，缺少闭合括号

	// 捕获panic来验证问题存在
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("P0-05 问题确认: extractMatch对无效正则发生了panic: %v", r)
			t.Log("问题: regexp.Compile错误被丢弃，导致后续操作panic")
		}
	}()

	// 如果没有panic，说明问题已修复
	result, err := engine.extractMatch(invalidPattern, "test content")
	if err != nil {
		t.Logf("P0-05 问题已修复: extractMatch正确返回错误: %v, result=%q", err, result)
	} else {
		t.Errorf("P0-05 未修复: extractMatch应返回错误但没有返回")
	}
}

// ==================== P0-06 测试: compiledPatterns非线程安全 ====================

// TestRuleEngine_ConcurrentAccess_P0_06 测试并发访问时的数据竞争
// 使用race detector检测数据竞争
func TestRuleEngine_ConcurrentAccess_P0_06(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	pattern := "test"
	content := "this is a test content"

	var wg sync.WaitGroup
	numGoroutines := 100

	// 并发调用matchRegex
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = engine.matchRegex(pattern, content)
		}()
	}

	// 同时并发调用extractMatch
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = engine.extractMatch(pattern, content)
		}()
	}

	// 同时并发调用Match
	rule := Rule{
		ID: "test-rule",
		Matchers: []Matcher{
			{Type: "regex_match", Pattern: pattern},
		},
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = engine.Match(rule, content)
		}()
	}

	wg.Wait()
	t.Log("P0-06 验证: 并发测试完成")
}

// TestRuleEngine_ConcurrentMapAccess_P0_06 测试map并发读写问题
func TestRuleEngine_ConcurrentMapAccess_P0_06(t *testing.T) {
	loader := NewRuleLoader()
	engine := NewRuleEngine(loader)

	patterns := []string{"test1", "test2", "test3", "test4", "test5"}
	content := "test1 test2 test3 test4 test5"

	var wg sync.WaitGroup
	for _, pattern := range patterns {
		p := pattern
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = engine.matchRegex(p, content)
				_, _ = engine.extractMatch(p, content)
			}
		}()
	}

	wg.Wait()
	t.Log("P0-06 验证: 并发读写测试完成")
}
