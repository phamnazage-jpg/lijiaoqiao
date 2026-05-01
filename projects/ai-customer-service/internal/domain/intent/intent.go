package intent

type Result struct {
	Intent     string            `json:"intent"`
	Confidence float64           `json:"confidence"`
	Entities   map[string]string `json:"entities,omitempty"`
	NeedsHuman bool              `json:"needs_human"`
	Sensitive  bool              `json:"sensitive"`
}

const (
	IntentQuota   = "quota"
	IntentToken   = "token"
	IntentError   = "error"
	IntentHandoff = "handoff"
	IntentGeneral = "general"
	IntentRefund  = "refund"
	IntentSecurity = "security"
)
