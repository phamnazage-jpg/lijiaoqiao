package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"lijiaoqiao/supply-api/internal/audit/model"
)

// 错误定义
var (
	ErrInvalidInput      = errors.New("invalid input: event is nil")
	ErrMissingEventName  = errors.New("invalid input: event name is required")
	ErrEventNotFound     = errors.New("event not found")
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
)

// CreateEventResult 事件创建结果
type CreateEventResult struct {
	EventID           string    `json:"event_id"`
	StatusCode        int       `json:"status_code"`
	Status            string    `json:"status"`
	OriginalCreatedAt *time.Time `json:"original_created_at,omitempty"`
	ErrorCode         string    `json:"error_code,omitempty"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	RetryAfterMs      int64     `json:"retry_after_ms,omitempty"`
}

// EventFilter 事件查询过滤器
type EventFilter struct {
	TenantID    int64
	Category    string
	EventName   string
	ObjectType  string
	ObjectID    int64
	StartTime   time.Time
	EndTime     time.Time
	Success     *bool
	Limit       int
	Offset      int
}

// AuditStoreInterface 审计存储接口
type AuditStoreInterface interface {
	Emit(ctx context.Context, event *model.AuditEvent) error
	EmitBatch(ctx context.Context, events []*model.AuditEvent) error
	Query(ctx context.Context, filter *EventFilter) ([]*model.AuditEvent, int64, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*model.AuditEvent, error)
	GetByEventID(ctx context.Context, eventID string) (*model.AuditEvent, error)
}

// 内存存储容量常量
const MaxEvents = 100000

// InMemoryAuditStore 内存审计存储
type InMemoryAuditStore struct {
	mu              sync.RWMutex
	events          []*model.AuditEvent
	nextID          int64
	idempotencyKeys map[string]*model.AuditEvent
}

// NewInMemoryAuditStore 创建内存审计存储
func NewInMemoryAuditStore() *InMemoryAuditStore {
	return &InMemoryAuditStore{
		events:          make([]*model.AuditEvent, 0),
		nextID:          1,
		idempotencyKeys: make(map[string]*model.AuditEvent),
	}
}

// Emit 发送事件
func (s *InMemoryAuditStore) Emit(ctx context.Context, event *model.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查容量，超过上限时清理旧事件（直接调用带锁版本，因为Emit已持有锁）
	if len(s.events) >= MaxEvents {
		s.cleanupOldEventsLocked(MaxEvents / 10)
	}

	// 生成事件ID
	if event.EventID == "" {
		event.EventID = generateEventID()
	}
	event.CreatedAt = time.Now()

	s.events = append(s.events, event)

	// 如果有幂等键，记录映射
	if event.IdempotencyKey != "" {
		s.idempotencyKeys[event.IdempotencyKey] = event
	}

	return nil
}

// EmitBatch 批量发送事件
func (s *InMemoryAuditStore) EmitBatch(ctx context.Context, events []*model.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, event := range events {
		// 检查容量，超过上限时清理旧事件
		if len(s.events) >= MaxEvents {
			s.cleanupOldEventsLocked(MaxEvents / 10)
		}

		// 生成事件ID
		if event.EventID == "" {
			event.EventID = generateEventID()
		}
		event.CreatedAt = time.Now()

		s.events = append(s.events, event)

		// 如果有幂等键，记录映射
		if event.IdempotencyKey != "" {
			s.idempotencyKeys[event.IdempotencyKey] = event
		}
	}

	return nil
}

// cleanupOldEventsLocked 清理旧事件（ caller 必须持锁）
func (s *InMemoryAuditStore) cleanupOldEventsLocked(removeCount int) {
	if removeCount <= 0 {
		removeCount = MaxEvents / 10
	}
	if removeCount >= len(s.events) {
		removeCount = len(s.events) - 1
	}

	// 保留最近的事件，删除旧事件
	remaining := len(s.events) - removeCount
	s.events = s.events[remaining:]
}

// cleanupOldEvents 清理旧事件，保留最近的 events
func (s *InMemoryAuditStore) cleanupOldEvents(removeCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupOldEventsLocked(removeCount)
}

// Query 查询事件
func (s *InMemoryAuditStore) Query(ctx context.Context, filter *EventFilter) ([]*model.AuditEvent, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*model.AuditEvent
	for _, e := range s.events {
		// 按租户过滤
		if filter.TenantID > 0 && e.TenantID != filter.TenantID {
			continue
		}
		// 按类别过滤
		if filter.Category != "" && e.EventCategory != filter.Category {
			continue
		}
		// 按事件名称过滤
		if filter.EventName != "" && e.EventName != filter.EventName {
			continue
		}
		// 按对象类型过滤
		if filter.ObjectType != "" && e.ObjectType != filter.ObjectType {
			continue
		}
		// 按对象ID过滤
		if filter.ObjectID > 0 && e.ObjectID != filter.ObjectID {
			continue
		}
		// 按时间范围过滤
		if !filter.StartTime.IsZero() && e.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && e.Timestamp.After(filter.EndTime) {
			continue
		}
		// 按成功状态过滤
		if filter.Success != nil && e.Success != *filter.Success {
			continue
		}

		result = append(result, e)
	}

	total := int64(len(result))

	// 分页
	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return []*model.AuditEvent{}, total, nil
		}
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, total, nil
}

// GetByIdempotencyKey 根据幂等键获取事件
func (s *InMemoryAuditStore) GetByIdempotencyKey(ctx context.Context, key string) (*model.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if event, ok := s.idempotencyKeys[key]; ok {
		return event, nil
	}
	return nil, ErrEventNotFound
}

// GetByEventID 根据事件ID获取事件
func (s *InMemoryAuditStore) GetByEventID(ctx context.Context, eventID string) (*model.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, event := range s.events {
		if event.EventID == eventID {
			return event, nil
		}
	}
	return nil, ErrEventNotFound
}

// generateEventID 生成事件ID（使用UUID避免冲突）
func generateEventID() string {
	return uuid.New().String()
}

// AuditService 审计服务
type AuditService struct {
	store            AuditStoreInterface
	idempotencyMu    sync.Mutex  // 保护幂等性检查的互斥锁
	processingDelay  time.Duration
}

// NewAuditService 创建审计服务
func NewAuditService(store AuditStoreInterface) *AuditService {
	return &AuditService{
		store: store,
	}
}

// SetProcessingDelay 设置处理延迟（用于模拟异步处理）
func (s *AuditService) SetProcessingDelay(delay time.Duration) {
	s.processingDelay = delay
}

// CreateEvent 创建审计事件
func (s *AuditService) CreateEvent(ctx context.Context, event *model.AuditEvent) (*CreateEventResult, error) {
	// 输入验证
	if event == nil {
		return nil, ErrInvalidInput
	}
	if event.EventName == "" {
		return nil, ErrMissingEventName
	}

	// 设置时间戳
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.TimestampMs == 0 {
		event.TimestampMs = event.Timestamp.UnixMilli()
	}

	// 如果没有事件ID，生成一个
	if event.EventID == "" {
		event.EventID = generateEventID()
	}

	// 处理幂等性 - 整个检查和插入都在锁保护下，防止竞态条件
	if event.IdempotencyKey != "" {
		s.idempotencyMu.Lock()
		defer s.idempotencyMu.Unlock()

		existing, err := s.store.GetByIdempotencyKey(ctx, event.IdempotencyKey)
		if err == nil && existing != nil {
			// 检查payload是否相同
			if isSamePayload(existing, event) {
				// 重放同参 - 返回200
				return &CreateEventResult{
					EventID:           existing.EventID,
					StatusCode:        200,
					Status:            "duplicate",
					OriginalCreatedAt: &existing.CreatedAt,
				}, nil
			} else {
				// 重放异参 - 返回409
				return &CreateEventResult{
					StatusCode:    409,
					Status:        "conflict",
					ErrorCode:     "IDEMPOTENCY_PAYLOAD_MISMATCH",
					ErrorMessage: "Idempotency key reused with different payload",
				}, nil
			}
		}

		// 首次创建 - 在锁保护下插入
		err = s.store.Emit(ctx, event)
		if err != nil {
			return nil, err
		}

		return &CreateEventResult{
			EventID:    event.EventID,
			StatusCode: 201,
			Status:     "created",
		}, nil
	}

	// 无幂等键的直接插入
	err := s.store.Emit(ctx, event)
	if err != nil {
		return nil, err
	}

	return &CreateEventResult{
		EventID:    event.EventID,
		StatusCode: 201,
		Status:     "created",
	}, nil
}

// CreateEventsBatch 批量创建审计事件
func (s *AuditService) CreateEventsBatch(ctx context.Context, events []*model.AuditEvent) (*CreateEventsBatchResult, error) {
	if len(events) == 0 {
		return &CreateEventsBatchResult{
			SuccessCount: 0,
			FailCount:    0,
		}, nil
	}

	result := &CreateEventsBatchResult{
		SuccessCount: 0,
		FailCount:    0,
		Errors:      make([]string, 0),
	}

	// 设置默认时间戳
	now := time.Now()
	for _, event := range events {
		if event.Timestamp.IsZero() {
			event.Timestamp = now
		}
		if event.TimestampMs == 0 {
			event.TimestampMs = event.Timestamp.UnixMilli()
		}
		if event.EventID == "" {
			event.EventID = generateEventID()
		}
	}

	// 批量发送到存储
	err := s.store.EmitBatch(ctx, events)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		result.FailCount = len(events)
		return result, err
	}

	result.SuccessCount = len(events)
	return result, nil
}

// CreateEventsBatchResult 批量创建结果
type CreateEventsBatchResult struct {
	SuccessCount int      `json:"success_count"`
	FailCount    int      `json:"fail_count"`
	Errors       []string `json:"errors,omitempty"`
}

// ListEvents 列出事件（带分页）
func (s *AuditService) ListEvents(ctx context.Context, tenantID int64, offset, limit int) ([]*model.AuditEvent, int64, error) {
	filter := &EventFilter{
		TenantID: tenantID,
		Offset:   offset,
		Limit:    limit,
	}
	return s.store.Query(ctx, filter)
}

// ListEventsWithFilter 列出事件（带过滤器）
func (s *AuditService) ListEventsWithFilter(ctx context.Context, filter *EventFilter) ([]*model.AuditEvent, int64, error) {
	return s.store.Query(ctx, filter)
}

// GetEventByID 根据事件ID获取单个事件
func (s *AuditService) GetEventByID(ctx context.Context, eventID string) (*model.AuditEvent, error) {
	if eventID == "" {
		return nil, ErrInvalidInput
	}
	return s.store.GetByEventID(ctx, eventID)
}

// HashIdempotencyKey 计算幂等键的哈希值
func (s *AuditService) HashIdempotencyKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// isSamePayload 检查两个事件的payload是否相同
func isSamePayload(a, b *model.AuditEvent) bool {
	// 比较关键字段
	if a.EventName != b.EventName {
		return false
	}
	if a.EventCategory != b.EventCategory {
		return false
	}
	if a.OperatorID != b.OperatorID {
		return false
	}
	if a.TenantID != b.TenantID {
		return false
	}
	if a.ObjectType != b.ObjectType {
		return false
	}
	if a.ObjectID != b.ObjectID {
		return false
	}
	if a.Action != b.Action {
		return false
	}
	if a.ActionDetail != b.ActionDetail {
		return false
	}
	if a.CredentialType != b.CredentialType {
		return false
	}
	if a.SourceType != b.SourceType {
		return false
	}
	if a.SourceIP != b.SourceIP {
		return false
	}
	if a.Success != b.Success {
		return false
	}
	if a.ResultCode != b.ResultCode {
		return false
	}
	if a.ResultMessage != b.ResultMessage {
		return false
	}
	// 比较Extensions
	if !compareExtensions(a.Extensions, b.Extensions) {
		return false
	}
	return true
}

// compareExtensions 比较两个map是否相等
func compareExtensions(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v1 := range a {
		v2, ok := b[k]
		if !ok {
			return false
		}
		// 简单的值比较，不处理嵌套map的情况
		if v1 != v2 {
			return false
		}
	}
	return true
}