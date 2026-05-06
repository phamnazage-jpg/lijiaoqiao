package platformdelivery

import (
	"testing"
	"time"
)

func TestSigner_ShouldProduceStableTimestampAndSignatureHeaders(t *testing.T) {
	signer := Signer{
		Secret:          "callback-secret",
		TimestampHeader: DefaultTimestampHeader,
		SignatureHeader: DefaultSignatureHeader,
	}
	body := []byte(`{"event_id":"evt-1"}`)
	now := time.Unix(1_777_777_777, 0).UTC()

	headers, err := signer.Headers(body, now)
	if err != nil {
		t.Fatalf("Headers() error = %v", err)
	}
	if headers.Get(DefaultTimestampHeader) != "1777777777" {
		t.Fatalf("timestamp header = %s, want 1777777777", headers.Get(DefaultTimestampHeader))
	}
	expectedSignature := computeSignature("callback-secret", "1777777777", body)
	if headers.Get(DefaultSignatureHeader) != expectedSignature {
		t.Fatalf("signature header = %s, want %s", headers.Get(DefaultSignatureHeader), expectedSignature)
	}
}
