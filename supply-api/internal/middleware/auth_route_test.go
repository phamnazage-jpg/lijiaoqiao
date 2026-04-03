package middleware

import (
	"testing"
)

func TestSanitizeRoute(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/api/v1/test", "/api/v1/test"},
		{"/", "/"},
		{"", ""},
		{"/api/../../../etc/passwd", "/sanitized"},
		{"../../etc/passwd", "/sanitized"},
		{"/api/v1/../admin", "/sanitized"},
		{"/api\\v1\\admin", "/sanitized"},
		{"/api/v1" + string(rune(0)) + "/admin", "/sanitized"},
		{"/api/v1\n/admin", "/sanitized"},
		{"/api/v1\r/admin", "/sanitized"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeRoute(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeRoute(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}