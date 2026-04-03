package store

import "context"

// IncidentRepo defines the persistence interface for incidents.
type IncidentRepo interface {
	Create(ctx context.Context, inc *Incident) (int64, error)
	GetByID(ctx context.Context, id int64) (*Incident, error)
	List(ctx context.Context, filter IncidentFilter) ([]Incident, error)
	UpdateStatus(ctx context.Context, id int64, status string) error
}

// ReportRepo defines the persistence interface for RCA reports.
type ReportRepo interface {
	Create(ctx context.Context, report *Report) (int64, error)
	GetByID(ctx context.Context, id int64) (*Report, error)
	List(ctx context.Context, filter ReportFilter) ([]Report, error)
	Search(ctx context.Context, query string, filter ReportFilter) ([]Report, error)
}

// FeedbackRepo defines the persistence interface for user feedback.
type FeedbackRepo interface {
	Create(ctx context.Context, fb *Feedback) (int64, error)
	ListByReport(ctx context.Context, reportID int64) ([]Feedback, error)
}

// EvidenceRepo defines the persistence interface for evidence items.
type EvidenceRepo interface {
	Create(ctx context.Context, evidence *Evidence) (int64, error)
	ListByReport(ctx context.Context, reportID int64) ([]Evidence, error)
}

// Incident is the persistent incident entity.
type Incident struct {
	ID          int64
	Source      string
	ServiceName string
	Severity    string
	Status      string
	TraceID     string
	CreatedAt   string
}

// Report is the persistent report entity.
type Report struct {
	ID          int64
	IncidentID  int64
	Summary     string
	RootCause   string
	Confidence  float64
	ReportJSON  string
	Status      string
	CreatedAt   string
}

// Evidence is the persistent evidence entity.
type Evidence struct {
	ID           int64
	ReportID     int64
	EvidenceType string
	Score        float64
	Payload      string
	SourceURL    string
	CreatedAt    string
}

// Feedback is the persistent feedback entity.
type Feedback struct {
	ID          int64
	ReportID    int64
	UserID      string
	Rating      int
	Comment     string
	ActionTaken string
	CreatedAt   string
}

// EmbeddingRepo defines the persistence interface for incident embeddings.
type EmbeddingRepo interface {
	Create(ctx context.Context, incidentID int64, service string, embedding []byte, model string) error
	GetByIncidentID(ctx context.Context, incidentID int64) (*Embedding, error)
	ListByService(ctx context.Context, service string) ([]Embedding, error)
}

// Embedding is the persistent embedding entity.
type Embedding struct {
	IncidentID int64
	Service    string
	Embedding  []byte
	Model      string
	CreatedAt  string
}

// IncidentFilter holds filter parameters for listing incidents.
type IncidentFilter struct {
	Service   string
	Severity  string
	Status    string
	StartTime string
	EndTime   string
	Limit     int
	Offset    int
}

// ReportFilter holds filter parameters for listing reports.
type ReportFilter struct {
	Service   string
	StartTime string
	EndTime   string
	Severity  string
	Limit     int
	Offset    int
}
