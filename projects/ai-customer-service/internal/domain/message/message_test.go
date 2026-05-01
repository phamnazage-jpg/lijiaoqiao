package message

import (
	"testing"
	"time"
)

func TestUnifiedMessage_Fields(t *testing.T) {
	now := time.Now()
	msg := UnifiedMessage{
		MessageID:   "msg_123",
		Channel:     "widget",
		OpenID:      "user_456",
		UserID:      "internal_789",
		Content:     "hello world",
		ContentType: "text/plain",
		Timestamp:   now,
		ReplyTo:     "parent_msg",
	}
	if msg.MessageID != "msg_123" {
		t.Errorf("MessageID = %q, want %q", msg.MessageID, "msg_123")
	}
	if msg.Channel != "widget" {
		t.Errorf("Channel = %q, want %q", msg.Channel, "widget")
	}
	if msg.Content != "hello world" {
		t.Errorf("Content = %q, want %q", msg.Content, "hello world")
	}
	if msg.ReplyTo != "parent_msg" {
		t.Errorf("ReplyTo = %q, want %q", msg.ReplyTo, "parent_msg")
	}
}

func TestUnifiedMessage_OptionalFields(t *testing.T) {
	msg := UnifiedMessage{
		MessageID: "msg_1",
		Channel:   "web",
		OpenID:    "u1",
		Content:   "hi",
	}
	if msg.UserID != "" {
		t.Errorf("UserID = %q, want empty", msg.UserID)
	}
	if msg.ContentType != "" {
		t.Errorf("ContentType = %q, want empty", msg.ContentType)
	}
	if msg.ReplyTo != "" {
		t.Errorf("ReplyTo = %q, want empty", msg.ReplyTo)
	}
}

func TestUnifiedMessage_Timestamp(t *testing.T) {
	now := time.Now()
	msg := UnifiedMessage{
		MessageID: "msg_1",
		Channel:   "widget",
		OpenID:    "u1",
		Content:   "test",
		Timestamp: now,
	}
	if msg.Timestamp.IsZero() {
		t.Error("Timestamp is zero, want time.Time")
	}
}

func TestUnifiedMessage_EmptyContent(t *testing.T) {
	msg := UnifiedMessage{
		MessageID: "msg_1",
		Channel:   "widget",
		OpenID:    "u1",
		Content:   "",
	}
	if msg.Content != "" {
		t.Errorf("Content = %q, want empty", msg.Content)
	}
}
