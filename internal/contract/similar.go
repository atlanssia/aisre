package contract

// SimilarResult represents a matched similar incident.
type SimilarResult struct {
	IncidentID int64   `json:"incident_id"`
	Similarity float64 `json:"similarity"`
	Summary    string  `json:"summary"`
	RootCause  string  `json:"root_cause"`
	Service    string  `json:"service"`
	Severity   string  `json:"severity"`
}

// EmbedRequest is the request to compute embedding for an incident.
type EmbedRequest struct {
	Text string `json:"text"`
}

// SimilarQuery holds parameters for similar incident search.
type SimilarQuery struct {
	TopK      int     `json:"top_k"`
	Threshold float64 `json:"threshold"`
}
