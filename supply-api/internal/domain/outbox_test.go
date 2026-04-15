package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOutboxRetryConfig(t *testing.T) {
	config := DefaultOutboxRetryConfig()

	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
	assert.Equal(t, DefaultInitialBackoffSeconds, config.InitialBackoffSeconds)
	assert.Equal(t, DefaultMaxBackoffSeconds, config.MaxBackoffSeconds)
	assert.Equal(t, DefaultOutboxBatchSize, config.BatchSize)
}

func TestCalculateOutboxBackoff(t *testing.T) {
	tests := []struct {
		name       string
		retryCount int
		expected   int
	}{
		{name: "first retry", retryCount: 1, expected: 1},
		{name: "second retry", retryCount: 2, expected: 2},
		{name: "third retry", retryCount: 3, expected: 4},
		{name: "zero retry coerced to first retry", retryCount: 0, expected: 1},
		{name: "backoff capped", retryCount: 100, expected: 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, CalculateOutboxBackoff(tt.retryCount, DefaultMaxRetries))
		})
	}
}
