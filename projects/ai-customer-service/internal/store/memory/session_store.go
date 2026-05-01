package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/session"
)

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*session.Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*session.Session)}
}

func sessionKey(channel, openID string) string {
	return fmt.Sprintf("%s:%s", channel, openID)
}

func (s *SessionStore) GetOrCreate(_ context.Context, channel, openID string, now time.Time) (*session.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionKey(channel, openID)
	if existing, ok := s.sessions[key]; ok {
		return cloneSession(existing), nil
	}

	created := &session.Session{
		ID:            key,
		Channel:       channel,
		OpenID:        openID,
		Status:        session.StatusIdle,
		TurnCount:     0,
		LastMessageAt: now,
		Context:       []session.MessageContext{},
	}
	s.sessions[key] = created
	return cloneSession(created), nil
}

func (s *SessionStore) Save(_ context.Context, sess *session.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = cloneSession(sess)
	return nil
}

func (s *SessionStore) GetByID(_ context.Context, id string) (*session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sess, ok := s.sessions[id]; ok {
		return cloneSession(sess), nil
	}
	return nil, fmt.Errorf("session not found: %s", id)
}

func (s *SessionStore) List() []*session.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]*session.Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		items = append(items, cloneSession(sess))
	}
	return items
}

func cloneSession(src *session.Session) *session.Session {
	if src == nil {
		return nil
	}
	cp := *src
	cp.Context = append([]session.MessageContext(nil), src.Context...)
	return &cp
}
