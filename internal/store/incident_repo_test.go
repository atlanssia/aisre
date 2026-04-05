package store

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := t.Name() + ".db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		os.Remove(dbPath)
	})
	if err := RunMigrations(db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestIncidentRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIncidentRepo(db)
	ctx := context.Background()

	inc := &Incident{
		Source:      "prometheus",
		ServiceName: "api-gateway",
		Severity:    "high",
		Status:      "open",
		TraceID:     "trace-123",
	}
	id, err := repo.Create(ctx, inc)
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}
}

func TestIncidentRepo_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIncidentRepo(db)
	ctx := context.Background()

	inc := &Incident{
		Source:      "prometheus",
		ServiceName: "api-gateway",
		Severity:    "high",
		Status:      "open",
	}
	id, err := repo.Create(ctx, inc)

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ServiceName != "api-gateway" {
		t.Errorf("expected api-gateway, got %s", got.ServiceName)
	}
	if got.Source != "prometheus" {
		t.Errorf("expected prometheus, got %s", got.Source)
	}
}

func TestIncidentRepo_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIncidentRepo(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, 9999)
	if err == nil {
		t.Error("expected error for non-existent id")
	}
}

func TestIncidentRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIncidentRepo(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		repo.Create(ctx, &Incident{
			Source:      "prometheus",
			ServiceName: "svc-" + string(rune('a'+i)),
			Severity:    "high",
			Status:      "open",
		})
	}

	items, err := repo.List(ctx, IncidentFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestIncidentRepo_List_FilterByService(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIncidentRepo(db)
	ctx := context.Background()

	repo.Create(ctx, &Incident{Source: "test", ServiceName: "api-gateway", Severity: "high", Status: "open"})
	repo.Create(ctx, &Incident{Source: "test", ServiceName: "payment-svc", Severity: "low", Status: "open"})

	items, err := repo.List(ctx, IncidentFilter{Service: "api-gateway", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestIncidentRepo_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewIncidentRepo(db)
	ctx := context.Background()

	id, _ := repo.Create(ctx, &Incident{
		Source: "test", ServiceName: "svc", Severity: "low", Status: "open",
	})

	err := repo.UpdateStatus(ctx, id, "analyzing")
	if err != nil {
		t.Fatal(err)
	}

	got, _ := repo.GetByID(ctx, id)
	if got.Status != "analyzing" {
		t.Errorf("expected analyzing, got %s", got.Status)
	}
}
