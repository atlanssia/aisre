package contract

// Postmortem represents a generated postmortem document for an incident.
type Postmortem struct {
	ID         int64  `json:"id"`
	IncidentID int64  `json:"incident_id"`
	Content    string `json:"content"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// UpdatePostmortemRequest is the DTO for updating a postmortem.
type UpdatePostmortemRequest struct {
	Content string `json:"content,omitempty"`
	Status  string `json:"status,omitempty"`
}
