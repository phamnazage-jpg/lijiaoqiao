package ticketstats

// Stats represents aggregated ticket statistics for monitoring dashboards.
type Stats struct {
	Total                    int              `json:"total_tickets"`
	Open                     int              `json:"open"`
	Resolved                 int              `json:"resolved"`
	Closed                   int              `json:"closed"`
	ByChannel                map[string]int   `json:"by_channel"`
	ByPriority               map[string]int   `json:"by_priority"`
	HandoffCount             int              `json:"handoff_count"`
	AvgResolutionTimeMinutes float64          `json:"avg_resolution_time_minutes"`
}
