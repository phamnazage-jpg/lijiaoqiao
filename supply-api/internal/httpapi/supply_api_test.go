package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/middleware"
)

// ==================== Mock Implementations ====================

// mockAccountService Mock账户服务
type mockAccountService struct {
	verifyResult *domain.VerifyResult
	verifyErr    error
	account      *domain.Account
	createErr    error
	activateErr  error
	suspendErr   error
	deleteErr    error
	lastVerifySupplierID int64
}

func (m *mockAccountService) Verify(ctx context.Context, supplierID int64, provider domain.Provider, accountType domain.AccountType, credential string) (*domain.VerifyResult, error) {
	m.lastVerifySupplierID = supplierID
	if m.verifyErr != nil {
		return nil, m.verifyErr
	}
	return m.verifyResult, nil
}

func (m *mockAccountService) Create(ctx context.Context, req *domain.CreateAccountRequest) (*domain.Account, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.account, nil
}

func (m *mockAccountService) Activate(ctx context.Context, supplierID, accountID int64) (*domain.Account, error) {
	if m.activateErr != nil {
		return nil, m.activateErr
	}
	return m.account, nil
}

func (m *mockAccountService) Suspend(ctx context.Context, supplierID, accountID int64) (*domain.Account, error) {
	if m.suspendErr != nil {
		return nil, m.suspendErr
	}
	return m.account, nil
}

func (m *mockAccountService) Delete(ctx context.Context, supplierID, accountID int64) error {
	return m.deleteErr
}

func (m *mockAccountService) GetByID(ctx context.Context, supplierID, accountID int64) (*domain.Account, error) {
	return m.account, nil
}

// mockPackageService Mock套餐服务
type mockPackageService struct {
	pkg           *domain.Package
	createDraftErr error
	publishErr     error
	pauseErr       error
	unlistErr      error
	cloneErr       error
	batchResp      *domain.BatchUpdatePriceResponse
	batchErr       error
}

func (m *mockPackageService) CreateDraft(ctx context.Context, supplierID int64, req *domain.CreatePackageDraftRequest) (*domain.Package, error) {
	if m.createDraftErr != nil {
		return nil, m.createDraftErr
	}
	return m.pkg, nil
}

func (m *mockPackageService) Publish(ctx context.Context, supplierID, packageID int64) (*domain.Package, error) {
	if m.publishErr != nil {
		return nil, m.publishErr
	}
	return m.pkg, nil
}

func (m *mockPackageService) Pause(ctx context.Context, supplierID, packageID int64) (*domain.Package, error) {
	if m.pauseErr != nil {
		return nil, m.pauseErr
	}
	return m.pkg, nil
}

func (m *mockPackageService) Unlist(ctx context.Context, supplierID, packageID int64) (*domain.Package, error) {
	if m.unlistErr != nil {
		return nil, m.unlistErr
	}
	return m.pkg, nil
}

func (m *mockPackageService) Clone(ctx context.Context, supplierID, packageID int64) (*domain.Package, error) {
	if m.cloneErr != nil {
		return nil, m.cloneErr
	}
	return m.pkg, nil
}

func (m *mockPackageService) BatchUpdatePrice(ctx context.Context, supplierID int64, req *domain.BatchUpdatePriceRequest) (*domain.BatchUpdatePriceResponse, error) {
	if m.batchErr != nil {
		return nil, m.batchErr
	}
	return m.batchResp, nil
}

func (m *mockPackageService) GetByID(ctx context.Context, supplierID, packageID int64) (*domain.Package, error) {
	return m.pkg, nil
}

// mockSettlementService Mock结算服务
type mockSettlementService struct {
	settlement *domain.Settlement
	withdrawErr error
	cancelErr    error
	getErr       error
}

func (m *mockSettlementService) Withdraw(ctx context.Context, supplierID int64, req *domain.WithdrawRequest) (*domain.Settlement, error) {
	if m.withdrawErr != nil {
		return nil, m.withdrawErr
	}
	return m.settlement, nil
}

func (m *mockSettlementService) Cancel(ctx context.Context, supplierID, settlementID int64) (*domain.Settlement, error) {
	if m.cancelErr != nil {
		return nil, m.cancelErr
	}
	return m.settlement, nil
}

func (m *mockSettlementService) GetByID(ctx context.Context, supplierID, settlementID int64) (*domain.Settlement, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.settlement, nil
}

