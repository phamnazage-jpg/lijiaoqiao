package platformadapter

import "testing"

func TestRegistry_ShouldResolveSub2APIAdapter(t *testing.T) {
	registry := NewRegistry(NewSub2APIAdapter(), NewNewAPIAdapter())
	adapter, ok := registry.Resolve("sub2api")
	if !ok {
		t.Fatal("expected sub2api adapter to resolve")
	}
	if got := adapter.Platform(); got != "sub2api" {
		t.Fatalf("adapter.Platform() = %q, want sub2api", got)
	}
}

func TestRegistry_ShouldRejectUnknownPlatform(t *testing.T) {
	registry := NewRegistry(NewSub2APIAdapter())
	if _, ok := registry.Resolve("unknown"); ok {
		t.Fatal("expected unknown platform to be rejected")
	}
}
