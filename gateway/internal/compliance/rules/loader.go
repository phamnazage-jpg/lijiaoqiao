package rules

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Rule 定义合规规则结构
type Rule struct {
	ID          string      `yaml:"id"`
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Severity    string      `yaml:"severity"`
	Matchers    []Matcher   `yaml:"matchers"`
	Action      Action      `yaml:"action"`
	Audit       Audit       `yaml:"audit"`
}

// Matcher 定义规则匹配器
type Matcher struct {
	Type    string `yaml:"type"`
	Pattern string `yaml:"pattern"`
	Target  string `yaml:"target"`
	Scope   string `yaml:"scope"`
}

// Action 定义规则动作
type Action struct {
	Primary   string `yaml:"primary"`
	Secondary string `yaml:"secondary"`
}

// Audit 定义审计配置
type Audit struct {
	EventName       string `yaml:"event_name"`
	EventCategory   string `yaml:"event_category"`
	EventSubCategory string `yaml:"event_sub_category"`
}

// RulesConfig YAML规则配置结构
type RulesConfig struct {
	Rules []Rule `yaml:"rules"`
}

// RuleLoader 规则加载器
type RuleLoader struct {
	ruleIDPattern *regexp.Regexp
}

// NewRuleLoader 创建新的规则加载器
func NewRuleLoader() *RuleLoader {
	// 规则ID格式: {Category}-{SubCategory}[-{Detail}]
	// Category: 大写字母, 2-4字符
	// SubCategory: 大写字母, 2-10字符
	// Detail: 可选, 大写字母+数字+连字符, 1-20字符
	pattern := regexp.MustCompile(`^[A-Z]{2,4}-[A-Z]{2,10}(-[A-Z0-9-]{1,20})?$`)

	return &RuleLoader{
		ruleIDPattern: pattern,
	}
}

// LoadFromFile 从YAML文件加载规则
func (l *RuleLoader) LoadFromFile(filePath string) ([]Rule, error) {
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	// 读取文件内容
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// 解析YAML
	var config RulesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// 验证规则
	for _, rule := range config.Rules {
		if err := l.validateRule(rule); err != nil {
			return nil, err
		}
	}

	return config.Rules, nil
}

// validateRule 验证规则完整性
func (l *RuleLoader) validateRule(rule Rule) error {
	// 检查必需字段
	if rule.ID == "" {
		return fmt.Errorf("missing required field: id")
	}
	if rule.Name == "" {
		return fmt.Errorf("missing required field: name for rule %s", rule.ID)
	}
	if rule.Severity == "" {
		return fmt.Errorf("missing required field: severity for rule %s", rule.ID)
	}
	if len(rule.Matchers) == 0 {
		return fmt.Errorf("missing required field: matchers for rule %s", rule.ID)
	}
	if rule.Action.Primary == "" {
		return fmt.Errorf("missing required field: action.primary for rule %s", rule.ID)
	}

	// 验证规则ID格式
	if !l.ValidateRuleID(rule.ID) {
		return fmt.Errorf("invalid rule ID format: %s (expected format: {Category}-{SubCategory}[-{Detail}])", rule.ID)
	}

	// 验证每个匹配器
	for i, matcher := range rule.Matchers {
		if matcher.Type == "" {
			return fmt.Errorf("missing required field: matchers[%d].type for rule %s", i, rule.ID)
		}
		if matcher.Pattern == "" {
			return fmt.Errorf("missing required field: matchers[%d].pattern for rule %s", i, rule.ID)
		}
		// 验证正则表达式是否有效
		if _, err := regexp.Compile(matcher.Pattern); err != nil {
			return fmt.Errorf("invalid regex pattern in matchers[%d] for rule %s: %w", i, rule.ID, err)
		}
	}

	return nil
}

// ValidateRuleID 验证规则ID格式
func (l *RuleLoader) ValidateRuleID(ruleID string) bool {
	return l.ruleIDPattern.MatchString(ruleID)
}