func (m *mockSettlementService) List(ctx context.Context, supplierID int64) ([]*domain.Settlement, error) {
	if m.settlement != nil {
		return []*domain.Settlement{m.settlement}, nil
	}
	return nil, nil
}

func (m *mockSettlementService) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
	return nil, nil
}

// mockEarningService Mock收益服务
type mockEarningService struct {
	records       []*domain.EarningRecord
	total         int
	billingSummary *domain.BillingSummary
	listErr        error
	billingErr     error
}

func (m *mockEarningService) ListRecords(ctx context.Context, supplierID int64, startDate, endDate string, page, pageSize int) ([]*domain.EarningRecord, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.records, m.total, nil
}

func (m *mockEarningService) GetBillingSummary(ctx context.Context, supplierID int64, startDate, endDate string) (*domain.BillingSummary, error) {
	if m.billingErr != nil {
		return nil, m.billingErr
	}
	return m.billingSummary, nil
}

// mockAuditStore Mock审计存储
type mockAuditStore struct {
	events []audit.Event
	event  audit.Event
	err    error
}

func (m *mockAuditStore) Emit(ctx context.Context, event audit.Event) error {
	return m.err
}

func (m *mockAuditStore) Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.events, nil
}

func (m *mockAuditStore) QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	return m.events, int64(len(m.events)), nil
}

func (m *mockAuditStore) GetByID(ctx context.Context, eventID string) (audit.Event, error) {
	if m.err != nil {
		return audit.Event{}, m.err
	}
	return m.event, nil
}

// ==================== Test Helpers ====================

