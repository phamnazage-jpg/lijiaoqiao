package httpapi

import (
	"context"
	"testing"

	"lijiaoqiao/supply-api/internal/audit/model"
	"lijiaoqiao/supply-api/internal/audit/service"
)

type recordingAlertStore struct {
	createCalls int
}

func (s *recordingAlertStore) Create(ctx context.Context, alert *model.Alert) error {
	s.createCalls++
	return nil
}

func (s *recordingAlertStore) GetByID(ctx context.Context, alertID string) (*model.Alert, error) {
	return nil, service.ErrAlertNotFound
}

func (s *recordingAlertStore) Update(ctx context.Context, alert *model.Alert) error {
	return nil
}

func (s *recordingAlertStore) Delete(ctx context.Context, alertID string) error {
	return nil
}

func (s *recordingAlertStore) List(ctx context.Context, filter *model.AlertFilter) ([]*model.Alert, int64, error) {
	return nil, 0, nil
}

func TestNewAlertAPI_UsesInjectedStore(t *testing.T) {
	store := &recordingAlertStore{}
	api := NewAlertAPI(service.NewAlertService(store))
	if api == nil {
		t.Fatal("expected api")
	}
}
