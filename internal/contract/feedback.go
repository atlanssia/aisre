package contract

// FeedbackRequest is the DTO for submitting feedback on an RCA report.
type FeedbackRequest struct {
	Rating      int    `json:"rating"`       // 1-5
	Comment     string `json:"comment"`
	UserID      string `json:"user_id"`
	ActionTaken string `json:"action_taken"` // accepted, partial, rejected
}

// FeedbackResponse is the DTO returned after submitting feedback.
type FeedbackResponse struct {
	ID          int64  `json:"id"`
	ReportID    int64  `json:"report_id"`
	Rating      int    `json:"rating"`
	Comment     string `json:"comment"`
	UserID      string `json:"user_id"`
	ActionTaken string `json:"action_taken"`
	CreatedAt   string `json:"created_at"`
}

// SearchReportsRequest holds query parameters for report search.
type SearchReportsRequest struct {
	Query    string `json:"query,omitempty"`
	Service  string `json:"service,omitempty"`
	Severity string `json:"severity,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}
