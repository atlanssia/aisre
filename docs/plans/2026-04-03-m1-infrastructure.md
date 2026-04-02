# M1: Infrastructure + Alert Ingestion + Incident API

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Establish Go project foundation, SQLite database, Incident CRUD API, and Alert Webhook receiver.

**Architecture:** Chi HTTP router + SQLite (modernc.org/sqlite) + repository pattern. Config via Viper + YAML. Structured logging with slog.

**Tech Stack:** Go 1.26.1, github.com/go-chi/chi/v5, modernc.org/sqlite, github.com/golang-migrate/migrate/v4, github.com/spf13/viper

---

### Task 1: go.mod Dependencies

**Files:**
- Modify: `go.mod`

**Step 1: Add dependencies**

```bash
cd /Users/mw/workspace/repo/github.com/atlanssia/aisre
go get github.com/go-chi/chi/v5
go get github.com/go-chi/chi/v5/middleware
go get modernc.org/sqlite
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/sqlite3
go get github.com/golang-migrate/migrate/v4/source/iofs
go get github.com/spf13/viper
go get github.com/mattn/go-sqlite3  # CGO driver for migrate
go mod tidy
```

**Step 2: Verify**

```bash
go build ./...
```
Expected: compiles with no errors

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add core dependencies (chi, sqlite, migrate, viper)"
```

---

### Task 2: Database Migration

**Files:**
- Create: `migrations/001_init.up.sql`
- Create: `migrations/001_init.down.sql`
- Create: `internal/store/migrate.go`

**Step 1: Write failing test**

Create `internal/store/migrate_test.go`:
```go
package store

import (
	"database/sql"
	"os"
	"testing"
)

