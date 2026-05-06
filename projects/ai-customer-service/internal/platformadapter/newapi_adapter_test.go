package platformadapter

import (
	"net/http"
	"testing"
	"time"
)

func TestNewAPIAdapter_ShouldBeRegisteredButDisabledByDefault(t *testing.T) {
	registry := NewRegistry(NewNewAPIAdapter())
	adapter, ok := registry.Resolve("newapi")
	if !ok {
		t.Fatal("expected newapi adapter to resolve")
	}
	if adapter.Platform() != "newapi" {
		t.Fatalf("adapter.Platform() = %s, want newapi", adapter.Platform())
	}

	_, _, err := adapter.ParseInbound(nil, nil, IngressContext{
		Platform:   "newapi",
		ReceivedAt: time.Now(),
	})
	reqErr, ok := err.(*RequestError)
	if !ok {
		t.Fatalf("expected RequestError, got %T", err)
	}
	if reqErr.Status != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", reqErr.Status)
	}
	if reqErr.Code != "CS_PLATFORM_5010" {
		t.Fatalf("code = %s, want CS_PLATFORM_5010", reqErr.Code)
	}
}
