package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lijiaoqiao/supply-api/internal/audit"
	"lijiaoqiao/supply-api/internal/domain"
	"lijiaoqiao/supply-api/internal/storage"
)

// Supply API 处理器
type SupplyAPI struct {
	accountService    domain.AccountService
	packageService    domain.PackageService
	settlementService domain.SettlementService
	earningService    domain.EarningService
	idempotencyStore  *storage.InMemoryIdempotencyStore
	auditStore        *audit.MemoryAuditStore
	supplierID        int64
	now               func() time.Time
}

func NewSupplyAPI(
	accountService domain.AccountService,
	packageService domain.PackageService,
	settlementService domain.SettlementService,
	earningService domain.EarningService,
	idempotencyStore *storage.InMemoryIdempotencyStore,
	auditStore *audit.MemoryAuditStore,
	supplierID int64,
	now func() time.Time,
) *SupplyAPI {
	return &SupplyAPI{
		accountService:    accountService,
		packageService:    packageService,
		settlementService: settlementService,
		earningService:    earningService,
		idempotencyStore:  idempotencyStore,
		auditStore:        auditStore,
		supplierID:        supplierID,
		now:               now,
	}
}

func (a *SupplyAPI) Register(mux *http.ServeMux) {
	// Supply Accounts
	mux.HandleFunc("/api/v1/supply/accounts/verify", a.handleVerifyAccount)
	mux.HandleFunc("/api/v1/supply/accounts", a.handleCreateAccount)
	mux.HandleFunc("/api/v1/supply/accounts/", a.handleAccountActions)

	// Supply Packages
	mux.HandleFunc("/api/v1/supply/packages/draft", a.handleCreatePackageDraft)
	mux.HandleFunc("/api/v1/supply/packages/batch-price", a.handleBatchUpdatePrice)
	mux.HandleFunc("/api/v1/supply/packages/", a.handlePackageActions)

	// Supply Billing
	mux.HandleFunc("/api/v1/supply/billing", a.handleGetBilling)
	mux.HandleFunc("/api/v1/supplier/billing", a.handleGetBilling) // 兼容别名

	// Supply Settlements
	mux.HandleFunc("/api/v1/supply/settlements/withdraw", a.handleWithdraw)
	mux.HandleFunc("/api/v1/supply/settlements/", a.handleSettlementActions)

	// Supply Earnings
	mux.HandleFunc("/api/v1/supply/earnings/records", a.handleGetEarningRecords)
}

// ==================== Account Handlers ====================

type VerifyAccountRequest struct {
	Provider          string  `json:"provider"`
	AccountType       string  `json:"account_type"`
	CredentialInput   string  `json:"credential_input"`
	MinQuotaThreshold float64 `json:"min_quota_threshold,omitempty"`
}

func (a *SupplyAPI) handleVerifyAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	defer r.Body.Close()

	var req VerifyAccountRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	result, err := a.accountService.Verify(r.Context(), a.supplierID,
		domain.Provider(req.Provider),
		domain.AccountType(req.AccountType),
		req.CredentialInput)

	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VERIFY_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data":       result,
	})
}

func (a *SupplyAPI) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	requestID := r.Header.Get("X-Request-Id")
	idempotencyKey := r.Header.Get("Idempotency-Key")

	// 幂等检查
	if idempotencyKey != "" {
		if record, found := a.idempotencyStore.Get(idempotencyKey); found {
			if record.Status == "succeeded" {
				writeJSON(w, http.StatusOK, map[string]any{
					"request_id":        requestID,
					"idempotent_replay": true,
					"data":              record.Response,
				})
				return
			}
		}
		a.idempotencyStore.SetProcessing(idempotencyKey, 24*time.Hour)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	defer r.Body.Close()

	// 解析请求
	var rawReq struct {
		Provider        string `json:"provider"`
		AccountType     string `json:"account_type"`
		CredentialInput string `json:"credential_input"`
		AccountAlias    string `json:"account_alias"`
		RiskAck         bool   `json:"risk_ack"`
	}

	if err := json.Unmarshal(body, &rawReq); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	createReq := &domain.CreateAccountRequest{
		SupplierID:  a.supplierID,
		Provider:    domain.Provider(rawReq.Provider),
		AccountType: domain.AccountType(rawReq.AccountType),
		Credential:  rawReq.CredentialInput,
		Alias:       rawReq.AccountAlias,
		RiskAck:     rawReq.RiskAck,
	}

	account, err := a.accountService.Create(r.Context(), createReq)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "CREATE_FAILED", err.Error())
		return
	}

	resp := map[string]any{
		"account_id":   account.ID,
		"provider":     account.Provider,
		"account_type": account.AccountType,
		"status":       account.Status,
		"created_at":   account.CreatedAt,
	}

	// 保存幂等结果
	if idempotencyKey != "" {
		a.idempotencyStore.SetSuccess(idempotencyKey, resp, 24*time.Hour)
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"request_id": requestID,
		"data":       resp,
	})
}

