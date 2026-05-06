package platformadapter

import "time"

type Sub2APIInboundPayload struct {
	MessageID   string    `json:"message_id"`
	Channel     string    `json:"channel"`
	OpenID      string    `json:"open_id"`
	UserID      string    `json:"user_id,omitempty"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type,omitempty"`
	Timestamp   time.Time `json:"timestamp,omitempty"`
	ReplyTo     string    `json:"reply_to,omitempty"`
}
