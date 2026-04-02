package store

import (
	"context"
	"testing"
)

func TestFeedbackRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	incRepo := NewIncidentRepo(db)
	reportRepo := NewReportRepo(db)
	feedbackRepo := NewFeedbackRepo(db)
	ctx := context.Background()

	incID, _ := incRepo.Create(ctx, &Incident{
		Source: "test", ServiceName: "svc", Severity: "high", Status: "open",
	})
	reportID, _ := reportRepo.Create(ctx, &Report{
		IncidentID: incID, Summary: "s", RootCause: "r", Confidence: 0.8, ReportJSON: "{}",
	})

	fb := &Feedback{
		ReportID:    reportID,
		UserID:      "user-1",
		Rating:      4,
		Comment:     "Helpful analysis",
		ActionTaken: "accepted",
	}
	id, err := feedbackRepo.Create(ctx, fb)
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}
}

func TestFeedbackRepo_ListByReport(t *testing.T) {
	db := setupTestDB(t)
	incRepo := NewIncidentRepo(db)
	reportRepo := NewReportRepo(db)
	feedbackRepo := NewFeedbackRepo(db)
	ctx := context.Background()

	incID, _ := incRepo.Create(ctx, &Incident{
		Source: "test", ServiceName: "svc", Severity: "high", Status: "open",
	})
	reportID, _ := reportRepo.Create(ctx, &Report{
		IncidentID: incID, Summary: "s", RootCause: "r", Confidence: 0.8, ReportJSON: "{}",
	})

	feedbackRepo.Create(ctx, &Feedback{
		ReportID: reportID, UserID: "u1", Rating: 5, ActionTaken: "accepted",
	})
	feedbackRepo.Create(ctx, &Feedback{
		ReportID: reportID, UserID: "u2", Rating: 3, ActionTaken: "partial",
	})

	items, err := feedbackRepo.ListByReport(ctx, reportID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestFeedbackRepo_ListByReport_Empty(t *testing.T) {
	db := setupTestDB(t)
	feedbackRepo := NewFeedbackRepo(db)
	ctx := context.Background()

	items, err := feedbackRepo.ListByReport(ctx, 9999)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestReportRepo_GetByID_IncludesStatus(t *testing.T) {
	db := setupTestDB(t)
	incRepo := NewIncidentRepo(db)
	repo := NewReportRepo(db)
	ctx := context.Background()

	incID, _ := incRepo.Create(ctx, &Incident{
		Source: "test", ServiceName: "svc", Severity: "high", Status: "open",
	})

	id, _ := repo.Create(ctx, &Report{
		IncidentID: incID, Summary: "s", RootCause: "r", Confidence: 0.9, ReportJSON: "{}",
	})

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "generated" {
		t.Errorf("expected status 'generated', got %q", got.Status)
	}
}

func TestReportRepo_List_IncludesStatus(t *testing.T) {
	db := setupTestDB(t)
	incRepo := NewIncidentRepo(db)
	repo := NewReportRepo(db)
	ctx := context.Background()

	incID, _ := incRepo.Create(ctx, &Incident{
		Source: "test", ServiceName: "svc", Severity: "high", Status: "open",
	})

	repo.Create(ctx, &Report{IncidentID: incID, Summary: "r1", RootCause: "rc1", Confidence: 0.8, ReportJSON: "{}"})

	items, err := repo.List(ctx, ReportFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Status != "generated" {
		t.Errorf("expected status 'generated', got %q", items[0].Status)
	}
}
