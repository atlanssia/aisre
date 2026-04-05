package store

import (
	"context"
	"testing"
)

func TestReportRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewReportRepo(db)
	incRepo := NewIncidentRepo(db)
	ctx := context.Background()

	// Create an incident first
	incID, _ := incRepo.Create(ctx, &Incident{
		Source:      "prometheus",
		ServiceName: "api-gateway",
		Severity:    "high",
		Status:      "open",
	})

	report := &Report{
		IncidentID: incID,
		Summary:    "Database connection pool exhaustion",
		RootCause:  "Connection leak in user-service",
		Confidence: 0.92,
		ReportJSON: `{"summary":"test"}`,
	}

	id, err := repo.Create(ctx, report)
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}
}

func TestReportRepo_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewReportRepo(db)
	incRepo := NewIncidentRepo(db)
	ctx := context.Background()

	incID, _ := incRepo.Create(ctx, &Incident{
		Source: "test", ServiceName: "svc", Severity: "low", Status: "open",
	})

	id, _ := repo.Create(ctx, &Report{
		IncidentID: incID,
		Summary:    "test summary",
		RootCause:  "test root cause",
		Confidence: 0.85,
		ReportJSON: `{"test":true}`,
	})

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "test summary" {
		t.Errorf("expected 'test summary', got %s", got.Summary)
	}
	if got.RootCause != "test root cause" {
		t.Errorf("expected 'test root cause', got %s", got.RootCause)
	}
	if got.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", got.Confidence)
	}
	if got.IncidentID != incID {
		t.Errorf("expected incident_id %d, got %d", incID, got.IncidentID)
	}
}

func TestReportRepo_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewReportRepo(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, 9999)
	if err == nil {
		t.Error("expected error for non-existent report")
	}
}

func TestReportRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewReportRepo(db)
	incRepo := NewIncidentRepo(db)
	ctx := context.Background()

	incID, _ := incRepo.Create(ctx, &Incident{
		Source: "test", ServiceName: "svc", Severity: "low", Status: "open",
	})

	for i := 0; i < 3; i++ {
		_, _ = repo.Create(ctx, &Report{
			IncidentID: incID,
			Summary:    "summary",
			RootCause:  "cause",
			Confidence: 0.8,
		})
	}

	items, err := repo.List(ctx, ReportFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestEvidenceRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	incRepo := NewIncidentRepo(db)
	reportRepo := NewReportRepo(db)
	evidenceRepo := NewEvidenceRepo(db)
	ctx := context.Background()

	incID, _ := incRepo.Create(ctx, &Incident{
		Source: "test", ServiceName: "svc", Severity: "low", Status: "open",
	})
	reportID, _ := reportRepo.Create(ctx, &Report{
		IncidentID: incID,
		Summary:    "test",
		Confidence: 0.8,
	})

	evidence := &Evidence{
		ReportID:     reportID,
		EvidenceType: "log",
		Score:        0.95,
		Payload:      `{"message":"connection refused"}`,
		SourceURL:    "http://logs.example.com/123",
	}

	id, err := evidenceRepo.Create(ctx, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}
}

func TestEvidenceRepo_ListByReport(t *testing.T) {
	db := setupTestDB(t)
	incRepo := NewIncidentRepo(db)
	reportRepo := NewReportRepo(db)
	evidenceRepo := NewEvidenceRepo(db)
	ctx := context.Background()

	incID, _ := incRepo.Create(ctx, &Incident{
		Source: "test", ServiceName: "svc", Severity: "low", Status: "open",
	})
	reportID, _ := reportRepo.Create(ctx, &Report{
		IncidentID: incID,
		Summary:    "test",
		Confidence: 0.8,
	})

	for i := 0; i < 3; i++ {
		_, _ = evidenceRepo.Create(ctx, &Evidence{
			ReportID:     reportID,
			EvidenceType: "log",
			Score:        0.8 + float64(i)*0.05,
			Payload:      `{"msg":"test"}`,
		})
	}

	items, err := evidenceRepo.ListByReport(ctx, reportID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestEvidenceRepo_ListByReport_Empty(t *testing.T) {
	db := setupTestDB(t)
	evidenceRepo := NewEvidenceRepo(db)
	ctx := context.Background()

	items, err := evidenceRepo.ListByReport(ctx, 9999)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}
