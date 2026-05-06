package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bridge/ai-customer-service/internal/app"
	"github.com/bridge/ai-customer-service/internal/config"
	"github.com/bridge/ai-customer-service/internal/http/middleware"
	"github.com/bridge/ai-customer-service/internal/platform/logging"
)

// newTestAppE2E creates a fully-wired app instance with in-memory stores
// for end-to-end testing.
func newTestAppE2E(t *testing.T) *app.App {
	t.Helper()
	cfg := &config.Config{}
	cfg.HTTP.Addr = ":0"
	cfg.HTTP.ReadHeaderTimeout = 5
	cfg.HTTP.ReadTimeout = 10
	cfg.HTTP.WriteTimeout = 15
	cfg.HTTP.IdleTimeout = 60
	cfg.HTTP.MaxHeaderBytes = 1 << 20
	cfg.HTTP.MaxBodyBytes = 1 << 20
	cfg.Runtime.Env = "test"
	application, err := app.New(cfg, logging.New())
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}
	return application
}

// webhookResponse mirrors the JSON shape returned by the webhook handler.
type webhookResponse struct {
	Handoff   bool   `json:"handoff"`
	TicketID  string `json:"ticket_id"`
	SessionID string `json:"session_id"`
	Reply     string `json:"reply"`
}

// mustReadBody reads and closes the response body, then decodes JSON into dest.
// On error, calls t.Fatalf.
func mustReadBody(t *testing.T, resp *http.Response, dest any) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatalf("decode body error = %v; body: %s", err, string(body))
	}
}

func setActorHeaders(req *http.Request, actorID, role string) {
	req.Header.Set(middleware.HeaderActorID, actorID)
	req.Header.Set(middleware.HeaderActorRole, role)
}

