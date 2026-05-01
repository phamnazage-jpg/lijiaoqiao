package reply

import (
	"context"

	domain "github.com/bridge/ai-customer-service/internal/domain/intent"
	"github.com/bridge/ai-customer-service/internal/store/memory"
)

type Service struct {
	knowledge *memory.KnowledgeStore
}

func NewService(knowledge *memory.KnowledgeStore) *Service {
	return &Service{knowledge: knowledge}
}

func (s *Service) Generate(_ context.Context, intent *domain.Result) string {
	if intent == nil {
		return s.knowledge.Answer(domain.IntentGeneral)
	}
	return s.knowledge.Answer(intent.Intent)
}
