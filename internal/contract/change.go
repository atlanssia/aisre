package contract

// ChangeEvent represents a deployment, config change, or infrastructure modification.
type ChangeEvent struct {
	ID         int64          `json:"id"`
	Service    string         `json:"service"`
	ChangeType string         `json:"change_type"`
	Summary    string         `json:"summary"`
	Author     string         `json:"author,omitempty"`
	Timestamp  string         `json:"timestamp"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// ChangeQuery holds parameters for querying change events.
type ChangeQuery struct {
	Service     string   `json:"service,omitempty"`
	StartTime   string   `json:"start_time,omitempty"`
	EndTime     string   `json:"end_time,omitempty"`
	ChangeTypes []string `json:"change_types,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	Offset      int      `json:"offset,omitempty"`
}

// ChangeCorrelation represents correlated changes for an incident.
type ChangeCorrelation struct {
	IncidentID int64         `json:"incident_id"`
	Changes    []ChangeEvent `json:"changes"`
	Score      float64       `json:"score"`
}
