package service

import (
	"context"
	"errors"

	"lijiaoqiao/supply-api/internal/audit/model"
	"lijiaoqiao/supply-api/internal/audit/repository"
)

// DatabaseAuditService 数据库-backed审计服务
type DatabaseAuditService struct {
	repo repository.AuditRepository
}

// NewDatabaseAuditService 创建数据库-backed审计服务
func NewDatabaseAuditService(repo repository.AuditRepository) *DatabaseAuditService {
	return &DatabaseAuditService{repo: repo}
}

// Ensure interface
var _ AuditStoreInterface = (*DatabaseAuditService)(nil)

// Emit 发送审计事件
func (s *DatabaseAuditService) Emit(ctx context.Context, event *model.AuditEvent) error {
	// 验证事件
	if event == nil {
		return ErrInvalidInput
	}
	if event.EventName == "" {
		return ErrMissingEventName
	}

	// 检查幂等键
	if event.IdempotencyKey != "" {
		existing, err := s.repo.GetByIdempotencyKey(ctx, event.IdempotencyKey)
		if err != nil {
			return err
		}
		if existing != nil {
			// 幂等键已存在，检查payload是否一致
			if isSamePayload(existing, event) {
				return repository.ErrDuplicateIdempotencyKey
			}
			return ErrIdempotencyConflict
		}
	}

	// 发送事件
	if err := s.repo.Emit(ctx, event); err != nil {
		if errors.Is(err, repository.ErrDuplicateIdempotencyKey) {
			return repository.ErrDuplicateIdempotencyKey
		}
		return err
	}

	return nil
}

// Query 查询审计事件
func (s *DatabaseAuditService) Query(ctx context.Context, filter *EventFilter) ([]*model.AuditEvent, int64, error) {
	if filter == nil {
		filter = &EventFilter{}
	}
	// 转换 filter 类型
	repoFilter := &repository.EventFilter{
		TenantID:  filter.TenantID,
		Category:  filter.Category,
		EventName: filter.EventName,
		Limit:     filter.Limit,
		Offset:    filter.Offset,
	}
	if !filter.StartTime.IsZero() {
		repoFilter.StartTime = &filter.StartTime
	}
	if !filter.EndTime.IsZero() {
		repoFilter.EndTime = &filter.EndTime
	}
	return s.repo.Query(ctx, repoFilter)
}

// GetByIdempotencyKey 根据幂等键获取事件
func (s *DatabaseAuditService) GetByIdempotencyKey(ctx context.Context, key string) (*model.AuditEvent, error) {
	return s.repo.GetByIdempotencyKey(ctx, key)
}

// NewDatabaseAuditServiceWithPool 从数据库连接池创建审计服务
func NewDatabaseAuditServiceWithPool(pool interface {
	Query(ctx context.Context, sql string, args ...interface{}) (interface{}, error)
	Exec(ctx context.Context, sql string, args ...interface{}) (interface{}, error)
}) *DatabaseAuditService {
	// 注意：这里需要一个适配器来将通用的pool接口转换为pgxpool.Pool
	// 在实际使用中，应该直接使用 NewDatabaseAuditService(repo)
	// 这个函数仅用于类型兼容性
	return nil
}
