package integration

import (
	"context"
	"testing"

	"github.com/bridge/ai-customer-service/internal/domain/message"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
	"github.com/bridge/ai-customer-service/internal/service/handoff"
	intentservice "github.com/bridge/ai-customer-service/internal/service/intent"
	"github.com/bridge/ai-customer-service/internal/service/reply"
	"github.com/bridge/ai-customer-service/internal/store/memory"
)

// TestDialogService_AC02_IntentMatrix covers the AC-02 intent recognition test matrix:
// - 退款意图 → P1 handoff
// - 数据泄露意图 → P1 handoff
// - 人工意图 → handoff
// - 正常查询 → bot 回复（无 handoff）
func TestDialogService_AC02_IntentMatrix(t *testing.T) {
	sessions := memory.NewSessionStore()
	audits := memory.NewAuditStore()
	tickets := memory.NewTicketStore()
	dedup := memory.NewDedupStore()
	knowledge := memory.NewKnowledgeStore()
	svc := dialog.NewService(sessions, audits, tickets, dedup, intentservice.NewService(), reply.NewService(knowledge), handoff.NewService())

	tests := []struct {
		name          string
		content       string
		wantIntent    string
		wantHandoff   bool
		wantPriority  string // empty if no handoff expected
		wantReply     bool   // whether to check reply is non-empty
	}{
		{
			name:         "AC-02: 退款意图 → P1 handoff",
			content:      "我要申请退款",
			wantIntent:   "refund",
			wantHandoff:  true,
			wantPriority: "P1",
			wantReply:    true,
		},
		{
			name:         "AC-02: 数据泄露意图 → P1 handoff",
			content:      "我的账户数据泄露了",
			wantIntent:   "security",
			wantHandoff:  true,
			wantPriority: "P1",
			wantReply:    true,
		},
		{
			name:         "AC-02: 人工意图 → handoff",
			content:      "转人工客服",
			wantIntent:   "handoff",
			wantHandoff:  true,
			wantPriority: "P1", // NeedsHuman=true → P1
			wantReply:    true,
		},
		{
			name:         "AC-02: 正常查询 → bot 回复无 handoff",
			content:      "查询额度",
			wantIntent:   "quota",
			wantHandoff:  false,
			wantReply:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.Process(context.Background(), &message.UnifiedMessage{
				MessageID: "m_" + tc.name,
				Channel:  "widget",
				OpenID:   "u_" + tc.name,
				Content:  tc.content,
			})
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			// Verify intent recognition
			if result.Intent.Intent != tc.wantIntent {
				t.Fatalf("intent = %s, want %s", result.Intent.Intent, tc.wantIntent)
			}

			// Verify handoff decision
			if result.Handoff.ShouldHandoff != tc.wantHandoff {
				t.Fatalf("handoff.ShouldHandoff = %v, want %v", result.Handoff.ShouldHandoff, tc.wantHandoff)
			}

			// Verify priority for handoff cases
			if tc.wantHandoff {
				if result.Handoff.Priority != tc.wantPriority {
					t.Fatalf("handoff.Priority = %s, want %s", result.Handoff.Priority, tc.wantPriority)
				}
				// ticket must be created
				if result.TicketID == "" {
					t.Fatalf("TicketID empty, want non-empty for handoff case")
				}
				// Verify ticket was actually stored
				stored := tickets.List()
				found := false
				for _, tk := range stored {
					if tk.ID == result.TicketID {
						found = true
						if string(tk.Priority) != tc.wantPriority {
							t.Fatalf("stored ticket priority = %s, want %s", tk.Priority, tc.wantPriority)
						}
						if tk.SessionID == "" {
							t.Fatalf("stored ticket session_id is empty")
						}
						break
					}
				}
				if !found {
					t.Fatalf("ticket %s not found in store", result.TicketID)
				}
			} else {
				// No handoff: ticket must NOT be created
				if result.TicketID != "" {
					t.Fatalf("TicketID = %s, want empty for non-handoff case", result.TicketID)
				}
			}

			// Verify reply
			if tc.wantReply && result.Reply == "" {
				t.Fatalf("Reply empty, want non-empty reply")
			}
		})
	}
}

func TestDialogService_Process(t *testing.T) {
	sessions := memory.NewSessionStore()
	audits := memory.NewAuditStore()
	tickets := memory.NewTicketStore()
	dedup := memory.NewDedupStore()
	knowledge := memory.NewKnowledgeStore()
	svc := dialog.NewService(sessions, audits, tickets, dedup, intentservice.NewService(), reply.NewService(knowledge), handoff.NewService())

	result, err := svc.Process(context.Background(), &message.UnifiedMessage{MessageID: "m1", Channel: "widget", OpenID: "u1", Content: "查询额度"})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result.Intent.Intent != "quota" {
		t.Fatalf("intent = %s, want quota", result.Intent.Intent)
	}
	if result.Handoff.ShouldHandoff {
		t.Fatalf("expected no handoff")
	}
	if len(audits.List()) != 1 {
		t.Fatalf("audit events = %d, want 1", len(audits.List()))
	}
}
