package intent

import "testing"

func TestResult_Fields(t *testing.T) {
	r := Result{
		Intent:     IntentQuota,
		Confidence: 0.95,
		Entities:   map[string]string{"key": "value"},
		NeedsHuman: false,
		Sensitive:  false,
	}
	if r.Intent != IntentQuota {
		t.Errorf("Intent = %q, want %q", r.Intent, IntentQuota)
	}
	if r.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", r.Confidence)
	}
	if r.NeedsHuman {
		t.Error("NeedsHuman = true, want false")
	}
}

func TestResult_NeedsHuman(t *testing.T) {
	r := Result{NeedsHuman: true}
	if !r.NeedsHuman {
		t.Error("NeedsHuman = false, want true")
	}
}

func TestResult_Sensitive(t *testing.T) {
	r := Result{Sensitive: true}
	if !r.Sensitive {
		t.Error("Sensitive = false, want true")
	}
}

func TestResult_EntitiesMap(t *testing.T) {
	r := Result{
		Intent:   IntentGeneral,
		Entities: map[string]string{"user": "alice", "action": "refund"},
	}
	if len(r.Entities) != 2 {
		t.Errorf("len(Entities) = %d, want 2", len(r.Entities))
	}
	if r.Entities["user"] != "alice" {
		t.Errorf("Entities[user] = %q, want %q", r.Entities["user"], "alice")
	}
}

func TestIntentConstants(t *testing.T) {
	intents := []string{IntentQuota, IntentToken, IntentError, IntentHandoff, IntentGeneral, IntentRefund, IntentSecurity}
	for _, intent := range intents {
		if intent == "" {
			t.Errorf("intent constant is empty string")
		}
	}
}

func TestIntentQuota(t *testing.T) {
	if IntentQuota != "quota" {
		t.Errorf("IntentQuota = %q, want %q", IntentQuota, "quota")
	}
}

func TestIntentHandoff(t *testing.T) {
	if IntentHandoff != "handoff" {
		t.Errorf("IntentHandoff = %q, want %q", IntentHandoff, "handoff")
	}
}
