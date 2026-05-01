package memory

import (
	"context"
	"fmt"
	"sync"
)

type DedupStore struct {
	mu    sync.Mutex
	items map[string]string
}

func NewDedupStore() *DedupStore {
	return &DedupStore{items: make(map[string]string)}
}

func (s *DedupStore) TryRecord(_ context.Context, channel, messageID, sessionID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s:%s", channel, messageID)
	if _, ok := s.items[key]; ok {
		return false, nil
	}
	s.items[key] = sessionID
	return true, nil
}
