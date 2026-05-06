package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bridge/ai-customer-service/internal/config"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
	"github.com/bridge/ai-customer-service/internal/domain/ticketstats"
	httpserver "github.com/bridge/ai-customer-service/internal/http"
	"github.com/bridge/ai-customer-service/internal/http/handlers"
	"github.com/bridge/ai-customer-service/internal/platform/health"
	"github.com/bridge/ai-customer-service/internal/platform/httpx"
	"github.com/bridge/ai-customer-service/internal/platformadapter"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
	"github.com/bridge/ai-customer-service/internal/service/handoff"
	intentservice "github.com/bridge/ai-customer-service/internal/service/intent"
	"github.com/bridge/ai-customer-service/internal/service/platformdelivery"
	"github.com/bridge/ai-customer-service/internal/service/reply"
	memoryStore "github.com/bridge/ai-customer-service/internal/store/memory"
	pgstore "github.com/bridge/ai-customer-service/internal/store/postgres"
)

type App struct {
	Server      *http.Server
	Probe       *health.Probe
	Logger      *slog.Logger
	closers     []func() error
	ticketStore ticketLister
}

// ticketLister abstracts the ticket store for test access.
type ticketLister interface {
	ListAll(ctx context.Context) ([]ticket.Ticket, error)
	GetStats(ctx context.Context) (ticketstats.Stats, error)
}