func (a *SupplyAPI) handleAccountActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/supply/accounts/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
		return
	}

	accountID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid account_id")
		return
	}

	action := parts[1]

	switch action {
	case "activate":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		a.handleActivateAccount(w, r, accountID)
	case "suspend":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		a.handleSuspendAccount(w, r, accountID)
	case "delete":
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		a.handleDeleteAccount(w, r, accountID)
	case "audit-logs":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		a.handleAccountAuditLogs(w, r, accountID)
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
	}
}

func (a *SupplyAPI) handleActivateAccount(w http.ResponseWriter, r *http.Request, accountID int64) {
	account, err := a.accountService.Activate(r.Context(), a.supplierID, accountID)
	if err != nil {
		if strings.Contains(err.Error(), "SUP_ACC") {
			writeError(w, http.StatusConflict, "CONFLICT", err.Error())
		} else {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data": map[string]any{
			"account_id": account.ID,
			"status":     account.Status,
			"updated_at": account.UpdatedAt,
		},
	})
}

func (a *SupplyAPI) handleSuspendAccount(w http.ResponseWriter, r *http.Request, accountID int64) {
	account, err := a.accountService.Suspend(r.Context(), a.supplierID, accountID)
	if err != nil {
		if strings.Contains(err.Error(), "SUP_ACC") {
			writeError(w, http.StatusConflict, "CONFLICT", err.Error())
		} else {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data": map[string]any{
			"account_id": account.ID,
			"status":     account.Status,
			"updated_at": account.UpdatedAt,
		},
	})
}

