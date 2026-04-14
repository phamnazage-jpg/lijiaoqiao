package app

import (
	"fmt"
	"strings"

	"lijiaoqiao/gateway/internal/adapter"
	"lijiaoqiao/gateway/internal/config"
)

func buildProvider(providerCfg config.ProviderConfig) (adapter.ProviderAdapter, error) {
	name := strings.TrimSpace(providerCfg.Name)
	if name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	providerType := strings.ToLower(strings.TrimSpace(providerCfg.Type))
	switch providerType {
	case "", "openai":
		baseURL := strings.TrimSpace(providerCfg.BaseURL)
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		return adapter.NewOpenAIAdapter(baseURL, providerCfg.APIKey, providerCfg.Models), nil
	default:
		return nil, fmt.Errorf("unsupported provider type %q", providerCfg.Type)
	}
}
