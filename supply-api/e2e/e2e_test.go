//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"lijiaoqiao/supply-api/internal/adapter"
	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/httpapi"
	"lijiaoqiao/supply-api/internal/middleware"
	"lijiaoqiao/supply-api/internal/pkg/logging"
)

type e2eOptions struct {
	withdrawEnabled bool
}

type e2eSystem struct {
	handler     http.Handler
	accountSvc  *e2eAccountService
	auditStore  *audit.MemoryAuditStore
	secretKey   string
	tokenIssuer string
}

type e2eAccountService struct {
	verifyResult         *domain.VerifyResult
	lastVerifySupplierID int64
}

func (s *e2eAccountService) Verify(ctx context.Context, supplierID int64, provider domain.Provider, accountType domain.AccountType, credential string) (*domain.VerifyResult, error) {
	s.lastVerifySupplierID = supplierID
	return s.verifyResult, nil
}

func (s *e2eAccountService) Create(ctx context.Context, req *domain.CreateAccountRequest) (*domain.Account, error) {
	return &domain.Account{ID: 1, SupplierID: req.SupplierID, Provider: req.Provider, AccountType: req.AccountType, Status: domain.AccountStatusActive}, nil
}

func (s *e2eAccountService) Activate(ctx context.Context, supplierID, accountID int64) (*domain.Account, error) {
	return &domain.Account{ID: accountID, SupplierID: supplierID, Status: domain.AccountStatusActive}, nil
}

func (s *e2eAccountService) Suspend(ctx context.Context, supplierID, accountID int64) (*domain.Account, error) {
	return &domain.Account{ID: accountID, SupplierID: supplierID, Status: domain.AccountStatusSuspended}, nil
}

func (s *e2eAccountService) Delete(ctx context.Context, supplierID, accountID int64) error {
	return nil
}

func (s *e2eAccountService) GetByID(ctx context.Context, supplierID, accountID int64) (*domain.Account, error) {
	return &domain.Account{ID: accountID, SupplierID: supplierID, Status: domain.AccountStatusActive}, nil
}

type e2ePackageService struct{}

func (s *e2ePackageService) CreateDraft(ctx context.Context, supplierID int64, req *domain.CreatePackageDraftRequest) (*domain.Package, error) {
	return &domain.Package{ID: 1, SupplierID: supplierID, Model: req.Model, Status: domain.PackageStatusDraft}, nil
}

func (s *e2ePackageService) Publish(ctx context.Context, supplierID, packageID int64) (*domain.Package, error) {
	return &domain.Package{ID: packageID, SupplierID: supplierID, Status: domain.PackageStatusActive}, nil
}

func (s *e2ePackageService) Pause(ctx context.Context, supplierID, packageID int64) (*domain.Package, error) {
	return &domain.Package{ID: packageID, SupplierID: supplierID, Status: domain.PackageStatusPaused}, nil
}

func (s *e2ePackageService) Unlist(ctx context.Context, supplierID, packageID int64) (*domain.Package, error) {
	return &domain.Package{ID: packageID, SupplierID: supplierID, Status: domain.PackageStatusExpired}, nil
}

func (s *e2ePackageService) Clone(ctx context.Context, supplierID, packageID int64) (*domain.Package, error) {
	return &domain.Package{ID: packageID + 1, SupplierID: supplierID, Status: domain.PackageStatusDraft}, nil
}

func (s *e2ePackageService) BatchUpdatePrice(ctx context.Context, supplierID int64, req *domain.BatchUpdatePriceRequest) (*domain.BatchUpdatePriceResponse, error) {
	return &domain.BatchUpdatePriceResponse{Total: len(req.Items), SuccessCount: len(req.Items)}, nil
}

func (s *e2ePackageService) GetByID(ctx context.Context, supplierID, packageID int64) (*domain.Package, error) {
	return &domain.Package{ID: packageID, SupplierID: supplierID, Status: domain.PackageStatusActive}, nil
}

type e2eSettlementService struct{}

