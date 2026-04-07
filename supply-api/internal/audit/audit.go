package audit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// 审计事件
type Event struct {
	EventID     string         `json:"event_id,omitempty"`
	TenantID    int64          `json:"tenant_id"`
	ObjectType  string         `json:"object_type"`
	ObjectID    int64          `json:"object_id"`
	Action      string         `json:"action"`
	BeforeState map[string]any `json:"before_state,omitempty"`
	AfterState  map[string]any `json:"after_state,omitempty"`
	RequestID   string         `json:"request_id,omitempty"`
	ResultCode  string         `json:"result_code"`
	SourceIP    string         `json:"source_ip,omitempty"` // C-002修复: 统一使用SourceIP
	CreatedAt   time.Time      `json:"created_at"`
}

// 审计存储接口
type AuditStore interface {
	Emit(ctx context.Context, event Event) error
	Query(ctx context.Context, filter EventFilter) ([]Event, error)
	QueryWithTotal(ctx context.Context, filter EventFilter) ([]Event, int64, error)
	GetByID(ctx context.Context, eventID string) (Event, error)
}

// 事件过滤器
type EventFilter struct {
	TenantID   int64
	ObjectType string
	ObjectID   int64
	Action     string
	StartDate  string
	EndDate    string
	Limit      int
}

// 内存审计存储
type MemoryAuditStore struct {
	mu     sync.RWMutex
	events []Event
	nextID int64
}

func NewMemoryAuditStore() *MemoryAuditStore {
	return &MemoryAuditStore{
		events: make([]Event, 0),
		nextID: 1,
	}
}

func (s *MemoryAuditStore) Emit(ctx context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	event.EventID = generateEventID()
	event.CreatedAt = time.Now()
	s.events = append(s.events, event)
	return nil
}

func (s *MemoryAuditStore) Query(ctx context.Context, filter EventFilter) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Event
	for _, event := range s.events {
		if filter.TenantID > 0 && event.TenantID != filter.TenantID {
			continue
		}
		if filter.ObjectType != "" && event.ObjectType != filter.ObjectType {
			continue
		}
		if filter.ObjectID > 0 && event.ObjectID != filter.ObjectID {
			continue
		}
		if filter.Action != "" && event.Action != filter.Action {
			continue
		}
		result = append(result, event)
	}

	// 限制返回数量
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, nil
}

// QueryWithTotal 查询事件并返回总数
func (s *MemoryAuditStore) QueryWithTotal(ctx context.Context, filter EventFilter) ([]Event, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Event
	total := int64(0)

	for _, event := range s.events {
		total++
		if filter.TenantID > 0 && event.TenantID != filter.TenantID {
			continue
		}
		if filter.ObjectType != "" && event.ObjectType != filter.ObjectType {
			continue
		}
		if filter.ObjectID > 0 && event.ObjectID != filter.ObjectID {
			continue
		}
		if filter.Action != "" && event.Action != filter.Action {
			continue
		}
		result = append(result, event)
	}

	// 限制返回数量
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, total, nil
}

// GetByID 根据事件ID获取单个事件
func (s *MemoryAuditStore) GetByID(ctx context.Context, eventID string) (Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, event := range s.events {
		if event.EventID == eventID {
			return event, nil
		}
	}
	return Event{}, fmt.Errorf("event not found")
}

func generateEventID() string {
	return time.Now().Format("20060102150405") + "-evt"
}
