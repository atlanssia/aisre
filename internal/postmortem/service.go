package postmortem

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// IncidentLookup defines the interface needed to fetch incident details.
// Defined in the consuming package (postmortem) per project convention.
type IncidentLookup interface {
	GetIncident(ctx context.Context, id int64) (*contract.Incident, error)
}


// LLMGenerator defines the interface for LLM text generation.
type LLMGenerator interface {
	GeneratePostmortem(ctx context.Context, incident *contract.Incident, report *contract.ReportResponse, feedback []store.Feedback) (string, error)
}

// Service handles postmortem generation and management.
type Service struct {
	repo       store.PostmortemRepo
	incidents  IncidentLookup
	reports    store.ReportRepo
	evidence   store.EvidenceRepo
	feedback   store.FeedbackRepo
	llm        LLMGenerator
	logger     *slog.Logger
}

// NewService creates a new postmortem service.
func NewService(repo store.PostmortemRepo, incidents IncidentLookup, reports store.ReportRepo, evidence store.EvidenceRepo, feedback store.FeedbackRepo, llm LLMGenerator) *Service {
	return &Service{
		repo:      repo,
		incidents: incidents,
		reports:   reports,
		evidence:  evidence,
		feedback:  feedback,
		llm:       llm,
		logger:    slog.Default(),
	}
}

// Generate creates a postmortem for the given incident.
// It fetches incident data, report, evidence, and feedback, then calls the LLM.
func (s *Service) Generate(ctx context.Context, incidentID int64) (*contract.Postmortem, error) {
	// Check if postmortem already exists for this incident
	existing, err := s.repo.GetByIncidentID(ctx, incidentID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("postmortem: generate: check existing: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("postmortem: already exists for incident %d", incidentID)
	}

	// Fetch incident
	incident, err := s.incidents.GetIncident(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("postmortem: generate: get incident: %w", err)
	}

	// Fetch the latest report for this incident
	reports, err := s.reports.List(ctx, store.ReportFilter{IncidentID: incidentID, Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("postmortem: generate: list reports: %w", err)
	}
	var reportForLLM *contract.ReportResponse
	if len(reports) > 0 {
		r := reports[0]
		reportForLLM = &contract.ReportResponse{
			ID:         r.ID,
			IncidentID: r.IncidentID,
			Summary:    r.Summary,
			RootCause:  r.RootCause,
		}
	}

	// Fetch feedback for the report
	var feedbackItems []store.Feedback
	if reportForLLM != nil {
		feedbackItems, err = s.feedback.ListByReport(ctx, reportForLLM.ID)
		if err != nil {
			s.logger.Warn("failed to fetch feedback for postmortem", "report_id", reportForLLM.ID, "error", err)
			feedbackItems = nil
		}
	}

	// Generate postmortem content via LLM
	content, err := s.llm.GeneratePostmortem(ctx, incident, reportForLLM, feedbackItems)
	if err != nil {
		return nil, fmt.Errorf("postmortem: generate: llm: %w", err)
	}

	// Store the postmortem
	pm := &store.Postmortem{
		IncidentID: incidentID,
		Content:    content,
		Status:     "draft",
	}
	id, err := s.repo.Create(ctx, pm)
	if err != nil {
		return nil, fmt.Errorf("postmortem: generate: save: %w", err)
	}
	pm.ID = id

	s.logger.Info("postmortem generated", "id", id, "incident_id", incidentID)

	return storeToContract(pm), nil
}

// List returns all postmortems.
func (s *Service) List(ctx context.Context) ([]contract.Postmortem, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("postmortem: list: %w", err)
	}
	result := make([]contract.Postmortem, 0, len(items))
	for i := range items {
		result = append(result, *storeToContract(&items[i]))
	}
	return result, nil
}

// Get returns a postmortem by ID.
func (s *Service) Get(ctx context.Context, id int64) (*contract.Postmortem, error) {
	pm, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("postmortem: get: %w", err)
	}
	return storeToContract(pm), nil
}

// Update modifies a postmortem (content edits or status transitions).
func (s *Service) Update(ctx context.Context, id int64, req contract.UpdatePostmortemRequest) (*contract.Postmortem, error) {
	pm, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("postmortem: update: get: %w", err)
	}

	if req.Content != "" {
		pm.Content = req.Content
	}

	if req.Status != "" {
		if err := validateStatusTransition(pm.Status, req.Status); err != nil {
			return nil, fmt.Errorf("postmortem: update: %w", err)
		}
		pm.Status = req.Status
	}

	if err := s.repo.Update(ctx, pm); err != nil {
		return nil, fmt.Errorf("postmortem: update: save: %w", err)
	}

	s.logger.Info("postmortem updated", "id", id, "status", pm.Status)
	return storeToContract(pm), nil
}

