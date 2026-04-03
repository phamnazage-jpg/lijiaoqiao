package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddleware_PreflightRequest(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowOrigins = []string{"https://example.com"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := CORSMiddleware(config)(handler)

	// 模拟OPTIONS预检请求
	req := httptest.NewRequest("OPTIONS", "/v1/chat/completions", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")

	w := httptest.NewRecorder()
	corsHandler.ServeHTTP(w, req)

	// 预检请求应返回204 No Content
	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for preflight, got %d", w.Code)
	}

	// 检查CORS响应头
	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected Access-Control-Allow-Origin to be 'https://example.com', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}

	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods to be set")
	}
}

func TestCORSMiddleware_ActualRequest(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowOrigins = []string{"https://example.com"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := CORSMiddleware(config)(handler)

	// 模拟实际请求
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Origin", "https://example.com")

	w := httptest.NewRecorder()
	corsHandler.ServeHTTP(w, req)

	// 正常请求应通过到handler
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 检查CORS响应头
	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected Access-Control-Allow-Origin to be 'https://example.com', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddleware_DisallowedOrigin(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowOrigins = []string{"https://allowed.com"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := CORSMiddleware(config)(handler)

	// 模拟来自未允许域名的请求
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Origin", "https://malicious.com")

	w := httptest.NewRecorder()
	corsHandler.ServeHTTP(w, req)

	// 预检请求应返回403 Forbidden
	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for disallowed origin, got %d", w.Code)
	}
}

func TestCORSMiddleware_WildcardOrigin(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowOrigins = []string{"*"} // 允许所有来源

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := CORSMiddleware(config)(handler)

	// 模拟请求
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Origin", "https://any-domain.com")

	w := httptest.NewRecorder()
	corsHandler.ServeHTTP(w, req)

	// 应该允许
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCORSMiddleware_SubdomainWildcard(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowOrigins = []string{"*.example.com"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := CORSMiddleware(config)(handler)

	// 测试子域名
	tests := []struct {
		origin      string
		shouldAllow bool
	}{
		{"https://app.example.com", true},
		{"https://api.example.com", true},
		{"https://example.com", true},
		{"https://malicious.com", false},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		req.Header.Set("Origin", tt.origin)

		w := httptest.NewRecorder()
		corsHandler.ServeHTTP(w, req)

		if tt.shouldAllow && w.Code != http.StatusOK {
			t.Errorf("origin %s should be allowed, got status %d", tt.origin, w.Code)
		}
		if !tt.shouldAllow && w.Code != http.StatusForbidden {
			t.Errorf("origin %s should be forbidden, got status %d", tt.origin, w.Code)
		}
	}
}

func TestMED08_CORSConfigurationExists(t *testing.T) {
	// MED-08: 验证CORS配置存在且可用
	config := DefaultCORSConfig()

	// 验证默认配置包含必要的设置
	if len(config.AllowMethods) == 0 {
		t.Error("default CORS config should have AllowMethods")
	}

	if len(config.AllowHeaders) == 0 {
		t.Error("default CORS config should have AllowHeaders")
	}

	// 验证CORS中间件函数存在
	corsMiddleware := CORSMiddleware(config)
	if corsMiddleware == nil {
		t.Error("CORSMiddleware should return a valid middleware function")
	}
}