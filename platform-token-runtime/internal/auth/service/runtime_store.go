package service

import "sync"

type InMemoryRuntimeStore struct {
	mu              sync.RWMutex
	records         map[string]*TokenRecord
	tokenToID       map[string]string
	idempotencyByKey map[string]idempotencyEntry
}

func NewInMemoryRuntimeStore() *InMemoryRuntimeStore {
	return &InMemoryRuntimeStore{
		records:         make(map[string]*TokenRecord),
		tokenToID:       make(map[string]string),
		idempotencyByKey: make(map[string]idempotencyEntry),
	}
}

func (s *InMemoryRuntimeStore) Save(record TokenRecord, idempotencyKey, requestHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	recordCopy := cloneRecord(record)
	s.records[record.TokenID] = &recordCopy
	s.tokenToID[record.AccessToken] = record.TokenID
	if idempotencyKey != "" {
		s.idempotencyByKey[idempotencyKey] = idempotencyEntry{
			RequestHash: requestHash,
			TokenID:     record.TokenID,
		}
	}
}

func (s *InMemoryRuntimeStore) GetByTokenID(tokenID string) (*TokenRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[tokenID]
	return record, ok
}

func (s *InMemoryRuntimeStore) GetByAccessToken(accessToken string) (*TokenRecord, bool) {
	tokenID, ok := s.tokenToID[accessToken]
	if !ok {
		return nil, false
	}
	record, ok := s.records[tokenID]
	return record, ok
}

func (s *InMemoryRuntimeStore) LookupIdempotency(idempotencyKey string) (idempotencyEntry, bool) {
	entry, ok := s.idempotencyByKey[idempotencyKey]
	return entry, ok
}

func (s *InMemoryRuntimeStore) TokenCount() int {
	return len(s.records)
}
