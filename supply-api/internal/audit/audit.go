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
	OperatorID  int64          `json:"operator_id,omitempty"` // 操作者ID（来自IAMTokenClaims.SubjectID解析）
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

// SubjectIDContextKey context key for operator subject ID
type SubjectIDContextKey string

const subjectIDKey SubjectIDContextKey = "audit_subject_id"

// WithSubjectID 将操作者SubjectID注入context（由auth中间件调用）
func WithSubjectID(ctx context.Context, subjectID string) context.Context {
	if subjectID == "" {
		return ctx
	}
	return context.WithValue(ctx, subjectIDKey, subjectID)
}

// GetSubjectID 从context提取操作者SubjectID
func GetSubjectID(ctx context.Context) string {
	if v := ctx.Value(subjectIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// EnrichEventWithSubjectID 从ctx提取SubjectID并填充到Event（domain service调用）
func EnrichEventWithSubjectID(ctx context.Context, event *Event) {
	if event == nil {
		return
	}
	subjectID := GetSubjectID(ctx)
	if subjectID == "" {
		return
	}
	// subjectID 是字符串（JWT解析后），Event.OperatorID 是 int64
	// 如果有解析后的数值型ID，从context中取数值型版本
	if opID := getOperatorIDFromContext(ctx); opID > 0 {
		event.OperatorID = opID
	}
}

// OperatorIDContextKey context key for numeric operator ID
type OperatorIDContextKey string

const operatorIDKey OperatorIDContextKey = "audit_operator_id"

// WithOperatorID 将数值型操作者ID注入context
func WithOperatorID(ctx context.Context, operatorID int64) context.Context {
	if operatorID <= 0 {
		return ctx
	}
	return context.WithValue(ctx, operatorIDKey, operatorID)
}

// getOperatorIDFromContext 内部提取数值型操作者ID
func getOperatorIDFromContext(ctx context.Context) int64 {
	if v := ctx.Value(operatorIDKey); v != nil {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}
