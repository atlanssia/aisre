package contract

// CreateIncidentRequest is the DTO for creating a new incident analysis.
type CreateIncidentRequest struct {
	Source    string `json:"source"`
	Service   string `json:"service"`
	Severity  string `json:"severity"`
	TimeRange string `json:"time_range"`
	TraceID   string `json:"trace_id,omitempty"`
}

// CreateIncidentResponse is the DTO returned after creating an incident.
type CreateIncidentResponse struct {
	IncidentID int64  `json:"incident_id"`
	ReportID   int64  `json:"report_id"`
	Status     string `json:"status"`
}

// WebhookPayload is the DTO for receiving alert webhooks.
type WebhookPayload struct {
	Source     string         `json:"source"`
	AlertName  string         `json:"alert_name"`
	Service    string         `json:"service"`
	Severity   string         `json:"severity"`
	TraceID    string         `json:"trace_id,omitempty"`
	Payload    map[string]any `json:"payload"`
}

// Incident represents a stored incident entity.
type Incident struct {
	ID          int64  `json:"id"`
	Source      string `json:"source"`
	ServiceName string `json:"service_name"`
	Severity    string `json:"severity"`
	Status      string `json:"status"`
	TraceID     string `json:"trace_id,omitempty"`
	CreatedAt   string `json:"created_at"`
}