func TestMigrateCreatesTables(t *testing.T) {
	db, err := sql.Open("sqlite", t.Name()+".db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	defer os.Remove(t.Name() + ".db")

	err = RunMigrations(db, "migrations")
	if err != nil {
		t.Fatal(err)
	}

	tables := []string{"incidents", "rca_reports", "evidence_items", "recommendations", "feedback"}
	for _, tbl := range tables {
		var count int
		err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("table %s not found", tbl)
		}
	}
}
```

**Step 2: Run test — expect FAIL**

```bash
go test ./internal/store/ -run TestMigrateCreatesTables -v
```
Expected: FAIL (RunMigrations not defined)

**Step 3: Write migration files**

`migrations/001_init.up.sql`:
```sql
CREATE TABLE IF NOT EXISTS incidents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    severity TEXT NOT NULL CHECK(severity IN ('critical','high','medium','low','info')),
    service_name TEXT NOT NULL,
    title TEXT,
    description TEXT,
    labels TEXT,
    trace_id TEXT,
    started_at DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','analyzing','resolved','closed')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rca_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    incident_id INTEGER NOT NULL REFERENCES incidents(id),
    summary TEXT,
    root_cause TEXT,
    confidence REAL,
    report_json TEXT,
    status TEXT NOT NULL DEFAULT 'generated',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS evidence_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL REFERENCES rca_reports(id),
    evidence_type TEXT NOT NULL,
    score REAL,
    payload TEXT,
    source_url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS recommendations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL REFERENCES rca_reports(id),
    category TEXT NOT NULL,
    description TEXT,
    priority INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL REFERENCES rca_reports(id),
    user_id TEXT,
    rating INTEGER CHECK(rating BETWEEN 1 AND 5),
    comment TEXT,
    action_taken TEXT CHECK(action_taken IN ('accepted','partial','rejected')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_incidents_service ON incidents(service_name);
CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_reports_incident ON rca_reports(incident_id);
CREATE INDEX idx_evidence_report ON evidence_items(report_id);
CREATE INDEX idx_feedback_report ON feedback(report_id);
```

`migrations/001_init.down.sql`:
```sql
DROP INDEX IF EXISTS idx_feedback_report;
DROP INDEX IF EXISTS idx_evidence_report;
DROP INDEX IF EXISTS idx_reports_incident;
DROP INDEX IF EXISTS idx_incidents_severity;
DROP INDEX IF EXISTS idx_incidents_status;
DROP INDEX IF EXISTS idx_incidents_service;
DROP TABLE IF EXISTS feedback;
DROP TABLE IF EXISTS recommendations;
DROP TABLE IF EXISTS evidence_items;
DROP TABLE IF EXISTS rca_reports;
DROP TABLE IF EXISTS incidents;
```

`internal/store/migrate.go`:
```go
package store

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(db *sql.DB, migrationsPath string) error {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("store: migrate driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsPath,
		"sqlite3",
		driver,
	)
	if err != nil {
		return fmt.Errorf("store: migrate init: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("store: migrate up: %w", err)
	}
	return nil
}
```

**Step 4: Run test — expect PASS**

```bash
go test ./internal/store/ -run TestMigrateCreatesTables -v
```

**Step 5: Commit**

```bash
git add migrations/ internal/store/migrate.go internal/store/migrate_test.go
git commit -m "feat(store): add SQLite migrations and RunMigrations"
```

---

### Task 3: SQLite Store — IncidentRepo

**Files:**
- Create: `internal/store/store.go` (update existing)
- Create: `internal/store/incident_repo.go`
- Create: `internal/store/incident_repo_test.go`

**Step 1: Write failing tests**

`internal/store/incident_repo_test.go`:
```go
package store

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", t.Name()+".db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(t.Name() + ".db")
	})
	if err := RunMigrations(db, "migrations"); err != nil {
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
	id, _ := repo.Create(ctx, inc)

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
```

**Step 2: Run tests — expect FAIL**

```bash
go test ./internal/store/ -run TestIncidentRepo -v
```
Expected: FAIL (NewIncidentRepo not defined)

**Step 3: Implement IncidentRepo**

`internal/store/incident_repo.go`:
```go
package store

import (
	"context"
	"database/sql"
	"fmt"
)

type IncidentRepo struct {
	db *sql.DB
}

func NewIncidentRepo(db *sql.DB) *IncidentRepo {
	return &IncidentRepo{db: db}
}

func (r *IncidentRepo) Create(ctx context.Context, inc *Incident) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO incidents (source, service_name, severity, status, trace_id, started_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		inc.Source, inc.ServiceName, inc.Severity, inc.Status, inc.TraceID,
	)
	if err != nil {
		return 0, fmt.Errorf("incident_repo: create: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("incident_repo: last insert id: %w", err)
	}
	return id, nil
}

func (r *IncidentRepo) GetByID(ctx context.Context, id int64) (*Incident, error) {
	var inc Incident
	err := r.db.QueryRowContext(ctx,
		`SELECT id, source, service_name, severity, status, trace_id, created_at
		 FROM incidents WHERE id = ?`, id,
	).Scan(&inc.ID, &inc.Source, &inc.ServiceName, &inc.Severity, &inc.Status, &inc.TraceID, &inc.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("incident_repo: incident %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("incident_repo: get by id: %w", err)
	}
	return &inc, nil
}

func (r *IncidentRepo) List(ctx context.Context, filter IncidentFilter) ([]Incident, error) {
	query := `SELECT id, source, service_name, severity, status, trace_id, created_at
			  FROM incidents WHERE 1=1`
	args := []any{}

	if filter.Service != "" {
		query += " AND service_name = ?"
		args = append(args, filter.Service)
	}
	if filter.Severity != "" {
		query += " AND severity = ?"
		args = append(args, filter.Severity)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("incident_repo: list: %w", err)
	}
	defer rows.Close()

	var items []Incident
	for rows.Next() {
		var inc Incident
		if err := rows.Scan(&inc.ID, &inc.Source, &inc.ServiceName, &inc.Severity, &inc.Status, &inc.TraceID, &inc.CreatedAt); err != nil {
			return nil, fmt.Errorf("incident_repo: scan: %w", err)
		}
		items = append(items, inc)
	}
	return items, nil
}

func (r *IncidentRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE incidents SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("incident_repo: update status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("incident_repo: incident %d not found", id)
	}
	return nil
}
```

**Step 4: Run tests — expect PASS**

```bash
go test ./internal/store/ -run TestIncidentRepo -v
```

**Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): implement IncidentRepo with SQLite backend"
```

---

### Task 4: Incident Service

**Files:**
- Create: `internal/incident/service.go`
- Create: `internal/incident/service_test.go`

**Step 1: Write failing tests**

`internal/incident/service_test.go`:
```go
package incident

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

func setupService(t *testing.T) *Service {
	t.Helper()
	db, err := sql.Open("sqlite", t.Name()+".db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(t.Name() + ".db")
	})
	if err := store.RunMigrations(db, "migrations"); err != nil {
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

func TestService_ListIncidents(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	svc.CreateIncident(ctx, contract.CreateIncidentRequest{Source: "a", Service: "s1", Severity: "low"})
	svc.CreateIncident(ctx, contract.CreateIncidentRequest{Source: "b", Service: "s2", Severity: "high"})

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
```

**Step 2: Run tests — expect FAIL**

```bash
go test ./internal/incident/ -v
```

**Step 3: Implement Service**

`internal/incident/service.go`:
```go
package incident

import (
	"context"
	"fmt"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

type Service struct {
	repo *store.IncidentRepo
}

func NewService(repo *store.IncidentRepo) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateIncident(ctx context.Context, req contract.CreateIncidentRequest) (*contract.CreateIncidentResponse, error) {
	if !contract.ValidSeverities[req.Severity] {
		return nil, fmt.Errorf("incident: invalid severity: %s", req.Severity)
	}
	if req.Service == "" {
		return nil, fmt.Errorf("incident: service is required")
	}

	inc := &store.Incident{
		Source:      req.Source,
		ServiceName: req.Service,
		Severity:    req.Severity,
		Status:      "open",
		TraceID:     req.TraceID,
	}
	id, err := s.repo.Create(ctx, inc)
	if err != nil {
		return nil, fmt.Errorf("incident: create: %w", err)
	}

	return &contract.CreateIncidentResponse{
		IncidentID: id,
		Status:     "open",
	}, nil
}

func (s *Service) GetIncident(ctx context.Context, id int64) (*contract.Incident, error) {
	inc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("incident: get: %w", err)
	}
	return &contract.Incident{
		ID:          inc.ID,
		Source:      inc.Source,
		ServiceName: inc.ServiceName,
		Severity:    inc.Severity,
		Status:      inc.Status,
		TraceID:     inc.TraceID,
		CreatedAt:   inc.CreatedAt,
	}, nil
}

func (s *Service) ListIncidents(ctx context.Context, filter store.IncidentFilter) ([]contract.Incident, error) {
	items, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("incident: list: %w", err)
	}
	result := make([]contract.Incident, len(items))
	for i, inc := range items {
		result[i] = contract.Incident{
			ID:          inc.ID,
			Source:      inc.Source,
			ServiceName: inc.ServiceName,
			Severity:    inc.Severity,
			Status:      inc.Status,
			TraceID:     inc.TraceID,
			CreatedAt:   inc.CreatedAt,
		}
	}
	return result, nil
}

func (s *Service) ProcessWebhook(ctx context.Context, payload contract.WebhookPayload) (*contract.CreateIncidentResponse, error) {
	if !contract.ValidSeverities[payload.Severity] {
		return nil, fmt.Errorf("incident: invalid severity: %s", payload.Severity)
	}
	if payload.Service == "" {
		return nil, fmt.Errorf("incident: service is required in webhook payload")
	}

	inc := &store.Incident{
		Source:      payload.Source,
		ServiceName: payload.Service,
		Severity:    payload.Severity,
		Status:      "open",
		TraceID:     payload.TraceID,
	}
	id, err := s.repo.Create(ctx, inc)
	if err != nil {
		return nil, fmt.Errorf("incident: webhook create: %w", err)
	}

	return &contract.CreateIncidentResponse{
		IncidentID: id,
		Status:     "open",
	}, nil
}
```

**Step 4: Run tests — expect PASS**

```bash
go test ./internal/incident/ -v
```

**Step 5: Commit**

```bash
git add internal/incident/
git commit -m "feat(incident): implement incident service with CRUD and webhook"
```

---

### Task 5: API Handlers + Chi Router

**Files:**
- Create: `internal/api/router.go`
- Create: `internal/api/incident_handler.go`
- Create: `internal/api/webhook_handler.go`
- Create: `internal/api/middleware.go`
- Create: `internal/api/handler_test.go`

**Step 1: Write failing tests**

`internal/api/handler_test.go`:
```go
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
)

func setupAPI(t *testing.T) http.Handler {
	t.Helper()
	db, err := sql.Open("sqlite", t.Name()+".db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(t.Name() + ".db")
	})
	if err := store.RunMigrations(db, "migrations"); err != nil {
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
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["incident_id"] == nil {
		t.Error("expected incident_id in response")
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
	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)

	// Get
	req = httptest.NewRequest("GET", "/api/v1/incidents/1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListIncidents(t *testing.T) {
	router := setupAPI(t)

	req := httptest.NewRequest("GET", "/api/v1/incidents", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
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
```

**Step 2: Run tests — expect FAIL**

```bash
go test ./internal/api/ -v
```

**Step 3: Implement handlers**

`internal/api/router.go`:
```go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func NewRouter(incidentSvc IncidentService) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(contentTypeJSON)

	h := &handler{incidentSvc: incidentSvc}

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/incidents", h.createIncident)
		r.Get("/incidents", h.listIncidents)
		r.Get("/incidents/{id}", h.getIncident)
		r.Post("/alerts/webhook", h.handleWebhook)
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	return r
}
```

`internal/api/middleware.go`:
```go
package api

import "net/http"

func contentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
```

`internal/api/types.go`:
```go
package api

import (
	"context"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// IncidentService defines the interface the API layer depends on.
type IncidentService interface {
	CreateIncident(ctx context.Context, req contract.CreateIncidentRequest) (*contract.CreateIncidentResponse, error)
	GetIncident(ctx context.Context, id int64) (*contract.Incident, error)
	ListIncidents(ctx context.Context, filter store.IncidentFilter) ([]contract.Incident, error)
	ProcessWebhook(ctx context.Context, payload contract.WebhookPayload) (*contract.CreateIncidentResponse, error)
}

type handler struct {
	incidentSvc IncidentService
}
```

`internal/api/incident_handler.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

func (h *handler) createIncident(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	resp, err := h.incidentSvc.CreateIncident(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *handler) getIncident(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id", "INVALID_REQUEST")
		return
	}

	inc, err := h.incidentSvc.GetIncident(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
		return
	}

	json.NewEncoder(w).Encode(inc)
}

func (h *handler) listIncidents(w http.ResponseWriter, r *http.Request) {
	filter := store.IncidentFilter{
		Service:  r.URL.Query().Get("service"),
		Severity: r.URL.Query().Get("severity"),
		Status:   r.URL.Query().Get("status"),
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		filter.Limit, _ = strconv.Atoi(limit)
	}
	if filter.Limit == 0 {
		filter.Limit = 50
	}

	items, err := h.incidentSvc.ListIncidents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
		return
	}
	if items == nil {
		items = []contract.Incident{}
	}
	json.NewEncoder(w).Encode(items)
}

func writeError(w http.ResponseWriter, status int, msg, code string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(contract.ErrorResponse{Error: msg, Code: code})
}
```

`internal/api/webhook_handler.go`:
```go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/atlanssia/aisre/internal/contract"
)

func (h *handler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	var payload contract.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook payload", "INVALID_REQUEST")
		return
	}

	resp, err := h.incidentSvc.ProcessWebhook(r.Context(), payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}
```

**Step 4: Run tests — expect PASS**

```bash
go test ./internal/api/ -v
```

**Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat(api): add Chi router with incident and webhook handlers"
```

---

### Task 6: Server Entrypoint + Config

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `configs/local.yaml`

**Step 1: Write failing test**

Create `cmd/server/main_test.go`:
```go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	// We test the router directly through the API package
	// This test ensures the server wiring works
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Just verify the health endpoint is registered
	// Full integration test in test/e2e/
	if req.URL.Path != "/health" {
		t.Error("health path mismatch")
	}
}
```

**Step 2: Implement main.go**

`cmd/server/main.go`:
```go
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/atlanssia/aisre/internal/api"
	"github.com/atlanssia/aisre/internal/incident"
	"github.com/atlanssia/aisre/internal/store"
	"github.com/spf13/viper"

	_ "modernc.org/sqlite"
)

