package service

import (
	"context"
	"sync"
)

type InMemoryRuntimeStore struct {
	mu               sync.RWMutex
	records          map[string]*TokenRecord
	tokenToID        map[string]string
	idempotencyByKey map[string]IdempotencyEntry
}

func NewInMemoryRuntimeStore() *InMemoryRuntimeStore {
	return &InMemoryRuntimeStore{
		records:         make(map[string]*TokenRecord),
		tokenToID:       make(map[string]string),
		idempotencyByKey: make(map[string]IdempotencyEntry),
	}
}

func (s *InMemoryRuntimeStore) Save(_ context.Context, record TokenRecord, idempotencyKey, requestHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	recordCopy := cloneRecord(record)
	s.records[record.TokenID] = &recordCopy
	if record.AccessToken != "" {
		s.tokenToID[record.AccessToken] = record.TokenID
	}
	if idempotencyKey != "" {
		s.idempotencyByKey[idempotencyKey] = IdempotencyEntry{
			RequestHash: requestHash,
			TokenID:     record.TokenID,
		}
	}
	return nil
}

func (s *InMemoryRuntimeStore) GetByTokenID(_ context.Context, tokenID string) (*TokenRecord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[tokenID]
	if !ok {
		return nil, false, nil
	}
	cloned := cloneRecord(*record)
	return &cloned, true, nil
}

func (s *InMemoryRuntimeStore) GetByAccessToken(_ context.Context, accessToken string) (*TokenRecord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tokenID, ok := s.tokenToID[accessToken]
	if !ok {
		return nil, false, nil
	}
	record, ok := s.records[tokenID]
	if !ok {
		return nil, false, nil
	}
	cloned := cloneRecord(*record)
	return &cloned, true, nil
}

func (s *InMemoryRuntimeStore) LookupIdempotency(_ context.Context, idempotencyKey string) (IdempotencyEntry, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.idempotencyByKey[idempotencyKey]
	return entry, ok, nil
}

func (s *InMemoryRuntimeStore) TokenCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}
