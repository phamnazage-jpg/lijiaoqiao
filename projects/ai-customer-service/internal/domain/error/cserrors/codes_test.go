package cserrors

import (
	"strings"
	"testing"
)

func TestCS_TKT_4002_And_CS_TICKET_4091_Alias(t *testing.T) {
	if CS_TKT_4002 != CS_TICKET_4091 {
		t.Errorf("CS_TKT_4002 (%q) != CS_TICKET_4091 (%q)", CS_TKT_4002, CS_TICKET_4091)
	}
}

func TestErrorMsg_AllCodes(t *testing.T) {
	codes := []string{
		// Session
		CS_SES_4001,
		CS_SES_4002,
		CS_SES_4003,
		// Identity
		CS_IDT_4001,
		CS_IDT_4002,
		// Ticket
		CS_TICKET_4001,
		CS_TICKET_4002,
		CS_TKT_4002,
		CS_TICKET_4091,
		CS_TICKET_4092,
		CS_TICKET_4093,
		// Knowledge-base
		CS_KB_4001,
		CS_KB_4002,
		// LLM
		CS_LLM_5001,
		CS_LLM_5002,
		// Auth
		CS_AUTH_4001,
		CS_AUTH_4031,
		CS_AUTH_4032,
		CS_AUTH_4033,
		CS_AUTH_4034,
		// HTTP/Request
		CS_HTTP_405,
		CS_REQ_4001,
		CS_REQ_4131,
		CS_REQ_4002,
		CS_REQ_4003,
		CS_REQ_4004,
		CS_REQ_4008,
		CS_REQ_4005,
		CS_REQ_4006,
		CS_REQ_4007,
		CS_REQ_4009,
		CS_REQ_4010,
		// System
		CS_SYS_5001,
		CS_SYS_5002,
	}

	for _, code := range codes {
		msg := ErrorMsg(code)
		if strings.TrimSpace(msg) == "" {
			t.Errorf("ErrorMsg(%q) returned empty string", code)
		}
		// For known codes (not default), message should be different from code
		if msg == code && strings.HasPrefix(code, "CS_") {
			t.Logf("Warning: ErrorMsg(%q) returned same value as code (default case?)", code)
		}
	}
}

func TestErrorMsg_UnknownCode(t *testing.T) {
	msg := ErrorMsg("CS_UNKNOWN_9999")
	// Default case returns the code itself
	if msg != "CS_UNKNOWN_9999" {
		t.Errorf("ErrorMsg for unknown code: expected %q, got %q", "CS_UNKNOWN_9999", msg)
	}
}

func TestErrorMsg_SpecificCodes(t *testing.T) {
	tests := []struct {
		code         string
		expectedMsg  string
	}{
		{CS_SES_4001, "session not found"},
		{CS_SES_4002, "message rate limit exceeded"},
		{CS_TICKET_4002, "ticket already assigned"},
		{CS_TKT_4002, "ticket already assigned"}, // same as CS_TICKET_4002
		{CS_KB_4001, "knowledge-base entry not found"},
		{CS_LLM_5001, "LLM service unavailable"},
		{CS_AUTH_4034, "invalid webhook signature"},
	}

	for _, tt := range tests {
		msg := ErrorMsg(tt.code)
		if msg != tt.expectedMsg {
			t.Errorf("ErrorMsg(%q): expected %q, got %q", tt.code, tt.expectedMsg, msg)
		}
	}
}

func TestErrorMsg_AllKnownCodesReturnNonEmpty(t *testing.T) {
	// Verify all codes defined in the switch have non-empty messages
	knownCodes := map[string]string{
		CS_SES_4001:   "session not found",
		CS_SES_4002:   "message rate limit exceeded",
		CS_SES_4003:   "identity verification locked",
		CS_IDT_4001:   "identity information mismatch",
		CS_IDT_4002:   "verification code incorrect",
		CS_TICKET_4001: "ticket not found",
		CS_TICKET_4002: "ticket already assigned",
		CS_TICKET_4092: "ticket resolve conflict",
		CS_TICKET_4093: "ticket close conflict",
		CS_KB_4001:    "knowledge-base entry not found",
		CS_KB_4002:    "entry name already exists",
		CS_LLM_5001:   "LLM service unavailable",
		CS_LLM_5002:   "LLM request timeout",
		CS_AUTH_4001:  "access denied",
		CS_AUTH_4031:  "missing webhook signature",
		CS_AUTH_4032:  "invalid webhook timestamp",
		CS_AUTH_4033:  "stale webhook request",
		CS_AUTH_4034:  "invalid webhook signature",
		CS_HTTP_405:   "method not allowed",
		CS_REQ_4001:   "invalid JSON",
		CS_REQ_4131:   "request body too large",
		CS_REQ_4002:   "channel, open_id and content are required",
		CS_REQ_4003:   "content exceeds maximum length",
		CS_REQ_4004:   "unable to read request body",
		CS_REQ_4008:   "channel is required",
		CS_REQ_4005:   "ticket_id and agent_id are required",
		CS_REQ_4006:   "ticket_id and resolution are required",
		CS_REQ_4007:   "ticket_id and resolution are required",
		CS_REQ_4009:   "feedback score must be between 1 and 5",
		CS_REQ_4010:   "handoff reason is required",
		CS_SYS_5001:   "internal server error",
		CS_SYS_5002:   "list tickets failed",
	}

	for code, expectedMsg := range knownCodes {
		msg := ErrorMsg(code)
		if msg != expectedMsg {
			t.Errorf("ErrorMsg(%q): expected %q, got %q", code, expectedMsg, msg)
		}
	}
}