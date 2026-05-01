package intent

import (
	"context"
	"testing"
)

func TestRecognize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		intent  string
		handoff bool
	}{
		{name: "quota", input: "我的配额还剩多少", intent: "quota", handoff: false},
		{name: "token", input: "今天 token 消耗是多少", intent: "token", handoff: false},
		{name: "refund", input: "我要申请退款", intent: "refund", handoff: true},
		{name: "security", input: "我的数据可能泄露了", intent: "security", handoff: true},
		{name: "general_default", input: "你好", intent: "general", handoff: false},
		{name: "error_keyword", input: "系统报错啦", intent: "error", handoff: false},
		{name: "handoff_human", input: "转人工客服", intent: "handoff", handoff: true},
		{name: "security_attack", input: "遭到攻击了", intent: "security", handoff: true},
	}

	svc := NewService()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.Recognize(context.Background(), "s1", tt.input, nil)
			if err != nil {
				t.Fatalf("Recognize() error = %v", err)
			}
			if got.Intent != tt.intent {
				t.Fatalf("intent = %s, want %s", got.Intent, tt.intent)
			}
			if got.NeedsHuman != tt.handoff {
				t.Fatalf("NeedsHuman = %v, want %v", got.NeedsHuman, tt.handoff)
			}
		})
	}
}

func TestRecognize_GeneralIntent(t *testing.T) {
	svc := NewService()
	got, err := svc.Recognize(context.Background(), "s1", "今天天气不错", nil)
	if err != nil {
		t.Fatalf("Recognize() error = %v", err)
	}
	if got.Intent != "general" {
		t.Errorf("intent = %s, want general", got.Intent)
	}
	if got.Confidence != 0.65 {
		t.Errorf("Confidence = %f, want 0.65", got.Confidence)
	}
}

func TestRecognize_CaseInsensitive(t *testing.T) {
	svc := NewService()
	got, err := svc.Recognize(context.Background(), "s1", "REFUND", nil)
	if err != nil {
		t.Fatalf("Recognize() error = %v", err)
	}
	if got.Intent != "refund" {
		t.Errorf("intent = %s, want refund (case insensitive)", got.Intent)
	}
}

func TestRecognize_WhitespaceTrimmed(t *testing.T) {
	svc := NewService()
	got, err := svc.Recognize(context.Background(), "s1", "  退款  ", nil)
	if err != nil {
		t.Fatalf("Recognize() error = %v", err)
	}
	if got.Intent != "refund" {
		t.Errorf("intent = %s, want refund (whitespace trimmed)", got.Intent)
	}
}

func TestContainsAny_Found(t *testing.T) {
	if !containsAny("hello world", "world") {
		t.Error("containsAny(hello world, world) = false, want true")
	}
}

func TestContainsAny_NotFound(t *testing.T) {
	if containsAny("hello world", "foo") {
		t.Error("containsAny(hello world, foo) = true, want false")
	}
}

func TestContainsAny_EmptyTerms(t *testing.T) {
	if containsAny("hello", "world") {
		t.Error("containsAny(hello, world) = true, want false")
	}
}
