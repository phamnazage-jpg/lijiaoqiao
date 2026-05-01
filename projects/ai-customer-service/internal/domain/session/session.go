package session

import "time"

type Status string

const (
	StatusIdle        Status = "idle"
	StatusProcessing  Status = "processing"
	StatusHandoff     Status = "handoff"
	StatusClosed      Status = "closed"
)

type MessageContext struct {
	Direction string    `json:"direction"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type Session struct {
	ID            string           `json:"id"`
	Channel       string           `json:"channel"`
	OpenID        string           `json:"open_id"`
	UserID        string           `json:"user_id,omitempty"`
	Status        Status           `json:"status"`
	TurnCount     int              `json:"turn_count"`
	LastMessageAt time.Time        `json:"last_message_at"`
	Context       []MessageContext `json:"context"`
}
