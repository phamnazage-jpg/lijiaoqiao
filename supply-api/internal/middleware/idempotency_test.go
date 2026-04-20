package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/repository"
)

// MockIdempotencyRepository 模拟幂等仓储
type MockIdempotencyRepository struct {
	records map[string]*repository.IdempotencyRecord
	nextID  int64
}

func NewMockIdempotencyRepository() *MockIdempotencyRepository {
	return &MockIdempotencyRepository{
		records: make(map[string]*repository.IdempotencyRecord),
		nextID:  1,
	}
}

func (r *MockIdempotencyRepository) GetByKey(ctx context.Context, tenantID, operatorID int64, apiPath, idempotencyKey string) (*repository.IdempotencyRecord, error) {
	key := buildKey(tenantID, operatorID, apiPath, idempotencyKey)
	if record, ok := r.records[key]; ok {
		if time.Now().Before(record.ExpiresAt) {
			return record, nil
		}
	}
	return nil, nil
}

func (r *MockIdempotencyRepository) Create(ctx context.Context, record *repository.IdempotencyRecord) error {
	key := buildKey(record.TenantID, record.OperatorID, record.APIPath, record.IdempotencyKey)
	if record.ID == 0 {
		record.ID = r.nextID
		r.nextID++
	}
	r.records[key] = record
	return nil
}

func (r *MockIdempotencyRepository) UpdateSuccess(ctx context.Context, id int64, responseCode int, responseBody json.RawMessage) error {
	for _, record := range r.records {
		if record.ID == id {
			record.ResponseCode = responseCode
			record.ResponseBody = append(json.RawMessage(nil), responseBody...)
			record.Status = repository.IdempotencyStatusSucceeded
			record.UpdatedAt = time.Now()
			return nil
		}
	}
	return nil
}

func (r *MockIdempotencyRepository) UpdateFailed(ctx context.Context, id int64, responseCode int, responseBody json.RawMessage) error {
	for _, record := range r.records {
		if record.ID == id {
			record.ResponseCode = responseCode
			record.ResponseBody = append(json.RawMessage(nil), responseBody...)
			record.Status = repository.IdempotencyStatusFailed
			record.UpdatedAt = time.Now()
			return nil
		}
	}
	return nil
}