// validateStatusTransition ensures the status transition is valid.
// Allowed: draft -> reviewed -> published
func validateStatusTransition(from, to string) error {
	validStatuses := map[string]bool{"draft": true, "reviewed": true, "published": true}
	if !validStatuses[to] {
		return fmt.Errorf("invalid status %q, must be one of: draft, reviewed, published", to)
	}

	transitions := map[string]string{
		"draft":    "reviewed",
		"reviewed": "published",
	}

	// Allow same status (no-op)
	if from == to {
		return nil
	}

	expected, ok := transitions[from]
	if !ok || to != expected {
		return fmt.Errorf("invalid transition from %q to %q", from, to)
	}
	return nil
}

// DefaultLLMGenerator implements LLMGenerator using analysis.LLMClient-compatible interface.
type DefaultLLMGenerator struct {
	completeFn func(ctx context.Context, messages []Message) (*LLMResponse, error)
}

// Message represents a chat message (mirrors analysis.Message for decoupling).
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMResponse represents an LLM response (mirrors analysis.LLMResponse for decoupling).
type LLMResponse struct {
	Content string
}

// NewDefaultLLMGenerator creates a generator that delegates to the provided complete function.
func NewDefaultLLMGenerator(completeFn func(ctx context.Context, messages []Message) (*LLMResponse, error)) *DefaultLLMGenerator {
	return &DefaultLLMGenerator{completeFn: completeFn}
}

// GeneratePostmortem calls the LLM to generate postmortem markdown content.
func (g *DefaultLLMGenerator) GeneratePostmortem(ctx context.Context, incident *contract.Incident, report *contract.ReportResponse, feedback []store.Feedback) (string, error) {
	var b strings.Builder

	b.WriteString("# Incident Postmortem Generation\n\n")
	b.WriteString("Generate a structured Markdown postmortem document based on the following incident data.\n\n")

	b.WriteString("## Incident Details\n")
	fmt.Fprintf(&b, "- **Service:** %s\n", incident.ServiceName)
	fmt.Fprintf(&b, "- **Severity:** %s\n", incident.Severity)
	fmt.Fprintf(&b, "- **Source:** %s\n", incident.Source)
	fmt.Fprintf(&b, "- **Status:** %s\n", incident.Status)
	if incident.TraceID != "" {
		fmt.Fprintf(&b, "- **Trace ID:** %s\n", incident.TraceID)
	}
	fmt.Fprintf(&b, "- **Created At:** %s\n\n", incident.CreatedAt)

	if report != nil {
		b.WriteString("## RCA Report\n")
		fmt.Fprintf(&b, "- **Summary:** %s\n", report.Summary)
		fmt.Fprintf(&b, "- **Root Cause:** %s\n", report.RootCause)
		fmt.Fprintf(&b, "- **Confidence:** %.2f\n", report.Confidence)
		if len(report.Recommendations) > 0 {
			b.WriteString("- **Recommendations:**\n")
			for _, rec := range report.Recommendations {
				fmt.Fprintf(&b, "  - %s\n", rec)
			}
		}
		if len(report.Timeline) > 0 {
			b.WriteString("- **Timeline:**\n")
			for _, ev := range report.Timeline {
				fmt.Fprintf(&b, "  - [%s] %s (%s): %s\n", ev.Time, ev.Type, ev.Service, ev.Description)
			}
		}
		if len(report.Evidence) > 0 {
			b.WriteString("- **Key Evidence:**\n")
			for _, ev := range report.Evidence {
				fmt.Fprintf(&b, "  - [%s] %s (score: %.2f)\n", ev.Type, ev.Summary, ev.Score)
			}
		}
		b.WriteString("\n")
	}

	if len(feedback) > 0 {
		b.WriteString("## Operator Feedback\n")
		for _, fb := range feedback {
			fmt.Fprintf(&b, "- **%s** (rating: %d/5): %s [action: %s]\n", fb.UserID, fb.Rating, fb.Comment, fb.ActionTaken)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Instructions\n")
	b.WriteString("Generate a complete postmortem document in Markdown format with these sections:\n")
	b.WriteString("1. **Summary** — brief overview of the incident\n")
	b.WriteString("2. **Impact** — affected services, users, duration\n")
	b.WriteString("3. **Timeline** — chronological sequence of events\n")
	b.WriteString("4. **Root Cause** — primary cause identified\n")
	b.WriteString("5. **Contributing Factors** — secondary factors\n")
	b.WriteString("6. **Action Items** — what to fix, who owns it\n")
	b.WriteString("7. **Lessons Learned** — what went well, what to improve\n\n")
	b.WriteString("Output only the Markdown content, no explanation.\n")

	messages := []Message{
		{Role: "system", Content: "You are an SRE postmortem writer. Generate clear, actionable postmortem documents in Markdown format."},
		{Role: "user", Content: b.String()},
	}

	resp, err := g.completeFn(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("llm generate: %w", err)
	}
	return resp.Content, nil
}

func storeToContract(pm *store.Postmortem) *contract.Postmortem {
	return &contract.Postmortem{
		ID:         pm.ID,
		IncidentID: pm.IncidentID,
		Content:    pm.Content,
		Status:     pm.Status,
		CreatedAt:  pm.CreatedAt,
		UpdatedAt:  pm.UpdatedAt,
	}
}
