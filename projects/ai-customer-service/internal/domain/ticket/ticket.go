package ticket

import "time"

type Status string

type Priority string

const (
	StatusOpen       Status = "open"
	StatusAssigned   Status = "assigned"
	StatusProcessing Status = "processing"
	StatusResolved   Status = "resolved"
	StatusClosed     Status = "closed"
)

const (
	PriorityP0 Priority = "P0"
	PriorityP1 Priority = "P1"
	PriorityP2 Priority = "P2"
	PriorityP3 Priority = "P3"
)

type Ticket struct {
	ID              string                 `json:"id"`
	SessionID       string                 `json:"session_id"`
	UserID          string                 `json:"user_id,omitempty"`
	Priority        Priority               `json:"priority"`
	Status          Status                 `json:"status"`
	HandoffReason   string                 `json:"handoff_reason"`
	AssignedTo      string                 `json:"assigned_to,omitempty"`
	ContextSnapshot map[string]any         `json:"context_snapshot"`
	Resolution      string                 `json:"resolution,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	ResolvedAt      *time.Time             `json:"resolved_at,omitempty"`
	UpdatedAt       time.Time              `json:"updated_at"`
}
