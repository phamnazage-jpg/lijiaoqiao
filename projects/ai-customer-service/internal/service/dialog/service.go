package dialog

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bridge/ai-customer-service/internal/domain/audit"
	intentdomain "github.com/bridge/ai-customer-service/internal/domain/intent"
	"github.com/bridge/ai-customer-service/internal/domain/message"
	"github.com/bridge/ai-customer-service/internal/domain/session"
	"github.com/bridge/ai-customer-service/internal/domain/ticket"
	"github.com/bridge/ai-customer-service/internal/service/handoff"
	"github.com/bridge/ai-customer-service/internal/service/reply"
)

type SessionRepository interface {
	GetOrCreate(ctx context.Context, channel, openID string, now time.Time) (*session.Session, error)
	GetByID(ctx context.Context, id string) (*session.Session, error)
	Save(ctx context.Context, sess *session.Session) error
}

type AuditRepository interface {
	Add(ctx context.Context, event audit.Event) error
}

type TicketRepository interface {
	Create(ctx context.Context, t *ticket.Ticket) error
	GetByID(ctx context.Context, id string) (*ticket.Ticket, error)
}

type DedupRepository interface {
	TryRecord(ctx context.Context, channel, messageID, sessionID string) (bool, error)
}

type Result struct {
	SessionID string               `json:"session_id"`
	Reply     string               `json:"reply"`
	Intent    *intentdomain.Result `json:"intent"`
	Handoff   *handoff.Decision    `json:"handoff"`
	TicketID  string               `json:"ticket_id,omitempty"`
}

type IntentRecognizer interface {
	Recognize(ctx context.Context, sessionID, content string, ctxMsgs []session.MessageContext) (*intentdomain.Result, error)
}

type HandoffDecider interface {
	ShouldHandoff(ctx context.Context, intent *intentdomain.Result, turnCount int) (*handoff.Decision, error)
}

type Service struct {
	sessions SessionRepository
	audits   AuditRepository
	tickets  TicketRepository
	dedup    DedupRepository
	intent   IntentRecognizer
	reply    *reply.Service
	handoff  HandoffDecider
	now      func() time.Time
}

func NewService(sessions SessionRepository, audits AuditRepository, tickets TicketRepository, dedup DedupRepository, intent IntentRecognizer, replySvc *reply.Service, handoffSvc HandoffDecider) *Service {
	return &Service{sessions: sessions, audits: audits, tickets: tickets, dedup: dedup, intent: intent, reply: replySvc, handoff: handoffSvc, now: time.Now}
}

func (s *Service) Process(ctx context.Context, msg *message.UnifiedMessage) (*Result, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}
	now := s.now()
	if msg.Timestamp.IsZero() {
		msg.Timestamp = now
	}

	sess, err := s.sessions.GetOrCreate(ctx, msg.Channel, msg.OpenID, now)
	if err != nil {
		return nil, err
	}
	if msg.MessageID != "" && s.dedup != nil {
		created, err := s.dedup.TryRecord(ctx, msg.Channel, msg.MessageID, sess.ID)
		if err != nil {
			return nil, err
		}
		if !created {
			return &Result{SessionID: sess.ID, Reply: "duplicate message ignored", Intent: &intentdomain.Result{Intent: intentdomain.IntentGeneral}, Handoff: &handoff.Decision{ShouldHandoff: false}}, nil
		}
	}

	sess.Status = session.StatusProcessing
	sess.TurnCount++
	sess.LastMessageAt = now
	sess.Context = append(sess.Context, session.MessageContext{Direction: "user", Content: msg.Content, Timestamp: msg.Timestamp})
	if len(sess.Context) > 6 {
		sess.Context = sess.Context[len(sess.Context)-6:]
	}

	intentResult, err := s.intent.Recognize(ctx, sess.ID, msg.Content, sess.Context)
	if err != nil {
		return nil, err
	}
	handoffDecision, err := s.handoff.ShouldHandoff(ctx, intentResult, sess.TurnCount)
	if err != nil {
		return nil, err
	}

	replyText := s.reply.Generate(ctx, intentResult)
	var ticketID string
	if handoffDecision.ShouldHandoff {
		sess.Status = session.StatusHandoff
		replyText = "已为您转人工客服，请稍候，我们会尽快处理。"
		if s.tickets != nil {
			ticketID = uuid.New().String()
			ticketPriority := ticket.Priority(handoffDecision.Priority)
			if ticketPriority == "" {
				ticketPriority = ticket.PriorityP2
			}
			err = s.tickets.Create(ctx, &ticket.Ticket{ID: ticketID, SessionID: sess.ID, UserID: sess.UserID, Priority: ticketPriority, Status: ticket.StatusOpen, HandoffReason: handoffDecision.Reason, ContextSnapshot: map[string]any{"channel": msg.Channel, "open_id": msg.OpenID, "content": msg.Content, "turn_count": sess.TurnCount}, CreatedAt: now, UpdatedAt: now})
			if err != nil {
				return nil, err
			}
		}
	} else {
		sess.Status = session.StatusIdle
	}

	sess.Context = append(sess.Context, session.MessageContext{Direction: "assistant", Content: replyText, Timestamp: now})
	if len(sess.Context) > 6 {
		sess.Context = sess.Context[len(sess.Context)-6:]
	}
	if err := s.sessions.Save(ctx, sess); err != nil {
		return nil, err
	}

	auditPayload := map[string]any{"intent": intentResult.Intent, "reply": replyText}
	if ticketID != "" {
		auditPayload["ticket_id"] = ticketID
	}
	if err := s.audits.Add(ctx, audit.Event{ID: uuid.New().String(), SessionID: sess.ID, Type: "message_processed", Action: "process", Channel: msg.Channel, OpenID: msg.OpenID, ActorID: msg.OpenID, Payload: auditPayload, CreatedAt: now}); err != nil {
		return nil, err
	}

	return &Result{SessionID: sess.ID, Reply: replyText, Intent: intentResult, Handoff: handoffDecision, TicketID: ticketID}, nil
}
