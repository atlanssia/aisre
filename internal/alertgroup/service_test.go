package alertgroup

import (
	"context"
	"fmt"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// mockAlertGroupRepo implements store.AlertGroupRepo for testing.
type mockAlertGroupRepo struct {
	groups map[string]*store.AlertGroup // keyed by fingerprint
	nextID int64
}

func newMockRepo() *mockAlertGroupRepo {
	return &mockAlertGroupRepo{groups: make(map[string]*store.AlertGroup)}
}

func (m *mockAlertGroupRepo) Create(_ context.Context, ag *store.AlertGroup) (int64, error) {
	m.nextID++
	ag.ID = m.nextID
	copy := *ag
	m.groups[ag.Fingerprint] = &copy
	return m.nextID, nil
}

func (m *mockAlertGroupRepo) GetByID(_ context.Context, id int64) (*store.AlertGroup, error) {
	for _, ag := range m.groups {
		if ag.ID == id {
			copy := *ag
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("not found: %d", id)
}

func (m *mockAlertGroupRepo) GetByFingerprint(_ context.Context, fp string) (*store.AlertGroup, error) {
	ag, ok := m.groups[fp]
	if !ok {
		return nil, fmt.Errorf("not found: %s", fp)
	}
	copy := *ag
	return &copy, nil
}

func (m *mockAlertGroupRepo) Update(_ context.Context, ag *store.AlertGroup) error {
	existing, ok := m.groups[ag.Fingerprint]
	if !ok {
		return fmt.Errorf("not found for update")
	}
	copy := *ag
	m.groups[existing.Fingerprint] = &copy
	return nil
}

func (m *mockAlertGroupRepo) List(_ context.Context, filter store.AlertGroupFilter) ([]store.AlertGroup, error) {
	var results []store.AlertGroup
	for _, ag := range m.groups {
		if filter.Severity != "" && ag.Severity != filter.Severity {
			continue
		}
		results = append(results, *ag)
	}
	return results, nil
}

// mockIncidentSvc implements IncidentCreator for testing.
type mockIncidentSvc struct {
	created bool
	lastID  int64
}

func (m *mockIncidentSvc) CreateIncident(_ context.Context, req contract.CreateIncidentRequest) (*contract.CreateIncidentResponse, error) {
	m.created = true
	m.lastID++
	return &contract.CreateIncidentResponse{IncidentID: m.lastID, Status: "open"}, nil
}

func TestIngest_NewAlert(t *testing.T) {
	repo := newMockRepo()
	incSvc := &mockIncidentSvc{}
	svc := NewService(repo, incSvc)

	group, err := svc.Ingest(context.Background(), contract.IncomingAlert{
		Title:    "High CPU on web-01",
		Severity: "critical",
		Labels:   map[string]string{"host": "web-01", "alertname": "HighCPU"},
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if group.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if group.Count != 1 {
		t.Errorf("count = %d, want 1", group.Count)
	}
	if group.Severity != "critical" {
		t.Errorf("severity = %q, want %q", group.Severity, "critical")
	}
	if group.Title != "High CPU on web-01" {
		t.Errorf("title = %q, want %q", group.Title, "High CPU on web-01")
	}
}

func TestIngest_Dedup(t *testing.T) {
	repo := newMockRepo()
	incSvc := &mockIncidentSvc{}
	svc := NewService(repo, incSvc)

	labels := map[string]string{"host": "web-01", "alertname": "HighCPU"}

	// First ingest
	group1, err := svc.Ingest(context.Background(), contract.IncomingAlert{
		Title: "High CPU", Severity: "high", Labels: labels,
	})
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}

	// Second ingest with same labels — should dedup
	group2, err := svc.Ingest(context.Background(), contract.IncomingAlert{
		Title: "High CPU Updated", Severity: "critical", Labels: labels,
	})
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	if group2.ID != group1.ID {
		t.Errorf("expected same ID for dedup, got %d and %d", group1.ID, group2.ID)
	}
	if group2.Count != 2 {
		t.Errorf("count = %d, want 2 after dedup", group2.Count)
	}
	if group2.Title != "High CPU Updated" {
		t.Errorf("title should be updated to latest, got %q", group2.Title)
	}
	if group2.Severity != "critical" {
		t.Errorf("severity should be updated to latest, got %q", group2.Severity)
	}
}

func TestIngest_DifferentLabels_NoDedup(t *testing.T) {
	repo := newMockRepo()
	incSvc := &mockIncidentSvc{}
	svc := NewService(repo, incSvc)

	group1, _ := svc.Ingest(context.Background(), contract.IncomingAlert{
		Title: "Alert A", Labels: map[string]string{"host": "web-01"},
	})
	group2, _ := svc.Ingest(context.Background(), contract.IncomingAlert{
		Title: "Alert B", Labels: map[string]string{"host": "web-02"},
	})

	if group1.ID == group2.ID {
		t.Error("different labels should create different groups")
	}
}

func TestIngest_TitleRequired(t *testing.T) {
	svc := NewService(newMockRepo(), &mockIncidentSvc{})
	_, err := svc.Ingest(context.Background(), contract.IncomingAlert{Title: ""})
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestIngest_DefaultSeverity(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, &mockIncidentSvc{})
	group, err := svc.Ingest(context.Background(), contract.IncomingAlert{
		Title: "Test", Severity: "",
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if group.Severity != "warning" {
		t.Errorf("severity = %q, want default 'warning'", group.Severity)
	}
}

func TestEscalate(t *testing.T) {
	repo := newMockRepo()
	incSvc := &mockIncidentSvc{}
	svc := NewService(repo, incSvc)

	group, _ := svc.Ingest(context.Background(), contract.IncomingAlert{
		Title: "Test Alert", Severity: "high", Labels: map[string]string{"svc": "api"},
	})

	resp, err := svc.Escalate(context.Background(), group.ID)
	if err != nil {
		t.Fatalf("escalate: %v", err)
	}
	if resp.AlertGroupID != group.ID {
		t.Errorf("alert group id = %d, want %d", resp.AlertGroupID, group.ID)
	}
	if resp.IncidentID == 0 {
		t.Error("expected non-zero incident ID")
	}
	if !incSvc.created {
		t.Error("expected incident to be created")
	}
}

func TestEscalate_AlreadyEscalated(t *testing.T) {
	repo := newMockRepo()
	incSvc := &mockIncidentSvc{}
	svc := NewService(repo, incSvc)

	group, _ := svc.Ingest(context.Background(), contract.IncomingAlert{
		Title: "Test Alert", Severity: "high", Labels: map[string]string{"svc": "api"},
	})
	svc.Escalate(context.Background(), group.ID)

	_, err := svc.Escalate(context.Background(), group.ID)
	if err == nil {
		t.Error("expected error for already escalated group")
	}
}

func TestList_InvalidSeverity(t *testing.T) {
	svc := NewService(newMockRepo(), &mockIncidentSvc{})
	_, err := svc.List(context.Background(), contract.AlertGroupFilter{Severity: "invalid"})
	if err == nil {
		t.Error("expected error for invalid severity")
	}
}

func TestList_DefaultLimit(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, &mockIncidentSvc{})

	// Create 2 groups
	svc.Ingest(context.Background(), contract.IncomingAlert{Title: "A", Labels: map[string]string{"a": "1"}})
	svc.Ingest(context.Background(), contract.IncomingAlert{Title: "B", Labels: map[string]string{"b": "2"}})

	groups, err := svc.List(context.Background(), contract.AlertGroupFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestComputeFingerprint_Deterministic(t *testing.T) {
	fp1 := computeFingerprint(map[string]string{"b": "2", "a": "1"})
	fp2 := computeFingerprint(map[string]string{"a": "1", "b": "2"})
	if fp1 != fp2 {
		t.Errorf("fingerprint should be deterministic regardless of map iteration order")
	}
}

func TestComputeFingerprint_Different(t *testing.T) {
	fp1 := computeFingerprint(map[string]string{"a": "1"})
	fp2 := computeFingerprint(map[string]string{"a": "2"})
	if fp1 == fp2 {
		t.Error("different labels should produce different fingerprints")
	}
}