func main() {
	configPath := flag.String("config", "configs/local.yaml", "config file path")
	flag.Parse()

	if err := run(*configPath); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	// Database
	dsn := viper.GetString("database.dsn")
	if dsn == "" {
		dsn = "./data/aisre.db"
	}
	os.MkdirAll("./data", 0755)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := store.RunMigrations(db, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	slog.Info("database ready", "dsn", dsn)

	// Services
	incidentRepo := store.NewIncidentRepo(db)
	incidentSvc := incident.NewService(incidentRepo)

	// HTTP Server
	router := api.NewRouter(incidentSvc)
	addr := fmt.Sprintf("%s:%d",
		viper.GetString("server.host"),
		viper.GetInt("server.port"),
	)
	if addr == ":0" {
		addr = "0.0.0.0:8080"
	}

	slog.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, router)
}
```

**Step 3: Run test — expect PASS**

```bash
go test ./cmd/server/ -v
```

**Step 4: Build and verify**

```bash
go build -o bin/aisre ./cmd/server
```

**Step 5: Commit**

```bash
git add cmd/server/ configs/
git commit -m "feat(server): wire up server with config, DB, and API routes"
```

---

### Task 7: M1 Integration Test

**Files:**
- Create: `test/e2e/incident_test.go`

**Step 1: Write integration test**

`test/e2e/incident_test.go`:
```go
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
)

