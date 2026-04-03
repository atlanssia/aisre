package contract

// RCAReport is the core output of the analysis engine.
type RCAReport struct {
	Summary         string   `json:"summary"`
	RootCause       string   `json:"root_cause"`
	Confidence      float64  `json:"confidence"`
	EvidenceIDs     []string `json:"evidence_ids"`
	Recommendations []string `json:"recommendations"`
}

// ReportResponse is the DTO for the report API response.
type ReportResponse struct {
	ID              int64          `json:"id"`
	IncidentID      int64          `json:"incident_id"`
	Summary         string         `json:"summary"`
	RootCause       string         `json:"root_cause"`
	Confidence      float64        `json:"confidence"`
	Evidence        []EvidenceItem `json:"evidence"`
	Recommendations []string       `json:"recommendations"`
	Timeline        []TimelineEvent `json:"timeline"`
	CreatedAt       string         `json:"created_at"`
}

// TimelineEvent represents a single event in the fault timeline.
type TimelineEvent struct {
	Time        string `json:"time"`
	Type        string `json:"type"`        // "symptom", "error", "deploy", "alert", "action"
	Service     string `json:"service"`
	Description string `json:"description"`
	Severity    string `json:"severity,omitempty"`
}

// EvidenceItem represents a single piece of evidence.
type EvidenceItem struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"` // trace, log, metric, change
	Score     float64        `json:"score"`
	Summary   string         `json:"summary"`
	SourceURL string         `json:"source_url,omitempty"`
	Payload   map[string]any `json:"payload"`
}

// ReportFilter is the DTO for listing/filtering reports.
type ReportFilter struct {
	Service   string `json:"service,omitempty"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
	Severity  string `json:"severity,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}
