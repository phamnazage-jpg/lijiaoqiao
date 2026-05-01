// Package cserrors defines unified customer-service error codes.
//
// Error codes follow the format CS_{DOMAIN}_{CODE}, e.g. CS_TICKET_4001.
// HTTP status is inferred from the error class (4xx = client error, 5xx = server error).
//
// Alignment: tech/INTERFACE.md §3.3 Error Codes.
package cserrors

// Session errors (CS_SES_xxxx)
const (
	// CS_SES_4001 — session not found.
	CS_SES_4001 = "CS_SES_4001"
	// CS_SES_4002 — message rate limit exceeded.
	CS_SES_4002 = "CS_SES_4002"
	// CS_SES_4003 — identity verification locked.
	CS_SES_4003 = "CS_SES_4003"
)

// Identity errors (CS_IDT_xxxx)
const (
	// CS_IDT_4001 — identity information mismatch.
	CS_IDT_4001 = "CS_IDT_4001"
	// CS_IDT_4002 — verification code incorrect.
	CS_IDT_4002 = "CS_IDT_4002"
)

// Ticket errors (CS_TKT_xxxx or CS_TICKET_xxxx)
const (
	// CS_TICKET_4001 — ticket not found.
	CS_TICKET_4001 = "CS_TICKET_4001"
	// CS_TICKET_4002 — ticket already assigned.
	CS_TICKET_4002 = "CS_TICKET_4002"
)

// Knowledge-base errors (CS_KB_xxxx)
const (
	// CS_KB_4001 — knowledge-base entry not found.
	CS_KB_4001 = "CS_KB_4001"
	// CS_KB_4002 — entry name already exists.
	CS_KB_4002 = "CS_KB_4002"
)

// LLM errors (CS_LLM_xxxx)
const (
	// CS_LLM_5001 — LLM service unavailable.
	CS_LLM_5001 = "CS_LLM_5001"
	// CS_LLM_5002 — LLM request timeout.
	CS_LLM_5002 = "CS_LLM_5002"
)

// Auth errors (CS_AUTH_xxxx)
const (
	// CS_AUTH_4001 — access denied (privilege escalation attempt).
	CS_AUTH_4001 = "CS_AUTH_4001"
	// CS_AUTH_4031 — webhook signature missing.
	CS_AUTH_4031 = "CS_AUTH_4031"
	// CS_AUTH_4032 — webhook timestamp invalid.
	CS_AUTH_4032 = "CS_AUTH_4032"
	// CS_AUTH_4033 — webhook request stale (timestamp skew).
	CS_AUTH_4033 = "CS_AUTH_4033"
	// CS_AUTH_4034 — webhook signature mismatch.
	CS_AUTH_4034 = "CS_AUTH_4034"
)

// HTTP/Request errors (CS_HTTP_xxxx, CS_REQ_xxxx)
const (
	// CS_HTTP_405 — method not allowed.
	CS_HTTP_405 = "CS_HTTP_405"
	// CS_REQ_4001 — invalid JSON body.
	CS_REQ_4001 = "CS_REQ_4001"
	// CS_REQ_4131 — request body too large.
	CS_REQ_4131 = "CS_REQ_4131"
	// CS_REQ_4002 — missing required fields.
	CS_REQ_4002 = "CS_REQ_4002"
	// CS_REQ_4003 — content exceeds maximum length.
	CS_REQ_4003 = "CS_REQ_4003"
	// CS_REQ_4004 — unable to read request body.
	CS_REQ_4004 = "CS_REQ_4004"
	// CS_REQ_4008 — channel is required (webhook path).
	CS_REQ_4008 = "CS_REQ_4008"
	// CS_REQ_4005 — ticket_id and agent_id required.
	CS_REQ_4005 = "CS_REQ_4005"
	// CS_REQ_4006 — ticket_id and resolution required.
	CS_REQ_4006 = "CS_REQ_4006"
	// CS_REQ_4007 — ticket_id and resolution required (close).
	CS_REQ_4007 = "CS_REQ_4007"
	// CS_REQ_4009 — feedback score out of valid range.
	CS_REQ_4009 = "CS_REQ_4009"
	// CS_REQ_4010 — handoff reason is required.
	CS_REQ_4010 = "CS_REQ_4010"
)

