package audit

import (
	"context"
	"time"

	"lijiaoqiao/supply-api/internal/audit/model"
	"lijiaoqiao/supply-api/internal/audit/repository"
)

// PostgresAuditStore DB-backed审计存储
// 实现了audit.AuditStore接口，供给domain层使用
type PostgresAuditStore struct {
	repo *repository.PostgresAuditRepository
}

// NewPostgresAuditStore 创建DB-backed审计存储
func NewPostgresAuditStore(repo *repository.PostgresAuditRepository) *PostgresAuditStore {
	return &PostgresAuditStore{repo: repo}
}

// Ensure interface - compile time check
var _ AuditStore = (*PostgresAuditStore)(nil)

// Emit 发送审计事件
func (s *PostgresAuditStore) Emit(ctx context.Context, event Event) error {
	// 转换 audit.Event -> model.AuditEvent
	modelEvent := &model.AuditEvent{
		EventID:        event.EventID,
		EventName:      event.Action,
		EventCategory:  "",
		EventSubCategory: "",
		Timestamp:      event.CreatedAt,
		TimestampMs:    event.CreatedAt.UnixMilli(),
		RequestID:      event.RequestID,
		IdempotencyKey: "",
		TenantID:       event.TenantID,
		ObjectType:     event.ObjectType,
		ObjectID:       event.ObjectID,
		Action:         event.Action,
		ResultCode:     event.ResultCode,
		SourceIP:       event.SourceIP,
	}
	return s.repo.Emit(ctx, modelEvent)
}

// Query 查询审计事件
func (s *PostgresAuditStore) Query(ctx context.Context, filter EventFilter) ([]Event, error) {
	// 转换 EventFilter -> repository.EventFilter
	repoFilter := &repository.EventFilter{
		TenantID:   filter.TenantID,
		EventName:  filter.Action,
		Limit:      filter.Limit,
	}

	if filter.StartDate != "" {
		t, err := time.Parse("2006-01-02", filter.StartDate)
		if err == nil {
			repoFilter.StartTime = &t
		}
	}
	if filter.EndDate != "" {
		t, err := time.Parse("2006-01-02", filter.EndDate)
		if err == nil {
			// 设置为当天的结束时间
			t = t.Add(24*time.Hour - time.Second)
			repoFilter.EndTime = &t
		}
	}

	modelEvents, _, err := s.repo.Query(ctx, repoFilter)
	if err != nil {
		return nil, err
	}

	// 转换 []model.AuditEvent -> []Event
	events := make([]Event, 0, len(modelEvents))
	for _, me := range modelEvents {
		events = append(events, convertEventFromModel(me))
	}
	return events, nil
}

// QueryWithTotal 查询事件并返回总数
func (s *PostgresAuditStore) QueryWithTotal(ctx context.Context, filter EventFilter) ([]Event, int64, error) {
	repoFilter := &repository.EventFilter{
		TenantID:   filter.TenantID,
		EventName:  filter.Action,
		Limit:      filter.Limit,
	}

	if filter.StartDate != "" {
		t, err := time.Parse("2006-01-02", filter.StartDate)
		if err == nil {
			repoFilter.StartTime = &t
		}
	}
	if filter.EndDate != "" {
		t, err := time.Parse("2006-01-02", filter.EndDate)
		if err == nil {
			t = t.Add(24*time.Hour - time.Second)
			repoFilter.EndTime = &t
		}
	}

	modelEvents, total, err := s.repo.Query(ctx, repoFilter)
	if err != nil {
		return nil, 0, err
	}

	events := make([]Event, 0, len(modelEvents))
	for _, me := range modelEvents {
		events = append(events, convertEventFromModel(me))
	}
	return events, total, nil
}

// GetByID 根据事件ID获取单个事件
func (s *PostgresAuditStore) GetByID(ctx context.Context, eventID string) (Event, error) {
	modelEvent, err := s.repo.GetByEventID(ctx, eventID)
	if err != nil {
		return Event{}, err
	}
	return convertEventFromModel(modelEvent), nil
}

// convertEventFromModel 将 model.AuditEvent 转换为 audit.Event
func convertEventFromModel(me *model.AuditEvent) Event {
	return Event{
		EventID:     me.EventID,
		TenantID:    me.TenantID,
		ObjectType:  me.ObjectType,
		ObjectID:    me.ObjectID,
		Action:      me.Action,
		BeforeState: me.BeforeState,
		AfterState:  me.AfterState,
		RequestID:   me.RequestID,
		ResultCode:  me.ResultCode,
		SourceIP:    me.SourceIP,
		CreatedAt:   me.CreatedAt,
	}
}