func (s *e2eSettlementService) Withdraw(ctx context.Context, supplierID int64, req *domain.WithdrawRequest) (*domain.Settlement, error) {
	now := time.Now().UTC()
	return &domain.Settlement{
		ID:           1,
		SupplierID:   supplierID,
		SettlementNo: "SET-001",
		Status:       domain.SettlementStatusPending,
		TotalAmount:  req.Amount,
		NetAmount:    req.Amount,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (s *e2eSettlementService) Cancel(ctx context.Context, supplierID, settlementID int64) (*domain.Settlement, error) {
	now := time.Now().UTC()
	return &domain.Settlement{ID: settlementID, SupplierID: supplierID, Status: domain.SettlementStatusFailed, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *e2eSettlementService) GetByID(ctx context.Context, supplierID, settlementID int64) (*domain.Settlement, error) {
	now := time.Now().UTC()
	return &domain.Settlement{ID: settlementID, SupplierID: supplierID, Status: domain.SettlementStatusPending, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *e2eSettlementService) List(ctx context.Context, supplierID int64) ([]*domain.Settlement, error) {
	now := time.Now().UTC()
	return []*domain.Settlement{{ID: 1, SupplierID: supplierID, Status: domain.SettlementStatusPending, CreatedAt: now, UpdatedAt: now}}, nil
}

func (s *e2eSettlementService) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
	return &domain.BillingSummary{
		Period:  domain.BillingPeriod{Start: startDate, End: endDate},
		Summary: domain.BillingTotal{TotalRevenue: 100, TotalOrders: 1, TotalUsage: 1000, TotalRequests: 10, AvgSuccessRate: 1, NetEarnings: 95},
	}, nil
}

type e2eEarningService struct{}

func (s *e2eEarningService) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*domain.EarningRecord, int, error) {
	return []*domain.EarningRecord{
		{ID: 1, SupplierID: supplierID, EarningsType: "usage", Amount: 100, Status: "available", EarnedAt: time.Now().UTC()},
	}, 1, nil
}

func (s *e2eEarningService) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
	return &domain.BillingSummary{
		Period:  domain.BillingPeriod{Start: startDate, End: endDate},
		Summary: domain.BillingTotal{TotalRevenue: 100, TotalOrders: 1, TotalUsage: 1000, TotalRequests: 10, AvgSuccessRate: 1, NetEarnings: 95},
	}, nil
}

type staticTokenBackend struct {
	statusByTokenID map[string]string
}

func (b *staticTokenBackend) CheckTokenStatus(ctx context.Context, tokenID string) (string, error) {
	if status, ok := b.statusByTokenID[tokenID]; ok {
		return status, nil
	}
	return "active", nil
}

func newE2ESystem(t *testing.T, opts e2eOptions) *e2eSystem {
	t.Helper()

	accountSvc := &e2eAccountService{
		verifyResult: &domain.VerifyResult{
			VerifyStatus:   "pass",
			AvailableQuota: 2048,
			RiskScore:      0,
			CheckItems: []domain.CheckItem{
				{Item: "credential_format", Result: "pass", Message: "ok"},
			},
		},
	}
	auditStore := audit.NewMemoryAuditStore()

	api := httpapi.NewSupplyAPI(
		accountSvc,
		&e2ePackageService{},
		&e2eSettlementService{},
		&e2eEarningService{},
		nil,
		auditStore,
		nil,
		0,
		"https://statements.example.com",
		func() time.Time { return time.Unix(1712800000, 0).UTC() },
	)
	api.SetWithdrawEnabled(opts.withdrawEnabled)

	mux := http.NewServeMux()
	healthHandler := httpapi.NewHealthHandlerWithDefaults(nil, nil)
	mux.HandleFunc("/actuator/health", healthHandler.ServeHealth)
	mux.HandleFunc("/actuator/health/live", healthHandler.ServeLiveness)
	mux.HandleFunc("/actuator/health/ready", healthHandler.ServeReadiness)
	api.Register(mux)

	authMiddleware := middleware.NewAuthMiddleware(
		middleware.AuthConfig{
			SecretKey: "e2e-secret-key-should-be-long",
			Algorithm: "HS256",
			Issuer:    "supply-api-e2e",
			Enabled:   true,
		},
		middleware.NewTokenCache(),
		&staticTokenBackend{statusByTokenID: map[string]string{}},
		adapter.NewAuditEmitterAdapter(auditStore),
	)

	logger := logging.NewLogger("supply-api-e2e", logging.LogLevelError)

	var handler http.Handler = mux
	handler = middleware.RequestID(handler)
	handler = middleware.Recovery(handler)
	handler = middleware.Logging(handler, logger)
	handler = middleware.TracingMiddleware(handler)
	handler = authMiddleware.TokenVerifyMiddleware(handler)
	handler = authMiddleware.BearerExtractMiddleware(handler)
	handler = authMiddleware.QueryKeyRejectMiddleware(handler)

	return &e2eSystem{
		handler:     handler,
		accountSvc:  accountSvc,
		auditStore:  auditStore,
		secretKey:   "e2e-secret-key-should-be-long",
		tokenIssuer: "supply-api-e2e",
	}
}

func (s *e2eSystem) tokenForTenant(t *testing.T, tokenID string, tenantID int64) string {
	t.Helper()

	claims := &middleware.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			Issuer:    s.tokenIssuer,
			Subject:   "subject-42",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		SubjectID: "42",
		Role:      "org_admin",
		Scope:     []string{"supply:write", "supply:read"},
		TenantID:  tenantID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.secretKey))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenString
}

func decodeJSONBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v, body=%s", err, recorder.Body.String())
	}
	return payload
}

func TestE2E_HealthProbe_IsPublicAndHealthy(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})

	req := httptest.NewRequest(http.MethodGet, "/actuator/health", nil)
	req.Header.Set("X-Request-Id", "health-req-001")
	recorder := httptest.NewRecorder()

	system.handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-Request-Id") != "health-req-001" {
		t.Fatalf("expected X-Request-Id response header to round-trip")
	}

	payload := decodeJSONBody(t, recorder)
	if payload["status"] != "healthy" {
		t.Fatalf("expected healthy status, got %v", payload["status"])
	}
}

func TestE2E_ProtectedRoute_RejectsMissingBearer(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/supply/billing?start_date=2026-04-01&end_date=2026-04-11", nil)
	recorder := httptest.NewRecorder()

	system.handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d, body=%s", recorder.Code, recorder.Body.String())
	}

	payload := decodeJSONBody(t, recorder)
	errBody, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error body, got %v", payload["error"])
	}
	if errBody["code"] != "AUTH_MISSING_BEARER" {
		t.Fatalf("expected AUTH_MISSING_BEARER, got %v", errBody["code"])
	}
}

func TestE2E_ProtectedRoute_RejectsQueryCredentialLeak(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/supply/billing?api_key=abcdefghijklmnopqrstuvwxyz123456", nil)
	recorder := httptest.NewRecorder()

	system.handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d, body=%s", recorder.Code, recorder.Body.String())
	}

	payload := decodeJSONBody(t, recorder)
	errBody, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error body, got %v", payload["error"])
	}
	if errBody["code"] != "QUERY_KEY_NOT_ALLOWED" {
		t.Fatalf("expected QUERY_KEY_NOT_ALLOWED, got %v", errBody["code"])
	}
}

func TestE2E_VerifyAccount_UsesTenantIDFromVerifiedToken(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	token := system.tokenForTenant(t, "tok-e2e-verify", 2001)

	body := `{"provider":"openai","account_type":"resource","credential_input":"sk-test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/supply/accounts/verify", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "verify-req-001")
	recorder := httptest.NewRecorder()

	system.handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if system.accountSvc.lastVerifySupplierID != 2001 {
		t.Fatalf("expected supplierID 2001 from token tenant, got %d", system.accountSvc.lastVerifySupplierID)
	}

	payload := decodeJSONBody(t, recorder)
	if payload["request_id"] != "verify-req-001" {
		t.Fatalf("expected request_id verify-req-001, got %v", payload["request_id"])
	}
}

func TestE2E_Withdraw_DisabledBeforeSMSIntegration(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	token := system.tokenForTenant(t, "tok-e2e-withdraw", 2002)

	body := `{"withdraw_amount":100,"payment_method":"bank","payment_account":"13800000000","sms_code":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/supply/settlements/withdraw", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	system.handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d, body=%s", recorder.Code, recorder.Body.String())
	}

	payload := decodeJSONBody(t, recorder)
	errBody, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error body, got %v", payload["error"])
	}
	// 验证功能禁用错误码（SUP_HTTP_5030 = FEATURE_DISABLED）
	if errBody["code"] != httpapi.CodeFeatureDisabled {
		t.Fatalf("expected %s, got %v", httpapi.CodeFeatureDisabled, errBody["code"])
	}
}

func TestE2E_AuditEvent_CanBeReadBackThroughAPI(t *testing.T) {
	system := newE2ESystem(t, e2eOptions{withdrawEnabled: false})
	token := system.tokenForTenant(t, "tok-e2e-audit", 3001)

	if err := system.auditStore.Emit(context.Background(), audit.Event{
		TenantID:   3001,
		ObjectType: "supply_account",
		ObjectID:   77,
		Action:     "verify",
		RequestID:  "audit-req-001",
		ResultCode: "OK",
		SourceIP:   "127.0.0.1",
	}); err != nil {
		t.Fatalf("failed to seed audit event: %v", err)
	}

	events, err := system.auditStore.Query(context.Background(), audit.EventFilter{TenantID: 3001, Limit: 1})
	if err != nil {
		t.Fatalf("failed to query seeded audit event: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/events/"+events[0].EventID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Request-Id", "audit-read-001")
	recorder := httptest.NewRecorder()

	system.handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}

	payload := decodeJSONBody(t, recorder)
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %v", payload["data"])
	}
	if data["event_id"] != events[0].EventID {
		t.Fatalf("expected event_id %s, got %v", events[0].EventID, data["event_id"])
	}
	if data["request_id"] != "audit-req-001" {
		t.Fatalf("expected seeded request_id audit-req-001, got %v", data["request_id"])
	}
}
