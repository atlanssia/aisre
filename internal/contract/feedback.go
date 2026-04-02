package contract

// FeedbackRequest is the DTO for submitting feedback on an RCA report.
type FeedbackRequest struct {
	Rating      int    `json:"rating"`       // 1-5
	Comment     string `json:"comment"`
	ActionTaken string `json:"action_taken"` // accepted, partial, rejected
}

// FeedbackResponse is the DTO returned after submitting feedback.
type FeedbackResponse struct {
	ID       int64  `json:"id"`
	ReportID int64  `json:"report_id"`
	Status   string `json:"status"`
}
