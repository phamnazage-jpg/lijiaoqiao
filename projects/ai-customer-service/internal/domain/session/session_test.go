package session

import (
	"testing"
	"time"
)

func TestSession_ID(t *testing.T) {
	sess := Session{
		ID: "channel:openid-123",
	}
	if sess.ID != "channel:openid-123" {
		t.Errorf("expected ID 'channel:openid-123', got %q", sess.ID)
	}
}

func TestSession_Channel(t *testing.T) {
	sess := Session{
		Channel: "wechat",
	}
	if sess.Channel != "wechat" {
		t.Errorf("expected Channel 'wechat', got %q", sess.Channel)
	}
}

func TestSession_OpenID(t *testing.T) {
	sess := Session{
		OpenID: "ou_abc123",
	}
	if sess.OpenID != "ou_abc123" {
		t.Errorf("expected OpenID 'ou_abc123', got %q", sess.OpenID)
	}
}

func TestSession_StatusConstants(t *testing.T) {
	if StatusIdle != "idle" {
		t.Errorf("StatusIdle: expected 'idle', got %q", StatusIdle)
	}
	if StatusProcessing != "processing" {
		t.Errorf("StatusProcessing: expected 'processing', got %q", StatusProcessing)
	}
	if StatusHandoff != "handoff" {
		t.Errorf("StatusHandoff: expected 'handoff', got %q", StatusHandoff)
	}
	if StatusClosed != "closed" {
		t.Errorf("StatusClosed: expected 'closed', got %q", StatusClosed)
	}
}

func TestSession_StatusTransitions(t *testing.T) {
	tests := []struct {
		name       string
		initial    Status
		transition Status
	}{
		{"idle to processing", StatusIdle, StatusProcessing},
		{"processing to handoff", StatusProcessing, StatusHandoff},
		{"handoff to closed", StatusHandoff, StatusClosed},
		{"idle directly to closed", StatusIdle, StatusClosed},
	}

	for _, tt := range tests {
		sess := Session{Status: tt.initial}
		if sess.Status != tt.initial {
			t.Errorf("%s: expected status %q, got %q", tt.name, tt.initial, sess.Status)
		}
		sess.Status = tt.transition
		if sess.Status != tt.transition {
			t.Errorf("%s: expected transitioned status %q, got %q", tt.name, tt.transition, sess.Status)
		}
	}
}

func TestSession_TurnCount(t *testing.T) {
	sess := Session{TurnCount: 0}
	if sess.TurnCount != 0 {
		t.Errorf("expected TurnCount 0, got %d", sess.TurnCount)
	}

	sess.TurnCount = 5
	if sess.TurnCount != 5 {
		t.Errorf("expected TurnCount 5, got %d", sess.TurnCount)
	}
}

func TestSession_LastMessageAt(t *testing.T) {
	now := time.Now()
	sess := Session{LastMessageAt: now}
	if !sess.LastMessageAt.Equal(now) {
		t.Errorf("LastMessageAt: expected %v, got %v", now, sess.LastMessageAt)
	}
}

func TestSession_Context(t *testing.T) {
	now := time.Now()
	sess := Session{
		Context: []MessageContext{
			{Direction: "inbound", Content: "hello", Timestamp: now},
			{Direction: "outbound", Content: "hi there", Timestamp: now},
		},
	}

	if len(sess.Context) != 2 {
		t.Errorf("expected 2 context entries, got %d", len(sess.Context))
	}
	if sess.Context[0].Content != "hello" {
		t.Errorf("expected first content 'hello', got %q", sess.Context[0].Content)
	}
	if sess.Context[1].Direction != "outbound" {
		t.Errorf("expected second direction 'outbound', got %q", sess.Context[1].Direction)
	}
}

func TestSession_EmptyContext(t *testing.T) {
	sess := Session{Context: []MessageContext{}}
	if len(sess.Context) != 0 {
		t.Errorf("expected empty context, got %d entries", len(sess.Context))
	}
}

func TestSession_UserID(t *testing.T) {
	sess := Session{UserID: "user-456"}
	if sess.UserID != "user-456" {
		t.Errorf("expected UserID 'user-456', got %q", sess.UserID)
	}

	// UserID can be empty
	sess2 := Session{}
	if sess2.UserID != "" {
		t.Errorf("expected empty UserID, got %q", sess2.UserID)
	}
}

func TestMessageContext(t *testing.T) {
	now := time.Now()
	msg := MessageContext{
		Direction: "inbound",
		Content:   "test message",
		Timestamp: now,
	}

	if msg.Direction != "inbound" {
		t.Errorf("Direction: expected 'inbound', got %q", msg.Direction)
	}
	if msg.Content != "test message" {
		t.Errorf("Content: expected 'test message', got %q", msg.Content)
	}
	if !msg.Timestamp.Equal(now) {
		t.Errorf("Timestamp: expected %v, got %v", now, msg.Timestamp)
	}
}

func TestSession_FullLifecycle(t *testing.T) {
	now := time.Now()
	sess := Session{
		ID:            "wechat:ou_abc",
		Channel:       "wechat",
		OpenID:        "ou_abc",
		Status:        StatusIdle,
		TurnCount:     0,
		LastMessageAt: now,
		Context:       []MessageContext{},
	}

	// Idle -> Processing
	sess.Status = StatusProcessing
	sess.TurnCount++
	if sess.Status != StatusProcessing {
		t.Error("failed to transition to Processing")
	}

	// Add message
	sess.Context = append(sess.Context, MessageContext{
		Direction: "inbound",
		Content:   "I need help",
		Timestamp: now,
	})

	// Processing -> Handoff
	sess.Status = StatusHandoff
	if sess.Status != StatusHandoff {
		t.Error("failed to transition to Handoff")
	}

	// Handoff -> Closed
	sess.Status = StatusClosed
	if sess.Status != StatusClosed {
		t.Error("failed to transition to Closed")
	}
}