package dialog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	"github.com/bridge/ai-customer-service/internal/domain/message"
	"github.com/bridge/ai-customer-service/internal/domain/session"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
	intentdomain "github.com/bridge/ai-customer-service/internal/domain/intent"
	"github.com/bridge/ai-customer-service/internal/service/handoff"
	intentservice "github.com/bridge/ai-customer-service/internal/service/intent"
	"github.com/bridge/ai-customer-service/internal/service/reply"
	"github.com/bridge/ai-customer-service/internal/store/memory"
)

// ------------------------------------------------------------------
// Mock implementations for targeted error injection
// ------------------------------------------------------------------

type mockSessionStore struct {
	getOrCreateFn func(ctx context.Context, channel, openID string, now time.Time) (*session.Session, error)
	saveFn        func(ctx context.Context, sess *session.Session) error
}

func (m *mockSessionStore) GetOrCreate(ctx context.Context, channel, openID string, now time.Time) (*session.Session, error) {
	if m.getOrCreateFn != nil {
		return m.getOrCreateFn(ctx, channel, openID, now)
	}
	s := memory.NewSessionStore()
	return s.GetOrCreate(ctx, channel, openID, now)
}
func (m *mockSessionStore) Save(ctx context.Context, sess *session.Session) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, sess)
	}
	return nil
}
func (m *mockSessionStore) GetByID(ctx context.Context, id string) (*session.Session, error) {
	s := memory.NewSessionStore()
	return s.GetByID(ctx, id)
}

type mockAuditStore struct {
	addFn func(ctx context.Context, event audit.Event) error
}

func (m *mockAuditStore) Add(ctx context.Context, event audit.Event) error {
	if m.addFn != nil {
		return m.addFn(ctx, event)
	}
	return nil
}

// errorTicketStore always fails on Create — used to cover the handoff path error branch.
type errorTicketStore struct{}

func (e *errorTicketStore) Create(ctx context.Context, t *ticket.Ticket) error {
	return errors.New("ticket creation failed")
}
func (e *errorTicketStore) GetByID(ctx context.Context, id string) (*ticket.Ticket, error) {
	return nil, nil
}

// mockIntentService wraps intentservice.Service so we can inject a Recognize error.
type mockIntentService struct {
	real        *intentservice.Service
	recognizeFn func(ctx context.Context, sessionID, content string, ctxMsgs []session.MessageContext) (*intentdomain.Result, error)
}

func (m *mockIntentService) Recognize(ctx context.Context, sessionID, content string, ctxMsgs []session.MessageContext) (*intentdomain.Result, error) {
	if m.recognizeFn != nil {
		return m.recognizeFn(ctx, sessionID, content, ctxMsgs)
	}
	return m.real.Recognize(ctx, sessionID, content, ctxMsgs)
}

// mockHandoffService wraps handoff.Service so we can inject a ShouldHandoff error.
type mockHandoffService struct {
	real            *handoff.Service
	shouldHandoffFn func(ctx context.Context, intent *intentdomain.Result, turnCount int) (*handoff.Decision, error)
}

func (m *mockHandoffService) ShouldHandoff(ctx context.Context, intent *intentdomain.Result, turnCount int) (*handoff.Decision, error) {
	if m.shouldHandoffFn != nil {
		return m.shouldHandoffFn(ctx, intent, turnCount)
	}
	return m.real.ShouldHandoff(ctx, intent, turnCount)
}

// ------------------------------------------------------------------
// Existing tests — kept intact
// ------------------------------------------------------------------

func TestProcessCreatesTicketOnHandoff(t *testing.T) {
	sessions := memory.NewSessionStore()
	audits := memory.NewAuditStore()
	tickets := memory.NewTicketStore()
	dedup := memory.NewDedupStore()
	knowledge := memory.NewKnowledgeStore()
	svc := NewService(sessions, audits, tickets, dedup, intentservice.NewService(), reply.NewService(knowledge), handoff.NewService())

	result, err := svc.Process(context.Background(), &message.UnifiedMessage{MessageID: "m1", Channel: "widget", OpenID: "u1", Content: "我要申请退款"})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if !result.Handoff.ShouldHandoff {
		t.Fatalf("expected handoff")
	}
	if result.TicketID == "" {
		t.Fatalf("expected ticket id")
	}
	if len(tickets.List()) != 1 {
		t.Fatalf("ticket count = %d, want 1", len(tickets.List()))
	}
	if len(audits.List()) != 1 {
		t.Fatalf("audit count = %d, want 1", len(audits.List()))
	}
	if audits.List()[0].Type != "message_processed" {
		t.Fatalf("audit type = %s", audits.List()[0].Type)
	}
}

