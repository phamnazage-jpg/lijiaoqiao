package handoff

import (
	"context"
	"testing"

	intentdomain "github.com/bridge/ai-customer-service/internal/domain/intent"
)

func TestShouldHandoff(t *testing.T) {
	svc := NewService()
	decision, err := svc.ShouldHandoff(context.Background(), &intentdomain.Result{Intent: intentdomain.IntentRefund, NeedsHuman: true, Sensitive: true, Confidence: 0.99}, 1)
	if err != nil {
		t.Fatalf("ShouldHandoff() error = %v", err)
	}
	if !decision.ShouldHandoff || decision.Priority != "P1" {
		t.Fatalf("unexpected decision: %+v", decision)
	}

	decision, err = svc.ShouldHandoff(context.Background(), &intentdomain.Result{Intent: intentdomain.IntentGeneral, Confidence: 0.5}, 5)
	if err != nil {
		t.Fatalf("ShouldHandoff() error = %v", err)
	}
	if !decision.ShouldHandoff || decision.Priority != "P2" {
		t.Fatalf("unexpected low confidence decision: %+v", decision)
	}
}

// TestShouldHandoff_ConfidenceBoundary tests the 0.60 confidence threshold.
// turnCount >= 5 AND confidence < 0.60 → handoff P2
// turnCount >= 5 AND confidence >= 0.60 → no handoff
func TestShouldHandoff_ConfidenceBoundary(t *testing.T) {
	svc := NewService()

	// confidence = 0.59 (below 0.60) at turnCount = 5 → handoff P2
	d, err := svc.ShouldHandoff(context.Background(), &intentdomain.Result{Intent: intentdomain.IntentGeneral, Confidence: 0.59}, 5)
	if err != nil {
		t.Fatalf("ShouldHandoff() error = %v", err)
	}
	if !d.ShouldHandoff || d.Priority != "P2" {
		t.Fatalf("turnCount=5, confidence=0.59: expected handoff P2, got %+v", d)
	}

	// confidence = 0.60 (at threshold) at turnCount = 5 → no handoff
	d, err = svc.ShouldHandoff(context.Background(), &intentdomain.Result{Intent: intentdomain.IntentGeneral, Confidence: 0.60}, 5)
	if err != nil {
		t.Fatalf("ShouldHandoff() error = %v", err)
	}
	if d.ShouldHandoff {
		t.Fatalf("turnCount=5, confidence=0.60: expected no handoff, got %+v", d)
	}

	// confidence = 0.61 (above 0.60) at turnCount = 5 → no handoff
	d, err = svc.ShouldHandoff(context.Background(), &intentdomain.Result{Intent: intentdomain.IntentGeneral, Confidence: 0.61}, 5)
	if err != nil {
		t.Fatalf("ShouldHandoff() error = %v", err)
	}
	if d.ShouldHandoff {
		t.Fatalf("turnCount=5, confidence=0.61: expected no handoff, got %+v", d)
	}

	// confidence = 0.59 at turnCount = 4 (below turn threshold) → no handoff
	d, err = svc.ShouldHandoff(context.Background(), &intentdomain.Result{Intent: intentdomain.IntentGeneral, Confidence: 0.59}, 4)
	if err != nil {
		t.Fatalf("ShouldHandoff() error = %v", err)
	}
	if d.ShouldHandoff {
		t.Fatalf("turnCount=4, confidence=0.59: expected no handoff, got %+v", d)
	}
}

// TestShouldHandoff_TurnCountBoundary tests the turnCount >= 5 threshold.
func TestShouldHandoff_TurnCountBoundary(t *testing.T) {
	svc := NewService()

	// turnCount = 4, confidence below 0.6 → no handoff (turn threshold not met)
	d, err := svc.ShouldHandoff(context.Background(), &intentdomain.Result{Intent: intentdomain.IntentGeneral, Confidence: 0.5}, 4)
	if err != nil {
		t.Fatalf("ShouldHandoff() error = %v", err)
	}
	if d.ShouldHandoff {
		t.Fatalf("turnCount=4: expected no handoff, got %+v", d)
	}

	// turnCount = 5, confidence below 0.6 → handoff P2
	d, err = svc.ShouldHandoff(context.Background(), &intentdomain.Result{Intent: intentdomain.IntentGeneral, Confidence: 0.5}, 5)
	if err != nil {
		t.Fatalf("ShouldHandoff() error = %v", err)
	}
	if !d.ShouldHandoff || d.Priority != "P2" {
		t.Fatalf("turnCount=5: expected handoff P2, got %+v", d)
	}

	// turnCount = 6 (well above threshold), confidence below 0.6 → handoff P2
	d, err = svc.ShouldHandoff(context.Background(), &intentdomain.Result{Intent: intentdomain.IntentGeneral, Confidence: 0.3}, 6)
	if err != nil {
		t.Fatalf("ShouldHandoff() error = %v", err)
	}
	if !d.ShouldHandoff || d.Priority != "P2" {
		t.Fatalf("turnCount=6: expected handoff P2, got %+v", d)
	}
}

// TestShouldHandoff_NilIntent returns no-handoff decision.
func TestShouldHandoff_NilIntent(t *testing.T) {
	svc := NewService()
	d, err := svc.ShouldHandoff(context.Background(), nil, 10)
	if err != nil {
		t.Fatalf("ShouldHandoff() error = %v", err)
	}
	if d.ShouldHandoff {
		t.Fatalf("nil intent: expected no handoff, got %+v", d)
	}
}

// TestShouldHandoff_NeedsHuman takes priority over confidence/turnCount.
func TestShouldHandoff_NeedsHumanTakesPriority(t *testing.T) {
	svc := NewService()
	d, err := svc.ShouldHandoff(context.Background(), &intentdomain.Result{Intent: intentdomain.IntentGeneral, NeedsHuman: true, Confidence: 0.1}, 1)
	if err != nil {
		t.Fatalf("ShouldHandoff() error = %v", err)
	}
	if !d.ShouldHandoff || d.Priority != "P1" {
		t.Fatalf("NeedsHuman=true: expected handoff P1, got %+v", d)
	}
}
