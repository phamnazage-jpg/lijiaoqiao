package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/app"
	"github.com/bridge/ai-customer-service/internal/config"
	"github.com/bridge/ai-customer-service/internal/http/handlers"
	"github.com/bridge/ai-customer-service/internal/platform/logging"
	pgstore "github.com/bridge/ai-customer-service/internal/store/postgres"
)

func platformTestDSN() string {
	return "host=localhost port=5434 user=ai_cs password=ai_cs_secret dbname=ai_customer_service sslmode=disable"
}

func openPlatformTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := pgstore.Open(pgstore.Config{
		DSN:             platformTestDSN(),
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("open postgres failed: %v", err)
	}
	if err := pgstore.RunMigrations(db, "../../db/migration"); err != nil {
		_ = db.Close()
		t.Fatalf("run migrations failed: %v", err)
	}
	return db
}

func newSub2APIIntegrationApp(t *testing.T) *app.App {
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
	cfg.Postgres.Enabled = true
	cfg.Postgres.DSN = platformTestDSN()
	cfg.Postgres.MigrationDir = "../../db/migration"
	cfg.Postgres.MaxOpenConns = 5
	cfg.Postgres.MaxIdleConns = 2
	cfg.Postgres.ConnMaxLifetime = 30
	cfg.Webhook.Secret = "default-webhook-secret"
	cfg.Webhook.TimestampHeader = "X-CS-Timestamp"
	cfg.Webhook.SignatureHeader = "X-CS-Signature"
	cfg.Webhook.MaxSkewSeconds = 300
	cfg.PlatformAdapters.Enabled = true
	cfg.PlatformAdapters.Sub2API.Enabled = true
	cfg.PlatformAdapters.Sub2API.IngressSecret = "sub2api-ingress-secret"

	application, err := app.New(cfg, logging.New())
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = application.Shutdown(context.Background())
	})
	return application
}

func TestSub2APIWebhookFlow_ShouldCreateSessionTicketAndOutboxEvents(t *testing.T) {
	db := openPlatformTestDB(t)
	defer db.Close()

	application := newSub2APIIntegrationApp(t)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	openID := "sub2api-intg-" + time.Now().UTC().Format("150405.000000000")
	payload := map[string]any{
		"message_id": "m-intg-" + time.Now().UTC().Format("150405.000000000"),
		"channel":    "sub2api",
		"open_id":    openID,
		"content":    "我要退款",
	}
	body, _ := json.Marshal(payload)
	timestamp, signature, err := handlers.SignWebhookRequest("sub2api-ingress-secret", time.Now().Unix(), body)
	if err != nil {
		t.Fatalf("SignWebhookRequest() error = %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/customer-service/platforms/sub2api/webhook", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CS-Timestamp", timestamp)
	req.Header.Set("X-CS-Signature", signature)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var ack map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		t.Fatalf("decode ack error = %v", err)
	}
	sessionID, _ := ack["session_id"].(string)
	ticketID, _ := ack["ticket_id"].(string)
	if sessionID == "" || ticketID == "" {
		t.Fatalf("ack session_id=%v ticket_id=%v, want both non-empty", ack["session_id"], ack["ticket_id"])
	}

	var storedSessionID string
	if err := db.QueryRowContext(context.Background(), `
		SELECT id
		FROM cs_sessions
		WHERE channel = 'sub2api' AND open_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, openID).Scan(&storedSessionID); err != nil {
		t.Fatalf("query session failed: %v", err)
	}
	if storedSessionID != sessionID {
		t.Fatalf("stored session id = %s, want %s", storedSessionID, sessionID)
	}

	var storedTicketID string
	if err := db.QueryRowContext(context.Background(), `
		SELECT id
		FROM cs_tickets
		WHERE id = $1 AND session_id = $2
	`, ticketID, sessionID).Scan(&storedTicketID); err != nil {
		t.Fatalf("query ticket failed: %v", err)
	}
	if storedTicketID != ticketID {
		t.Fatalf("stored ticket id = %s, want %s", storedTicketID, ticketID)
	}

	var outboxCount int
	if err := db.QueryRowContext(context.Background(), `
		SELECT COUNT(1)
		FROM cs_platform_event_outbox
		WHERE session_id = $1 AND platform = 'sub2api'
	`, sessionID).Scan(&outboxCount); err != nil {
		t.Fatalf("query outbox count failed: %v", err)
	}
	if outboxCount != 6 {
		t.Fatalf("outbox count = %d, want 6", outboxCount)
	}
}
