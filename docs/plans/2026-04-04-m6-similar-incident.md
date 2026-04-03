# M6: Similar Incident Retrieval Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为每个 Incident 计算文本 embedding 并检索历史相似事件，在 RCA 分析时注入相似事件上下文。

**Architecture:** 独立的 `EmbeddingClient`（独立配置）→ `internal/similar/` service → SQLite 存储 embedding → 余弦相似度检索 → 注入 `PromptInput.SimilarRCA` 到 RCA Pipeline。

**Tech Stack:** Go 1.26.1 + SQLite (BLOB embedding 存储) + OpenAI-compatible Embedding API + `encoding/binary` (紧凑向量格式)

---

## Task 1: Migration 003 — incident_embeddings table

**Files:**
- Create: `migrations/003_incident_embeddings.up.sql`
- Create: `migrations/003_incident_embeddings.down.sql`

**Step 1: Write the up migration**

```sql
-- migrations/003_incident_embeddings.up.sql
CREATE TABLE IF NOT EXISTS incident_embeddings (
    incident_id INTEGER NOT NULL,
    service     TEXT NOT NULL DEFAULT '',
    embedding   BLOB NOT NULL,
    model       TEXT NOT NULL DEFAULT 'text-embedding-3-small',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (incident_id),
    FOREIGN KEY (incident_id) REFERENCES incidents(id)
);
CREATE INDEX IF NOT EXISTS idx_embeddings_service ON incident_embeddings(service);
```

**Step 2: Write the down migration**

```sql
-- migrations/003_incident_embeddings.down.sql
DROP INDEX IF EXISTS idx_embeddings_service;
DROP TABLE IF EXISTS incident_embeddings;
```

**Step 3: Verify migration runs**

Run: `cd /Users/mw/workspace/repo/github.com/atlanssia/aisre && go test ./internal/store/... -run TestRunMigrations -v`
Expected: PASS

**Step 4: Commit**

```bash
git add migrations/003_incident_embeddings.up.sql migrations/003_incident_embeddings.down.sql
git commit -m "feat(similar): add incident_embeddings migration (003)"
```

---

## Task 2: Contract DTOs — similar types

**Files:**
- Create: `internal/contract/similar.go`
- Test: `test/contract/similar_test.go`

**Step 1: Write the failing test**

File: `test/contract/similar_test.go`

```go
package contract_test

import (
	"encoding/json"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
)

func TestSimilarResult_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		result contract.SimilarResult
	}{
		{
			name: "full result",
			result: contract.SimilarResult{
				IncidentID: 42,
				Similarity: 0.87,
				Summary:    "Redis connection pool exhaustion",
				RootCause:  "max_connections=50 too low for traffic spike",
				Service:    "api-gateway",
				Severity:   "critical",
			},
		},
		{
			name: "zero similarity",
			result: contract.SimilarResult{
				IncidentID: 1,
				Similarity: 0.0,
				Summary:    "",
				RootCause:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.result)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var got contract.SimilarResult
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if got.IncidentID != tt.result.IncidentID {
				t.Errorf("IncidentID: got %d, want %d", got.IncidentID, tt.result.IncidentID)
			}
			if got.Similarity != tt.result.Similarity {
				t.Errorf("Similarity: got %f, want %f", got.Similarity, tt.result.Similarity)
			}
			if got.Summary != tt.result.Summary {
				t.Errorf("Summary: got %q, want %q", got.Summary, tt.result.Summary)
			}
		})
	}
}

func TestEmbedRequest_JSONRoundTrip(t *testing.T) {
	req := contract.EmbedRequest{Text: "api-gateway timeout error"}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"text":"api-gateway timeout error"}`
	if string(b) != want {
		t.Errorf("got %s, want %s", string(b), want)
	}
}