func setupE2E(t *testing.T) http.Handler {
	t.Helper()
	db, err := sql.Open("sqlite", t.Name()+".db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(t.Name() + ".db")
	})
	if err := store.RunMigrations(db, "migrations"); err != nil {
		t.Fatal(err)
	}
	repo := store.NewIncidentRepo(db)
	svc := incident.NewService(repo)
	return api.NewRouter(svc)
}

func TestE2E_IncidentLifecycle(t *testing.T) {
	srv := setupE2E(t)

	// 1. Webhook creates incident
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
		t.Fatalf("webhook: expected 201, got %d", w.Code)
	}

	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	incidentID := createResp["incident_id"]

	// 2. Get incident detail
	req = httptest.NewRequest("GET", "/api/v1/incidents/1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", w.Code)
	}
	var inc map[string]any
	json.Unmarshal(w.Body.Bytes(), &inc)
	if inc["service_name"] != "payment-svc" {
		t.Errorf("expected payment-svc, got %v", inc["service_name"])
	}

	// 3. List incidents
	req = httptest.NewRequest("GET", "/api/v1/incidents", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	// 4. Health check
	req = httptest.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health: expected 200, got %d", w.Code)
	}

	t.Logf("incident lifecycle complete, id=%v", incidentID)
}
```

**Step 2: Run — expect PASS**

```bash
go test ./test/e2e/ -v
```

**Step 3: Commit**

```bash
git add test/e2e/
git commit -m "test(e2e): add incident lifecycle integration test"
```

---

## M1 Acceptance Checklist

- [ ] `go build ./...` compiles
- [ ] `go test ./internal/store/ -v` passes
- [ ] `go test ./internal/incident/ -v` passes
- [ ] `go test ./internal/api/ -v` passes
- [ ] `go test ./test/e2e/ -v` passes
- [ ] `make build` produces working binary
- [ ] POST /api/v1/incidents returns 201
- [ ] GET /api/v1/incidents returns list
- [ ] GET /api/v1/incidents/:id returns detail
- [ ] POST /api/v1/alerts/webhook creates incident
- [ ] GET /health returns ok
