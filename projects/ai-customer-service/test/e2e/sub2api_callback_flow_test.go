package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/app"
	"github.com/bridge/ai-customer-service/internal/config"
	"github.com/bridge/ai-customer-service/internal/domain/platformevent"
	"github.com/bridge/ai-customer-service/internal/http/handlers"
	"github.com/bridge/ai-customer-service/internal/platform/logging"
	pgstore "github.com/bridge/ai-customer-service/internal/store/postgres"
)

func e2ePlatformDSN() string {
	return "host=localhost port=5434 user=ai_cs password=ai_cs_secret dbname=ai_customer_service sslmode=disable"
}

func openE2EPlatformDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := pgstore.Open(pgstore.Config{
		DSN:             e2ePlatformDSN(),
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

func newSub2APIE2EApp(t *testing.T, callbackURL string, callbackSecret string, maxRetries int) *app.App {
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
	cfg.Postgres.DSN = e2ePlatformDSN()
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
	cfg.PlatformAdapters.Sub2API.CallbackBaseURL = callbackURL
	cfg.PlatformAdapters.Sub2API.CallbackSecret = callbackSecret
	cfg.PlatformAdapters.Sub2API.CallbackTimeoutMS = 2000
	cfg.PlatformAdapters.Sub2API.CallbackMaxRetries = maxRetries

	application, err := app.New(cfg, logging.New())
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = application.Shutdown(context.Background())
	})
	return application
}

func eventually(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("condition not satisfied before timeout")
}

func TestSub2APICallbackFlow_ShouldDeliverOrderedEventsWithStableEventIDs(t *testing.T) {
	db := openE2EPlatformDB(t)
	defer db.Close()

	var (
		mu       sync.Mutex
		received []platformevent.Event
	)
	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var event platformevent.Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Fatalf("decode callback body failed: %v", err)
		}
		received = append(received, event)
		w.WriteHeader(http.StatusOK)
	}))
	defer callbackServer.Close()

	application := newSub2APIE2EApp(t, callbackServer.URL, "sub2api-callback-secret", 3)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	openID := "sub2api-e2e-" + time.Now().UTC().Format("150405.000000000")
	payload := map[string]any{
		"message_id": "m-e2e-" + time.Now().UTC().Format("150405.000000000"),
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
		t.Fatalf("decode ack failed: %v", err)
	}
	sessionID, _ := ack["session_id"].(string)
	if sessionID == "" {
		t.Fatalf("ack session_id = %v, want non-empty", ack["session_id"])
	}

	eventually(t, 8*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		count := 0
		for _, event := range received {
			if event.SessionID == sessionID {
				count++
			}
		}
		return count == 6
	})

	mu.Lock()
	defer mu.Unlock()
	filtered := make([]platformevent.Event, 0, 6)
	for _, event := range received {
		if event.SessionID == sessionID {
			filtered = append(filtered, event)
		}
	}
	wantTypes := []string{
		platformevent.TypeMessageReceived,
		platformevent.TypeMessageProcessing,
		platformevent.TypeIntentResolved,
		platformevent.TypeHandoffTriggered,
		platformevent.TypeTicketCreated,
		platformevent.TypeReplyGenerated,
	}
	seenIDs := make(map[string]struct{}, len(filtered))
	for i, event := range filtered {
		if event.EventType != wantTypes[i] {
			t.Fatalf("event[%d].type = %s, want %s", i, event.EventType, wantTypes[i])
		}
		if event.ID == "" {
			t.Fatalf("event[%d] id is empty", i)
		}
		if _, exists := seenIDs[event.ID]; exists {
			t.Fatalf("duplicate event id: %s", event.ID)
		}
		seenIDs[event.ID] = struct{}{}
	}

	var deliveredCount int
	if err := db.QueryRowContext(context.Background(), `
		SELECT COUNT(1)
		FROM cs_platform_event_outbox
		WHERE platform = 'sub2api' AND status = 'delivered' AND session_id IN (
			SELECT id FROM cs_sessions WHERE channel = 'sub2api' AND open_id = $1
		)
	`, openID).Scan(&deliveredCount); err != nil {
		t.Fatalf("query delivered count failed: %v", err)
	}
	if deliveredCount != 6 {
		t.Fatalf("delivered count = %d, want 6", deliveredCount)
	}
}

func TestSub2APICallbackFlow_ShouldDeadLetterAfterMaxRetries(t *testing.T) {
	db := openE2EPlatformDB(t)
	defer db.Close()

	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream"}`))
	}))
	defer callbackServer.Close()

	application := newSub2APIE2EApp(t, callbackServer.URL, "sub2api-callback-secret", 1)
	server := httptest.NewServer(application.Server.Handler)
	defer server.Close()

	openID := "sub2api-dead-" + time.Now().UTC().Format("150405.000000000")
	payload := map[string]any{
		"message_id": "m-dead-" + time.Now().UTC().Format("150405.000000000"),
		"channel":    "sub2api",
		"open_id":    openID,
		"content":    "晚上好",
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

	eventually(t, 8*time.Second, func() bool {
		var deadCount int
		err := db.QueryRowContext(context.Background(), `
			SELECT COUNT(1)
			FROM cs_platform_event_dead_letters dl
			JOIN cs_platform_event_outbox o ON o.id = dl.event_id
			WHERE o.platform = 'sub2api' AND o.session_id IN (
				SELECT id FROM cs_sessions WHERE channel = 'sub2api' AND open_id = $1
			)
		`, openID).Scan(&deadCount)
		return err == nil && deadCount == 4
	})
}
