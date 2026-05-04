package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireRoles_RejectsWhenHeadersMissing(t *testing.T) {
	called := false
	handler := RequireRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}), "admin")

	req := httptest.NewRequest(http.MethodPost, "/admin", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if called {
		t.Fatal("expected wrapped handler not to be called")
	}
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.Code)
	}
}

func TestRequireRoles_RejectsWhenRoleNotAllowed(t *testing.T) {
	called := false
	handler := RequireRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}), "admin", "supervisor")

	req := httptest.NewRequest(http.MethodPost, "/admin", nil)
	req.Header.Set(HeaderActorID, "agent-1")
	req.Header.Set(HeaderActorRole, "agent")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if called {
		t.Fatal("expected wrapped handler not to be called")
	}
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.Code)
	}
}

func TestRequireRoles_AllowsAndInjectsActor(t *testing.T) {
	handler := RequireRoles(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := ActorFromContext(r.Context())
		if !ok {
			t.Fatal("expected actor in context")
		}
		if actor.ID != "admin-1" {
			t.Fatalf("actor id = %s, want admin-1", actor.ID)
		}
		if actor.Role != "admin" {
			t.Fatalf("actor role = %s, want admin", actor.Role)
		}
		w.WriteHeader(http.StatusOK)
	}), "admin")

	req := httptest.NewRequest(http.MethodPost, "/admin", nil)
	req.Header.Set(HeaderActorID, "admin-1")
	req.Header.Set(HeaderActorRole, "ADMIN")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
}
