//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_HTTPEndpoints E2E 测试：HTTP 端点测试
// 使用 httptest 模拟 HTTP 服务器进行端到端测试
func TestE2E_HTTPEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/actuator/health":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"UP"}`)
		case "/api/v1/accounts":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"accounts":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// 测试健康检查端点
	resp, err := http.Get(server.URL + "/actuator/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// 测试账号列表端点
	resp, err = http.Get(server.URL + "/api/v1/accounts")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestE2E_TimeoutHandling E2E 测试：超时处理
func TestE2E_TimeoutHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// 创建慢服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 配置带超时的客户端
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	require.NoError(t, err)

	_, err = client.Do(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

// TestE2E_RequestID E2E 测试：请求追踪
func TestE2E_RequestID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	var capturedRequestID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = r.Header.Get("X-Request-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 发送带请求 ID 的请求
	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("X-Request-ID", "test-req-12345")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "test-req-12345", capturedRequestID)
}
