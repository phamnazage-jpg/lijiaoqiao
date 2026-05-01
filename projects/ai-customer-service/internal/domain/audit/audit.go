package audit

import "time"

type Event struct {
	ID          string                 `json:"id"`
	SessionID   string                 `json:"session_id,omitempty"`
	TicketID    string                 `json:"ticket_id,omitempty"`
	Type        string                 `json:"type"`
	Action      string                 `json:"action,omitempty"`
	Channel     string                 `json:"channel,omitempty"`
	OpenID      string                 `json:"open_id,omitempty"`
	ActorID     string                 `json:"actor_id,omitempty"`
	SourceIP    string                 `json:"source_ip,omitempty"`
	Payload     map[string]any         `json:"payload,omitempty"`
	BeforeState map[string]any         `json:"before_state,omitempty"`
	AfterState  map[string]any         `json:"after_state,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}