func (r *MockIdempotencyRepository) AcquireLock(ctx context.Context, tenantID, operatorID int64, apiPath, idempotencyKey, requestID, payloadHash string, ttl time.Duration) (*repository.IdempotencyRecord, error) {
	key := buildKey(tenantID, operatorID, apiPath, idempotencyKey)
	record := &repository.IdempotencyRecord{
		ID:             r.nextID,
		TenantID:       tenantID,
		OperatorID:     operatorID,
		APIPath:        apiPath,
		IdempotencyKey: idempotencyKey,
		RequestID:      requestID,
		PayloadHash:    payloadHash,
		Status:         repository.IdempotencyStatusProcessing,
		ExpiresAt:      time.Now().Add(ttl),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	r.nextID++
	r.records[key] = record
	return record, nil
}

func buildKey(tenantID, operatorID int64, apiPath, idempotencyKey string) string {
	return fmt.Sprintf("%d:%d:%s:%s", tenantID, operatorID, apiPath, idempotencyKey)
}

func TestComputePayloadHash(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		expected string
	}{
		{
			name:     "empty body",
			body:     []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple JSON",
			body:     []byte(`{"key":"value"}`),
			expected: computeExpectedHash(`{"key":"value"}`),
		},
		{
			name:     "JSON with spaces",
			body:     []byte(`{ "key": "value" }`),
			expected: computeExpectedHash(`{ "key": "value" }`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputePayloadHash(tt.body)
			if result != tt.expected {
				t.Errorf("ComputePayloadHash() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func computeExpectedHash(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

func TestExtractIdempotencyKey(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		expectError bool
		errorCode   string
	}{
		{
			name: "valid headers",
			headers: map[string]string{
				"X-Request-Id":    "req-123",
				"Idempotency-Key": "idem-key-12345678",
			},
			expectError: false,
		},
		{
			name: "missing X-Request-Id",
			headers: map[string]string{
				"Idempotency-Key": "idem-key-12345678",
			},
			expectError: true,
			errorCode:   "missing X-Request-Id header",
		},
		{
			name: "missing Idempotency-Key",
			headers: map[string]string{
				"X-Request-Id": "req-123",
			},
			expectError: true,
			errorCode:   "missing Idempotency-Key header",
		},
		{
			name: "Idempotency-Key too short",
			headers: map[string]string{
				"X-Request-Id":    "req-123",
				"Idempotency-Key": "short",
			},
			expectError: true,
			errorCode:   "Idempotency-Key length must be 16-128",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/supply/accounts", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result, err := ExtractIdempotencyKey(req, 1, 1)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
				}
				if err != nil && !strings.Contains(err.Error(), tt.errorCode) {
					t.Errorf("error = %v, want contains %v", err, tt.errorCode)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestIdempotentHandler(t *testing.T) {
	// 创建测试handler
	testHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, record *repository.IdempotencyRecord) error {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "created"})
		return nil
	}

	middleware := NewIdempotencyMiddleware(nil, IdempotencyConfig{
		Enabled: false, // 禁用幂等，只测试handler包装
	})

	handler := middleware.Wrap(testHandler)

	t.Run("handler executes successfully", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/supply/accounts", strings.NewReader(`{"key":"value"}`))
		req.Header.Set("X-Request-Id", "req-123")
		req.Header.Set("Idempotency-Key", "idem-key-12345678")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
		}
	})
}

func TestIdempotentHandler_EnabledWithoutRepositoryReturnsServiceUnavailable(t *testing.T) {
	testHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, record *repository.IdempotencyRecord) error {
		w.WriteHeader(http.StatusCreated)
		return nil
	}

	handler := NewIdempotencyMiddleware(nil, IdempotencyConfig{
		Enabled: true,
	}).Wrap(testHandler)

	req := httptest.NewRequest("POST", "/api/v1/supply/accounts", strings.NewReader(`{"key":"value"}`))
	req.Header.Set("X-Request-Id", "req-123")
	req.Header.Set("Idempotency-Key", "idem-key-12345678")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestIdempotencyMiddleware_ReplaysSucceededRequestWithSamePayload(t *testing.T) {
	repo := NewMockIdempotencyRepository()
	callCount := 0

	testHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, record *repository.IdempotencyRecord) error {
		callCount++
		w.WriteHeader(http.StatusCreated)
		return json.NewEncoder(w).Encode(map[string]any{
			"request_id": record.RequestID,
			"status":     "created",
		})
	}

	middleware := &IdempotencyMiddleware{
		idempotencyRepo: repo,
		config: IdempotencyConfig{
			Enabled: true,
			TTL:     24 * time.Hour,
		},
	}
	handler := middleware.Wrap(testHandler)

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts", strings.NewReader(`{"name":"acct-1"}`))
	firstReq = firstReq.WithContext(WithOperatorID(WithTenantID(firstReq.Context(), 1001), 2001))
	firstReq.Header.Set("X-Request-Id", "req-1")
	firstReq.Header.Set("Idempotency-Key", "idem-key-12345678")
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected first request to create resource, got=%d body=%s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts", strings.NewReader(`{"name":"acct-1"}`))
	secondReq = secondReq.WithContext(WithOperatorID(WithTenantID(secondReq.Context(), 1001), 2001))
	secondReq.Header.Set("X-Request-Id", "req-2")
	secondReq.Header.Set("Idempotency-Key", "idem-key-12345678")
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusCreated {
		t.Fatalf("expected replay request to return original success, got=%d body=%s", secondRec.Code, secondRec.Body.String())
	}
	if secondRec.Header().Get("X-Idempotent-Replay") != "true" {
		t.Fatalf("expected replay header to be set, got headers=%v", secondRec.Header())
	}
	if callCount != 1 {
		t.Fatalf("expected handler to run once, got=%d", callCount)
	}
}

func TestIdempotencyMiddleware_RejectsDifferentPayloadForSameKey(t *testing.T) {
	repo := NewMockIdempotencyRepository()

	testHandler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, record *repository.IdempotencyRecord) error {
		w.WriteHeader(http.StatusCreated)
		return json.NewEncoder(w).Encode(map[string]string{"status": "created"})
	}

	middleware := NewIdempotencyMiddleware(repo, IdempotencyConfig{
		Enabled: true,
		TTL:     24 * time.Hour,
	})
	handler := middleware.Wrap(testHandler)

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts", strings.NewReader(`{"name":"acct-1"}`))
	firstReq = firstReq.WithContext(WithOperatorID(WithTenantID(firstReq.Context(), 1001), 2001))
	firstReq.Header.Set("X-Request-Id", "req-a")
	firstReq.Header.Set("Idempotency-Key", "idem-key-87654321")
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, firstReq)
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first request success, got=%d body=%s", first.Code, first.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts", strings.NewReader(`{"name":"acct-2"}`))
	secondReq = secondReq.WithContext(WithOperatorID(WithTenantID(secondReq.Context(), 1001), 2001))
	secondReq.Header.Set("X-Request-Id", "req-b")
	secondReq.Header.Set("Idempotency-Key", "idem-key-87654321")
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, secondReq)
	if second.Code != http.StatusConflict {
		t.Fatalf("expected payload mismatch conflict, got=%d body=%s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "IDEMPOTENCY_PAYLOAD_MISMATCH") {
		t.Fatalf("expected payload mismatch code, got body=%s", second.Body.String())
	}
}
