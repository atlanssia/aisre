package store

import (
	"context"
	"testing"
)

func TestChangeRepo_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := NewChangeRepo(db)
	ctx := context.Background()

	id, err := repo.Create(ctx, &Change{
		Service: "api-gw", ChangeType: "deploy", Summary: "Deploy v3.2.1",
		Author: "ci-bot", Timestamp: "2025-01-15T09:30:00Z", Metadata: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Fatal("expected positive ID")
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Service != "api-gw" {
		t.Errorf("service: got %q, want %q", got.Service, "api-gw")
	}
	if got.ChangeType != "deploy" {
		t.Errorf("change_type: got %q, want %q", got.ChangeType, "deploy")
	}
}

func TestChangeRepo_ListByFilter(t *testing.T) {
	db := setupTestDB(t)
	repo := NewChangeRepo(db)
	ctx := context.Background()

	changes := []*Change{
		{Service: "api-gw", ChangeType: "deploy", Summary: "Deploy v1", Timestamp: "2025-01-15T09:00:00Z", Metadata: "{}"},
		{Service: "api-gw", ChangeType: "config", Summary: "Config update", Timestamp: "2025-01-15T10:00:00Z", Metadata: "{}"},
		{Service: "payment", ChangeType: "deploy", Summary: "Deploy v2", Timestamp: "2025-01-15T11:00:00Z", Metadata: "{}"},
	}
	for _, ch := range changes {
		repo.Create(ctx, ch)
	}

	// Filter by service
	results, err := repo.List(ctx, ChangeFilter{Service: "api-gw"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for api-gw, got %d", len(results))
	}

	// Filter by change_type
	results, err = repo.List(ctx, ChangeFilter{ChangeTypes: []string{"deploy"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 deploy results, got %d", len(results))
	}
}

func TestChangeRepo_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewChangeRepo(db)

	_, err := repo.GetByID(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for non-existent change")
	}
}