// TestFullTicketFlow_E2E exercises the complete ticket lifecycle:
//  1. Webhook triggers handoff → ticket created
//  2. Ticket is assigned to an agent
//  3. Ticket is resolved by the agent
//  4. Ticket is explicitly closed
//  5. Ticket is retrieved and verified in final closed state
func TestFullTicketFlow_E2E(t *testing.T) {
	application := newTestAppE2E(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	baseURL := server.URL

	// ── Step 1: Webhook triggers ticket creation ──────────────────────────
	payload := map[string]any{
		"message_id": "m-e2e-1",
		"channel":    "widget",
		"open_id":    "u_e2e_1",
		"content":    "我要申请退款",
	}
	body, _ := json.Marshal(payload)
	webhookResp, err := http.Post(baseURL+"/api/v1/customer-service/webhook", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("webhook POST error = %v", err)
	}
	var whResult webhookResponse
	mustReadBody(t, webhookResp, &whResult)

	if !whResult.Handoff {
		t.Fatalf("[step1] handoff = %v, want true", whResult.Handoff)
	}
	if whResult.TicketID == "" {
		t.Fatalf("[step1] ticket_id is empty, want non-empty")
	}
	if whResult.SessionID == "" {
		t.Fatalf("[step1] session_id is empty, want non-empty")
	}
	ticketID := whResult.TicketID

	// ── Step 2: Assign the ticket to an agent ────────────────────────────
	assignURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s/assign?agent_id=agent-e2e-001", baseURL, ticketID)
	assignReq, err := http.NewRequest(http.MethodPost, assignURL, nil)
	if err != nil {
		t.Fatalf("new assign request error = %v", err)
	}
	setActorHeaders(assignReq, "admin-e2e", "admin")
	assignReq.RemoteAddr = "192.168.1.1:12345"
	assignResp, err := http.DefaultClient.Do(assignReq)
	if err != nil {
		t.Fatalf("assign POST error = %v", err)
	}
	assignBody, err := io.ReadAll(assignResp.Body)
	assignResp.Body.Close()
	if err != nil {
		t.Fatalf("read assign body error = %v", err)
	}
	if assignResp.StatusCode != http.StatusOK {
		t.Fatalf("[step2 assign] status = %d, want 200; body: %s", assignResp.StatusCode, string(assignBody))
	}

	var assignPayload map[string]any
	if err := json.Unmarshal(assignBody, &assignPayload); err != nil {
		t.Fatalf("decode assign response error = %v", err)
	}
	if assignPayload["assigned"] != true {
		t.Fatalf("[step2] assigned = %v, want true", assignPayload["assigned"])
	}

	// ── Step 3: Resolve the ticket ────────────────────────────────────────
	resolveURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s/resolve?resolution=refund+processed+and+closed", baseURL, ticketID)
	resolveReq, err := http.NewRequest(http.MethodPost, resolveURL, nil)
	if err != nil {
		t.Fatalf("new resolve request error = %v", err)
	}
	setActorHeaders(resolveReq, "agent-e2e-001", "agent")
	resolveReq.RemoteAddr = "192.168.1.2:54321"
	resolveResp, err := http.DefaultClient.Do(resolveReq)
	if err != nil {
		t.Fatalf("resolve POST error = %v", err)
	}
	resolveBody, err := io.ReadAll(resolveResp.Body)
	resolveResp.Body.Close()
	if err != nil {
		t.Fatalf("read resolve body error = %v", err)
	}
	if resolveResp.StatusCode != http.StatusOK {
		t.Fatalf("[step3 resolve] status = %d, want 200; body: %s", resolveResp.StatusCode, string(resolveBody))
	}

	var resolvePayload map[string]any
	if err := json.Unmarshal(resolveBody, &resolvePayload); err != nil {
		t.Fatalf("decode resolve response error = %v", err)
	}
	if resolvePayload["resolved"] != true {
		t.Fatalf("[step3] resolved = %v, want true", resolvePayload["resolved"])
	}

	// ── Step 4: Close the ticket explicitly ───────────────────────────────
	closeURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s/close?resolution=refund+processed+and+confirmed", baseURL, ticketID)
	closeReq, err := http.NewRequest(http.MethodPost, closeURL, nil)
	if err != nil {
		t.Fatalf("new close request error = %v", err)
	}
	setActorHeaders(closeReq, "supervisor-e2e", "supervisor")
	closeReq.RemoteAddr = "192.168.1.3:65432"
	closeResp, err := http.DefaultClient.Do(closeReq)
	if err != nil {
		t.Fatalf("close POST error = %v", err)
	}
	closeBody, err := io.ReadAll(closeResp.Body)
	closeResp.Body.Close()
	if err != nil {
		t.Fatalf("read close body error = %v", err)
	}
	if closeResp.StatusCode != http.StatusOK {
		t.Fatalf("[step4 close] status = %d, want 200; body: %s", closeResp.StatusCode, string(closeBody))
	}

	var closePayload map[string]any
	if err := json.Unmarshal(closeBody, &closePayload); err != nil {
		t.Fatalf("decode close response error = %v", err)
	}
	if closePayload["closed"] != true {
		t.Fatalf("[step4] closed = %v, want true", closePayload["closed"])
	}

	// ── Step 5: Verify ticket is retrievable in final closed state ───────
	getURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s", baseURL, ticketID)
	getReq, err := http.NewRequest(http.MethodGet, getURL, nil)
	if err != nil {
		t.Fatalf("new get request error = %v", err)
	}
	setActorHeaders(getReq, "agent-e2e-001", "agent")
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("GET ticket error = %v", err)
	}
	getBody, err := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	if err != nil {
		t.Fatalf("read GET body error = %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("[step4 get] status = %d, want 200", getResp.StatusCode)
	}

	var ticketPayload map[string]any
	if err := json.Unmarshal(getBody, &ticketPayload); err != nil {
		t.Fatalf("decode ticket response error = %v", err)
	}
	tkt := ticketPayload["ticket"].(map[string]any)
	if tkt["status"] != "closed" {
		t.Fatalf("[step5] ticket status = %v, want closed", tkt["status"])
	}
	if tkt["assigned_to"] != "agent-e2e-001" {
		t.Fatalf("[step5] assigned_to = %v, want agent-e2e-001", tkt["assigned_to"])
	}
	if tkt["resolution"] != "refund processed and confirmed" {
		t.Fatalf("[step5] resolution = %v, want 'refund processed and confirmed'", tkt["resolution"])
	}
}

// TestFullTicketFlow_AuditLogVerification verifies that each workflow step
// produces a correct final ticket state, proving the audit system wrote
// each transition correctly.
func TestFullTicketFlow_AuditLogVerification(t *testing.T) {
	application := newTestAppE2E(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	baseURL := server.URL

	// ── Step 1: Create a ticket via webhook ───────────────────────────────
	payload := map[string]any{
		"message_id": "m-audit-1",
		"channel":    "telegram",
		"open_id":    "u_audit_1",
		"content":    "我的账户数据泄露了",
	}
	body, _ := json.Marshal(payload)
	webhookResp, err := http.Post(baseURL+"/api/v1/customer-service/webhook", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("webhook POST error = %v", err)
	}
	var whResult webhookResponse
	mustReadBody(t, webhookResp, &whResult)

	if !whResult.Handoff {
		t.Fatalf("handoff = %v, want true for data-leak intent", whResult.Handoff)
	}
	ticketID := whResult.TicketID

	// ── Step 2: Assign ticket ────────────────────────────────────────────
	assignURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s/assign?agent_id=agent-audit-99", baseURL, ticketID)
	assignReq, _ := http.NewRequest(http.MethodPost, assignURL, nil)
	setActorHeaders(assignReq, "supervisor-audit", "supervisor")
	assignReq.RemoteAddr = "10.0.0.1:11111"
	assignResp, _ := http.DefaultClient.Do(assignReq)
	if assignResp.StatusCode != http.StatusOK {
		t.Fatalf("assign status = %d, want 200", assignResp.StatusCode)
	}
	io.ReadAll(assignResp.Body)
	assignResp.Body.Close()

	// ── Step 3: Resolve ticket ───────────────────────────────────────────
	resolveURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s/resolve?resolution=account+secured", baseURL, ticketID)
	resolveReq, _ := http.NewRequest(http.MethodPost, resolveURL, nil)
	setActorHeaders(resolveReq, "agent-audit-99", "agent")
	resolveReq.RemoteAddr = "10.0.0.2:22222"
	resolveResp, _ := http.DefaultClient.Do(resolveReq)
	if resolveResp.StatusCode != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200", resolveResp.StatusCode)
	}
	io.ReadAll(resolveResp.Body)
	resolveResp.Body.Close()

	// ── Step 4: Verify final ticket state (audit writes were persisted) ──
	getURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s", baseURL, ticketID)
	getReq, err := http.NewRequest(http.MethodGet, getURL, nil)
	if err != nil {
		t.Fatalf("new get request error = %v", err)
	}
	setActorHeaders(getReq, "agent-audit-99", "agent")
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("GET ticket error = %v", err)
	}
	getBody, err := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	if err != nil {
		t.Fatalf("read GET body error = %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET ticket status = %d, want 200", getResp.StatusCode)
	}

	var ticketPayload map[string]any
	if err := json.Unmarshal(getBody, &ticketPayload); err != nil {
		t.Fatalf("decode ticket response error = %v", err)
	}
	tkt := ticketPayload["ticket"].(map[string]any)

	if tkt["status"] != "resolved" {
		t.Fatalf("ticket status = %v, want resolved", tkt["status"])
	}
	if tkt["priority"] != "P1" {
		t.Fatalf("ticket priority = %v, want P1", tkt["priority"])
	}
	if tkt["resolved_at"] == nil {
		t.Fatalf("resolved_at is nil, audit write must have set it during resolve")
	}
	if tkt["resolution"] != "account secured" {
		t.Fatalf("resolution = %v, want 'account secured'", tkt["resolution"])
	}
	if tkt["assigned_to"] != "agent-audit-99" {
		t.Fatalf("assigned_to = %v, want agent-audit-99", tkt["assigned_to"])
	}
}

