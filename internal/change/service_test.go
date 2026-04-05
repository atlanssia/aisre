package change

import (
	"context"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
	"github.com/atlanssia/aisre/internal/testkit"
)

func TestService_GetChanges(t *testing.T) {
	db, cleanup := testkit.NewTestDB(t)
	defer cleanup()
	changeRepo := store.NewChangeRepo(db)
	incRepo := store.NewIncidentRepo(db)
	rptRepo := store.NewReportRepo(db)
	svc := NewService(changeRepo, incRepo, rptRepo)
	ctx := context.Background()

	// Ingest some changes
	_, _ = svc.IngestChange(ctx, contract.ChangeEvent{
		Service: "api-gw", ChangeType: "deploy", Summary: "Deploy v1", Timestamp: "2025-01-15T09:00:00Z",
	})
	_, _ = svc.IngestChange(ctx, contract.ChangeEvent{
		Service: "payment", ChangeType: "config", Summary: "Config update", Timestamp: "2025-01-15T10:00:00Z",
	})

	// List all
	results, err := svc.GetChanges(ctx, contract.ChangeQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(results))
	}

	// Filter by service
	results, err = svc.GetChanges(ctx, contract.ChangeQuery{Service: "api-gw"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 change for api-gw, got %d", len(results))
	}
	if results[0].Summary != "Deploy v1" {
		t.Errorf("summary: got %q", results[0].Summary)
	}
}

func TestService_GetChangesForIncident(t *testing.T) {
	db, cleanup := testkit.NewTestDB(t)
	defer cleanup()
	changeRepo := store.NewChangeRepo(db)
	incRepo := store.NewIncidentRepo(db)
	rptRepo := store.NewReportRepo(db)
	svc := NewService(changeRepo, incRepo, rptRepo)
	ctx := context.Background()

	// Create incident
	incID, _ := incRepo.Create(ctx, &store.Incident{
		Source: "test", ServiceName: "api-gw", Severity: "critical", Status: "open",
	})

	// Look up incident to get the CreatedAt timestamp for constructing a matching change
	inc, _ := incRepo.GetByID(ctx, incID)

	// Ingest change for same service with a timestamp that falls within the 2h lookback window
	_, _ = svc.IngestChange(ctx, contract.ChangeEvent{
		Service: "api-gw", ChangeType: "deploy", Summary: "Deploy v3.2.1", Timestamp: inc.CreatedAt,
	})

	corr, err := svc.GetChangesForIncident(ctx, incID)
	if err != nil {
		t.Fatal(err)
	}
	if corr.IncidentID != incID {
		t.Errorf("incident_id: got %d, want %d", corr.IncidentID, incID)
	}
	if len(corr.Changes) != 1 {
		t.Fatalf("expected 1 correlated change, got %d", len(corr.Changes))
	}
	if corr.Changes[0].Summary != "Deploy v3.2.1" {
		t.Errorf("summary: got %q", corr.Changes[0].Summary)
	}
}

func TestService_IngestChange(t *testing.T) {
	db, cleanup := testkit.NewTestDB(t)
	defer cleanup()
	svc := NewService(store.NewChangeRepo(db), store.NewIncidentRepo(db), store.NewReportRepo(db))
	ctx := context.Background()

	id, err := svc.IngestChange(ctx, contract.ChangeEvent{
		Service: "api-gw", ChangeType: "deploy", Summary: "Deploy v1",
		Author: "ci-bot", Timestamp: "2025-01-15T09:00:00Z",
		Metadata: map[string]any{"version": "1.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Error("expected positive ID")
	}
}
