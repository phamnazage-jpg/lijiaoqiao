package middleware

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRemoteTokenRuntime_VerifyAndResolve(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/v1/platform/tokens/introspect" {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     make(http.Header),
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"request_id":"req-1",
					"data":{
						"token_id":"tok-1",
						"subject_id":"user-1",
						"role":"org_admin",
						"status":"active",
						"scope":["gateway:invoke"],
						"issued_at":"2026-04-10T10:00:00Z",
						"expires_at":"2026-04-10T11:00:00Z"
					}
				}`)),
				Header: make(http.Header),
			}, nil
		}),
	}

	runtime := NewRemoteTokenRuntime("http://token-runtime.internal", httpClient, func() time.Time {
		return time.Date(2026, 4, 10, 10, 30, 0, 0, time.UTC)
	})

	claims, err := runtime.Verify(context.Background(), "raw-token")
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if claims.TokenID != "tok-1" {
		t.Fatalf("expected token id tok-1, got %s", claims.TokenID)
	}

	status, err := runtime.Resolve(context.Background(), claims.TokenID)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if status != TokenStatusActive {
		t.Fatalf("expected active status, got %s", status)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
