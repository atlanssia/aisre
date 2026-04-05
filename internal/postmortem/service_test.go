package postmortem

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// --- Mocks ---

type mockPostmortemRepo struct {
	items      []store.Postmortem
	nextID     int64
	byIncident map[int64]*store.Postmortem
}

func newMockPostmortemRepo() *mockPostmortemRepo {
	return &mockPostmortemRepo{
		byIncident: make(map[int64]*store.Postmortem),
	}
}

func (m *mockPostmortemRepo) Create(_ context.Context, pm *store.Postmortem) (int64, error) {
	m.nextID++
	pm.ID = m.nextID
	pm.CreatedAt = "2026-01-01T00:00:00Z"
	pm.UpdatedAt = "2026-01-01T00:00:00Z"
	m.items = append(m.items, *pm)
	m.byIncident[pm.IncidentID] = pm
	return m.nextID, nil
}

func (m *mockPostmortemRepo) GetByID(_ context.Context, id int64) (*store.Postmortem, error) {
	for _, pm := range m.items {
		if pm.ID == id {
			return &pm, nil
		}
	}
	return nil, fmt.Errorf("postmortem_repo: %d not found: %w", id, sql.ErrNoRows)
}

func (m *mockPostmortemRepo) GetByIncidentID(_ context.Context, incidentID int64) (*store.Postmortem, error) {
	if pm, ok := m.byIncident[incidentID]; ok {
		return pm, nil
	}
	return nil, fmt.Errorf("postmortem_repo: incident %d not found: %w", incidentID, sql.ErrNoRows)
}

func (m *mockPostmortemRepo) List(_ context.Context) ([]store.Postmortem, error) {
	return m.items, nil
}

func (m *mockPostmortemRepo) Update(_ context.Context, pm *store.Postmortem) error {
	for i, item := range m.items {
		if item.ID == pm.ID {
			m.items[i] = *pm
			m.byIncident[pm.IncidentID] = pm
			return nil
		}
	}
	return fmt.Errorf("postmortem_repo: %d not found for update", pm.ID)
}

type mockIncidentLookup struct {
	incident *contract.Incident
	err      error
}

func (m *mockIncidentLookup) GetIncident(_ context.Context, id int64) (*contract.Incident, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.incident, nil
}

type mockReportRepo struct {
	reports []store.Report
}

func (m *mockReportRepo) Create(_ context.Context, report *store.Report) (int64, error) {
	return 0, nil
}
func (m *mockReportRepo) GetByID(_ context.Context, id int64) (*store.Report, error) {
	return nil, nil
}
func (m *mockReportRepo) List(_ context.Context, filter store.ReportFilter) ([]store.Report, error) {
	return m.reports, nil
}
func (m *mockReportRepo) Search(_ context.Context, query string, filter store.ReportFilter) ([]store.Report, error) {
	return nil, nil
}

type mockEvidenceRepo struct{}

func (m *mockEvidenceRepo) Create(_ context.Context, evidence *store.Evidence) (int64, error) {
	return 0, nil
}
func (m *mockEvidenceRepo) ListByReport(_ context.Context, reportID int64) ([]store.Evidence, error) {
	return nil, nil
}

type mockFeedbackRepo struct {
	feedback []store.Feedback
}

func (m *mockFeedbackRepo) Create(_ context.Context, fb *store.Feedback) (int64, error) {
	return 0, nil
}
func (m *mockFeedbackRepo) ListByReport(_ context.Context, reportID int64) ([]store.Feedback, error) {
	return m.feedback, nil
}

type mockLLMGenerator struct {
	content string
	err     error
}

func (m *mockLLMGenerator) GeneratePostmortem(_ context.Context, _ *contract.Incident, _ *contract.ReportResponse, _ []store.Feedback) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.content, nil
}

// --- Tests ---

