package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/atlanssia/aisre/internal/incident"
	"github.com/atlanssia/aisre/internal/store"
	_ "modernc.org/sqlite"
)

func setupAPI(t *testing.T) http.Handler {
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
	if err := store.RunMigrations(db, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	repo := store.NewIncidentRepo(db)
	svc := incident.NewService(repo)
	return NewRouter(svc)
}

func TestCreateIncident(t *testing.T) {
	router := setupAPI(t)

	body, _ := json.Marshal(map[string]string{
		"source":   "prometheus",
		"service":  "api-gateway",
		"severity": "high",
	})
	req := httptest.NewRequest("POST", "/api/v1/incidents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["incident_id"] == nil {
		t.Error("expected incident_id in response")
	}
}

func TestCreateIncident_InvalidBody(t *testing.T) {
	router := setupAPI(t)

	req := httptest.NewRequest("POST", "/api/v1/incidents", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateIncident_InvalidSeverity(t *testing.T) {
	router := setupAPI(t)

	body, _ := json.Marshal(map[string]string{
		"source": "test", "service": "svc", "severity": "bad",
	})
	req := httptest.NewRequest("POST", "/api/v1/incidents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetIncident(t *testing.T) {
	router := setupAPI(t)

	// Create first
	body, _ := json.Marshal(map[string]string{
		"source": "test", "service": "svc", "severity": "low",
	})
	req := httptest.NewRequest("POST", "/api/v1/incidents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Get
	req = httptest.NewRequest("GET", "/api/v1/incidents/1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var inc map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &inc)
	if inc["service_name"] != "svc" {
		t.Errorf("expected svc, got %v", inc["service_name"])
	}
}

func TestGetIncident_NotFound(t *testing.T) {
	router := setupAPI(t)

	req := httptest.NewRequest("GET", "/api/v1/incidents/9999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetIncident_InvalidID(t *testing.T) {
	router := setupAPI(t)

	req := httptest.NewRequest("GET", "/api/v1/incidents/abc", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListIncidents(t *testing.T) {
	router := setupAPI(t)

	// Create two incidents
	for _, svc := range []string{"s1", "s2"} {
		body, _ := json.Marshal(map[string]string{
			"source": "test", "service": svc, "severity": "low",
		})
		req := httptest.NewRequest("POST", "/api/v1/incidents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	req := httptest.NewRequest("GET", "/api/v1/incidents", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var items []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &items)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestListIncidents_Empty(t *testing.T) {
	router := setupAPI(t)

	req := httptest.NewRequest("GET", "/api/v1/incidents", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Should return empty array, not null
	if w.Body.String() == "null" {
		t.Error("expected empty array, got null")
	}
}

func TestWebhook(t *testing.T) {
	router := setupAPI(t)

	body, _ := json.Marshal(map[string]any{
		"source":     "alertmanager",
		"alert_name": "HighErrorRate",
		"service":    "payment-svc",
		"severity":   "critical",
	})
	req := httptest.NewRequest("POST", "/api/v1/alerts/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhook_InvalidPayload(t *testing.T) {
	router := setupAPI(t)

	body, _ := json.Marshal(map[string]any{
		"source": "test",
		// missing service
	})
	req := httptest.NewRequest("POST", "/api/v1/alerts/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHealthEndpoint(t *testing.T) {
	router := setupAPI(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != `{"status":"ok"}` {
		t.Errorf("unexpected health response: %s", w.Body.String())
	}
}