func TestProcessDeduplicatesMessage(t *testing.T) {
	sessions := memory.NewSessionStore()
	audits := memory.NewAuditStore()
	tickets := memory.NewTicketStore()
	dedup := memory.NewDedupStore()
	knowledge := memory.NewKnowledgeStore()
	svc := NewService(sessions, audits, tickets, dedup, intentservice.NewService(), reply.NewService(knowledge), handoff.NewService())

	_, err := svc.Process(context.Background(), &message.UnifiedMessage{MessageID: "m1", Channel: "widget", OpenID: "u1", Content: "查询额度"})
	if err != nil {
		t.Fatalf("first Process() error = %v", err)
	}
	result, err := svc.Process(context.Background(), &message.UnifiedMessage{MessageID: "m1", Channel: "widget", OpenID: "u1", Content: "查询额度"})
	if err != nil {
		t.Fatalf("second Process() error = %v", err)
	}
	if result.Reply != "duplicate message ignored" {
		t.Fatalf("reply = %q, want duplicate message ignored", result.Reply)
	}
}

// ------------------------------------------------------------------
// Table-driven tests for uncovered branches
// ------------------------------------------------------------------

func TestProcessBranches(t *testing.T) {
	fixedTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		setup      func(t *testing.T) *Service
		msg        *message.UnifiedMessage
		wantErr    string
		assertions func(t *testing.T, result *Result)
	}{
		// Branch 1: intent.Recognize returns error
		{
			name: "intent_recognize_error",
			setup: func(t *testing.T) *Service {
				intentSvc := &mockIntentService{real: intentservice.NewService()}
				intentSvc.recognizeFn = func(ctx context.Context, sessionID, content string, ctxMsgs []session.MessageContext) (*intentdomain.Result, error) {
					return nil, errors.New("intent recognition failed")
				}
				hSvc := &mockHandoffService{real: handoff.NewService()}
				svc := NewService(
					memory.NewSessionStore(),
					memory.NewAuditStore(),
					memory.NewTicketStore(),
					memory.NewDedupStore(),
					intentSvc, // implements IntentRecognizer
					reply.NewService(memory.NewKnowledgeStore()),
					hSvc, // implements HandoffDecider
				)
				svc.now = func() time.Time { return fixedTime }
				return svc
			},
			msg:     &message.UnifiedMessage{MessageID: "m1", Channel: "widget", OpenID: "u1", Content: "hello"},
			wantErr: "intent recognition failed",
		},

		// Branch 2: handoff.ShouldHandoff returns error
		{
			name: "handoff_should_handoff_error",
			setup: func(t *testing.T) *Service {
				intentSvc := &mockIntentService{real: intentservice.NewService()}
				hSvc := &mockHandoffService{real: handoff.NewService()}
				hSvc.shouldHandoffFn = func(ctx context.Context, intent *intentdomain.Result, turnCount int) (*handoff.Decision, error) {
					return nil, errors.New("handoff check failed")
				}
				svc := NewService(
					memory.NewSessionStore(),
					memory.NewAuditStore(),
					memory.NewTicketStore(),
					memory.NewDedupStore(),
					intentSvc,
					reply.NewService(memory.NewKnowledgeStore()),
					hSvc,
				)
				svc.now = func() time.Time { return fixedTime }
				return svc
			},
			msg:     &message.UnifiedMessage{MessageID: "m1", Channel: "widget", OpenID: "u1", Content: "hello"},
			wantErr: "handoff check failed",
		},

		// Branch 3: tickets.Create returns error (handoff path)
		{
			name: "tickets_create_error_handoff_path",
			setup: func(t *testing.T) *Service {
				intentSvc := &mockIntentService{real: intentservice.NewService()}
				hSvc := &mockHandoffService{real: handoff.NewService()}
				svc := NewService(
					memory.NewSessionStore(),
					memory.NewAuditStore(),
					&errorTicketStore{}, // always fails on Create
					memory.NewDedupStore(),
					intentSvc,
					reply.NewService(memory.NewKnowledgeStore()),
					hSvc,
				)
				svc.now = func() time.Time { return fixedTime }
				return svc
			},
			msg:     &message.UnifiedMessage{MessageID: "m1", Channel: "widget", OpenID: "u1", Content: "我要申请退款"},
			wantErr: "ticket creation failed",
		},

		// Branch 4: sessions.Save returns error
		{
			name: "sessions_save_error",
			setup: func(t *testing.T) *Service {
				sessStore := &mockSessionStore{}
				sessStore.getOrCreateFn = func(ctx context.Context, channel, openID string, now time.Time) (*session.Session, error) {
					return &session.Session{
						ID:            "test-session",
						Channel:       channel,
						OpenID:        openID,
						Status:        session.StatusIdle,
						TurnCount:     0,
						LastMessageAt: now,
						Context:       []session.MessageContext{},
					}, nil
				}
				sessStore.saveFn = func(ctx context.Context, sess *session.Session) error {
					return errors.New("session save failed")
				}
				intentSvc := &mockIntentService{real: intentservice.NewService()}
				hSvc := &mockHandoffService{real: handoff.NewService()}
				svc := NewService(
					sessStore,
					memory.NewAuditStore(),
					memory.NewTicketStore(),
					memory.NewDedupStore(),
					intentSvc,
					reply.NewService(memory.NewKnowledgeStore()),
					hSvc,
				)
				svc.now = func() time.Time { return fixedTime }
				return svc
			},
			msg:     &message.UnifiedMessage{MessageID: "m1", Channel: "widget", OpenID: "u1", Content: "hello"},
			wantErr: "session save failed",
		},

		// Branch 5: audits.Add returns error
		{
			name: "audits_add_error",
			setup: func(t *testing.T) *Service {
				auditStore := &mockAuditStore{}
				auditStore.addFn = func(ctx context.Context, event audit.Event) error {
					return errors.New("audit add failed")
				}
				intentSvc := &mockIntentService{real: intentservice.NewService()}
				hSvc := &mockHandoffService{real: handoff.NewService()}
				svc := NewService(
					memory.NewSessionStore(),
					auditStore,
					memory.NewTicketStore(),
					memory.NewDedupStore(),
					intentSvc,
					reply.NewService(memory.NewKnowledgeStore()),
					hSvc,
				)
				svc.now = func() time.Time { return fixedTime }
				return svc
			},
			msg:     &message.UnifiedMessage{MessageID: "m1", Channel: "widget", OpenID: "u1", Content: "hello"},
			wantErr: "audit add failed",
		},

		// Branch 6: msg.Timestamp is NOT zero (timestamp already set path)
		{
			name: "timestamp_already_set",
			setup: func(t *testing.T) *Service {
				intentSvc := &mockIntentService{real: intentservice.NewService()}
				hSvc := &mockHandoffService{real: handoff.NewService()}
				svc := NewService(
					memory.NewSessionStore(),
					memory.NewAuditStore(),
					memory.NewTicketStore(),
					memory.NewDedupStore(),
					intentSvc,
					reply.NewService(memory.NewKnowledgeStore()),
					hSvc,
				)
				svc.now = func() time.Time { return fixedTime }
				return svc
			},
			msg: &message.UnifiedMessage{
				MessageID: "m1",
				Channel:   "widget",
				OpenID:    "u1",
				Content:   "hello",
				Timestamp: fixedTime.Add(time.Hour), // non-zero — service should NOT overwrite
			},
			wantErr: "",
			assertions: func(t *testing.T, result *Result) {
				if result == nil {
					t.Fatal("expected non-nil result")
				}
			},
		},

		// Branch 7: dedup is nil (dedup check is skipped entirely)
		{
			name: "dedup_nil_skipped",
			setup: func(t *testing.T) *Service {
				intentSvc := &mockIntentService{real: intentservice.NewService()}
				hSvc := &mockHandoffService{real: handoff.NewService()}
				svc := NewService(
					memory.NewSessionStore(),
					memory.NewAuditStore(),
					memory.NewTicketStore(),
					nil, // nil dedup
					intentSvc,
					reply.NewService(memory.NewKnowledgeStore()),
					hSvc,
				)
				svc.now = func() time.Time { return fixedTime }
				return svc
			},
			msg: &message.UnifiedMessage{
				MessageID: "m1",
				Channel:   "widget",
				OpenID:    "u1",
				Content:   "hello with nil dedup",
			},
			wantErr: "",
			assertions: func(t *testing.T, result *Result) {
				if result.Reply == "duplicate message ignored" {
					t.Error("reply should NOT be duplicate-ignored when dedup is nil, even with MessageID set")
				}
			},
		},

		// Branch 8: Non-handoff path — normal reply, no ticket created
		{
			name: "non_handoff_path_normal_reply",
			setup: func(t *testing.T) *Service {
				intentSvc := &mockIntentService{real: intentservice.NewService()}
				hSvc := &mockHandoffService{real: handoff.NewService()}
				svc := NewService(
					memory.NewSessionStore(),
					memory.NewAuditStore(),
					memory.NewTicketStore(),
					memory.NewDedupStore(),
					intentSvc,
					reply.NewService(memory.NewKnowledgeStore()),
					hSvc,
				)
				svc.now = func() time.Time { return fixedTime }
				return svc
			},
			msg: &message.UnifiedMessage{
				MessageID: "m1",
				Channel:   "widget",
				OpenID:    "u1",
				Content:   "今天天气怎么样", // no handoff trigger
			},
			wantErr: "",
			assertions: func(t *testing.T, result *Result) {
				if result.Handoff.ShouldHandoff {
					t.Error("expected no handoff for normal query")
				}
				if result.TicketID != "" {
					t.Errorf("expected no ticket ID, got %q", result.TicketID)
				}
				if result.Reply == "" {
					t.Error("expected non-empty reply from reply service")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tc.setup(t)
			result, err := svc.Process(context.Background(), tc.msg)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Process() expected error containing %q, got nil", tc.wantErr)
				}
				if !contains(err.Error(), tc.wantErr) {
					t.Fatalf("Process() error = %q, want error containing %q", err.Error(), tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Process() unexpected error = %v", err)
			}
			if tc.assertions != nil {
				tc.assertions(t, result)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
