package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bridge/ai-customer-service/internal/config"
	httpserver "github.com/bridge/ai-customer-service/internal/http"
	"github.com/bridge/ai-customer-service/internal/domain/ticketstats"
	"github.com/bridge/ai-customer-service/internal/http/handlers"
	"github.com/bridge/ai-customer-service/internal/platform/health"
	"github.com/bridge/ai-customer-service/internal/platform/httpx"
	intentservice "github.com/bridge/ai-customer-service/internal/service/intent"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
	"github.com/bridge/ai-customer-service/internal/service/handoff"
	"github.com/bridge/ai-customer-service/internal/service/reply"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
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

	var (
		sessions          dialog.SessionRepository
		audits            dialog.AuditRepository
		tickets           dialog.TicketRepository
		dedup             dialog.DedupRepository
		ticketService     handlers.TicketService
		checkers          []health.Checker
		closers           []func() error
		ticketListerStore ticketLister
		sessionStore      dialog.SessionRepository
		ticketStore       dialog.TicketRepository
	)

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
		sessions = sessionStore
		audits = auditStore
		tickets = ticketStore
		dedup = dedupStore
		ticketService = pgstore.NewTicketWorkflowStore(db, auditStore)
		checkers = append(checkers, pgstore.NewDBChecker(db))
		closers = append(closers, db.Close)
		ticketListerStore = ticketStore
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
	}

	knowledgeStore := memoryStore.NewKnowledgeStore()
	intentSvc := intentservice.NewService()
	replySvc := reply.NewService(knowledgeStore)
	handoffSvc := handoff.NewService()
	dialogSvc := dialog.NewService(sessions, audits, tickets, dedup, intentSvc, replySvc, handoffSvc)
	// P1-2: webhook rate limiter — 10 messages per second per IP
	rateLimiter := httpx.NewRateLimiter(time.Second, 10)

	probe := health.NewProbe()
	healthHandler := handlers.NewHealthHandler(probe, checkers...)
	webhookHandler := handlers.NewWebhookHandler(dialogSvc, logger, audits)
	ticketHandler := handlers.NewTicketHandler(ticketService, audits)
	ticketStatsHandler := handlers.NewTicketStatsHandler(ticketListerStore, audits)
	sessionHandler := handlers.NewSessionHandler(sessionStore, ticketStore, audits)
	webhookSecurity := handlers.WebhookSecurity{Secret: cfg.Webhook.Secret, TimestampHeader: cfg.Webhook.TimestampHeader, SignatureHeader: cfg.Webhook.SignatureHeader, MaxSkew: time.Duration(cfg.Webhook.MaxSkewSeconds) * time.Second, Audit: audits}
	router := httpserver.NewRouter(httpserver.RouterDeps{Health: healthHandler, Webhook: webhookHandler, Tickets: ticketHandler, TicketStats: ticketStatsHandler, Sessions: sessionHandler, WebhookAuth: webhookSecurity, MaxBodyBytes: cfg.HTTP.MaxBodyBytes, RateLimiter: rateLimiter})

	probe.SetReady(true)
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
