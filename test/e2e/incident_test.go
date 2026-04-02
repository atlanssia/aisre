package e2e

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/atlanssia/aisre/internal/api"
	"github.com/atlanssia/aisre/internal/incident"
	"github.com/atlanssia/aisre/internal/store"
	_ "modernc.org/sqlite"
)

func setupE2E(t *testing.T) http.Handler {
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
	svc := incident.NewService(repo)
	return api.NewRouter(svc)
}

func TestE2E_IncidentLifecycle(t *testing.T) {
	srv := setupE2E(t)

	// Step 1: Webhook creates incident
	t.Log("Step 1: POST /api/v1/alerts/webhook")
	body, _ := json.Marshal(map[string]any{
		"source":     "alertmanager",
		"alert_name": "HighErrorRate",
		"service":    "payment-svc",
		"severity":   "critical",
		"trace_id":   "abc-123",
	})
	req := httptest.NewRequest("POST", "/api/v1/alerts/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("webhook: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var webhookResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &webhookResp)
	if webhookResp["incident_id"] == nil {
		t.Fatal("expected incident_id in webhook response")
	}

	// Step 2: Get incident detail
	t.Log("Step 2: GET /api/v1/incidents/1")
	req = httptest.NewRequest("GET", "/api/v1/incidents/1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get incident: expected 200, got %d", w.Code)
	}

	var inc map[string]any
	json.Unmarshal(w.Body.Bytes(), &inc)
	if inc["service_name"] != "payment-svc" {
		t.Errorf("expected payment-svc, got %v", inc["service_name"])
	}
	if inc["severity"] != "critical" {
		t.Errorf("expected critical, got %v", inc["severity"])
	}
	if inc["status"] != "open" {
		t.Errorf("expected open, got %v", inc["status"])
	}

	// Step 3: Create another incident via API
	t.Log("Step 3: POST /api/v1/incidents")
	body, _ = json.Marshal(map[string]string{
		"source":   "prometheus",
		"service":  "api-gateway",
		"severity": "high",
	})
	req = httptest.NewRequest("POST", "/api/v1/incidents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create incident: expected 201, got %d", w.Code)
	}

	// Step 4: List incidents
	t.Log("Step 4: GET /api/v1/incidents")
	req = httptest.NewRequest("GET", "/api/v1/incidents", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list incidents: expected 200, got %d", w.Code)
	}

	var incidents []map[string]any
	json.Unmarshal(w.Body.Bytes(), &incidents)
	if len(incidents) != 2 {
		t.Errorf("expected 2 incidents, got %d", len(incidents))
	}

	// Step 5: Health check
	t.Log("Step 5: GET /health")
	req = httptest.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health: expected 200, got %d", w.Code)
	}

	t.Log("E2E lifecycle complete - all steps passed")
}

func TestE2E_WebhookWithInvalidData(t *testing.T) {
	srv := setupE2E(t)

	// Missing required fields
	body, _ := json.Marshal(map[string]any{
		"source": "test",
	})
	req := httptest.NewRequest("POST", "/api/v1/alerts/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	// Invalid severity
	body, _ = json.Marshal(map[string]any{
		"source":   "test",
		"service":  "svc",
		"severity": "extreme",
	})
	req = httptest.NewRequest("POST", "/api/v1/alerts/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid severity, got %d", w.Code)
	}
}

func TestE2E_IncidentNotFound(t *testing.T) {
	srv := setupE2E(t)

	req := httptest.NewRequest("GET", "/api/v1/incidents/9999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
