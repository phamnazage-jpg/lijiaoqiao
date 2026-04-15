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
	api, err := NewAlertAPI(service.NewAlertService(store))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if api == nil {
		t.Fatal("expected api")
	}
}

func TestNewAlertAPI_ReturnsErrorWhenServiceMissing(t *testing.T) {
	api, err := NewAlertAPI(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if api != nil {
		t.Fatal("expected nil api")
	}
}
