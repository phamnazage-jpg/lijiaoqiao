package ticketstats

import "testing"

func TestStats_Fields(t *testing.T) {
	stats := Stats{
		Total:                    100,
		Open:                     30,
		Resolved:                 50,
		Closed:                   20,
		ByChannel:                map[string]int{"widget": 60, "web": 40},
		ByPriority:               map[string]int{"P1": 10, "P2": 30, "P3": 60},
		HandoffCount:             5,
		AvgResolutionTimeMinutes: 42.5,
	}
	if stats.Total != 100 {
		t.Errorf("Total = %d, want 100", stats.Total)
	}
	if stats.Open != 30 {
		t.Errorf("Open = %d, want 30", stats.Open)
	}
	if stats.Resolved != 50 {
		t.Errorf("Resolved = %d, want 50", stats.Resolved)
	}
	if stats.Closed != 20 {
		t.Errorf("Closed = %d, want 20", stats.Closed)
	}
	if stats.HandoffCount != 5 {
		t.Errorf("HandoffCount = %d, want 5", stats.HandoffCount)
	}
	if stats.AvgResolutionTimeMinutes != 42.5 {
		t.Errorf("AvgResolutionTimeMinutes = %f, want 42.5", stats.AvgResolutionTimeMinutes)
	}
}

func TestStats_ByChannel(t *testing.T) {
	stats := Stats{ByChannel: map[string]int{"widget": 10, "api": 20}}
	if stats.ByChannel["widget"] != 10 {
		t.Errorf("ByChannel[widget] = %d, want 10", stats.ByChannel["widget"])
	}
	if stats.ByChannel["api"] != 20 {
		t.Errorf("ByChannel[api] = %d, want 20", stats.ByChannel["api"])
	}
}

func TestStats_ByPriority(t *testing.T) {
	stats := Stats{ByPriority: map[string]int{"P1": 5, "P2": 15, "P3": 80}}
	if stats.ByPriority["P1"] != 5 {
		t.Errorf("ByPriority[P1] = %d, want 5", stats.ByPriority["P1"])
	}
}

func TestStats_ZeroValues(t *testing.T) {
	stats := Stats{}
	if stats.Total != 0 {
		t.Errorf("Total = %d, want 0", stats.Total)
	}
	if stats.Open != 0 {
		t.Errorf("Open = %d, want 0", stats.Open)
	}
	if stats.AvgResolutionTimeMinutes != 0 {
		t.Errorf("AvgResolutionTimeMinutes = %f, want 0", stats.AvgResolutionTimeMinutes)
	}
	if stats.ByChannel != nil && len(stats.ByChannel) != 0 {
		t.Errorf("ByChannel = %v, want nil or empty", stats.ByChannel)
	}
}

func TestStats_NilMaps(t *testing.T) {
	stats := Stats{Total: 0}
	// ByChannel and ByPriority may be nil (zero value of map)
	if stats.ByChannel == nil && len(stats.ByChannel) == 0 {
		// nil map is valid
	}
	if stats.ByPriority == nil && len(stats.ByPriority) == 0 {
		// nil map is valid
	}
}
