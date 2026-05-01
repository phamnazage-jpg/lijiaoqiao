package health

import (
	"context"
	"errors"
	"testing"
)

func TestProbe_IsReady_DefaultsToFalse(t *testing.T) {
	// NewProbe sets ready to false by default
	probe := NewProbe()
	if got := probe.IsReady(); got != false {
		t.Errorf("IsReady() on new probe = %v, want false", got)
	}
}

func TestProbe_IsLive_DefaultsToTrue(t *testing.T) {
	// NewProbe sets live to true by default
	probe := NewProbe()
	if got := probe.IsLive(); got != true {
		t.Errorf("IsLive() on new probe = %v, want true", got)
	}
}

func TestProbe_SetLive_IsLive(t *testing.T) {
	tests := []struct {
		name     string
		setValue bool
		want     bool
	}{
		{"SetLive(false) returns false", false, false},
		{"SetLive(true) returns true", true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			probe := NewProbe()
			probe.SetLive(tc.setValue)
			if got := probe.IsLive(); got != tc.want {
				t.Errorf("IsLive() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProbe_SetReady_IsReady(t *testing.T) {
	tests := []struct {
		name     string
		setValue bool
		want     bool
	}{
		{"SetReady(false) returns false", false, false},
		{"SetReady(true) returns true", true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			probe := NewProbe()
			probe.SetReady(tc.setValue)
			if got := probe.IsReady(); got != tc.want {
				t.Errorf("IsReady() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEvaluate_NoCheckers_ReturnsTrue(t *testing.T) {
	ctx := context.Background()
	healthy, results := Evaluate(ctx, nil)
	if !healthy {
		t.Errorf("Evaluate(nil) healthy = %v, want true", healthy)
	}
	if results != nil {
		t.Errorf("Evaluate(nil) results = %v, want nil", results)
	}
}

func TestEvaluate_EmptyCheckers_ReturnsTrue(t *testing.T) {
	ctx := context.Background()
	healthy, results := Evaluate(ctx, []Checker{})
	if !healthy {
		t.Errorf("Evaluate([]) healthy = %v, want true", healthy)
	}
	if results != nil {
		t.Errorf("Evaluate([]) results = %v, want nil", results)
	}
}

func TestEvaluate_AllCheckersPass_ReturnsTrue(t *testing.T) {
	ctx := context.Background()
	checkers := []Checker{
		stubChecker{name: "db", err: nil},
		stubChecker{name: "redis", err: nil},
	}
	healthy, results := Evaluate(ctx, checkers)
	if !healthy {
		t.Errorf("Evaluate() healthy = %v, want true", healthy)
	}
	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2", len(results))
	}
	for _, r := range results {
		if r.Status != "UP" {
			t.Errorf("result %s status = %s, want UP", r.Name, r.Status)
		}
	}
}

func TestEvaluate_SomeCheckersFail_ReturnsFalse(t *testing.T) {
	ctx := context.Background()
	checkers := []Checker{
		stubChecker{name: "db", err: nil},
		stubChecker{name: "redis", err: errors.New("connection refused")},
	}
	healthy, results := Evaluate(ctx, checkers)
	if healthy {
		t.Errorf("Evaluate() healthy = %v, want false", healthy)
	}
	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2", len(results))
	}
	for _, r := range results {
		if r.Name == "redis" && r.Status != "DOWN" {
			t.Errorf("redis result status = %s, want DOWN", r.Status)
		}
		if r.Name == "db" && r.Status != "UP" {
			t.Errorf("db result status = %s, want UP", r.Status)
		}
	}
}

func TestEvaluate_NilChecker_Skipped(t *testing.T) {
	ctx := context.Background()
	checkers := []Checker{
		stubChecker{name: "db", err: nil},
		nil,
		stubChecker{name: "cache", err: nil},
	}
	healthy, results := Evaluate(ctx, checkers)
	if !healthy {
		t.Errorf("Evaluate() healthy = %v, want true (nil skipped)", healthy)
	}
	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2 (nil skipped)", len(results))
	}
}

func TestEvaluate_AllCheckersFail_ReturnsFalse(t *testing.T) {
	ctx := context.Background()
	checkers := []Checker{
		stubChecker{name: "db", err: errors.New("db down")},
		stubChecker{name: "redis", err: errors.New("redis down")},
	}
	healthy, results := Evaluate(ctx, checkers)
	if healthy {
		t.Errorf("Evaluate() healthy = %v, want false", healthy)
	}
	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2", len(results))
	}
	for _, r := range results {
		if r.Status != "DOWN" {
			t.Errorf("result %s status = %s, want DOWN", r.Name, r.Status)
		}
	}
}

// stubChecker is a test double for Checker interface.
type stubChecker struct {
	name string
	err  error
}

func (s stubChecker) Name() string { return s.name }

func (s stubChecker) Check(_ context.Context) error { return s.err }
