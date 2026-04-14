package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type MemoryAuditStore struct {
	mu     sync.RWMutex
	events []AuditEvent
	now    func() time.Time
}

type MemoryAuditEmitter = MemoryAuditStore

func NewMemoryAuditStore() *MemoryAuditStore {
	return &MemoryAuditStore{now: time.Now}
}

func NewMemoryAuditEmitter() *MemoryAuditEmitter {
	return NewMemoryAuditStore()
}

func (e *MemoryAuditStore) Emit(_ context.Context, event AuditEvent) error {
	if event.EventID == "" {
		eventID, err := generateEventID()
		if err != nil {
			return err
		}
		event.EventID = eventID
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = e.now()
	}
	e.mu.Lock()
	e.events = append(e.events, event)
	e.mu.Unlock()
	return nil
}

func (e *MemoryAuditStore) Events() []AuditEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	copied := make([]AuditEvent, len(e.events))
	copy(copied, e.events)
	return copied
}

func (e *MemoryAuditStore) QueryEvents(_ context.Context, filter AuditEventFilter) ([]AuditEvent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	result := make([]AuditEvent, 0, minInt(limit, len(e.events)))
	for idx := len(e.events) - 1; idx >= 0; idx-- {
		ev := e.events[idx]
		if !matchAuditFilter(ev, filter) {
			continue
		}
		result = append(result, ev)
		if len(result) >= limit {
			break
		}
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

func (e *MemoryAuditStore) LastEvent() (AuditEvent, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.events) == 0 {
		return AuditEvent{}, false
	}
	return e.events[len(e.events)-1], true
}

func emitAudit(emitter AuditEmitter, event AuditEvent, now func() time.Time) {
	if emitter == nil {
		return
	}
	if now == nil {
		now = time.Now
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now()
	}
	_ = emitter.Emit(context.Background(), event)
}

func matchAuditFilter(ev AuditEvent, filter AuditEventFilter) bool {
	if filter.RequestID != "" && ev.RequestID != filter.RequestID {
		return false
	}
	if filter.TokenID != "" && ev.TokenID != filter.TokenID {
		return false
	}
	if filter.SubjectID != "" && ev.SubjectID != filter.SubjectID {
		return false
	}
	if filter.EventName != "" && ev.EventName != filter.EventName {
		return false
	}
	if filter.ResultCode != "" && ev.ResultCode != filter.ResultCode {
		return false
	}
	return true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func generateEventID() (string, error) {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	return "evt_" + hex.EncodeToString(entropy[:]), nil
}
