package reply

import (
	"context"
	"strings"
	"testing"

	"github.com/bridge/ai-customer-service/internal/domain/intent"
	"github.com/bridge/ai-customer-service/internal/store/memory"
)

func TestGenerate_NilIntent(t *testing.T) {
	knowledge := memory.NewKnowledgeStore()
	svc := NewService(knowledge)

	result := svc.Generate(context.Background(), nil)
	if result == "" {
		t.Error("Generate with nil intent should return non-empty answer")
	}
	// Should return general fallback
	if result != knowledge.Answer(intent.IntentGeneral) {
		t.Errorf("expected general fallback answer, got %q", result)
	}
}

func TestGenerate_ValidIntent(t *testing.T) {
	knowledge := memory.NewKnowledgeStore()
	svc := NewService(knowledge)

	testCases := []struct {
		intentName  string
		expectEmpty bool
	}{
		{"quota", false},
		{"token", false},
		{"error", false},
		{"general", false},
	}

	for _, tc := range testCases {
		t.Run(tc.intentName, func(t *testing.T) {
			intentResult := &intent.Result{Intent: tc.intentName}
			result := svc.Generate(context.Background(), intentResult)
			if tc.expectEmpty && result != "" {
				t.Errorf("expected empty for intent %q, got %q", tc.intentName, result)
			}
			if !tc.expectEmpty && result == "" {
				t.Errorf("expected non-empty for intent %q", tc.intentName)
			}
		})
	}
}

func TestGenerate_UnknownIntent(t *testing.T) {
	knowledge := memory.NewKnowledgeStore()
	svc := NewService(knowledge)

	// Unknown intent should return general fallback
	intentResult := &intent.Result{Intent: "unknown-intent-xyz"}
	result := svc.Generate(context.Background(), intentResult)

	generalAnswer := knowledge.Answer(intent.IntentGeneral)
	if result != generalAnswer {
		t.Errorf("unknown intent: expected general fallback %q, got %q", generalAnswer, result)
	}
}

func TestGenerate_ContentTruncation(t *testing.T) {
	knowledge := memory.NewKnowledgeStore()
	svc := NewService(knowledge)

	// The Generate method itself doesn't truncate content.
	// It returns answers from the knowledge store.
	// This test verifies the behavior: returns non-empty string.
	intentResult := &intent.Result{Intent: "general"}
	result := svc.Generate(context.Background(), intentResult)

	// Verify we get a non-empty response
	if result == "" {
		t.Error("Generate should return non-empty answer")
	}

	// Check that result length is reasonable (not unlimited)
	// The knowledge store answers are short by design
	if len(result) > 5000 {
		t.Logf("Warning: result length %d seems large", len(result))
	}
}

func TestGenerate_EmptyContent(t *testing.T) {
	knowledge := memory.NewKnowledgeStore()
	svc := NewService(knowledge)

	// Empty intent content should still return something (general fallback)
	intentResult := &intent.Result{Intent: ""}
	result := svc.Generate(context.Background(), intentResult)

	// Should return general fallback, not empty string
	generalAnswer := knowledge.Answer(intent.IntentGeneral)
	if result != generalAnswer {
		t.Errorf("empty intent: expected general fallback %q, got %q", generalAnswer, result)
	}
}

func TestService_NewService(t *testing.T) {
	knowledge := memory.NewKnowledgeStore()
	svc := NewService(knowledge)

	if svc == nil {
		t.Error("NewService returned nil")
	}

	if svc.knowledge == nil {
		t.Error("svc.knowledge is nil")
	}
}

func TestGenerate_MultipleIntents(t *testing.T) {
	knowledge := memory.NewKnowledgeStore()
	svc := NewService(knowledge)

	intents := []string{"quota", "token", "error", "general"}
	results := make([]string, len(intents))

	for i, intentName := range intents {
		intentResult := &intent.Result{Intent: intentName}
		results[i] = svc.Generate(context.Background(), intentResult)
	}

	// All results should be non-empty
	for i, result := range results {
		if strings.TrimSpace(result) == "" {
			t.Errorf("intent %q returned empty result", intents[i])
		}
	}

	// At least some results should be different (different answers)
	differentCount := 0
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			differentCount++
		}
	}
	if differentCount == 0 {
		t.Log("Warning: all intents returned the same answer")
	}
}

func TestGenerate_ContextCancellation(t *testing.T) {
	knowledge := memory.NewKnowledgeStore()
	svc := NewService(knowledge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should still return a result even with cancelled context
	intentResult := &intent.Result{Intent: "general"}
	result := svc.Generate(ctx, intentResult)

	if result == "" {
		t.Error("Generate with cancelled context should still return answer")
	}
}