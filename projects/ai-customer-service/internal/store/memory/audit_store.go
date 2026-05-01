package memory

import (
	"context"
	"sync"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
)

type AuditStore struct {
	mu     sync.RWMutex
	events []audit.Event
}

func NewAuditStore() *AuditStore {
	return &AuditStore{events: make([]audit.Event, 0, 16)}
}

func (s *AuditStore) Add(_ context.Context, event audit.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	s.events = append(s.events, event)
	return nil
}

func (s *AuditStore) List() []audit.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]audit.Event, len(s.events))
	copy(items, s.events)
	return items
}
