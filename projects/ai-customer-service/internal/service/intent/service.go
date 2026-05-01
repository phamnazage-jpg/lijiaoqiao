package intent

import (
	"context"
	"strings"

	domain "github.com/bridge/ai-customer-service/internal/domain/intent"
	"github.com/bridge/ai-customer-service/internal/domain/session"
)

type Service struct{}

func NewService() *Service { return &Service{} }

func (s *Service) Recognize(_ context.Context, _ string, message string, _ []session.MessageContext) (*domain.Result, error) {
	content := strings.ToLower(strings.TrimSpace(message))
	result := &domain.Result{
		Intent:     domain.IntentGeneral,
		Confidence: 0.65,
		Entities:   map[string]string{},
	}

	switch {
	case containsAny(content, "退款", "refund"):
		result.Intent = domain.IntentRefund
		result.Confidence = 0.99
		result.NeedsHuman = true
		result.Sensitive = true
	case containsAny(content, "泄露", "安全", "被盗", "攻击"):
		result.Intent = domain.IntentSecurity
		result.Confidence = 0.99
		result.NeedsHuman = true
		result.Sensitive = true
	case containsAny(content, "人工", "客服", "human"):
		result.Intent = domain.IntentHandoff
		result.Confidence = 0.98
		result.NeedsHuman = true
	case containsAny(content, "额度", "配额", "quota"):
		result.Intent = domain.IntentQuota
		result.Confidence = 0.92
	case containsAny(content, "token", "消耗", "用量"):
		result.Intent = domain.IntentToken
		result.Confidence = 0.91
	case containsAny(content, "报错", "错误", "error", "异常"):
		result.Intent = domain.IntentError
		result.Confidence = 0.88
	}

	return result, nil
}

func containsAny(content string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(content, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
