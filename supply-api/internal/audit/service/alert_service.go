package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"lijiaoqiao/supply-api/internal/audit/model"
)

// 错误定义
var (
	ErrAlertNotFound     = errors.New("alert not found")
	ErrInvalidAlertInput = errors.New("invalid alert input")
	ErrAlertConflict     = errors.New("alert conflict")
)

// AlertStoreInterface 告警存储接口
type AlertStoreInterface interface {
	Create(ctx context.Context, alert *model.Alert) error
	GetByID(ctx context.Context, alertID string) (*model.Alert, error)
	Update(ctx context.Context, alert *model.Alert) error
	Delete(ctx context.Context, alertID string) error
	List(ctx context.Context, filter *model.AlertFilter) ([]*model.Alert, int64, error)
}

// InMemoryAlertStore 内存告警存储
type InMemoryAlertStore struct {
	mu     sync.RWMutex
	alerts map[string]*model.Alert
}

// NewInMemoryAlertStore 创建内存告警存储
func NewInMemoryAlertStore() *InMemoryAlertStore {
	return &InMemoryAlertStore{
		alerts: make(map[string]*model.Alert),
	}
}

// Create 创建告警
func (s *InMemoryAlertStore) Create(ctx context.Context, alert *model.Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if alert.AlertID == "" {
		alert.AlertID = "ALT-" + uuid.New().String()[:8]
	}
	alert.CreatedAt = time.Now()
	alert.UpdatedAt = time.Now()

	s.alerts[alert.AlertID] = alert
	return nil
}

// GetByID 根据ID获取告警
func (s *InMemoryAlertStore) GetByID(ctx context.Context, alertID string) (*model.Alert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if alert, ok := s.alerts[alertID]; ok {
		return alert, nil
	}
	return nil, ErrAlertNotFound
}

// Update 更新告警
func (s *InMemoryAlertStore) Update(ctx context.Context, alert *model.Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.alerts[alert.AlertID]; !ok {
		return ErrAlertNotFound
	}

	alert.UpdatedAt = time.Now()
	s.alerts[alert.AlertID] = alert
	return nil
}

// Delete 删除告警
func (s *InMemoryAlertStore) Delete(ctx context.Context, alertID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.alerts[alertID]; !ok {
		return ErrAlertNotFound
	}

	delete(s.alerts, alertID)
	return nil
}

// List 查询告警列表
func (s *InMemoryAlertStore) List(ctx context.Context, filter *model.AlertFilter) ([]*model.Alert, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*model.Alert
	for _, alert := range s.alerts {
		// 按租户过滤
		if filter.TenantID > 0 && alert.TenantID != filter.TenantID {
			continue
		}
		// 按供应商过滤
		if filter.SupplierID > 0 && alert.SupplierID != filter.SupplierID {
			continue
		}
		// 按类型过滤
		if filter.AlertType != "" && alert.AlertType != filter.AlertType {
			continue
		}
		// 按级别过滤
		if filter.AlertLevel != "" && alert.AlertLevel != filter.AlertLevel {
			continue
		}
		// 按状态过滤
		if filter.Status != "" && alert.Status != filter.Status {
			continue
		}
		// 按时间范围过滤
		if !filter.StartTime.IsZero() && alert.CreatedAt.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && alert.CreatedAt.After(filter.EndTime) {
			continue
		}
		// 关键字搜索
		if filter.Keywords != "" {
			kw := filter.Keywords
			if !strings.Contains(alert.Title, kw) && !strings.Contains(alert.Message, kw) {
				continue
			}
		}

		result = append(result, alert)
	}

	total := int64(len(result))

	// 分页
	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return []*model.Alert{}, total, nil
		}
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, total, nil
}

// AlertService 告警服务
type AlertService struct {
	store AlertStoreInterface
}

// NewAlertService 创建告警服务
func NewAlertService(store AlertStoreInterface) *AlertService {
	return &AlertService{store: store}
}

// CreateAlert 创建告警
func (s *AlertService) CreateAlert(ctx context.Context, alert *model.Alert) (*model.Alert, error) {
	if alert == nil {
		return nil, ErrInvalidAlertInput
	}
	if alert.Title == "" {
		return nil, errors.New("alert title is required")
	}

	// 设置默认值
	if alert.AlertID == "" {
		alert.AlertID = model.NewAlert("", "", "", "", "", "").AlertID
	}
	if alert.Status == "" {
		alert.Status = model.AlertStatusActive
	}
	now := time.Now()
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = now
	}
	if alert.UpdatedAt.IsZero() {
		alert.UpdatedAt = now
	}
	if alert.FirstSeenAt.IsZero() {
		alert.FirstSeenAt = now
	}
	if alert.LastSeenAt.IsZero() {
		alert.LastSeenAt = now
	}

	err := s.store.Create(ctx, alert)
	if err != nil {
		return nil, err
	}
	return alert, nil
}

// GetAlert 获取告警
func (s *AlertService) GetAlert(ctx context.Context, alertID string) (*model.Alert, error) {
	if alertID == "" {
		return nil, ErrInvalidAlertInput
	}
	return s.store.GetByID(ctx, alertID)
}

// UpdateAlert 更新告警
func (s *AlertService) UpdateAlert(ctx context.Context, alert *model.Alert) (*model.Alert, error) {
	if alert == nil || alert.AlertID == "" {
		return nil, ErrInvalidAlertInput
	}

	alert.UpdatedAt = time.Now()
	err := s.store.Update(ctx, alert)
	if err != nil {
		return nil, err
	}
	return alert, nil
}

// DeleteAlert 删除告警
func (s *AlertService) DeleteAlert(ctx context.Context, alertID string) error {
	if alertID == "" {
		return ErrInvalidAlertInput
	}
	return s.store.Delete(ctx, alertID)
}

// ListAlerts 列出告警
func (s *AlertService) ListAlerts(ctx context.Context, filter *model.AlertFilter) ([]*model.Alert, int64, error) {
	if filter == nil {
		filter = &model.AlertFilter{}
	}
	if filter.Limit == 0 {
		filter.Limit = 100
	}
	return s.store.List(ctx, filter)
}

// ResolveAlert 解决告警
func (s *AlertService) ResolveAlert(ctx context.Context, alertID, resolvedBy, note string) (*model.Alert, error) {
	alert, err := s.store.GetByID(ctx, alertID)
	if err != nil {
		return nil, err
	}

	alert.Resolve(resolvedBy, note)
	err = s.store.Update(ctx, alert)
	if err != nil {
		return nil, err
	}
	return alert, nil
}

// AcknowledgeAlert 确认告警
func (s *AlertService) AcknowledgeAlert(ctx context.Context, alertID string) (*model.Alert, error) {
	alert, err := s.store.GetByID(ctx, alertID)
	if err != nil {
		return nil, err
	}

	alert.Acknowledge()
	err = s.store.Update(ctx, alert)
	if err != nil {
		return nil, err
	}
	return alert, nil
}
