package store

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func setupEmbeddingTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := t.Name() + ".db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(dbPath)
	})
	if err := RunMigrations(db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestEmbeddingRepo_CreateAndGet(t *testing.T) {
	db := setupEmbeddingTestDB(t)
	repo := NewEmbeddingRepo(db)
	ctx := context.Background()

	// Need an incident first (FK constraint)
	incID, err := NewIncidentRepo(db).Create(ctx, &Incident{
		Source: "test", ServiceName: "api-gw", Severity: "critical", Status: "open",
	})
	if err != nil {
		t.Fatal(err)
	}

	embedding := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	err = repo.Create(ctx, incID, "api-gw", embedding, "text-embedding-test")
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByIncidentID(ctx, incID)
	if err != nil {
		t.Fatal(err)
	}
	if got.IncidentID != incID {
		t.Errorf("incident_id: got %d, want %d", got.IncidentID, incID)
	}
	if got.Service != "api-gw" {
		t.Errorf("service: got %q, want %q", got.Service, "api-gw")
	}
	if string(got.Embedding) != string(embedding) {
		t.Errorf("embedding bytes mismatch")
	}
}

func TestEmbeddingRepo_ListByService(t *testing.T) {
	db := setupEmbeddingTestDB(t)
	incRepo := NewIncidentRepo(db)
	embRepo := NewEmbeddingRepo(db)
	ctx := context.Background()

	// Create 3 incidents: 2 for api-gw, 1 for payment
	for i, svc := range []string{"api-gw", "api-gw", "payment"} {
		id, _ := incRepo.Create(ctx, &Incident{
			Source: "test", ServiceName: svc, Severity: "high", Status: "open",
		})
		embRepo.Create(ctx, id, svc, []byte{byte(i)}, "test-model")
	}

	results, err := embRepo.ListByService(ctx, "api-gw")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for api-gw, got %d", len(results))
	}
}

func TestEmbeddingRepo_GetByIncidentID_NotFound(t *testing.T) {
	db := setupEmbeddingTestDB(t)
	repo := NewEmbeddingRepo(db)

	_, err := repo.GetByIncidentID(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for non-existent embedding")
	}
}
