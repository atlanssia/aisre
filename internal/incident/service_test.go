package incident

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
	_ "modernc.org/sqlite"
)

func setupService(t *testing.T) *Service {
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
	if err := store.RunMigrations(db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	repo := store.NewIncidentRepo(db)
	return NewService(repo)
}

func TestService_CreateIncident(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	resp, err := svc.CreateIncident(ctx, contract.CreateIncidentRequest{
		Source:   "prometheus",
		Service:  "api-gateway",
		Severity: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.IncidentID <= 0 {
		t.Error("expected positive incident id")
	}
	if resp.Status != "open" {
		t.Errorf("expected open, got %s", resp.Status)
	}
}

func TestService_CreateIncident_InvalidSeverity(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	_, err := svc.CreateIncident(ctx, contract.CreateIncidentRequest{
		Source:   "test",
		Service:  "svc",
		Severity: "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid severity")
	}
}

func TestService_CreateIncident_EmptyService(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	_, err := svc.CreateIncident(ctx, contract.CreateIncidentRequest{
		Source:   "test",
		Severity: "low",
	})
	if err == nil {
		t.Error("expected error for empty service")
	}
}

func TestService_GetIncident(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	resp, _ := svc.CreateIncident(ctx, contract.CreateIncidentRequest{
		Source: "test", Service: "svc", Severity: "low",
	})

	got, err := svc.GetIncident(ctx, resp.IncidentID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ServiceName != "svc" {
		t.Errorf("expected svc, got %s", got.ServiceName)
	}
}

func TestService_GetIncident_NotFound(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	_, err := svc.GetIncident(ctx, 9999)
	if err == nil {
		t.Error("expected error for non-existent incident")
	}
}

func TestService_ListIncidents(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	_, _ = svc.CreateIncident(ctx, contract.CreateIncidentRequest{Source: "a", Service: "s1", Severity: "low"})
	_, _ = svc.CreateIncident(ctx, contract.CreateIncidentRequest{Source: "b", Service: "s2", Severity: "high"})

	items, err := svc.ListIncidents(ctx, store.IncidentFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2, got %d", len(items))
	}
}

func TestService_ProcessWebhook(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	resp, err := svc.ProcessWebhook(ctx, contract.WebhookPayload{
		Source:    "alertmanager",
		AlertName: "HighErrorRate",
		Service:   "payment-svc",
		Severity:  "critical",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.IncidentID <= 0 {
		t.Error("expected positive incident id")
	}
}

func TestService_ProcessWebhook_InvalidSeverity(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	_, err := svc.ProcessWebhook(ctx, contract.WebhookPayload{
		Source:   "test",
		Service:  "svc",
		Severity: "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid severity")
	}
}
