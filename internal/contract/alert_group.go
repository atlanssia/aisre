package contract

// AlertGroup represents a deduplicated group of related alerts.
type AlertGroup struct {
	ID         int64             `json:"id"`
	Fingerprint string           `json:"fingerprint"`
	Title      string            `json:"title"`
	Severity   string            `json:"severity"`
	Labels     map[string]string `json:"labels"`
	IncidentID *int64            `json:"incident_id,omitempty"`
	Count      int               `json:"count"`
	FirstSeen  string            `json:"first_seen"`
	LastSeen   string            `json:"last_seen"`
	CreatedAt  string            `json:"created_at"`
}

// IncomingAlert is the API request to ingest an alert.
type IncomingAlert struct {
	Title    string            `json:"title"`
	Severity string            `json:"severity"`
	Labels   map[string]string `json:"labels"`
	Time     string            `json:"time,omitempty"`
}

// AlertGroupFilter holds filter parameters for listing alert groups.
type AlertGroupFilter struct {
	Severity string
	StartTime string
	EndTime   string
	Limit     int
	Offset    int
}

// EscalateResponse is the API response when escalating an alert group to an incident.
type EscalateResponse struct {
	AlertGroupID int64 `json:"alert_group_id"`
	IncidentID   int64 `json:"incident_id"`
}