func (a *SupplyAPI) handleDeleteAccount(w http.ResponseWriter, r *http.Request, accountID int64) {
	err := a.accountService.Delete(r.Context(), a.supplierID, accountID)
	if err != nil {
		if strings.Contains(err.Error(), "SUP_ACC") {
			writeError(w, http.StatusConflict, "CONFLICT", err.Error())
		} else {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *SupplyAPI) handleAccountAuditLogs(w http.ResponseWriter, r *http.Request, accountID int64) {
	page := getQueryInt(r, "page", 1)
	pageSize := getQueryInt(r, "page_size", 20)

	events, err := a.auditStore.Query(r.Context(), audit.EventFilter{
		TenantID:   a.supplierID,
		ObjectType: "supply_account",
		ObjectID:   accountID,
		Limit:      pageSize,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	var items []map[string]any
	for _, ev := range events {
		items = append(items, map[string]any{
			"event_id":    ev.EventID,
			"operator_id": ev.TenantID,
			"tenant_id":   ev.TenantID,
			"object_type": ev.ObjectType,
			"object_id":   ev.ObjectID,
			"action":      ev.Action,
			"request_id":  ev.RequestID,
			"created_at":  ev.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data":       items,
		"pagination": map[string]int{
			"page":      page,
			"page_size": pageSize,
			"total":     len(items),
		},
	})
}

// ==================== Package Handlers ====================

func (a *SupplyAPI) handleCreatePackageDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	defer r.Body.Close()

	var req struct {
		SupplyAccountID  int64   `json:"supply_account_id"`
		Model            string  `json:"model"`
		TotalQuota       float64 `json:"total_quota"`
		PricePer1MInput  float64 `json:"price_per_1m_input"`
		PricePer1MOutput float64 `json:"price_per_1m_output"`
		ValidDays        int     `json:"valid_days"`
		MaxConcurrent    int     `json:"max_concurrent"`
		RateLimitRPM     int     `json:"rate_limit_rpm"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	createReq := &domain.CreatePackageDraftRequest{
		SupplierID:       a.supplierID,
		AccountID:        req.SupplyAccountID,
		Model:            req.Model,
		TotalQuota:       req.TotalQuota,
		PricePer1MInput:  req.PricePer1MInput,
		PricePer1MOutput: req.PricePer1MOutput,
		ValidDays:        req.ValidDays,
		MaxConcurrent:    req.MaxConcurrent,
		RateLimitRPM:     req.RateLimitRPM,
	}

	pkg, err := a.packageService.CreateDraft(r.Context(), a.supplierID, createReq)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "CREATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"request_id": getRequestID(r),
		"data": map[string]any{
			"package_id":        pkg.ID,
			"supply_account_id": pkg.SupplierID,
			"model":             pkg.Model,
			"status":            pkg.Status,
			"total_quota":       pkg.TotalQuota,
			"available_quota":   pkg.AvailableQuota,
			"created_at":        pkg.CreatedAt,
		},
	})
}

func (a *SupplyAPI) handlePackageActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/supply/packages/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
		return
	}

	// 批量调价
	if len(parts) == 1 && parts[0] == "batch-price" {
		a.handleBatchUpdatePrice(w, r)
		return
	}

	packageID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid package_id")
		return
	}

	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
		return
	}

	action := parts[1]

	switch action {
	case "publish":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		a.handlePublishPackage(w, r, packageID)
	case "pause":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		a.handlePausePackage(w, r, packageID)
	case "unlist":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		a.handleUnlistPackage(w, r, packageID)
	case "clone":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		a.handleClonePackage(w, r, packageID)
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
	}
}

func (a *SupplyAPI) handlePublishPackage(w http.ResponseWriter, r *http.Request, packageID int64) {
	pkg, err := a.packageService.Publish(r.Context(), a.supplierID, packageID)
	if err != nil {
		if strings.Contains(err.Error(), "SUP_PKG") {
			writeError(w, http.StatusConflict, "CONFLICT", err.Error())
		} else {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data": map[string]any{
			"package_id": pkg.ID,
			"status":     pkg.Status,
			"updated_at": pkg.UpdatedAt,
		},
	})
}

func (a *SupplyAPI) handlePausePackage(w http.ResponseWriter, r *http.Request, packageID int64) {
	pkg, err := a.packageService.Pause(r.Context(), a.supplierID, packageID)
	if err != nil {
		if strings.Contains(err.Error(), "SUP_PKG") {
			writeError(w, http.StatusConflict, "CONFLICT", err.Error())
		} else {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data": map[string]any{
			"package_id": pkg.ID,
			"status":     pkg.Status,
			"updated_at": pkg.UpdatedAt,
		},
	})
}

func (a *SupplyAPI) handleUnlistPackage(w http.ResponseWriter, r *http.Request, packageID int64) {
	pkg, err := a.packageService.Unlist(r.Context(), a.supplierID, packageID)
	if err != nil {
		if strings.Contains(err.Error(), "SUP_PKG") {
			writeError(w, http.StatusConflict, "CONFLICT", err.Error())
		} else {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data": map[string]any{
			"package_id": pkg.ID,
			"status":     pkg.Status,
			"updated_at": pkg.UpdatedAt,
		},
	})
}

func (a *SupplyAPI) handleClonePackage(w http.ResponseWriter, r *http.Request, packageID int64) {
	pkg, err := a.packageService.Clone(r.Context(), a.supplierID, packageID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"request_id": getRequestID(r),
		"data": map[string]any{
			"package_id":        pkg.ID,
			"supply_account_id": pkg.SupplierID,
			"model":             pkg.Model,
			"status":            pkg.Status,
			"created_at":        pkg.CreatedAt,
		},
	})
}

func (a *SupplyAPI) handleBatchUpdatePrice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	defer r.Body.Close()

	var rawReq struct {
		Items []struct {
			PackageID        int64   `json:"package_id"`
			PricePer1MInput  float64 `json:"price_per_1m_input"`
			PricePer1MOutput float64 `json:"price_per_1m_output"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &rawReq); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	req := &domain.BatchUpdatePriceRequest{
		Items: make([]domain.BatchPriceItem, len(rawReq.Items)),
	}
	for i, item := range rawReq.Items {
		req.Items[i] = domain.BatchPriceItem{
			PackageID:        item.PackageID,
			PricePer1MInput:  item.PricePer1MInput,
			PricePer1MOutput: item.PricePer1MOutput,
		}
	}

	resp, err := a.packageService.BatchUpdatePrice(r.Context(), a.supplierID, req)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "BATCH_UPDATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data":       resp,
	})
}

// ==================== Billing Handlers ====================

func (a *SupplyAPI) handleGetBilling(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	summary, err := a.earningService.GetBillingSummary(r.Context(), a.supplierID, startDate, endDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data":       summary,
	})
}

// ==================== Settlement Handlers ====================

func (a *SupplyAPI) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	requestID := r.Header.Get("X-Request-Id")
	idempotencyKey := r.Header.Get("Idempotency-Key")

	// 幂等检查
	if idempotencyKey != "" {
		if record, found := a.idempotencyStore.Get(idempotencyKey); found {
			if record.Status == "succeeded" {
				writeJSON(w, http.StatusOK, map[string]any{
					"request_id":        requestID,
					"idempotent_replay": true,
					"data":              record.Response,
				})
				return
			}
		}
		a.idempotencyStore.SetProcessing(idempotencyKey, 72*time.Hour) // 提现类72h
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	defer r.Body.Close()

	var req struct {
		WithdrawAmount float64 `json:"withdraw_amount"`
		PaymentMethod  string  `json:"payment_method"`
		PaymentAccount string  `json:"payment_account"`
		SMSCode        string  `json:"sms_code"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	withdrawReq := &domain.WithdrawRequest{
		Amount:         req.WithdrawAmount,
		PaymentMethod:  domain.PaymentMethod(req.PaymentMethod),
		PaymentAccount: req.PaymentAccount,
		SMSCode:        req.SMSCode,
	}

	settlement, err := a.settlementService.Withdraw(r.Context(), a.supplierID, withdrawReq)
	if err != nil {
		if strings.Contains(err.Error(), "SUP_SET") {
			writeError(w, http.StatusConflict, "WITHDRAW_FAILED", err.Error())
		} else {
			writeError(w, http.StatusUnprocessableEntity, "WITHDRAW_FAILED", err.Error())
		}
		return
	}

	resp := map[string]any{
		"settlement_id": settlement.ID,
		"settlement_no": settlement.SettlementNo,
		"status":        settlement.Status,
		"total_amount":  settlement.TotalAmount,
		"net_amount":    settlement.NetAmount,
		"created_at":    settlement.CreatedAt,
	}

	// 保存幂等结果
	if idempotencyKey != "" {
		a.idempotencyStore.SetSuccess(idempotencyKey, resp, 72*time.Hour)
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"request_id": requestID,
		"data":       resp,
	})
}

func (a *SupplyAPI) handleSettlementActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/supply/settlements/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
		return
	}

	settlementID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid settlement_id")
		return
	}

	action := parts[1]

	switch action {
	case "cancel":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		a.handleCancelSettlement(w, r, settlementID)
	case "statement":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		a.handleGetStatement(w, r, settlementID)
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
	}
}

func (a *SupplyAPI) handleCancelSettlement(w http.ResponseWriter, r *http.Request, settlementID int64) {
	settlement, err := a.settlementService.Cancel(r.Context(), a.supplierID, settlementID)
	if err != nil {
		if strings.Contains(err.Error(), "SUP_SET") {
			writeError(w, http.StatusConflict, "CONFLICT", err.Error())
		} else {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data": map[string]any{
			"settlement_id": settlement.ID,
			"status":        settlement.Status,
			"updated_at":    settlement.UpdatedAt,
		},
	})
}

func (a *SupplyAPI) handleGetStatement(w http.ResponseWriter, r *http.Request, settlementID int64) {
	settlement, err := a.settlementService.GetByID(r.Context(), a.supplierID, settlementID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data": map[string]any{
			"settlement_id": settlement.ID,
			"file_name":     fmt.Sprintf("statement_%s.pdf", settlement.SettlementNo),
			"download_url":  fmt.Sprintf("https://example.com/statements/%s.pdf", settlement.SettlementNo),
			"expires_at":    a.now().Add(1 * time.Hour),
		},
	})
}

// ==================== Earning Handlers ====================

func (a *SupplyAPI) handleGetEarningRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")
	page := getQueryInt(r, "page", 1)
	pageSize := getQueryInt(r, "page_size", 20)

	records, total, err := a.earningService.ListRecords(r.Context(), a.supplierID, startDate, endDate, page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}

	var items []map[string]any
	for _, record := range records {
		items = append(items, map[string]any{
			"record_id":     record.ID,
			"earnings_type": record.EarningsType,
			"amount":        record.Amount,
			"status":        record.Status,
			"earned_at":     record.EarnedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_id": getRequestID(r),
		"data":       items,
		"pagination": map[string]int{
			"page":      page,
			"page_size": pageSize,
			"total":     total,
		},
	})
}

// ==================== Helpers ====================

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"request_id": "",
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func getRequestID(r *http.Request) string {
	if id := r.Header.Get("X-Request-Id"); id != "" {
		return id
	}
	return r.Header.Get("X-Request-ID")
}

func getQueryInt(r *http.Request, key string, defaultVal int) int {
	if val := r.URL.Query().Get(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
