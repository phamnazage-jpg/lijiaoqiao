package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGetClientIP_HeaderInjection
// TDD: 验证 getClientIP 函数能够处理恶意构造的 X-Forwarded-For 头
func TestGetClientIP_HeaderInjection(t *testing.T) {
	// 可信代理配置
	trustedProxies := []string{"192.168.0.0/16", "10.0.0.0/8"}

	tests := []struct {
		name        string
		headers     map[string]string
		remoteAddr  string
		trusted     []string
		expectedIP  string
	}{
		{
			name:        "IP with port should be cleaned (trusted proxy)",
			headers:     map[string]string{"X-Forwarded-For": "203.0.113.1:8080"},
			remoteAddr:  "192.168.1.1:1234",
			trusted:     trustedProxies,
			expectedIP:  "203.0.113.1",
		},
		{
			name:        "Multiple IPs with spaces (trusted proxy)",
			headers:     map[string]string{"X-Forwarded-For": "  203.0.113.1  ,  198.51.100.1  "},
			remoteAddr:  "192.168.1.1:1234",
			trusted:     trustedProxies,
			expectedIP:  "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			ip := getClientIP(req, tt.trusted...)

			if ip != tt.expectedIP {
				t.Errorf("expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

// TestGetClientIP_PrivateIPInHeaders
// TDD: 验证来自不可信源的私有IP不应该被信任
func TestGetClientIP_PrivateIPInHeaders(t *testing.T) {
	// 当请求来自外部（remoteAddr是公网IP），X-Forwarded-For中的私有IP可能是伪造的
	tests := []struct {
		name        string
		headers     map[string]string
		remoteAddr  string
		trustedCIDR []string
		expectedIP  string
	}{
		{
			name:        "Private IP in X-Forwarded-For from untrusted source",
			headers:     map[string]string{"X-Forwarded-For": "192.168.1.1"},
			remoteAddr:  "203.0.113.1:1234", // 公网IP
			trustedCIDR: nil,                // 未配置可信代理
			expectedIP:  "203.0.113.1",     // 应该拒绝私有IP，使用remoteAddr
		},
		{
			name:        "Loopback in X-Forwarded-For",
			headers:     map[string]string{"X-Forwarded-For": "127.0.0.1"},
			remoteAddr:  "203.0.113.1:1234",
			trustedCIDR: nil,
			expectedIP:  "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			ip := getClientIP(req, tt.trustedCIDR...)

			if ip != tt.expectedIP {
				t.Errorf("expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

// TestGetClientIP_TrustedProxy
// TDD: 配置了可信代理时，应该信任来自该代理的 X-Forwarded-For
func TestGetClientIP_TrustedProxy(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		remoteAddr  string
		trustedCIDR []string
		expectedIP  string
	}{
		{
			name:        "Trusted proxy returns first IP",
			headers:     map[string]string{"X-Forwarded-For": "203.0.113.1, 198.51.100.1"},
			remoteAddr:  "10.0.0.1:1234",
			trustedCIDR: []string{"10.0.0.0/8"},
			expectedIP:  "203.0.113.1",
		},
		{
			name:        "Untrusted source ignores X-Forwarded-For",
			headers:     map[string]string{"X-Forwarded-For": "203.0.113.1"},
			remoteAddr:  "203.0.113.1:1234", // 公网IP
			trustedCIDR: []string{"10.0.0.0/8"}, // 10.0.0.1不在可信范围内
			expectedIP:  "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			ip := getClientIP(req, tt.trustedCIDR...)

			if ip != tt.expectedIP {
				t.Errorf("expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

// TestIsTrustedProxy_WithCIDR [SEC-003]
// 验证 isTrustedProxy 正确识别可信代理
func TestIsTrustedProxy_WithCIDR(t *testing.T) {
	tests := []struct {
		name        string
		remoteAddr  string
		trustedCIDR []string
		expected    bool
	}{
		{
			name:        "Private network is trusted",
			remoteAddr:  "10.0.0.5:1234",
			trustedCIDR: []string{"10.0.0.0/8"},
			expected:    true,
		},
		{
			name:        "Public IP is not trusted",
			remoteAddr:  "203.0.113.1:1234",
			trustedCIDR: []string{"10.0.0.0/8"},
			expected:    false,
		},
		{
			name:        "No trusted proxies configured",
			remoteAddr:  "10.0.0.5:1234",
			trustedCIDR: nil,
			expected:    false,
		},
		{
			name:        "192.168 network is trusted",
			remoteAddr:  "192.168.1.1:1234",
			trustedCIDR: []string{"192.168.0.0/16"},
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTrustedProxy(tt.remoteAddr, tt.trustedCIDR)
			if result != tt.expected {
				t.Errorf("isTrustedProxy(%s, %v) = %v, expected %v", tt.remoteAddr, tt.trustedCIDR, result, tt.expected)
			}
		})
	}
}

// TestIsPublicIP [SEC-003]
// 验证 isPublicIP 正确识别公网IP
func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"10.0.0.1", false},
		{"192.168.1.1", false},
		{"172.16.0.1", false},
		{"127.0.0.1", false},
		{"169.254.0.1", false},
		{"0.0.0.0", false},
		{"203.0.113.1", true},
		{"198.51.100.1", true},
		{"8.8.8.8", true},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := isPublicIP(tt.ip)
			if result != tt.expected {
				t.Errorf("isPublicIP(%s) = %v, expected %v", tt.ip, result, tt.expected)
			}
		})
	}
}

// isPublicIP 检查是否为公网IP
func isPublicIP(ip string) bool {
	// 检查是否为私有IP范围
	if strings.HasPrefix(ip, "10.") {
		return false
	}
	if strings.HasPrefix(ip, "192.168.") {
		return false
	}
	if strings.HasPrefix(ip, "172.") {
		// 172.16.0.0 - 172.31.255.255
		parts := strings.Split(ip, ".")
		if len(parts) >= 2 {
			var secondOctet int
			fmt.Sscanf(parts[1], "%d", &secondOctet)
			if secondOctet >= 16 && secondOctet <= 31 {
				return false
			}
		}
	}
	if strings.HasPrefix(ip, "127.") {
		return false
	}
	if strings.HasPrefix(ip, "169.254.") {
		return false
	}
	if strings.HasPrefix(ip, "0.") {
		return false
	}
	return true
}