func newTestAPI() (*SupplyAPI, *mockAccountService, *mockPackageService, *mockSettlementService, *mockEarningService, *mockAuditStore) {
	accountSvc := &mockAccountService{
		account: &domain.Account{
			ID:          1,
			SupplierID:  100,
			Provider:    domain.ProviderOpenAI,
			AccountType: domain.AccountTypeAPIKey,
			Status:      domain.AccountStatusActive,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		verifyResult: &domain.VerifyResult{
			VerifyStatus:    "pass",
			AvailableQuota:  1000,
			RiskScore:       0,
		},
	}

	packageSvc := &mockPackageService{
		pkg: &domain.Package{
			ID:             1,
			SupplierID:     100,
			Model:          "gpt-4",
			Status:         domain.PackageStatusActive,
			TotalQuota:     10000,
			AvailableQuota: 8000,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	settlementSvc := &mockSettlementService{
		settlement: &domain.Settlement{
			ID:          1,
			SupplierID:  100,
			Status:      domain.SettlementStatusPending,
			TotalAmount: 1000,
			NetAmount:   950,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	earningSvc := &mockEarningService{
		records: []*domain.EarningRecord{
			{
				ID:     1,
				Amount: 100,
				Status: "available",
			},
		},
		total: 1,
		billingSummary: &domain.BillingSummary{},
	}

	auditSvc := &mockAuditStore{
		events: []audit.Event{
			{
				EventID:    "evt_123",
				TenantID:   100,
				ObjectType: "supply_account",
				ObjectID:   1,
				Action:     "create",
				CreatedAt:  time.Now(),
			},
		},
		event: audit.Event{
			EventID:    "evt_123",
			TenantID:   100,
			ObjectType: "supply_account",
			ObjectID:   1,
			Action:     "create",
			CreatedAt:  time.Now(),
		},
	}

	api := NewSupplyAPI(
		accountSvc,
		packageSvc,
		settlementSvc,
		earningSvc,
		nil, // idempotencyMw
		auditSvc,
		nil, // fkValidator
		100, // supplierID
		"https://statements.example.com",
		time.Now,
	)

	return api, accountSvc, packageSvc, settlementSvc, earningSvc, auditSvc
}

// ==================== Account Handler Tests ====================

func TestSupplyAPI_VerifyAccount_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	body := `{"provider":"openai","account_type":"resource","credential_input":"sk-test123"}`
	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "test-req-001")
	w := httptest.NewRecorder()

	api.handleVerifyAccount(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["request_id"] != "test-req-001" {
		t.Errorf("expected request_id test-req-001, got %v", resp["request_id"])
	}
}

func TestSupplyAPI_VerifyAccount_MethodNotAllowed(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/accounts/verify", nil)
	w := httptest.NewRecorder()

	api.handleVerifyAccount(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_VerifyAccount_InvalidJSON(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	body := `{invalid json}`
	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleVerifyAccount(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSupplyAPI_VerifyAccount_VerifyFailed(t *testing.T) {
	api, accountSvc, _, _, _, _ := newTestAPI()
	accountSvc.verifyErr = errors.New("SUP_ACC_4001: verification failed")

	body := `{"provider":"openai","account_type":"resource","credential_input":"invalid"}`
	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleVerifyAccount(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", w.Code)
	}
}

func TestSupplyAPI_VerifyAccount_UsesTenantIDFromContext(t *testing.T) {
	api, accountSvc, _, _, _, _ := newTestAPI()

	body := `{"provider":"openai","account_type":"resource","credential_input":"sk-test123"}`
	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/verify", strings.NewReader(body))
	req = req.WithContext(middleware.WithTenantID(req.Context(), 200))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleVerifyAccount(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if accountSvc.lastVerifySupplierID != 200 {
		t.Fatalf("expected tenant supplier ID 200, got %d", accountSvc.lastVerifySupplierID)
	}
}

func TestSupplyAPI_VerifyAccount_RejectsMissingTenantContextWithoutDefaultSupplier(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()
	api.supplierID = 0

	body := `{"provider":"openai","account_type":"resource","credential_input":"sk-test123"}`
	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleVerifyAccount(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestSupplyAPI_CreateAccount_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	body := `{"provider":"openai","account_type":"resource","credential_input":"sk-test","account_alias":"test","risk_ack":true}`
	req := httptest.NewRequest("POST", "/api/v1/supply/accounts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleCreateAccount(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestSupplyAPI_CreateAccount_MethodNotAllowed(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/accounts", nil)
	w := httptest.NewRecorder()

	api.handleCreateAccount(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_ActivateAccount_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/1/activate", nil)
	w := httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSupplyAPI_ActivateAccount_NotFound(t *testing.T) {
	api, accountSvc, _, _, _, _ := newTestAPI()
	accountSvc.activateErr = errors.New("account not found")

	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/1/activate", nil)
	w := httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestSupplyAPI_SuspendAccount_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/1/suspend", nil)
	w := httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSupplyAPI_SuspendAccount_Conflict(t *testing.T) {
	api, accountSvc, _, _, _, _ := newTestAPI()
	accountSvc.suspendErr = errors.New("SUP_ACC_4091: account state conflict")

	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/1/suspend", nil)
	w := httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

func TestSupplyAPI_SuspendAccount_WrongMethod(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/accounts/1/suspend", nil)
	w := httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_DeleteAccount_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("DELETE", "/api/v1/supply/accounts/1/delete", nil)
	w := httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestSupplyAPI_DeleteAccount_Conflict(t *testing.T) {
	api, accountSvc, _, _, _, _ := newTestAPI()
	accountSvc.deleteErr = errors.New("SUP_ACC_4092: cannot delete account with active packages")

	req := httptest.NewRequest("DELETE", "/api/v1/supply/accounts/1/delete", nil)
	w := httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

func TestSupplyAPI_DeleteAccount_WrongMethod(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/1/delete", nil)
	w := httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_AccountAuditLogs_Success(t *testing.T) {
	api, _, _, _, _, auditSvc := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/accounts/1/audit-logs", nil)
	w := httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatal("expected data array in response")
	}
	if len(data) != 1 {
		t.Errorf("expected 1 event, got %d", len(data))
	}

	auditSvc.err = errors.New("query failed")
	req = httptest.NewRequest("GET", "/api/v1/supply/accounts/1/audit-logs", nil)
	w = httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestSupplyAPI_AccountActions_InvalidID(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/invalid/activate", nil)
	w := httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSupplyAPI_AccountActions_UnknownRoute(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/accounts/1/unknown", nil)
	w := httptest.NewRecorder()

	api.handleAccountActions(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ==================== Package Handler Tests ====================

func TestSupplyAPI_CreatePackageDraft_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	body := `{"supply_account_id":1,"model":"gpt-4","total_quota":10000,"price_per_1m_input":0.1,"price_per_1m_output":0.2,"valid_days":30,"max_concurrent":10,"rate_limit_rpm":1000}`
	req := httptest.NewRequest("POST", "/api/v1/supply/packages/draft", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleCreatePackageDraft(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestSupplyAPI_CreatePackageDraft_MethodNotAllowed(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/packages/draft", nil)
	w := httptest.NewRecorder()

	api.handleCreatePackageDraft(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_PublishPackage_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/packages/1/publish", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSupplyAPI_PublishPackage_NotFound(t *testing.T) {
	api, _, packageSvc, _, _, _ := newTestAPI()
	packageSvc.publishErr = errors.New("package not found")

	req := httptest.NewRequest("POST", "/api/v1/supply/packages/1/publish", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestSupplyAPI_PausePackage_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/packages/1/pause", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSupplyAPI_PausePackage_Conflict(t *testing.T) {
	api, _, packageSvc, _, _, _ := newTestAPI()
	packageSvc.pauseErr = errors.New("SUP_PKG_4092: cannot pause active package")

	req := httptest.NewRequest("POST", "/api/v1/supply/packages/1/pause", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

func TestSupplyAPI_PausePackage_WrongMethod(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/packages/1/pause", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_UnlistPackage_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/packages/1/unlist", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSupplyAPI_UnlistPackage_Conflict(t *testing.T) {
	api, _, packageSvc, _, _, _ := newTestAPI()
	packageSvc.unlistErr = errors.New("SUP_PKG_4093: cannot unlist package")

	req := httptest.NewRequest("POST", "/api/v1/supply/packages/1/unlist", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

func TestSupplyAPI_UnlistPackage_WrongMethod(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/packages/1/unlist", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_ClonePackage_WrongMethod(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/packages/1/clone", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_ClonePackage_NotFound(t *testing.T) {
	api, _, packageSvc, _, _, _ := newTestAPI()
	packageSvc.cloneErr = errors.New("package not found")

	req := httptest.NewRequest("POST", "/api/v1/supply/packages/1/clone", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestSupplyAPI_PublishPackage_WrongMethod(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/packages/1/publish", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_ClonePackage_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/packages/1/clone", nil)
	w := httptest.NewRecorder()

	api.handlePackageActions(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestSupplyAPI_BatchUpdatePrice_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()
	api.packageService.(*mockPackageService).batchResp = &domain.BatchUpdatePriceResponse{
		Total:        2,
		SuccessCount: 2,
		FailedCount:  0,
	}

	body := `{"items":[{"package_id":1,"price_per_1m_input":0.15,"price_per_1m_output":0.25},{"package_id":2,"price_per_1m_input":0.12,"price_per_1m_output":0.22}]}`
	req := httptest.NewRequest("POST", "/api/v1/supply/packages/batch-price", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleBatchUpdatePrice(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSupplyAPI_BatchUpdatePrice_MethodNotAllowed(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/packages/batch-price", nil)
	w := httptest.NewRecorder()

	api.handleBatchUpdatePrice(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_BatchUpdatePrice_InvalidJSON(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	body := `{invalid}`
	req := httptest.NewRequest("POST", "/api/v1/supply/packages/batch-price", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleBatchUpdatePrice(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSupplyAPI_BatchUpdatePrice_BatchFailed(t *testing.T) {
	api, _, packageSvc, _, _, _ := newTestAPI()
	packageSvc.batchErr = errors.New("batch update failed")

	body := `{"items":[{"package_id":1,"price_per_1m_input":0.15,"price_per_1m_output":0.25}]}`
	req := httptest.NewRequest("POST", "/api/v1/supply/packages/batch-price", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleBatchUpdatePrice(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", w.Code)
	}
}

// ==================== Billing Handler Tests ====================

func TestSupplyAPI_GetBilling_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/billing?start_date=2024-01-01&end_date=2024-01-31", nil)
	w := httptest.NewRecorder()

	api.handleGetBilling(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSupplyAPI_GetBilling_MethodNotAllowed(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/billing", nil)
	w := httptest.NewRecorder()

	api.handleGetBilling(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_GetBilling_QueryFailed(t *testing.T) {
	api, _, _, _, earningSvc, _ := newTestAPI()
	earningSvc.billingErr = errors.New("query failed")

	req := httptest.NewRequest("GET", "/api/v1/supply/billing", nil)
	w := httptest.NewRecorder()

	api.handleGetBilling(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// ==================== Settlement Handler Tests ====================

func TestSupplyAPI_Withdraw_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	body := `{"withdraw_amount":1000,"payment_method":"bank","payment_account":"1234567890","sms_code":"123456"}`
	req := httptest.NewRequest("POST", "/api/v1/supply/settlements/withdraw", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleWithdraw(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestSupplyAPI_Withdraw_MethodNotAllowed(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/settlements/withdraw", nil)
	w := httptest.NewRecorder()

	api.handleWithdraw(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_Withdraw_InvalidJSON(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	body := `{invalid}`
	req := httptest.NewRequest("POST", "/api/v1/supply/settlements/withdraw", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleWithdraw(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSupplyAPI_Withdraw_WithdrawFailed(t *testing.T) {
	api, _, _, settlementSvc, _, _ := newTestAPI()
	settlementSvc.withdrawErr = errors.New("SUP_SET_4001: insufficient balance")

	body := `{"withdraw_amount":1000000,"payment_method":"bank","payment_account":"1234567890","sms_code":"123456"}`
	req := httptest.NewRequest("POST", "/api/v1/supply/settlements/withdraw", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleWithdraw(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

func TestSupplyAPI_CancelSettlement_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/settlements/1/cancel", nil)
	w := httptest.NewRecorder()

	api.handleSettlementActions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSupplyAPI_CancelSettlement_NotFound(t *testing.T) {
	api, _, _, settlementSvc, _, _ := newTestAPI()
	settlementSvc.cancelErr = errors.New("settlement not found")

	req := httptest.NewRequest("POST", "/api/v1/supply/settlements/1/cancel", nil)
	w := httptest.NewRecorder()

	api.handleSettlementActions(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestSupplyAPI_GetStatement_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/settlements/1/statement", nil)
	w := httptest.NewRecorder()

	api.handleSettlementActions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data in response")
	}

	if data["file_name"] == nil {
		t.Error("expected file_name in data")
	}
	if data["download_url"] == nil {
		t.Error("expected download_url in data")
	}
	if data["expires_at"] == nil {
		t.Error("expected expires_at in data")
	}
}

func TestSupplyAPI_GetStatement_NotFound(t *testing.T) {
	api, _, _, settlementSvc, _, _ := newTestAPI()
	settlementSvc.getErr = errors.New("settlement not found")

	req := httptest.NewRequest("GET", "/api/v1/supply/settlements/1/statement", nil)
	w := httptest.NewRecorder()

	api.handleSettlementActions(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestSupplyAPI_SettlementActions_InvalidID(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/settlements/invalid/cancel", nil)
	w := httptest.NewRecorder()

	api.handleSettlementActions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSupplyAPI_SettlementActions_UnknownAction(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/settlements/1/unknown", nil)
	w := httptest.NewRecorder()

	api.handleSettlementActions(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ==================== Earning Handler Tests ====================

func TestSupplyAPI_GetEarningRecords_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/supply/earnings/records?start_date=2024-01-01&end_date=2024-01-31&page=1&page_size=20", nil)
	w := httptest.NewRecorder()

	api.handleGetEarningRecords(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := resp["data"].([]any)
	if !ok {
		t.Fatal("expected data array in response")
	}
	if len(data) != 1 {
		t.Errorf("expected 1 record, got %d", len(data))
	}

	pagination, ok := resp["pagination"].(map[string]any)
	if !ok {
		t.Fatal("expected pagination in response")
	}
	if pagination["total"] != float64(1) {
		t.Errorf("expected total 1, got %v", pagination["total"])
	}
}

func TestSupplyAPI_GetEarningRecords_MethodNotAllowed(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/supply/earnings/records", nil)
	w := httptest.NewRecorder()

	api.handleGetEarningRecords(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSupplyAPI_GetEarningRecords_QueryFailed(t *testing.T) {
	api, _, _, _, earningSvc, _ := newTestAPI()
	earningSvc.listErr = errors.New("query failed")

	req := httptest.NewRequest("GET", "/api/v1/supply/earnings/records", nil)
	w := httptest.NewRecorder()

	api.handleGetEarningRecords(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// ==================== Audit Event Handler Tests ====================

func TestSupplyAPI_GetAuditEvent_Success(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/audit/events/evt_123", nil)
	w := httptest.NewRecorder()

	api.handleAuditEvent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data in response")
	}
	if data["event_id"] != "evt_123" {
		t.Errorf("expected event_id evt_123, got %v", data["event_id"])
	}
}

func TestSupplyAPI_GetAuditEvent_NotFound(t *testing.T) {
	api, _, _, _, _, auditSvc := newTestAPI()
	auditSvc.err = errors.New("not found")

	req := httptest.NewRequest("GET", "/api/v1/audit/events/evt_999", nil)
	w := httptest.NewRecorder()

	api.handleAuditEvent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestSupplyAPI_GetAuditEvent_MissingID(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("GET", "/api/v1/audit/events/", nil)
	w := httptest.NewRecorder()

	api.handleAuditEvent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSupplyAPI_GetAuditEvent_MethodNotAllowed(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()

	req := httptest.NewRequest("POST", "/api/v1/audit/events/evt_123", nil)
	w := httptest.NewRecorder()

	api.handleAuditEvent(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

// ==================== Helper Function Tests ====================

func TestGetRequestID(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-Id", "req-123")

	id := getRequestID(req)
	if id != "req-123" {
		t.Errorf("expected req-123, got %s", id)
	}

	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "req-456")

	id = getRequestID(req)
	if id != "req-456" {
		t.Errorf("expected req-456, got %s", id)
	}

	req = httptest.NewRequest("GET", "/", nil)
	id = getRequestID(req)
	if id != "" {
		t.Errorf("expected empty string, got %s", id)
	}
}

func TestGetQueryInt(t *testing.T) {
	req := httptest.NewRequest("GET", "/?page=5&page_size=100", nil)

	if getQueryInt(req, "page", 1) != 5 {
		t.Error("expected page 5")
	}
	if getQueryInt(req, "page_size", 20) != 100 {
		t.Error("expected page_size 100")
	}
	if getQueryInt(req, "missing", 10) != 10 {
		t.Error("expected default 10 for missing param")
	}
	if getQueryInt(req, "invalid", 1) != 1 {
		t.Error("expected default 1 for invalid value")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()

	writeJSON(w, http.StatusOK, map[string]any{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("expected Content-Type application/json")
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	writeError(w, http.StatusBadRequest, "TEST_ERROR", "test message")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "TEST_ERROR" {
		t.Errorf("expected code TEST_ERROR, got %v", errObj["code"])
	}
	if errObj["message"] != "test message" {
		t.Errorf("expected message 'test message', got %v", errObj["message"])
	}
}

// ==================== Integration Tests ====================

func TestSupplyAPI_Register(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()
	mux := http.NewServeMux()

	api.Register(mux)

	// 验证路由已注册（不会panic）
	_ = mux
}

func TestSupplyAPI_EndToEnd_Withdraw(t *testing.T) {
	api, _, _, settlementSvc, _, _ := newTestAPI()
	settlementSvc.settlement = &domain.Settlement{
		ID:          1,
		SupplierID:  100,
		SettlementNo: "SET_20240101_001",
		Status:      domain.SettlementStatusPending,
		TotalAmount: 1000,
		NetAmount:   950,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	body := `{"withdraw_amount":500,"payment_method":"bank","payment_account":"1234567890","sms_code":"123456"}`
	req := httptest.NewRequest("POST", "/api/v1/supply/settlements/withdraw", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "test-req-001")
	w := httptest.NewRecorder()

	api.handleWithdraw(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["request_id"] != "test-req-001" {
		t.Errorf("expected request_id test-req-001, got %v", resp["request_id"])
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data in response")
	}
	if data["settlement_id"] != float64(1) {
		t.Errorf("expected settlement_id 1, got %v", data["settlement_id"])
	}
	if data["status"] != "pending" {
		t.Errorf("expected status pending, got %v", data["status"])
	}
}

func TestSupplyAPI_WithdrawDisabled_ReturnsServiceUnavailable(t *testing.T) {
	api, _, _, _, _, _ := newTestAPI()
	api.withdrawEnabled = false

	body := `{"withdraw_amount":500,"payment_method":"bank","payment_account":"1234567890","sms_code":"123456"}`
	req := httptest.NewRequest("POST", "/api/v1/supply/settlements/withdraw", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.handleWithdraw(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestSupplyAPI_EndToEnd_BillingSummary(t *testing.T) {
	api, _, _, _, earningSvc, _ := newTestAPI()
	earningSvc.billingSummary = &domain.BillingSummary{
		Period: domain.BillingPeriod{
			Start: "2024-01-01",
			End:   "2024-01-31",
		},
		Summary: domain.BillingTotal{
			TotalRevenue:   10000,
			TotalOrders:    100,
			TotalUsage:     1000000,
			TotalRequests:  5000000,
			AvgSuccessRate: 99.5,
		},
	}

	req := httptest.NewRequest("GET", "/api/v1/supply/billing?start_date=2024-01-01&end_date=2024-01-31", nil)
	w := httptest.NewRecorder()

	api.handleGetBilling(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data in response")
	}

	summary, ok := data["summary"].(map[string]any)
	if !ok {
		t.Fatal("expected summary in data")
	}
	if summary["total_revenue"] != float64(10000) {
		t.Errorf("expected total_revenue 10000, got %v", summary["total_revenue"])
	}
}
