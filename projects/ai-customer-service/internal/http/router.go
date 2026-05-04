package httpserver

import (
	"net/http"
	"strings"

	"github.com/bridge/ai-customer-service/internal/domain/error/cserrors"
	"github.com/bridge/ai-customer-service/internal/http/handlers"
	"github.com/bridge/ai-customer-service/internal/http/middleware"
	"github.com/bridge/ai-customer-service/internal/platform/httpx"
)

type RouterDeps struct {
	Health       *handlers.HealthHandler
	Webhook      *handlers.WebhookHandler
	Tickets      *handlers.TicketHandler
	TicketStats  *handlers.TicketStatsHandler
	Sessions     *handlers.SessionHandler
	WebhookAuth  handlers.WebhookSecurity
	MaxBodyBytes int64
	RateLimiter  *httpx.RateLimiter
}

func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/actuator/health", deps.Health.Health)
	mux.HandleFunc("/actuator/health/live", deps.Health.Live)
	mux.HandleFunc("/actuator/health/ready", deps.Health.Ready)

	webhook := httpx.WithBodyLimit(http.HandlerFunc(deps.Webhook.Handle), deps.MaxBodyBytes)
	if deps.RateLimiter != nil {
		webhook = deps.RateLimiter.WithRateLimit(webhook)
	}
	webhook = deps.WebhookAuth.Wrap(webhook)
	mux.Handle("/api/v1/customer-service/webhook", webhook)

	webhookChannel := httpx.WithBodyLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		channel := strings.TrimPrefix(r.URL.Path, "/api/v1/customer-service/webhook/")
		channel = strings.TrimSuffix(channel, "/")
		channel = strings.Trim(channel, "/")
		if channel == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"` + cserrors.CS_REQ_4008 + `","message":"channel is required"}}`))
			return
		}
		deps.Webhook.HandleChannel(w, r, channel)
	}), deps.MaxBodyBytes)
	if deps.RateLimiter != nil {
		webhookChannel = deps.RateLimiter.WithRateLimit(webhookChannel)
	}
	webhookChannel = deps.WebhookAuth.Wrap(webhookChannel)
	mux.Handle("/api/v1/customer-service/webhook/", webhookChannel)

	if deps.Tickets != nil {
		mux.HandleFunc("/api/v1/customer-service/tickets", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				writeMethodNotAllowed(w)
				return
			}
			middleware.RequireRoles(http.HandlerFunc(deps.Tickets.List), "agent", "supervisor", "admin").ServeHTTP(w, r)
		})
		mux.HandleFunc("/api/v1/customer-service/tickets/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/api/v1/customer-service/tickets/stats" {
				if deps.TicketStats != nil {
					middleware.RequireRoles(http.HandlerFunc(deps.TicketStats.Get), "supervisor", "admin").ServeHTTP(w, r)
					return
				}
			}
			// P1-3: GET /api/v1/customer-service/tickets/{id} — Phase 1 minimum implementation
			if r.Method == http.MethodGet {
				middleware.RequireRoles(http.HandlerFunc(deps.Tickets.Get), "agent", "supervisor", "admin").ServeHTTP(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/assign") {
				if r.Method != http.MethodPost {
					writeMethodNotAllowed(w)
					return
				}
				middleware.RequireRoles(http.HandlerFunc(deps.Tickets.Assign), "supervisor", "admin").ServeHTTP(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/resolve") {
				if r.Method != http.MethodPost {
					writeMethodNotAllowed(w)
					return
				}
				middleware.RequireRoles(http.HandlerFunc(deps.Tickets.Resolve), "agent", "supervisor", "admin").ServeHTTP(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/close") {
				if r.Method != http.MethodPost {
					writeMethodNotAllowed(w)
					return
				}
				middleware.RequireRoles(http.HandlerFunc(deps.Tickets.Close), "supervisor", "admin").ServeHTTP(w, r)
				return
			}
			writeMethodNotAllowed(w)
		})
	}

	// Phase 1: session feedback and manual handoff endpoints
	if deps.Sessions != nil {
		mux.HandleFunc("/api/v1/customer-service/sessions/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/feedback") {
				if r.Method != http.MethodPost {
					writeMethodNotAllowed(w)
					return
				}
				deps.Sessions.Feedback(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/handoff") {
				if r.Method != http.MethodPost {
					writeMethodNotAllowed(w)
					return
				}
				middleware.RequireRoles(http.HandlerFunc(deps.Sessions.Handoff), "agent", "supervisor", "admin").ServeHTTP(w, r)
				return
			}
			writeMethodNotAllowed(w)
		})
	}

	return mux
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	_, _ = w.Write([]byte(`{"error":{"code":"` + cserrors.CS_HTTP_405 + `","message":"method not allowed"}}`))
}
