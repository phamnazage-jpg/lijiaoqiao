package memory

import (
	"context"
	"sync"

	"github.com/bridge/ai-customer-service/internal/domain/ticket"
	"github.com/bridge/ai-customer-service/internal/domain/ticketstats"
)

type TicketStore struct {
	mu      sync.RWMutex
	tickets []ticket.Ticket
}

func NewTicketStore() *TicketStore {
	return &TicketStore{tickets: make([]ticket.Ticket, 0, 8)}
}

func (s *TicketStore) Create(_ context.Context, t *ticket.Ticket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tickets = append(s.tickets, *t)
	return nil
}

func (s *TicketStore) List() []ticket.Ticket {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]ticket.Ticket, len(s.tickets))
	copy(items, s.tickets)
	return items
}

func (s *TicketStore) ListAll(_ context.Context) ([]ticket.Ticket, error) {
	return s.List(), nil
}

func (s *TicketStore) GetByID(_ context.Context, id string) (*ticket.Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.tickets {
		if s.tickets[i].ID == id {
			return &s.tickets[i], nil
		}
	}
	return nil, nil
}

// GetStats aggregates ticket statistics in memory.
func (s *TicketStore) GetStats(_ context.Context) (ticketstats.Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var stats ticketstats.Stats
	stats.ByChannel = make(map[string]int)
	stats.ByPriority = make(map[string]int)

	for _, t := range s.tickets {
		stats.Total++
		// Count by status
		switch t.Status {
		case ticket.StatusOpen, ticket.StatusAssigned, ticket.StatusProcessing:
			stats.Open++
		case ticket.StatusResolved:
			stats.Resolved++
		case ticket.StatusClosed:
			stats.Closed++
		}
		// Count by priority
		stats.ByPriority[string(t.Priority)]++
		// Channel from context snapshot
		if ch, ok := t.ContextSnapshot["channel"].(string); ok {
			stats.ByChannel[ch]++
		}
		// Handoff count
		if t.HandoffReason != "" {
			stats.HandoffCount++
		}
		// Resolution time
		if t.ResolvedAt != nil {
			diff := t.ResolvedAt.Sub(t.CreatedAt).Seconds()
			stats.AvgResolutionTimeMinutes += diff / 60.0
		}
	}

	// Compute average resolution time
	resolvedCount := stats.Resolved + stats.Closed
	if resolvedCount > 0 {
		stats.AvgResolutionTimeMinutes /= float64(resolvedCount)
	}

	return stats, nil
}

// Assign, Resolve, Close, ListOpen are defined in ticket_workflow.go
// to match the handlers.TicketService interface signature.
