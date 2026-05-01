package handoff

import (
	"context"

	domain "github.com/bridge/ai-customer-service/internal/domain/intent"
)

type Decision struct {
	ShouldHandoff bool   `json:"should_handoff"`
	Reason        string `json:"reason"`
	Priority      string `json:"priority"`
}

type Service struct{}

func NewService() *Service { return &Service{} }

func (s *Service) ShouldHandoff(_ context.Context, intent *domain.Result, turnCount int) (*Decision, error) {
	if intent == nil {
		return &Decision{}, nil
	}
	if intent.NeedsHuman || intent.Sensitive {
		return &Decision{ShouldHandoff: true, Reason: intent.Intent, Priority: "P1"}, nil
	}
	if turnCount >= 5 && intent.Confidence < 0.60 {
		return &Decision{ShouldHandoff: true, Reason: "low_confidence", Priority: "P2"}, nil
	}
	return &Decision{ShouldHandoff: false, Priority: "P3"}, nil
}