func TestSimilarQuery_JSONFields(t *testing.T) {
	q := contract.SimilarQuery{TopK: 5, Threshold: 0.7}
	b, _ := json.Marshal(q)
	var m map[string]any
	json.Unmarshal(b, &m)

	if m["top_k"] != float64(5) {
		t.Errorf("top_k: got %v, want 5", m["top_k"])
	}
	if m["threshold"] != 0.7 {
		t.Errorf("threshold: got %v, want 0.7", m["threshold"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./test/contract/... -run TestSimilarResult -v`
Expected: FAIL — `contract.SimilarResult` undefined

**Step 3: Write minimal implementation**

File: `internal/contract/similar.go`

```go
package contract

// SimilarResult represents a matched similar incident.
type SimilarResult struct {
	IncidentID int64   `json:"incident_id"`
	Similarity float64 `json:"similarity"`
	Summary    string  `json:"summary"`
	RootCause  string  `json:"root_cause"`
	Service    string  `json:"service"`
	Severity   string  `json:"severity"`
}

// EmbedRequest is the request to compute embedding for an incident.
type EmbedRequest struct {
	Text string `json:"text"`
}

// SimilarQuery holds parameters for similar incident search.
type SimilarQuery struct {
	TopK      int     `json:"top_k"`
	Threshold float64 `json:"threshold"`
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./test/contract/... -run "TestSimilarResult|TestEmbedRequest|TestSimilarQuery" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/contract/similar.go test/contract/similar_test.go
git commit -m "feat(similar): add contract DTOs for similar incident"
```

---

## Task 3: Embedding Client — independent HTTP client for embedding API

**Files:**
- Create: `internal/analysis/embedding_client.go`
- Create: `internal/analysis/embedding_client_test.go`

**Step 1: Write the failing test**

File: `internal/analysis/embedding_client_test.go`

```go
package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbeddingClient_Embed(t *testing.T) {
	t.Run("successful embedding", func(t *testing.T) {
		wantDims := 4
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/embeddings" {
				t.Errorf("expected /embeddings, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer emb-key-123" {
				t.Errorf("expected Bearer emb-key-123, got %s", r.Header.Get("Authorization"))
			}

			// Verify request body
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			if req["model"] != "text-embedding-test" {
				t.Errorf("model: got %v, want text-embedding-test", req["model"])
			}

			resp := map[string]any{
				"data": []map[string]any{
					{"embedding": []float64{0.1, 0.2, 0.3, 0.4}, "index": 0},
					{"embedding": []float64{0.5, 0.6, 0.7, 0.8}, "index": 1},
				},
				"model": "text-embedding-test",
				"usage": map[string]any{"prompt_tokens": 10, "total_tokens": 10},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		cfg := EmbeddingConfig{
			BaseURL:    server.URL,
			APIKey:     "emb-key-123",
			Model:      "text-embedding-test",
			Dimensions: wantDims,
		}
		client := NewEmbeddingClient(cfg)

		vecs, err := client.Embed(context.Background(), []string{"hello", "world"})
		if err != nil {
			t.Fatal(err)
		}
		if len(vecs) != 2 {
			t.Fatalf("expected 2 vectors, got %d", len(vecs))
		}
		if len(vecs[0]) != wantDims {
			t.Errorf("vector 0 dims: got %d, want %d", len(vecs[0]), wantDims)
		}
		if vecs[0][0] != 0.1 {
			t.Errorf("vector 0[0]: got %f, want 0.1", vecs[0][0])
		}
	})

	t.Run("API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "rate limited"}`))
		}))
		defer server.Close()

		cfg := EmbeddingConfig{BaseURL: server.URL, APIKey: "key", Model: "test"}
		client := NewEmbeddingClient(cfg)

		_, err := client.Embed(context.Background(), []string{"test"})
		if err == nil {
			t.Fatal("expected error for 429 response")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		cfg := EmbeddingConfig{BaseURL: "http://localhost", APIKey: "key", Model: "test"}
		client := NewEmbeddingClient(cfg)

		vecs, err := client.Embed(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(vecs) != 0 {
			t.Errorf("expected 0 vectors for nil input, got %d", len(vecs))
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/analysis/... -run TestEmbeddingClient -v`
Expected: FAIL — `EmbeddingConfig` undefined

**Step 3: Write minimal implementation**

File: `internal/analysis/embedding_client.go`

```go
package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// EmbeddingConfig holds configuration for the embedding API client.
// This is independent from LLMConfig — different provider, endpoint, and credentials.
type EmbeddingConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	Dimensions int
}

// embeddingRequest is the request body for the embeddings API.
type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embeddingResponse is the response from the embeddings API.
type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// EmbeddingClient calls OpenAI-compatible embedding APIs.
type EmbeddingClient struct {
	cfg  EmbeddingConfig
	http *http.Client
}

// NewEmbeddingClient creates a new embedding client with the given configuration.
func NewEmbeddingClient(cfg EmbeddingConfig) *EmbeddingClient {
	return &EmbeddingClient{
		cfg: cfg,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Embed calls the embedding API and returns vectors for the given texts.
func (c *EmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := embeddingRequest{
		Model: c.cfg.Model,
		Input: texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("embedding_client: marshal request: %w", err)
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding_client: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding_client: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("embedding_client: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding_client: API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("embedding_client: unmarshal response: %w", err)
	}

	result := make([][]float64, len(embResp.Data))
	for i, d := range embResp.Data {
		result[d.Index] = d.Embedding
	}
	return result, nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/analysis/... -run TestEmbeddingClient -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/analysis/embedding_client.go internal/analysis/embedding_client_test.go
git commit -m "feat(similar): add independent EmbeddingClient for embedding API"
```

---

## Task 4: Vector utilities — binary encoding + cosine similarity

**Files:**
- Create: `internal/similar/vector.go`
- Create: `internal/similar/vector_test.go`

**Step 1: Write the failing test**

File: `internal/similar/vector_test.go`

```go
package similar

import (
	"math"
	"testing"
)

func TestEncodeDecodeVector(t *testing.T) {
	original := []float64{0.1, -0.2, 0.3, 0.0, 1.5}

	encoded := EncodeVector(original)
	if len(encoded) != len(original)*8 {
		t.Fatalf("encoded length: got %d, want %d", len(encoded), len(original)*8)
	}

	decoded := DecodeVector(encoded)
	if len(decoded) != len(original) {
		t.Fatalf("decoded length: got %d, want %d", len(decoded), len(original))
	}
	for i := range original {
		diff := math.Abs(decoded[i] - original[i])
		if diff > 1e-10 {
			t.Errorf("decoded[%d]: got %f, want %f (diff %e)", i, decoded[i], original[i], diff)
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"identical", []float64{1.0, 0.0, 0.0}, []float64{1.0, 0.0, 0.0}, 1.0},
		{"orthogonal", []float64{1.0, 0.0}, []float64{0.0, 1.0}, 0.0},
		{"opposite", []float64{1.0, 0.0}, []float64{-1.0, 0.0}, -1.0},
		{"partial", []float64{1.0, 1.0}, []float64{1.0, 0.0}, 1.0 / math.Sqrt2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > 1e-10 {
				t.Errorf("got %f, want %f", got, tt.want)
			}
		})
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	got := CosineSimilarity([]float64{0, 0, 0}, []float64{1, 2, 3})
	if got != 0.0 {
		t.Errorf("expected 0 for zero vector, got %f", got)
	}
}

func TestCosineSimilarity_LengthMismatch(t *testing.T) {
	got := CosineSimilarity([]float64{1.0}, []float64{1.0, 2.0})
	if got != 0.0 {
		t.Errorf("expected 0 for mismatched lengths, got %f", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/similar/... -v`
Expected: FAIL — package doesn't exist

**Step 3: Write minimal implementation**

File: `internal/similar/vector.go`

```go
package similar

import (
	"encoding/binary"
	"math"
)

// EncodeVector encodes a float64 slice to a compact binary representation.
// Uses encoding/binary.LittleEndian — 8 bytes per float64, ~3x smaller than JSON.
func EncodeVector(v []float64) []byte {
	buf := make([]byte, len(v)*8)
	for i, f := range v {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(f))
	}
	return buf
}

// DecodeVector decodes a binary representation back to a float64 slice.
func DecodeVector(buf []byte) []float64 {
	n := len(buf) / 8
	v := make([]float64, n)
	for i := range v {
		v[i] = math.Float64frombits(binary.LittleEndian.Uint64(buf[i*8:]))
	}
	return v
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 for zero vectors or length mismatch.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

**Step 4: Run tests**

Run: `go test ./internal/similar/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/similar/vector.go internal/similar/vector_test.go
git commit -m "feat(similar): add vector encoding and cosine similarity"
```

---

## Task 5: Embedding Repository — SQLite storage for embeddings

**Files:**
- Modify: `internal/store/store.go` — add EmbeddingRepo interface + entity
- Create: `internal/store/embedding_repo.go`
- Create: `internal/store/embedding_repo_test.go`

**Step 1: Write the failing test**

File: `internal/store/embedding_repo_test.go`

```go
package store

import (
	"context"
	"testing"
)

func TestEmbeddingRepo_CreateAndGet(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
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
	db, cleanup := newTestDB(t)
	defer cleanup()
	incRepo := NewIncidentRepo(db)
	embRepo := NewEmbeddingRepo(db)
	ctx := context.Background()

	// Create 3 incidents: 2 for api-gw, 1 for payment
	ids := make([]int64, 3)
	for i, svc := range []string{"api-gw", "api-gw", "payment"} {
		id, _ := incRepo.Create(ctx, &Incident{
			Source: "test", ServiceName: svc, Severity: "high", Status: "open",
		})
		ids[i] = id
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
	db, cleanup := newTestDB(t)
	defer cleanup()
	repo := NewEmbeddingRepo(db)

	_, err := repo.GetByIncidentID(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for non-existent embedding")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -run TestEmbeddingRepo -v`
Expected: FAIL — `EmbeddingRepo` undefined

**Step 3: Add interface + entity to store.go**

Add to `internal/store/store.go`:

```go
// EmbeddingRepo defines the persistence interface for incident embeddings.
type EmbeddingRepo interface {
	Create(ctx context.Context, incidentID int64, service string, embedding []byte, model string) error
	GetByIncidentID(ctx context.Context, incidentID int64) (*Embedding, error)
	ListByService(ctx context.Context, service string) ([]Embedding, error)
}

// Embedding is the persistent embedding entity.
type Embedding struct {
	IncidentID int64
	Service    string
	Embedding  []byte
	Model      string
	CreatedAt  string
}
```

**Step 4: Write implementation**

File: `internal/store/embedding_repo.go`

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
)

type sqliteEmbeddingRepo struct {
	db *sql.DB
}

// NewEmbeddingRepo creates a new EmbeddingRepo backed by SQLite.
func NewEmbeddingRepo(db *sql.DB) EmbeddingRepo {
	return &sqliteEmbeddingRepo{db: db}
}

func (r *sqliteEmbeddingRepo) Create(ctx context.Context, incidentID int64, service string, embedding []byte, model string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO incident_embeddings (incident_id, service, embedding, model)
		 VALUES (?, ?, ?, ?)`,
		incidentID, service, embedding, model,
	)
	if err != nil {
		return fmt.Errorf("embedding_repo: create: %w", err)
	}
	return nil
}

func (r *sqliteEmbeddingRepo) GetByIncidentID(ctx context.Context, incidentID int64) (*Embedding, error) {
	var e Embedding
	err := r.db.QueryRowContext(ctx,
		`SELECT incident_id, service, embedding, model, created_at
		 FROM incident_embeddings WHERE incident_id = ?`, incidentID,
	).Scan(&e.IncidentID, &e.Service, &e.Embedding, &e.Model, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("embedding_repo: embedding for incident %d not found", incidentID)
	}
	if err != nil {
		return nil, fmt.Errorf("embedding_repo: get: %w", err)
	}
	return &e, nil
}

func (r *sqliteEmbeddingRepo) ListByService(ctx context.Context, service string) ([]Embedding, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT incident_id, service, embedding, model, created_at
		 FROM incident_embeddings WHERE service = ?`, service,
	)
	if err != nil {
		return nil, fmt.Errorf("embedding_repo: list by service: %w", err)
	}
	defer rows.Close()

	var results []Embedding
	for rows.Next() {
		var e Embedding
		if err := rows.Scan(&e.IncidentID, &e.Service, &e.Embedding, &e.Model, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("embedding_repo: scan: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}
```

**Step 5: Run tests**

Run: `go test ./internal/store/... -run TestEmbeddingRepo -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/store/store.go internal/store/embedding_repo.go internal/store/embedding_repo_test.go
git commit -m "feat(similar): add EmbeddingRepo for SQLite embedding storage"
```

---

## Task 6: Similar Service — core business logic

**Files:**
- Create: `internal/similar/service.go`
- Create: `internal/similar/service_test.go`

**Step 1: Write the failing test**

File: `internal/similar/service_test.go`

```go
package similar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atlanssia/aisre/internal/analysis"
	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

func TestService_ComputeEmbedding(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	incRepo := store.NewIncidentRepo(db)
	embRepo := store.NewEmbeddingRepo(db)
	reportRepo := store.NewReportRepo(db)
	ctx := context.Background()

	// Create incident + report with summary
	incID, _ := incRepo.Create(ctx, &store.Incident{
		Source: "test", ServiceName: "api-gw", Severity: "critical", Status: "open",
	})
	reportJSON, _ := json.Marshal(map[string]any{
		"summary": "Redis connection pool exhaustion", "root_cause": "pool too small",
	})
	reportRepo.Create(ctx, &store.Report{
		IncidentID: incID, Summary: "Redis pool exhaustion",
		RootCause: "pool too small", ReportJSON: string(reportJSON), Status: "generated",
	})

	// Mock embedding server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"embedding": []float64{0.1, 0.2, 0.3}, "index": 0},
			},
			"model": "text-embedding-test",
			"usage": map[string]any{"prompt_tokens": 5, "total_tokens": 5},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embClient := analysis.NewEmbeddingClient(analysis.EmbeddingConfig{
		BaseURL: server.URL, APIKey: "test", Model: "text-embedding-test",
	})

	svc := NewService(embClient, embRepo, incRepo, reportRepo)

	err := svc.ComputeEmbedding(ctx, incID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify stored
	emb, err := embRepo.GetByIncidentID(ctx, incID)
	if err != nil {
		t.Fatal(err)
	}
	if emb.Service != "api-gw" {
		t.Errorf("service: got %q, want %q", emb.Service, "api-gw")
	}
	vec := DecodeVector(emb.Embedding)
	if len(vec) != 3 {
		t.Errorf("vector dims: got %d, want 3", len(vec))
	}
}

func TestService_FindSimilar(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	incRepo := store.NewIncidentRepo(db)
	embRepo := store.NewEmbeddingRepo(db)
	reportRepo := store.NewReportRepo(db)
	ctx := context.Background()

	// Create 3 incidents with embeddings: 2 similar, 1 different
	for i, svc := range []string{"api-gw", "api-gw", "payment"} {
		id, _ := incRepo.Create(ctx, &store.Incident{
			Source: "test", ServiceName: svc, Severity: "high", Status: "open",
		})
		reportJSON, _ := json.Marshal(map[string]any{
			"summary": "test summary", "root_cause": "test cause",
		})
		reportRepo.Create(ctx, &store.Report{
			IncidentID: id, Summary: "test", RootCause: "test",
			ReportJSON: string(reportJSON), Status: "generated",
		})

		// Same direction vectors for api-gw, opposite for payment
		var vec []float64
		if svc == "api-gw" {
			vec = []float64{1.0, 0.0, 0.0}
		} else {
			vec = []float64{0.0, 1.0, 0.0}
		}
		embRepo.Create(ctx, id, svc, EncodeVector(vec), "test-model")
	}

	// Query with same direction as api-gw
	queryID := int64(1) // first api-gw incident
	embClient := analysis.NewEmbeddingClient(analysis.EmbeddingConfig{
		BaseURL: "http://unused", APIKey: "test", Model: "test",
	})
	svc := NewService(embClient, embRepo, incRepo, reportRepo)

	results, err := svc.FindSimilar(ctx, queryID, 5, 0.0)
	if err != nil {
		t.Fatal(err)
	}

	// Should find the other api-gw incident (high similarity) and payment (low similarity)
	if len(results) < 1 {
		t.Fatal("expected at least 1 similar result")
	}

	// The other api-gw should be first (highest similarity)
	if results[0].Service != "api-gw" {
		t.Errorf("first result service: got %q, want %q", results[0].Service, "api-gw")
	}
	if results[0].Similarity <= 0.99 {
		t.Errorf("api-gw similarity: got %f, want > 0.99", results[0].Similarity)
	}
}

// newTestDB creates an in-memory SQLite database with migrations applied.
func newTestDB(t *testing.T) (*store.DB, func()) {
	// Reuse testkit pattern
	return testkit.NewTestDB(t)
}
```

Note: The `newTestDB` helper should use `internal/testkit.NewTestDB`. Adjust import as needed based on actual testkit API. If testkit returns `*sql.DB` instead, adjust accordingly.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/similar/... -v`
Expected: FAIL — `Service` undefined

**Step 3: Write minimal implementation**

File: `internal/similar/service.go`

```go
package similar

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/atlanssia/aisre/internal/analysis"
	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// Service computes embeddings and finds similar incidents.
type Service struct {
	embClient analysis.EmbeddingClientInterface
	embRepo   store.EmbeddingRepo
	incRepo   store.IncidentRepo
	rptRepo   store.ReportRepo
	logger    *slog.Logger
}

// ServiceConfig holds dependencies for the similar incident service.
type ServiceConfig struct {
	EmbeddingClient *analysis.EmbeddingClient
	EmbeddingRepo   store.EmbeddingRepo
	IncidentRepo    store.IncidentRepo
	ReportRepo      store.ReportRepo
	Logger          *slog.Logger
}

// NewService creates a new similar incident service.
func NewService(embClient *analysis.EmbeddingClient, embRepo store.EmbeddingRepo, incRepo store.IncidentRepo, rptRepo store.ReportRepo) *Service {
	return &Service{
		embClient: embClient,
		embRepo:   embRepo,
		incRepo:   incRepo,
		rptRepo:   rptRepo,
		logger:    slog.Default(),
	}
}

// ComputeEmbedding generates and stores an embedding for the given incident.
// It extracts text from the incident's latest report summary + root cause.
func (s *Service) ComputeEmbedding(ctx context.Context, incidentID int64) error {
	inc, err := s.incRepo.GetByID(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("similar: get incident: %w", err)
	}

	// Build text from incident + report
	text := inc.ServiceName + " " + inc.Severity
	reports, rerr := s.rptRepo.List(ctx, store.ReportFilter{Limit: 1})
	if rerr == nil && len(reports) > 0 {
		for _, r := range reports {
			if r.IncidentID == incidentID {
				text = r.Summary + " " + r.RootCause + " " + text
				break
			}
		}
	}

	vecs, err := s.embClient.Embed(ctx, []string{text})
	if err != nil {
		return fmt.Errorf("similar: embed: %w", err)
	}
	if len(vecs) == 0 {
		return fmt.Errorf("similar: no embedding returned")
	}

	encoded := EncodeVector(vecs[0])
	if err := s.embRepo.Create(ctx, incidentID, inc.ServiceName, encoded, s.embClient.Model()); err != nil {
		return fmt.Errorf("similar: store embedding: %w", err)
	}

	s.logger.Info("embedding computed", "incident_id", incidentID, "dims", len(vecs[0]))
	return nil
}

// FindSimilar searches for incidents with similar embeddings.
// Returns results sorted by similarity (descending), excluding the query incident.
func (s *Service) FindSimilar(ctx context.Context, incidentID int64, topK int, threshold float64) ([]contract.SimilarResult, error) {
	queryEmb, err := s.embRepo.GetByIncidentID(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("similar: get query embedding: %w", err)
	}

	queryVec := DecodeVector(queryEmb.Embedding)

	// Get candidates: same service first, then all
	inc, _ := s.incRepo.GetByID(ctx, incidentID)
	var candidates []store.Embedding

	if inc != nil {
		sameService, err := s.embRepo.ListByService(ctx, inc.ServiceName)
		if err == nil {
			candidates = append(candidates, sameService...)
		}
	}

	// Compute similarities
	var results []contract.SimilarResult
	for _, c := range candidates {
		if c.IncidentID == incidentID {
			continue // skip self
		}

		candidateVec := DecodeVector(c.Embedding)
		sim := CosineSimilarity(queryVec, candidateVec)

		if sim >= threshold {
			// Enrich with incident/report data
			result := contract.SimilarResult{
				IncidentID: c.IncidentID,
				Similarity: sim,
				Service:    c.Service,
			}
			cInc, err := s.incRepo.GetByID(ctx, c.IncidentID)
			if err == nil {
				result.Severity = cInc.Severity
			}
			// Get latest report for summary/root_cause
			reports, _ := s.rptRepo.List(ctx, store.ReportFilter{Limit: 10})
			for _, r := range reports {
				if r.IncidentID == c.IncidentID {
					result.Summary = r.Summary
					result.RootCause = r.RootCause
					break
				}
			}
			results = append(results, result)
		}
	}

	// Sort by similarity descending
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Truncate to topK
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}
```

**Important:** Need to add a `Model()` method to `EmbeddingClient`:

```go
// In internal/analysis/embedding_client.go, add:
func (c *EmbeddingClient) Model() string {
	return c.cfg.Model
}
```

**Step 4: Run tests**

Run: `go test ./internal/similar/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/similar/service.go internal/similar/service_test.go internal/analysis/embedding_client.go
git commit -m "feat(similar): add SimilarService with embedding computation and similarity search"
```

---

## Task 7: API Handlers — similar + embed endpoints

**Files:**
- Modify: `internal/api/router.go` — add SimilarService interface + routes
- Create: `internal/api/similar_handler.go`
- Create: `internal/api/similar_handler_test.go`

**Step 1: Write the failing test**

File: `internal/api/similar_handler_test.go`

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
)

// mockSimilarService implements SimilarService for testing.
type mockSimilarService struct {
	results []contract.SimilarResult
	err     error
	called  bool
}

func (m *mockSimilarService) FindSimilar(ctx context.Context, incidentID int64, topK int, threshold float64) ([]contract.SimilarResult, error) {
	m.called = true
	return m.results, m.err
}

func (m *mockSimilarService) ComputeEmbedding(ctx context.Context, incidentID int64) error {
	m.called = true
	return m.err
}

func TestSimilarHandler_GetSimilar(t *testing.T) {
	mock := &mockSimilarService{
		results: []contract.SimilarResult{
			{IncidentID: 2, Similarity: 0.92, Summary: "Redis timeout", Service: "api-gw"},
		},
	}

	router := NewRouterFull(nil, nil, nil, nil, nil)
	// Note: Will need to wire mock into router — adjust based on final router design

	req := httptest.NewRequest(http.MethodGet, "/api/v1/incidents/1/similar", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Will need to adjust once router is wired
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 200 or 404", w.Code)
	}
}

func TestSimilarHandler_ComputeEmbedding(t *testing.T) {
	mock := &mockSimilarService{}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/1/embed", nil)
	w := httptest.NewRecorder()

	// Test the handler directly
	h := &handler{similarSvc: mock}
	h.computeEmbedding(w, req)

	if !mock.called {
		t.Error("expected ComputeEmbedding to be called")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/api/... -run TestSimilarHandler -v`
Expected: FAIL — `similarSvc` undefined

**Step 3: Modify router + add handler**

Add `SimilarService` interface and `similarSvc` field to `handler` struct in `internal/api/router.go`:

```go
// Add to router.go:
type SimilarService interface {
	FindSimilar(ctx context.Context, incidentID int64, topK int, threshold float64) ([]contract.SimilarResult, error)
	ComputeEmbedding(ctx context.Context, incidentID int64) error
}

// Add to handler struct:
type handler struct {
	svc         IncidentService
	analysisSvc AnalysisService
	feedbackRepo store.FeedbackRepo
	reportRepo   store.ReportRepo
	similarSvc   SimilarService  // Phase 2
}

// Add routes in NewRouterFull, inside the JSON API group:
r.Get("/incidents/{id}/similar", h.getSimilar)
r.Post("/incidents/{id}/embed", h.computeEmbedding)
```

Create `internal/api/similar_handler.go`:

```go
package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (h *handler) getSimilar(w http.ResponseWriter, r *http.Request) {
	if h.similarSvc == nil {
		writeError(w, http.StatusNotFound, "similar incident feature not enabled", "FEATURE_DISABLED")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid incident id", "INVALID_ID")
		return
	}

	topK := 5
	if v := r.URL.Query().Get("top_k"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			topK = n
		}
	}

	threshold := 0.5
	if v := r.URL.Query().Get("threshold"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			threshold = f
		}
	}

	results, err := h.similarSvc.FindSimilar(r.Context(), id, topK, threshold)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "SIMILAR_ERROR")
		return
	}

	if results == nil {
		results = []contract.SimilarResult{}
	}
	json.NewEncoder(w).Encode(results)
}

func (h *handler) computeEmbedding(w http.ResponseWriter, r *http.Request) {
	if h.similarSvc == nil {
		writeError(w, http.StatusNotFound, "similar incident feature not enabled", "FEATURE_DISABLED")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid incident id", "INVALID_ID")
		return
	}

	if err := h.similarSvc.ComputeEmbedding(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "EMBED_ERROR")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 4: Run tests**

Run: `go test ./internal/api/... -run TestSimilarHandler -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/api/router.go internal/api/similar_handler.go internal/api/similar_handler_test.go
git commit -m "feat(similar): add similar incident API endpoints"
```

---

## Task 8: Wire into main.go — config + feature flag + DI

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Add embedding config reading**

In `cmd/server/main.go`, after LLM client setup, add:

```go
// Embedding Client (independent from main LLM)
var similarSvc *similar.Service
if viper.GetBool("features.similar_incident.enabled") {
	embCfg := analysis.EmbeddingConfig{
		BaseURL:    viper.GetString("embedding.base_url"),
		APIKey:     viper.GetString("embedding.api_key"),
		Model:      viper.GetString("embedding.model"),
		Dimensions: viper.GetInt("embedding.dimensions"),
	}
	if embCfg.BaseURL == "" {
		// Fallback: use main LLM config as embedding provider
		embCfg.BaseURL = llmCfg.BaseURL
		embCfg.APIKey = llmCfg.APIKey
		embCfg.Model = "text-embedding-3-small"
	}
	embClient := analysis.NewEmbeddingClient(embCfg)
	embRepo := store.NewEmbeddingRepo(db)
	similarSvc = similar.NewService(embClient, embRepo, incidentRepo, reportRepo)
	slog.Info("similar incident feature enabled", "model", embCfg.Model)
}
```

**Step 2: Pass similarSvc to router**

Modify `NewRouterFull` call to accept `similarSvc`, or add a new constructor `NewRouterWithSimilar`.

Simplest approach: add `similarSvc` parameter to handler struct construction in `NewRouterFull`. Update the function signature and handler init.

**Step 3: Run full test suite**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add cmd/server/main.go internal/api/router.go
git commit -m "feat(similar): wire embedding client and similar service into main"
```

---

## Task 9: Integration with RCA Pipeline — inject similar incidents into prompt

**Files:**
- Modify: `internal/analysis/service.go` — inject SimilarRCA into PromptInput

**Step 1: Modify AnalyzeIncident to fetch similar incidents before building prompt**

In `internal/analysis/service.go`, add an optional `SimilarFinder` interface:

```go
// SimilarFinder finds similar incidents (optional dependency).
type SimilarFinder interface {
	FindSimilar(ctx context.Context, incidentID int64, topK int, threshold float64) ([]contract.SimilarResult, error)
}
```

Add `SimilarFinder` field to `RCAServiceConfig` and `RCAService`.

In the analysis pipeline, before building the prompt:

```go
if s.similarFinder != nil {
	similarResults, err := s.similarFinder.FindSimilar(ctx, incidentID, 3, 0.5)
	if err == nil && len(similarResults) > 0 {
		// Convert to contract.RCAReport for PromptInput.SimilarRCA
		for _, r := range similarResults {
			similarRCA = append(similarRCA, contract.RCAReport{
				Summary:   r.Summary,
				RootCause: r.RootCause,
			})
		}
	}
}
```

Then set `input.SimilarRCA = similarRCA` before calling `builder.Build(input)`.

**Step 2: Run tests**

Run: `go test ./internal/analysis/... -v`
Expected: PASS (existing tests still pass, similar not injected when SimilarFinder is nil)

**Step 3: Commit**

```bash
git add internal/analysis/service.go
git commit -m "feat(similar): inject similar incidents into RCA pipeline prompt"
```

---

## Task 10: Config file — add embedding + feature flags

**Files:**
- Modify: `configs/local.yaml`
- Modify: `configs/e2e-demo.yaml`

**Step 1: Add embedding and feature config**

```yaml
# In configs/local.yaml, add:
embedding:
  base_url: "https://api.openai.com/v1"
  api_key: "${EMBEDDING_API_KEY}"
  model: "text-embedding-3-small"
  dimensions: 1536

features:
  similar_incident:
    enabled: false  # Enable when embedding API is configured
```

```yaml
# In configs/e2e-demo.yaml, add:
embedding:
  base_url: "http://localhost:9999"
  api_key: "mock-key"
  model: "text-embedding-mock"
  dimensions: 3

features:
  similar_incident:
    enabled: true
```

**Step 2: Commit**

```bash
git add configs/local.yaml configs/e2e-demo.yaml
git commit -m "feat(similar): add embedding config and feature flags"
```

---

## Task 11: Run full test suite + fix any issues

**Step 1: Run all tests**

Run: `go test ./... -v -count=1`
Expected: ALL PASS

**Step 2: Run linter**

Run: `golangci-lint run ./...`
Expected: No errors

**Step 3: Final commit if fixes needed**

```bash
git add -A
git commit -m "fix(similar): address lint warnings and test fixes"
```
