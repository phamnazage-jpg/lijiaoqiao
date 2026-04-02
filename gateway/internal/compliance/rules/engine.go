package rules

import (
	"regexp"
)

// MatchResult 匹配结果
type MatchResult struct {
	Matched  bool
	RuleID   string
	Matchers []MatcherResult
}

// MatcherResult 单个匹配器的结果
type MatcherResult struct {
	MatcherIndex int
	MatcherType string
	Pattern     string
	MatchValue  string
	IsMatch     bool
}

// RuleEngine 规则引擎
type RuleEngine struct {
	loader     *RuleLoader
	compiledPatterns map[string][]*regexp.Regexp
}

// NewRuleEngine 创建新的规则引擎
func NewRuleEngine(loader *RuleLoader) *RuleEngine {
	return &RuleEngine{
		loader:          loader,
		compiledPatterns: make(map[string][]*regexp.Regexp),
	}
}

// Match 执行规则匹配
func (e *RuleEngine) Match(rule Rule, content string) MatchResult {
	result := MatchResult{
		Matched:  false,
		RuleID:   rule.ID,
		Matchers: make([]MatcherResult, len(rule.Matchers)),
	}

	for i, matcher := range rule.Matchers {
		matcherResult := MatcherResult{
			MatcherIndex: i,
			MatcherType: matcher.Type,
			Pattern:     matcher.Pattern,
			IsMatch:     false,
		}

		switch matcher.Type {
		case "regex_match":
			matcherResult.IsMatch = e.matchRegex(matcher.Pattern, content)
			if matcherResult.IsMatch {
				matcherResult.MatchValue = e.extractMatch(matcher.Pattern, content)
			}
		default:
			// 未知匹配器类型，默认不匹配
		}

		result.Matchers[i] = matcherResult
		if matcherResult.IsMatch {
			result.Matched = true
		}
	}

	return result
}

// matchRegex 执行正则表达式匹配
func (e *RuleEngine) matchRegex(pattern string, content string) bool {
	// 编译并缓存正则表达式
	regex, ok := e.compiledPatterns[pattern]
	if !ok {
		var err error
		regex = make([]*regexp.Regexp, 1)
		regex[0], err = regexp.Compile(pattern)
		if err != nil {
			return false
		}
		e.compiledPatterns[pattern] = regex
	}

	return regex[0].MatchString(content)
}

// extractMatch 提取匹配值
func (e *RuleEngine) extractMatch(pattern string, content string) string {
	regex, ok := e.compiledPatterns[pattern]
	if !ok {
		regex = make([]*regexp.Regexp, 1)
		regex[0], _ = regexp.Compile(pattern)
		e.compiledPatterns[pattern] = regex
	}

	matches := regex[0].FindString(content)
	return matches
}

// MatchFromConfig 从规则配置执行匹配
func (e *RuleEngine) MatchFromConfig(ruleID string, ruleConfig Rule, content string) (bool, error) {
	// 验证规则
	if err := e.validateRuleForMatch(ruleConfig); err != nil {
		return false, err
	}

	result := e.Match(ruleConfig, content)
	return result.Matched, nil
}

// validateRuleForMatch 验证规则是否可用于匹配
func (e *RuleEngine) validateRuleForMatch(rule Rule) error {
	if rule.ID == "" {
		return ErrInvalidRule
	}
	if len(rule.Matchers) == 0 {
		return ErrNoMatchers
	}
	return nil
}

// Custom errors
var (
	ErrInvalidRule = &RuleEngineError{"invalid rule: missing required fields"}
	ErrNoMatchers  = &RuleEngineError{"invalid rule: no matchers defined"}
)

// RuleEngineError 规则引擎错误
type RuleEngineError struct {
	Message string
}

func (e *RuleEngineError) Error() string {
	return e.Message
}
