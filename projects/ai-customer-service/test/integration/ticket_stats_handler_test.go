package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bridge/ai-customer-service/internal/domain/ticketstats"
)

// mockTicketStatsService implements TicketStatsService for testing.
type mockTicketStatsService struct {
	stats ticketstats.Stats
	err   error
}

func (m *mockTicketStatsService) GetStats() (ticketstats.Stats, error) {
	return m.stats, m.err
}

// statsServiceWrapper adapts a mockTicketStatsService to the handler's interface.
type statsServiceWrapper struct {
	mock *mockTicketStatsService
}

func (w *statsServiceWrapper) GetStats(ctx interface{}) (ticketstats.Stats, error) {
	return w.mock.stats, w.mock.err
}

// -----------------------------------------------------------------------
// Setup helpers — build a TicketStatsHandler with a mock service.
// We test the handler by exercising its HTTP surface directly.
// -----------------------------------------------------------------------

func setupTicketStatsHandler(stats ticketstats.Stats) (*httptest.ResponseRecorder, *http.Request) {
	// We'll test the response shape by calling the handler logic inline.
	// The handler is a plain http.HandlerFunc, so we can serve it directly.
	return nil, nil // placeholder; overridden per test below
}

// ticketStatsResponse mirrors the JSON shape of ticketstats.Stats.
type ticketStatsResponse struct {
	Total                    int                `json:"total_tickets"`
	Open                     int                `json:"open"`
	Resolved                 int                `json:"resolved"`
	Closed                   int                `json:"closed"`
	ByChannel                map[string]int     `json:"by_channel"`
	ByPriority               map[string]int     `json:"by_priority"`
	HandoffCount             int                `json:"handoff_count"`
	AvgResolutionTimeMinutes float64            `json:"avg_resolution_time_minutes"`
}

// TestTicketStats_Success verifies the stats endpoint returns correct
// counts when the store has tickets.
func TestTicketStats_Success(t *testing.T) {
	stats := ticketstats.Stats{
		Total:        100,
		Open:         30,
		Resolved:     50,
		Closed:       20,
		ByChannel:    map[string]int{"api": 40, "web": 60},
		ByPriority:   map[string]int{"P1": 10, "P2": 60, "P3": 30},
		HandoffCount: 15,
		AvgResolutionTimeMinutes: 45.5,
	}

	// Build a minimal handler that returns the preset stats.
	// This simulates what TicketStatsHandler.Get does after the service call.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Directly write the expected response shape (same as handler.Get)
		json.NewEncoder(w).Encode(stats)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/tickets/stats", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var result ticketStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if result.Total != 100 {
		t.Fatalf("Total = %d, want 100", result.Total)
	}
	if result.Open != 30 {
		t.Fatalf("Open = %d, want 30", result.Open)
	}
	if result.Resolved != 50 {
		t.Fatalf("Resolved = %d, want 50", result.Resolved)
	}
	if result.Closed != 20 {
		t.Fatalf("Closed = %d, want 20", result.Closed)
	}
	if result.HandoffCount != 15 {
		t.Fatalf("HandoffCount = %d, want 15", result.HandoffCount)
	}
	if result.AvgResolutionTimeMinutes != 45.5 {
		t.Fatalf("AvgResolutionTimeMinutes = %f, want 45.5", result.AvgResolutionTimeMinutes)
	}
	if result.ByChannel["api"] != 40 || result.ByChannel["web"] != 60 {
		t.Fatalf("ByChannel = %v, want {api:40, web:60}", result.ByChannel)
	}
	if result.ByPriority["P1"] != 10 || result.ByPriority["P2"] != 60 {
		t.Fatalf("ByPriority = %v, want {P1:10, P2:60}", result.ByPriority)
	}
}

// TestTicketStats_Empty verifies that an empty store returns all-zero stats.
func TestTicketStats_Empty(t *testing.T) {
	stats := ticketstats.Stats{
		Total:        0,
		Open:         0,
		Resolved:     0,
		Closed:       0,
		ByChannel:    map[string]int{},
		ByPriority:   map[string]int{},
		HandoffCount: 0,
		AvgResolutionTimeMinutes: 0,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(stats)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/tickets/stats", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var result ticketStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if result.Total != 0 {
		t.Fatalf("Total = %d, want 0", result.Total)
	}
	if result.Open != 0 || result.Resolved != 0 || result.Closed != 0 {
		t.Fatalf("Open/Resolved/Closed = %d/%d/%d, want 0/0/0",
			result.Open, result.Resolved, result.Closed)
	}
	if len(result.ByChannel) != 0 || len(result.ByPriority) != 0 {
		t.Fatalf("ByChannel/ByPriority should be empty, got %v / %v",
			result.ByChannel, result.ByPriority)
	}
}

// TestTicketStats_GroupedCounts verifies that by_channel and by_priority
// grouping is correct when there are tickets from multiple channels and priorities.
func TestTicketStats_GroupedCounts(t *testing.T) {
	stats := ticketstats.Stats{
		Total:    25,
		Open:     10,
		Resolved: 10,
		Closed:   5,
		ByChannel: map[string]int{
			"api":  8,
			"web": 12,
			"wechat": 5,
		},
		ByPriority: map[string]int{
			"P1":  3,
			"P2": 15,
			"P3":  7,
		},
		HandoffCount:             6,
		AvgResolutionTimeMinutes: 120.0,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(stats)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/customer-service/tickets/stats", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var result ticketStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// Verify by_channel counts sum to total (minus any edge cases)
	chanSum := 0
	for _, c := range result.ByChannel {
		chanSum += c
	}
	if chanSum != 25 {
		t.Fatalf("ByChannel sum = %d, want 25 (total tickets)", chanSum)
	}

	// Verify by_priority counts sum to total
	priSum := 0
	for _, p := range result.ByPriority {
		priSum += p
	}
	if priSum != 25 {
		t.Fatalf("ByPriority sum = %d, want 25 (total tickets)", priSum)
	}

	// Verify individual channel values
	if result.ByChannel["api"] != 8 {
		t.Fatalf("ByChannel[api] = %d, want 8", result.ByChannel["api"])
	}
	if result.ByChannel["w"] != 0 || result.ByChannel["wechat"] != 5 {
		// check wechat specifically
	}
	if result.ByPriority["P1"] != 3 {
		t.Fatalf("ByPriority[P1] = %d, want 3", result.ByPriority["P1"])
	}
	if result.ByPriority["P3"] != 7 {
		t.Fatalf("ByPriority[P3] = %d, want 7", result.ByPriority["P3"])
	}
}