// System errors (CS_SYS_xxxx)
const (
	// CS_SYS_5001 — internal server error (webhook process).
	CS_SYS_5001 = "CS_SYS_5001"
	// CS_SYS_5002 — internal server error (list tickets).
	CS_SYS_5002 = "CS_SYS_5002"
)

// Ticket workflow errors (CS_TICKET_xxxx, 409x range for conflict)
const (
	// CS_TKT_4002 — ticket already assigned (409 Conflict).
	// DEPRECATED alias: CS_TICKET_4091 kept for backward compatibility.
	CS_TKT_4002 = "CS_TKT_4002"
	// CS_TKT_4003 — ticket not found (404).
	CS_TKT_4003 = "CS_TKT_4003"
	// CS_TICKET_4091 — DEPRECATED: alias for CS_TKT_4002. Use CS_TKT_4002 for new code.
	CS_TICKET_4091 = CS_TKT_4002
	// CS_TICKET_4092 — ticket state conflict on resolve.
	CS_TICKET_4092 = "CS_TICKET_4092"
	// CS_TICKET_4093 — ticket state conflict on close.
	CS_TICKET_4093 = "CS_TICKET_4093"
)

// ErrorMsg returns the human-readable message for a code.
func ErrorMsg(code string) string {
	switch code {
	// Session
	case CS_SES_4001:
		return "session not found"
	case CS_SES_4002:
		return "message rate limit exceeded"
	case CS_SES_4003:
		return "identity verification locked"
	// Identity
	case CS_IDT_4001:
		return "identity information mismatch"
	case CS_IDT_4002:
		return "verification code incorrect"
	// Ticket
	case CS_TICKET_4001:
		return "ticket not found"
	case CS_TICKET_4002:
		return "ticket already assigned"
	case CS_TKT_4002:
		return "ticket already assigned"
	case CS_TICKET_4092:
		return "ticket resolve conflict"
	case CS_TICKET_4093:
		return "ticket close conflict"
	case CS_TKT_4003:
		return "ticket not found"
	// Knowledge-base
	case CS_KB_4001:
		return "knowledge-base entry not found"
	case CS_KB_4002:
		return "entry name already exists"
	// LLM
	case CS_LLM_5001:
		return "LLM service unavailable"
	case CS_LLM_5002:
		return "LLM request timeout"
	// Auth
	case CS_AUTH_4001:
		return "access denied"
	case CS_AUTH_4031:
		return "missing webhook signature"
	case CS_AUTH_4032:
		return "invalid webhook timestamp"
	case CS_AUTH_4033:
		return "stale webhook request"
	case CS_AUTH_4034:
		return "invalid webhook signature"
	// HTTP/Request
	case CS_HTTP_405:
		return "method not allowed"
	case CS_REQ_4001:
		return "invalid JSON"
	case CS_REQ_4131:
		return "request body too large"
	case CS_REQ_4002:
		return "channel, open_id and content are required"
	case CS_REQ_4003:
		return "content exceeds maximum length"
	case CS_REQ_4004:
		return "unable to read request body"
	case CS_REQ_4008:
		return "channel is required"
	case CS_REQ_4005:
		return "ticket_id and agent_id are required"
	case CS_REQ_4006:
		return "ticket_id and resolution are required"
	case CS_REQ_4007:
		return "ticket_id and resolution are required"
	case CS_REQ_4009:
		return "feedback score must be between 1 and 5"
	case CS_REQ_4010:
		return "handoff reason is required"
	// System
	case CS_SYS_5001:
		return "internal server error"
	case CS_SYS_5002:
		return "list tickets failed"
	default:
		return code
	}
}
