package app

import (
	"testing"

	"lijiaoqiao/gateway/internal/config"
)

func TestBuildProvider_OpenAI(t *testing.T) {
	provider, err := buildProvider(config.ProviderConfig{
		Name:    "openai",
		Type:    "openai",
		BaseURL: "https://api.openai.com",
		APIKey:  "secret",
		Models:  []string{"gpt-4o"},
	})
	if err != nil {
		t.Fatalf("buildProvider returned error: %v", err)
	}
	if provider.ProviderName() != "openai" {
		t.Fatalf("unexpected provider name: %s", provider.ProviderName())
	}
}

func TestBuildProvider_RejectsUnsupportedType(t *testing.T) {
	_, err := buildProvider(config.ProviderConfig{
		Name: "anthropic",
		Type: "anthropic",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