func TestGenerate(t *testing.T) {
	repo := newMockPostmortemRepo()
	incLookup := &mockIncidentLookup{
		incident: &contract.Incident{
			ID:          1,
			ServiceName: "api-gateway",
			Severity:    "critical",
			Source:      "prometheus",
			Status:      "resolved",
			CreatedAt:   "2026-01-01T10:00:00Z",
		},
	}
	reportRepo := &mockReportRepo{
		reports: []store.Report{
			{ID: 10, IncidentID: 1, Summary: "High error rate", RootCause: "DB timeout", Confidence: 0.85},
		},
	}
	feedbackRepo := &mockFeedbackRepo{
		feedback: []store.Feedback{
			{ID: 1, ReportID: 10, UserID: "alice", Rating: 4, Comment: "Helpful analysis", ActionTaken: "accepted"},
		},
	}
	llm := &mockLLMGenerator{content: "# Postmortem\n\n## Summary\n..."}

	svc := NewService(repo, incLookup, reportRepo, &mockEvidenceRepo{}, feedbackRepo, llm)

	pm, err := svc.Generate(context.Background(), 1)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if pm.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if pm.IncidentID != 1 {
		t.Errorf("incident_id = %d, want 1", pm.IncidentID)
	}
	if pm.Status != "draft" {
		t.Errorf("status = %q, want draft", pm.Status)
	}
	if pm.Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestGenerate_AlreadyExists(t *testing.T) {
	repo := newMockPostmortemRepo()
	// Pre-create a postmortem for incident 1
	_, _ = repo.Create(context.Background(), &store.Postmortem{IncidentID: 1, Status: "draft"})
	repo.byIncident[1] = &store.Postmortem{ID: 1, IncidentID: 1, Status: "draft"}

	incLookup := &mockIncidentLookup{
		incident: &contract.Incident{ID: 1, ServiceName: "svc", Severity: "high"},
	}
	llm := &mockLLMGenerator{content: "# PM"}
	svc := NewService(repo, incLookup, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, llm)

	_, err := svc.Generate(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error for duplicate postmortem")
	}
	if err.Error() != "postmortem: already exists for incident 1" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerate_IncidentNotFound(t *testing.T) {
	repo := newMockPostmortemRepo()
	incLookup := &mockIncidentLookup{
		err: fmt.Errorf("not found: %w", sql.ErrNoRows),
	}
	llm := &mockLLMGenerator{content: "# PM"}
	svc := NewService(repo, incLookup, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, llm)

	_, err := svc.Generate(context.Background(), 99)
	if err == nil {
		t.Fatal("expected error for missing incident")
	}
}

func TestGenerate_LLMError(t *testing.T) {
	repo := newMockPostmortemRepo()
	incLookup := &mockIncidentLookup{
		incident: &contract.Incident{ID: 1, ServiceName: "svc", Severity: "high"},
	}
	llm := &mockLLMGenerator{err: fmt.Errorf("LLM unavailable")}
	svc := NewService(repo, incLookup, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, llm)

	_, err := svc.Generate(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error for LLM failure")
	}
}

func TestList(t *testing.T) {
	repo := newMockPostmortemRepo()
	_, _ = repo.Create(context.Background(), &store.Postmortem{IncidentID: 1, Content: "PM1", Status: "draft"})
	_, _ = repo.Create(context.Background(), &store.Postmortem{IncidentID: 2, Content: "PM2", Status: "reviewed"})

	svc := NewService(repo, &mockIncidentLookup{}, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, &mockLLMGenerator{})
	items, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestList_Empty(t *testing.T) {
	repo := newMockPostmortemRepo()
	svc := NewService(repo, &mockIncidentLookup{}, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, &mockLLMGenerator{})
	items, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if items == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestGet(t *testing.T) {
	repo := newMockPostmortemRepo()
	id, _ := repo.Create(context.Background(), &store.Postmortem{IncidentID: 1, Content: "PM content", Status: "draft"})

	svc := NewService(repo, &mockIncidentLookup{}, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, &mockLLMGenerator{})
	pm, err := svc.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if pm.Content != "PM content" {
		t.Errorf("content = %q, want %q", pm.Content, "PM content")
	}
}

func TestGet_NotFound(t *testing.T) {
	repo := newMockPostmortemRepo()
	svc := NewService(repo, &mockIncidentLookup{}, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, &mockLLMGenerator{})
	_, err := svc.Get(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for missing postmortem")
	}
}

func TestUpdate_Content(t *testing.T) {
	repo := newMockPostmortemRepo()
	id, _ := repo.Create(context.Background(), &store.Postmortem{IncidentID: 1, Content: "old content", Status: "draft"})

	svc := NewService(repo, &mockIncidentLookup{}, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, &mockLLMGenerator{})
	pm, err := svc.Update(context.Background(), id, contract.UpdatePostmortemRequest{
		Content: "new content",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if pm.Content != "new content" {
		t.Errorf("content = %q, want %q", pm.Content, "new content")
	}
	if pm.Status != "draft" {
		t.Errorf("status should remain draft, got %q", pm.Status)
	}
}

func TestUpdate_StatusTransition(t *testing.T) {
	repo := newMockPostmortemRepo()
	id, _ := repo.Create(context.Background(), &store.Postmortem{IncidentID: 1, Content: "PM", Status: "draft"})

	svc := NewService(repo, &mockIncidentLookup{}, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, &mockLLMGenerator{})

	// draft -> reviewed
	pm, err := svc.Update(context.Background(), id, contract.UpdatePostmortemRequest{Status: "reviewed"})
	if err != nil {
		t.Fatalf("update to reviewed: %v", err)
	}
	if pm.Status != "reviewed" {
		t.Errorf("status = %q, want reviewed", pm.Status)
	}

	// reviewed -> published
	pm, err = svc.Update(context.Background(), id, contract.UpdatePostmortemRequest{Status: "published"})
	if err != nil {
		t.Fatalf("update to published: %v", err)
	}
	if pm.Status != "published" {
		t.Errorf("status = %q, want published", pm.Status)
	}
}

func TestUpdate_InvalidStatusTransition(t *testing.T) {
	repo := newMockPostmortemRepo()
	id, _ := repo.Create(context.Background(), &store.Postmortem{IncidentID: 1, Content: "PM", Status: "draft"})

	svc := NewService(repo, &mockIncidentLookup{}, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, &mockLLMGenerator{})

	// draft -> published (skipping reviewed) should fail
	_, err := svc.Update(context.Background(), id, contract.UpdatePostmortemRequest{Status: "published"})
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
}

func TestUpdate_InvalidStatus(t *testing.T) {
	repo := newMockPostmortemRepo()
	id, _ := repo.Create(context.Background(), &store.Postmortem{IncidentID: 1, Content: "PM", Status: "draft"})

	svc := NewService(repo, &mockIncidentLookup{}, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, &mockLLMGenerator{})

	_, err := svc.Update(context.Background(), id, contract.UpdatePostmortemRequest{Status: "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestUpdate_NotFound(t *testing.T) {
	repo := newMockPostmortemRepo()
	svc := NewService(repo, &mockIncidentLookup{}, &mockReportRepo{}, &mockEvidenceRepo{}, &mockFeedbackRepo{}, &mockLLMGenerator{})

	_, err := svc.Update(context.Background(), 999, contract.UpdatePostmortemRequest{Status: "reviewed"})
	if err == nil {
		t.Fatal("expected error for missing postmortem")
	}
}

func TestValidateStatusTransition(t *testing.T) {
	tests := []struct {
		from    string
		to      string
		wantErr bool
	}{
		{"draft", "reviewed", false},
		{"draft", "draft", false}, // same status allowed
		{"draft", "published", true},
		{"reviewed", "published", false},
		{"reviewed", "draft", true},
		{"published", "draft", true},
		{"published", "reviewed", true},
		{"draft", "invalid", true},
	}

	for _, tt := range tests {
		err := validateStatusTransition(tt.from, tt.to)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateStatusTransition(%q, %q) = %v, wantErr=%v", tt.from, tt.to, err, tt.wantErr)
		}
	}
}

func TestGenerate_NoReport(t *testing.T) {
	repo := newMockPostmortemRepo()
	incLookup := &mockIncidentLookup{
		incident: &contract.Incident{ID: 5, ServiceName: "svc", Severity: "low"},
	}
	reportRepo := &mockReportRepo{reports: []store.Report{}} // no reports
	llm := &mockLLMGenerator{content: "# Postmortem (no report)"}

	svc := NewService(repo, incLookup, reportRepo, &mockEvidenceRepo{}, &mockFeedbackRepo{}, llm)

	pm, err := svc.Generate(context.Background(), 5)
	if err != nil {
		t.Fatalf("generate with no report: %v", err)
	}
	if pm.Content == "" {
		t.Error("expected non-empty content even without report")
	}
}

func TestDefaultLLMGenerator(t *testing.T) {
	called := false
	gen := NewDefaultLLMGenerator(func(ctx context.Context, messages []Message) (*LLMResponse, error) {
		called = true
		if messages[0].Role != "system" {
			t.Errorf("expected system role, got %q", messages[0].Role)
		}
		return &LLMResponse{Content: "# Generated Postmortem"}, nil
	})

	content, err := gen.GeneratePostmortem(
		context.Background(),
		&contract.Incident{ID: 1, ServiceName: "svc", Severity: "high", Source: "test", Status: "resolved", CreatedAt: "2026-01-01"},
		&contract.ReportResponse{Summary: "test", RootCause: "unknown", Confidence: 0.9},
		nil,
	)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !called {
		t.Error("expected LLM to be called")
	}
	if content != "# Generated Postmortem" {
		t.Errorf("content = %q, want %q", content, "# Generated Postmortem")
	}
}