func New(cfg *config.Config, logger *slog.Logger) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if !cfg.Postgres.Enabled && cfg.Runtime.Env == "" {
		return nil, fmt.Errorf("runtime env is required when postgres is disabled; memory mode must be explicitly limited to non-prod")
	}

	var (
		sessions          dialog.SessionRepository
		audits            dialog.AuditRepository
		tickets           dialog.TicketRepository
		dedup             dialog.DedupRepository
		platformEvents    *pgstore.PlatformEventStore
		ticketService     handlers.TicketService
		checkers          []health.Checker
		closers           []func() error
		workerClosers     []func() error
		ticketListerStore ticketLister
		sessionStore      dialog.SessionRepository
		ticketStore       dialog.TicketRepository
	)

	probe := health.NewProbe()

	if cfg.Postgres.Enabled {
		db, err := pgstore.Open(pgstore.Config{DSN: cfg.Postgres.DSN, MaxOpenConns: cfg.Postgres.MaxOpenConns, MaxIdleConns: cfg.Postgres.MaxIdleConns, ConnMaxLifetime: time.Duration(cfg.Postgres.ConnMaxLifetime) * time.Second})
		if err != nil {
			return nil, err
		}
		if err := pgstore.RunMigrations(db, cfg.Postgres.MigrationDir); err != nil {
			_ = db.Close()
			return nil, err
		}
		sessionStore := pgstore.NewSessionStore(db)
		auditStore := pgstore.NewAuditStore(db)
		ticketStore := pgstore.NewTicketStore(db)
		dedupStore := pgstore.NewDedupStore(db)
		platformEvents = pgstore.NewPlatformEventStore(db)
		sessions = sessionStore
		audits = auditStore
		tickets = ticketStore
		dedup = dedupStore
		ticketService = pgstore.NewTicketWorkflowStore(db, auditStore)
		checkers = append(checkers, pgstore.NewDBChecker(db))
		closers = append(closers, db.Close)
		ticketListerStore = ticketStore
		probe.SetReady(true)
	} else {
		sessionStore := memoryStore.NewSessionStore()
		auditStore := memoryStore.NewAuditStore()
		ticketStore := memoryStore.NewTicketStore()
		dedupStore := memoryStore.NewDedupStore()
		sessions = sessionStore
		audits = auditStore
		tickets = ticketStore
		dedup = dedupStore
		ticketService = ticketStore
		ticketListerStore = ticketStore
		probe.SetReady(false)
	}

	knowledgeStore := memoryStore.NewKnowledgeStore()
	intentSvc := intentservice.NewService()
	replySvc := reply.NewService(knowledgeStore)
	handoffSvc := handoff.NewService()
	dialogSvc := dialog.NewService(sessions, audits, tickets, dedup, intentSvc, replySvc, handoffSvc)
	rateLimiter := httpx.NewRateLimiter(time.Second, 10)

	healthHandler := handlers.NewHealthHandler(probe, checkers...)
	webhookHandler := handlers.NewWebhookHandler(dialogSvc, logger, audits)
	ticketHandler := handlers.NewTicketHandler(ticketService, audits)
	ticketStatsHandler := handlers.NewTicketStatsHandler(ticketListerStore, audits)
	sessionHandler := handlers.NewSessionHandler(sessionStore, ticketStore, audits)
	webhookSecurity := handlers.WebhookSecurity{Secret: cfg.Webhook.Secret, TimestampHeader: cfg.Webhook.TimestampHeader, SignatureHeader: cfg.Webhook.SignatureHeader, MaxSkew: time.Duration(cfg.Webhook.MaxSkewSeconds) * time.Second, Audit: audits}

	var (
		platformWebhookHandler *handlers.PlatformWebhookHandler
		platformWebhookAuth    handlers.PlatformWebhookSecurity
	)
	if cfg.PlatformAdapters.Enabled {
		var adapters []platformadapter.PlatformAdapter
		if cfg.PlatformAdapters.Sub2API.Enabled {
			adapters = append(adapters, platformadapter.NewSub2APIAdapter())
		}
		if cfg.PlatformAdapters.NewAPI.Enabled {
			adapters = append(adapters, platformadapter.NewNewAPIAdapter())
		}
		if len(adapters) > 0 {
			platformWebhookHandler = handlers.NewPlatformWebhookHandler(dialogSvc, platformadapter.NewRegistry(adapters...), platformEvents)
			platformWebhookAuth = handlers.PlatformWebhookSecurity{
				TimestampHeader: cfg.Webhook.TimestampHeader,
				SignatureHeader: cfg.Webhook.SignatureHeader,
				MaxSkew:         time.Duration(cfg.Webhook.MaxSkewSeconds) * time.Second,
				Audit:           audits,
				Sub2APISecret:   cfg.PlatformAdapters.Sub2API.IngressSecret,
				NewAPISecret:    cfg.PlatformAdapters.NewAPI.IngressSecret,
			}
		}
	}

	router := httpserver.NewRouter(httpserver.RouterDeps{
		Health:              healthHandler,
		Webhook:             webhookHandler,
		PlatformWebhook:     platformWebhookHandler,
		PlatformWebhookAuth: platformWebhookAuth,
		Tickets:             ticketHandler,
		TicketStats:         ticketStatsHandler,
		Sessions:            sessionHandler,
		WebhookAuth:         webhookSecurity,
		MaxBodyBytes:        cfg.HTTP.MaxBodyBytes,
		RateLimiter:         rateLimiter,
	})

	if cfg.PlatformAdapters.Enabled && platformEvents != nil {
		startWorker := func(platform string, profile config.PlatformAdapterProfileConfig) {
			if !profile.Enabled || profile.CallbackBaseURL == "" || profile.CallbackSecret == "" {
				return
			}
			workerCtx, cancel := context.WithCancel(context.Background())
			workerClosers = append(workerClosers, func() error {
				cancel()
				return nil
			})
			worker := platformdelivery.NewWorker(
				platform,
				profile.CallbackBaseURL,
				platformEvents,
				&http.Client{Timeout: time.Duration(profile.CallbackTimeoutMS) * time.Millisecond},
				platformdelivery.Signer{
					Secret:          profile.CallbackSecret,
					TimestampHeader: cfg.Webhook.TimestampHeader,
					SignatureHeader: cfg.Webhook.SignatureHeader,
				},
				profile.CallbackMaxRetries,
			)
			worker.Logger = logger
			go worker.Start(workerCtx)
		}
		startWorker("sub2api", cfg.PlatformAdapters.Sub2API)
		startWorker("newapi", cfg.PlatformAdapters.NewAPI)
	}
	closers = append(workerClosers, closers...)

	return &App{
		Server: &http.Server{
			Addr:              cfg.HTTP.Addr,
			Handler:           router,
			ReadHeaderTimeout: time.Duration(cfg.HTTP.ReadHeaderTimeout) * time.Second,
			ReadTimeout:       time.Duration(cfg.HTTP.ReadTimeout) * time.Second,
			WriteTimeout:      time.Duration(cfg.HTTP.WriteTimeout) * time.Second,
			IdleTimeout:       time.Duration(cfg.HTTP.IdleTimeout) * time.Second,
			MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
		},
		Probe:       probe,
		Logger:      logger,
		closers:     closers,
		ticketStore: ticketListerStore,
	}, nil
}

func (a *App) TicketStore() ticketLister {
	return a.ticketStore
}

func (a *App) Shutdown(ctx context.Context) error {
	if a == nil || a.Server == nil {
		return nil
	}
	if a.Probe != nil {
		a.Probe.SetReady(false)
		a.Probe.SetLive(false)
	}
	err := a.Server.Shutdown(ctx)
	for _, closeFn := range a.closers {
		if closeErr := closeFn(); err == nil && closeErr != nil {
			err = closeErr
		}
	}
	return err
}
