package health

import (
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