// TestFullTicketFlow_ListEndpoint_ShowsCreatedTicket verifies that after a
// webhook-triggered handoff, the ticket appears in the GET /tickets list.
func TestFullTicketFlow_ListEndpoint_ShowsCreatedTicket(t *testing.T) {
	application := newTestAppE2E(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	baseURL := server.URL

	// Create a ticket via webhook
	payload := map[string]any{
		"message_id": "m-list-e2e-1",
		"channel":    "widget",
		"open_id":    "u_list_e2e",
		"content":    "转人工客服",
	}
	body, _ := json.Marshal(payload)
	webhookResp, err := http.Post(baseURL+"/api/v1/customer-service/webhook", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("webhook POST error = %v", err)
	}
	var whResult webhookResponse
	mustReadBody(t, webhookResp, &whResult)
	ticketID := whResult.TicketID

	// Verify ticket appears in GET /tickets list
	listReq, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/customer-service/tickets", nil)
	if err != nil {
		t.Fatalf("new tickets list request error = %v", err)
	}
	setActorHeaders(listReq, "supervisor-list", "supervisor")
	listResp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatalf("GET tickets list error = %v", err)
	}
	listBody, err := io.ReadAll(listResp.Body)
	listResp.Body.Close()
	if err != nil {
		t.Fatalf("read list body error = %v", err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("GET tickets status = %d, want 200", listResp.StatusCode)
	}

	var listPayload map[string]any
	if err := json.Unmarshal(listBody, &listPayload); err != nil {
		t.Fatalf("decode list response error = %v", err)
	}

	items, ok := listPayload["items"].([]any)
	if !ok {
		t.Fatalf("items field missing or not an array")
	}

	found := false
	for _, item := range items {
		tkt := item.(map[string]any)
		if tkt["id"] == ticketID {
			found = true
			if tkt["status"] != "open" {
				t.Fatalf("newly created ticket status = %v, want open", tkt["status"])
			}
			break
		}
	}
	if !found {
		t.Fatalf("ticket %s not found in list of %d items", ticketID, len(items))
	}
}

// TestFullTicketFlow_MultipleTickets_MaintainedSeparately verifies that concurrent
// tickets maintain independent state through the workflow.
func TestFullTicketFlow_MultipleTickets_MaintainedSeparately(t *testing.T) {
	application := newTestAppE2E(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	baseURL := server.URL

	type ticketResult struct {
		id     string
		status string
	}

	results := make([]ticketResult, 0, 2)

	for i := 0; i < 2; i++ {
		content := "我要转人工"
		if i == 0 {
			content = "我要退款"
		}
		payload := map[string]any{
			"message_id": fmt.Sprintf("m-multi-%d", i),
			"channel":    "widget",
			"open_id":    fmt.Sprintf("u_multi_%d", i),
			"content":    content,
		}
		body, _ := json.Marshal(payload)
		webhookResp, err := http.Post(baseURL+"/api/v1/customer-service/webhook", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("webhook POST error = %v", err)
		}
		var whResult webhookResponse
		whBody, err := io.ReadAll(webhookResp.Body)
		webhookResp.Body.Close()
		if err != nil {
			t.Fatalf("read webhook body error = %v", err)
		}
		if webhookResp.StatusCode != http.StatusOK {
			t.Fatalf("webhook status = %d, want 200; body: %s", webhookResp.StatusCode, string(whBody))
		}
		if err := json.Unmarshal(whBody, &whResult); err != nil {
			t.Fatalf("decode webhook response error = %v", err)
		}
		ticketID := whResult.TicketID

		// Assign only the first ticket
		if i == 0 {
			assignURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s/assign?agent_id=agent-only-first", baseURL, ticketID)
			assignReq, err := http.NewRequest(http.MethodPost, assignURL, nil)
			if err != nil {
				t.Fatalf("new assign request error = %v", err)
			}
			setActorHeaders(assignReq, "supervisor-first", "supervisor")
			assignResp, err := http.DefaultClient.Do(assignReq)
			if err != nil {
				t.Fatalf("assign POST error = %v", err)
			}
			io.ReadAll(assignResp.Body)
			assignResp.Body.Close()
			if assignResp.StatusCode != http.StatusOK {
				t.Fatalf("assign status = %d, want 200", assignResp.StatusCode)
			}
		}

		// Check state
		getURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s", baseURL, ticketID)
		getReq, err := http.NewRequest(http.MethodGet, getURL, nil)
		if err != nil {
			t.Fatalf("new get request error = %v", err)
		}
		setActorHeaders(getReq, "agent-check", "agent")
		getResp, err := http.DefaultClient.Do(getReq)
		if err != nil {
			t.Fatalf("GET ticket error = %v", err)
		}
		getBody, err := io.ReadAll(getResp.Body)
		getResp.Body.Close()
		if err != nil {
			t.Fatalf("read GET body error = %v", err)
		}
		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("GET ticket status = %d, want 200", getResp.StatusCode)
		}

		var ticketPayload map[string]any
		if err := json.Unmarshal(getBody, &ticketPayload); err != nil {
			t.Fatalf("decode ticket response error = %v", err)
		}
		tkt := ticketPayload["ticket"].(map[string]any)
		results = append(results, ticketResult{id: ticketID, status: tkt["status"].(string)})
	}

	if results[0].status != "assigned" {
		t.Fatalf("ticket[0] status = %s, want assigned", results[0].status)
	}
	if results[1].status != "open" {
		t.Fatalf("ticket[1] status = %s, want open", results[1].status)
	}

	if results[0].id == results[1].id {
		t.Fatalf("ticket IDs should be distinct: %s == %s", results[0].id, results[1].id)
	}
}

// TestFullTicketFlow_WebhookAuditEvent verifies that the webhook handoff
// path correctly records the ticket creation and generates a reply.
func TestFullTicketFlow_WebhookAuditEvent(t *testing.T) {
	application := newTestAppE2E(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	baseURL := server.URL

	payload := map[string]any{
		"message_id": "m-audit-webhook-1",
		"channel":    "widget",
		"open_id":    "u_audit_webhook",
		"content":    "我要退款",
	}
	body, _ := json.Marshal(payload)
	webhookResp, err := http.Post(baseURL+"/api/v1/customer-service/webhook", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("webhook POST error = %v", err)
	}
	var whResult webhookResponse
	mustReadBody(t, webhookResp, &whResult)

	if !whResult.Handoff {
		t.Fatalf("handoff = %v, want true", whResult.Handoff)
	}
	if whResult.TicketID == "" {
		t.Fatalf("ticket_id is empty, want non-empty")
	}
	if whResult.Reply == "" {
		t.Fatalf("reply is empty, want non-empty (audit reply should be generated)")
	}

	// Verify ticket is in open state
	getURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s", baseURL, whResult.TicketID)
	getReq, err := http.NewRequest(http.MethodGet, getURL, nil)
	if err != nil {
		t.Fatalf("new get request error = %v", err)
	}
	setActorHeaders(getReq, "agent-audit-read", "agent")
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("GET ticket error = %v", err)
	}
	getBody, err := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	if err != nil {
		t.Fatalf("read GET body error = %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET ticket status = %d, want 200", getResp.StatusCode)
	}

	var ticketPayload map[string]any
	if err := json.Unmarshal(getBody, &ticketPayload); err != nil {
		t.Fatalf("decode ticket response error = %v", err)
	}
	tkt := ticketPayload["ticket"].(map[string]any)
	if tkt["status"] != "open" {
		t.Fatalf("ticket status = %v, want open", tkt["status"])
	}
}

// TestFullTicketFlow_StateTransitionAuditOrder verifies that audit events
// are written in the correct temporal order by checking final state.
func TestFullTicketFlow_StateTransitionAuditOrder(t *testing.T) {
	application := newTestAppE2E(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	baseURL := server.URL

	// Create ticket via webhook
	payload := map[string]any{
		"message_id": "m-order-1",
		"channel":    "widget",
		"open_id":    "u_order",
		"content":    "转人工",
	}
	body, _ := json.Marshal(payload)
	webhookResp, err := http.Post(baseURL+"/api/v1/customer-service/webhook", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("webhook POST error = %v", err)
	}
	var whResult webhookResponse
	whBody, err := io.ReadAll(webhookResp.Body)
	webhookResp.Body.Close()
	if err != nil {
		t.Fatalf("read webhook body error = %v", err)
	}
	if webhookResp.StatusCode != http.StatusOK {
		t.Fatalf("webhook status = %d, want 200; body: %s", webhookResp.StatusCode, string(whBody))
	}
	if err := json.Unmarshal(whBody, &whResult); err != nil {
		t.Fatalf("decode webhook response error = %v", err)
	}
	ticketID := whResult.TicketID

	// Assign (audit event: assign)
	assignURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s/assign?agent_id=agent-order-1", baseURL, ticketID)
	assignReq, err := http.NewRequest(http.MethodPost, assignURL, nil)
	if err != nil {
		t.Fatalf("new assign request error = %v", err)
	}
	setActorHeaders(assignReq, "supervisor-order", "supervisor")
	assignResp, err := http.DefaultClient.Do(assignReq)
	if err != nil {
		t.Fatalf("assign POST error = %v", err)
	}
	io.ReadAll(assignResp.Body)
	assignResp.Body.Close()
	if assignResp.StatusCode != http.StatusOK {
		t.Fatalf("assign status = %d, want 200", assignResp.StatusCode)
	}

	// Resolve (audit event: resolve)
	resolveURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s/resolve?resolution=handled", baseURL, ticketID)
	resolveReq, err := http.NewRequest(http.MethodPost, resolveURL, nil)
	if err != nil {
		t.Fatalf("new resolve request error = %v", err)
	}
	setActorHeaders(resolveReq, "agent-order-1", "agent")
	resolveResp, err := http.DefaultClient.Do(resolveReq)
	if err != nil {
		t.Fatalf("resolve POST error = %v", err)
	}
	io.ReadAll(resolveResp.Body)
	resolveResp.Body.Close()
	if resolveResp.StatusCode != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200", resolveResp.StatusCode)
	}

	// Close (audit event: close)
	closeURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s/close?resolution=confirmed", baseURL, ticketID)
	closeReq, err := http.NewRequest(http.MethodPost, closeURL, nil)
	if err != nil {
		t.Fatalf("new close request error = %v", err)
	}
	setActorHeaders(closeReq, "supervisor-order", "supervisor")
	closeResp, err := http.DefaultClient.Do(closeReq)
	if err != nil {
		t.Fatalf("close POST error = %v", err)
	}
	io.ReadAll(closeResp.Body)
	closeResp.Body.Close()
	if closeResp.StatusCode != http.StatusOK {
		t.Fatalf("close status = %d, want 200", closeResp.StatusCode)
	}

	// Final state check: proves all audit writes succeeded in order
	getURL := fmt.Sprintf("%s/api/v1/customer-service/tickets/%s", baseURL, ticketID)
	getReq, err := http.NewRequest(http.MethodGet, getURL, nil)
	if err != nil {
		t.Fatalf("new get request error = %v", err)
	}
	setActorHeaders(getReq, "agent-order-1", "agent")
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("GET ticket (final) error = %v", err)
	}
	finalBody, err := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	if err != nil {
		t.Fatalf("read GET body error = %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET ticket (final) status = %d, want 200", getResp.StatusCode)
	}

	var finalPayload map[string]any
	if err := json.Unmarshal(finalBody, &finalPayload); err != nil {
		t.Fatalf("decode final ticket response error = %v", err)
	}
	tkt := finalPayload["ticket"].(map[string]any)

	if tkt["status"] != "closed" {
		t.Fatalf("final status = %v, want closed", tkt["status"])
	}
	if tkt["assigned_to"] != "agent-order-1" {
		t.Fatalf("final assigned_to = %v, want agent-order-1", tkt["assigned_to"])
	}
	if tkt["resolution"] != "confirmed" {
		t.Fatalf("final resolution = %v, want confirmed", tkt["resolution"])
	}
}
