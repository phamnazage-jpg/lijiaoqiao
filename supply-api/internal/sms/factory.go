package sms

import (
	"fmt"
)

// NewSMSService creates an SMS service based on the configuration.
func NewSMSService(config *Config) (SMSService, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if !config.Enabled {
		return NewMockSMSService(config), nil
	}

	switch config.Provider {
	case ProviderTencent:
		return NewTencentSMSService(config), nil
	case ProviderAliyun:
		svc, err := NewAliyunSMSService(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create Aliyun SMS service: %w", err)
		}
		return svc, nil
	default:
		return nil, fmt.Errorf("unsupported SMS provider: %s", config.Provider)
	}
}

// NewSMSServiceWithCodeStore creates an SMS service with a shared code store.
// This is useful when you want to send and verify codes through different channels.
func NewSMSServiceWithCodeStore(config *Config, store *InMemoryCodeStore) (SMSService, *InMemoryCodeStore, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if !config.Enabled {
		mockSvc := &MockSMSService{
			store:  store,
			config: config,
		}
		return mockSvc, store, nil
	}

	switch config.Provider {
	case ProviderTencent:
		return NewTencentSMSService(config), store, nil
	case ProviderAliyun:
		svc, err := NewAliyunSMSService(config)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create Aliyun SMS service: %w", err)
		}
		return svc, store, nil
	default:
		return nil, nil, fmt.Errorf("unsupported SMS provider: %s", config.Provider)
	}
}
