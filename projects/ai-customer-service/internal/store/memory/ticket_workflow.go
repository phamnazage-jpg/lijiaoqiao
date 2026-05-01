package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/ticket"
)

func (s *TicketStore) ListOpen(_ context.Context, limit int) ([]ticket.Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.tickets) {
		limit = len(s.tickets)
	}
	items := make([]ticket.Ticket, 0, limit)
	for _, item := range s.tickets {
		if item.Status == ticket.StatusOpen || item.Status == ticket.StatusAssigned || item.Status == ticket.StatusProcessing {
			items = append(items, item)
			if len(items) == limit {
				break
			}
		}
	}
	return items, nil
}

func (s *TicketStore) Assign(_ context.Context, ticketID, agentID, _, _ string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tickets {
		if s.tickets[i].ID == ticketID && s.tickets[i].Status == ticket.StatusOpen {
			s.tickets[i].AssignedTo = agentID
			s.tickets[i].Status = ticket.StatusAssigned
			s.tickets[i].UpdatedAt = now
			return nil
		}
	}
	return fmt.Errorf("ticket not assignable")
}

func (s *TicketStore) Resolve(_ context.Context, ticketID, resolution, _, _ string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tickets {
		if s.tickets[i].ID == ticketID {
			resolvedAt := now
			s.tickets[i].Resolution = resolution
			s.tickets[i].Status = ticket.StatusResolved
			s.tickets[i].ResolvedAt = &resolvedAt
			s.tickets[i].UpdatedAt = now
			return nil
		}
	}
	return fmt.Errorf("ticket not resolvable")
}

func (s *TicketStore) Close(_ context.Context, ticketID, resolution, _, _ string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tickets {
		if s.tickets[i].ID == ticketID && (s.tickets[i].Status == ticket.StatusResolved || s.tickets[i].Status == ticket.StatusAssigned || s.tickets[i].Status == ticket.StatusProcessing) {
			resolvedAt := now
			s.tickets[i].Resolution = resolution
			s.tickets[i].Status = ticket.StatusClosed
			if s.tickets[i].ResolvedAt == nil {
				s.tickets[i].ResolvedAt = &resolvedAt
			}
			s.tickets[i].UpdatedAt = now
			return nil
		}
	}
	return fmt.Errorf("ticket not closable")
}
