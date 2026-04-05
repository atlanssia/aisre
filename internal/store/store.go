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

// ChangeRepo defines the persistence interface for change events.
type ChangeRepo interface {
	Create(ctx context.Context, ch *Change) (int64, error)
	GetByID(ctx context.Context, id int64) (*Change, error)
	List(ctx context.Context, filter ChangeFilter) ([]Change, error)
	ListByService(ctx context.Context, service string, startTime, endTime string) ([]Change, error)
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

// Change is the persistent change event entity.
type Change struct {
	ID         int64
	Service    string
	ChangeType string
	Summary    string
	Author     string
	Timestamp  string
	Metadata   string // JSON
	CreatedAt  string
}

// ChangeFilter holds filter parameters for listing changes.
type ChangeFilter struct {
	Service     string
	ChangeTypes []string
	StartTime   string
	EndTime     string
	Limit       int
	Offset      int
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

// TopologyRepo defines the persistence interface for topology edges.
type TopologyRepo interface {
	Create(ctx context.Context, edge *TopologyEdge) (int64, error)
	List(ctx context.Context) ([]TopologyEdge, error)
	ListBySource(ctx context.Context, source string) ([]TopologyEdge, error)
	Delete(ctx context.Context, id int64) error
}

// TopologyEdge is the persistent topology edge entity.
type TopologyEdge struct {
	ID        int64
	Source    string
	Target    string
	Relation  string
	Metadata  string // JSON
	CreatedAt string
	UpdatedAt string
}

// PromptTemplateRepo defines the persistence interface for prompt templates.
type PromptTemplateRepo interface {
	Create(ctx context.Context, tpl *PromptTemplate) (int64, error)
	GetByID(ctx context.Context, id int64) (*PromptTemplate, error)
	List(ctx context.Context) ([]PromptTemplate, error)
	Update(ctx context.Context, tpl *PromptTemplate) error
	GetByStage(ctx context.Context, stage string) (*PromptTemplate, error)
}

// PromptTemplate is the persistent prompt template entity.
type PromptTemplate struct {
	ID        int64
	Name      string
	Stage     string
	SystemTpl string
	UserTpl   string
	Variables string // JSON array of variable names
	IsDefault bool
	Version   int
	CreatedAt string
	UpdatedAt string
}

// AlertGroupRepo defines the persistence interface for alert groups.
type AlertGroupRepo interface {
	Create(ctx context.Context, ag *AlertGroup) (int64, error)
	GetByID(ctx context.Context, id int64) (*AlertGroup, error)
	GetByFingerprint(ctx context.Context, fp string) (*AlertGroup, error)
	Update(ctx context.Context, ag *AlertGroup) error
	List(ctx context.Context, filter AlertGroupFilter) ([]AlertGroup, error)
}

// AlertGroup is the persistent alert group entity.
type AlertGroup struct {
	ID         int64
	Fingerprint string
	Title      string
	Severity   string
	Labels     string // JSON
	IncidentID *int64
	Count      int
	FirstSeen  string
	LastSeen   string
	CreatedAt  string
}

// AlertGroupFilter holds filter parameters for listing alert groups.
type AlertGroupFilter struct {
	Severity  string
	StartTime string
	EndTime   string
	Limit     int
	Offset    int
}

// ReportFilter holds filter parameters for listing reports.
type ReportFilter struct {
	IncidentID int64
	Service    string
	StartTime  string
	EndTime    string
	Severity   string
	Limit      int
	Offset     int
}

// PostmortemRepo defines the persistence interface for postmortems.
type PostmortemRepo interface {
	Create(ctx context.Context, pm *Postmortem) (int64, error)
	GetByID(ctx context.Context, id int64) (*Postmortem, error)
	GetByIncidentID(ctx context.Context, incidentID int64) (*Postmortem, error)
	List(ctx context.Context) ([]Postmortem, error)
	Update(ctx context.Context, pm *Postmortem) error
}

// Postmortem is the persistent postmortem entity.
type Postmortem struct {
	ID         int64
	IncidentID int64
	Content    string
	Status     string
	CreatedAt  string
	UpdatedAt  string
}
