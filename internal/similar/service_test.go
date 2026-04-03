package similar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atlanssia/aisre/internal/analysis"
	"github.com/atlanssia/aisre/internal/store"
	"github.com/atlanssia/aisre/internal/testkit"
)

// newMockEmbeddingServer creates an httptest.Server that simulates an
// OpenAI-compatible /embeddings endpoint. The embedding vector is fixed
// so that callers can predict similarity outcomes.
func newMockEmbeddingServer(t *testing.T, vector []float64) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/embeddings" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		data := make([]map[string]any, len(req.Input))
		for i := range req.Input {
			data[i] = map[string]any{
				"embedding": vector,
				"index":     i,
			}
		}

		resp := map[string]any{
			"data":  data,
			"model": "text-embedding-3-small",
			"usage": map[string]any{
				"prompt_tokens": 10,
				"total_tokens":  10,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(handler)
}

// setupService creates a SimilarService with a real test database and a mock
// embedding server that returns the given vector for every call.
func setupService(t *testing.T, embVector []float64) (*Service, store.IncidentRepo, store.ReportRepo, store.EmbeddingRepo, func()) {
	t.Helper()

	db, dbCleanup := testkit.NewTestDB(t)

	incRepo := store.NewIncidentRepo(db)
	rptRepo := store.NewReportRepo(db)
	embRepo := store.NewEmbeddingRepo(db)

	server := newMockEmbeddingServer(t, embVector)
	t.Cleanup(server.Close)

	embClient := analysis.NewEmbeddingClient(analysis.EmbeddingConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "text-embedding-3-small",
	})

	svc := NewService(embClient, embRepo, incRepo, rptRepo)

	return svc, incRepo, rptRepo, embRepo, dbCleanup
}

func TestService_ComputeEmbedding(t *testing.T) {
	ctx := context.Background()
	embVector := []float64{0.1, 0.2, 0.3, 0.4, 0.5}

	svc, incRepo, rptRepo, embRepo, cleanup := setupService(t, embVector)
	defer cleanup()

	// Create incident.
	incID, err := incRepo.Create(ctx, &store.Incident{
		Source:      "prometheus",
		ServiceName: "api-gateway",
		Severity:    "high",
		Status:      "open",
		TraceID:     "trace-001",
	})
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}

	// Create report for the incident.
	_, err = rptRepo.Create(ctx, &store.Report{
		IncidentID: incID,
		Summary:    "High error rate on api-gateway",
		RootCause:  "Connection pool exhaustion",
		Confidence: 0.9,
		ReportJSON: `{"summary":"High error rate"}`,
	})
	if err != nil {
		t.Fatalf("create report: %v", err)
	}

	// Compute embedding.
	if err := svc.ComputeEmbedding(ctx, incID); err != nil {
		t.Fatalf("ComputeEmbedding: %v", err)
	}

	// Verify stored embedding.
	emb, err := embRepo.GetByIncidentID(ctx, incID)
	if err != nil {
		t.Fatalf("GetByIncidentID: %v", err)
	}
	if emb.Service != "api-gateway" {
		t.Errorf("service: got %q, want %q", emb.Service, "api-gateway")
	}
	if emb.Model != "text-embedding-3-small" {
		t.Errorf("model: got %q, want %q", emb.Model, "text-embedding-3-small")
	}

	decoded := DecodeVector(emb.Embedding)
	if len(decoded) != len(embVector) {
		t.Fatalf("embedding vector length: got %d, want %d", len(decoded), len(embVector))
	}
	for i := range embVector {
		if decoded[i] != embVector[i] {
			t.Errorf("embedding[%d]: got %f, want %f", i, decoded[i], embVector[i])
		}
	}
}

func TestService_FindSimilar(t *testing.T) {
	ctx := context.Background()

	// Use distinct vectors: vecA for api-gw incidents, vecB for payment.
	vecA := []float64{1.0, 0.0, 0.0}
	vecB := []float64{0.0, 1.0, 0.0}

	// We'll swap the mock vector per-embedding via direct store inserts
	// so we control exactly which vector each incident gets.
	db, dbCleanup := testkit.NewTestDB(t)
	defer dbCleanup()

	incRepo := store.NewIncidentRepo(db)
	rptRepo := store.NewReportRepo(db)
	embRepo := store.NewEmbeddingRepo(db)

	// Dummy embedding server (not used since we insert directly).
	server := newMockEmbeddingServer(t, vecA)
	t.Cleanup(server.Close)

	embClient := analysis.NewEmbeddingClient(analysis.EmbeddingConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "text-embedding-3-small",
	})

	svc := NewService(embClient, embRepo, incRepo, rptRepo)

	// Create 3 incidents in the same service.
	incidents := []struct {
		service  string
		severity string
		summary  string
		rootCause string
		vector   []float64
	}{
		{"api-gw", "critical", "High latency spike", "Connection pool exhaustion", vecA},
		{"api-gw", "high", "Timeout errors", "Upstream service down", vecA},
		{"api-gw", "low", "Payment slow", "Database lock contention", vecB},
	}

	incIDs := make([]int64, len(incidents))
	for i, tc := range incidents {
		id, err := incRepo.Create(ctx, &store.Incident{
			Source:      "prometheus",
			ServiceName: tc.service,
			Severity:    tc.severity,
			Status:      "open",
			TraceID:     "trace-" + tc.service,
		})
		if err != nil {
			t.Fatalf("create incident %d: %v", i, err)
		}
		incIDs[i] = id

		_, err = rptRepo.Create(ctx, &store.Report{
			IncidentID: id,
			Summary:    tc.summary,
			RootCause:  tc.rootCause,
			Confidence: 0.85,
			ReportJSON: `{"summary":"` + tc.summary + `"}`,
		})
		if err != nil {
			t.Fatalf("create report %d: %v", i, err)
		}

		// Store embedding directly to control vectors.
		err = embRepo.Create(ctx, id, tc.service, EncodeVector(tc.vector), "text-embedding-3-small")
		if err != nil {
			t.Fatalf("store embedding %d: %v", i, err)
		}
	}

	// Query with first incident, topK=3, low threshold.
	results, err := svc.FindSimilar(ctx, incIDs[0], 3, 0.0)
	if err != nil {
		t.Fatalf("FindSimilar: %v", err)
	}

	// Should get 2 results (skip self).
	if len(results) != 2 {
		t.Fatalf("results count: got %d, want 2", len(results))
	}

	// First result should be the other api-gw incident (same vector → cosine=1.0).
	if results[0].IncidentID != incIDs[1] {
		t.Errorf("first result incident: got %d, want %d", results[0].IncidentID, incIDs[1])
	}
	if results[0].Similarity != 1.0 {
		t.Errorf("first result similarity: got %f, want 1.0", results[0].Similarity)
	}
	if results[0].Severity != "high" {
		t.Errorf("first result severity: got %q, want %q", results[0].Severity, "high")
	}

	// Second result is the payment incident (orthogonal → cosine=0.0).
	if results[1].IncidentID != incIDs[2] {
		t.Errorf("second result incident: got %d, want %d", results[1].IncidentID, incIDs[2])
	}
	if results[1].Similarity != 0.0 {
		t.Errorf("second result similarity: got %f, want 0.0", results[1].Similarity)
	}

	// Verify enrichment fields.
	if results[0].Service != "api-gw" {
		t.Errorf("service: got %q, want %q", results[0].Service, "api-gw")
	}
	if results[0].Summary != "Timeout errors" {
		t.Errorf("summary: got %q, want %q", results[0].Summary, "Timeout errors")
	}
	if results[0].RootCause != "Upstream service down" {
		t.Errorf("root_cause: got %q, want %q", results[0].RootCause, "Upstream service down")
	}
}

func TestService_FindSimilar_NoEmbedding(t *testing.T) {
	ctx := context.Background()
	embVector := []float64{0.1, 0.2, 0.3}

	svc, incRepo, _, _, cleanup := setupService(t, embVector)
	defer cleanup()

	// Create incident without embedding.
	incID, err := incRepo.Create(ctx, &store.Incident{
		Source:      "prometheus",
		ServiceName: "api-gateway",
		Severity:    "high",
		Status:      "open",
		TraceID:     "trace-001",
	})
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}

	_, err = svc.FindSimilar(ctx, incID, 5, 0.5)
	if err == nil {
		t.Fatal("expected error for missing embedding, got nil")
	}
}

func TestService_FindSimilar_BelowThreshold(t *testing.T) {
	ctx := context.Background()

	vecA := []float64{1.0, 0.0, 0.0}
	vecB := []float64{0.0, 1.0, 0.0}

	db, dbCleanup := testkit.NewTestDB(t)
	defer dbCleanup()

	incRepo := store.NewIncidentRepo(db)
	rptRepo := store.NewReportRepo(db)
	embRepo := store.NewEmbeddingRepo(db)

	server := newMockEmbeddingServer(t, vecA)
	t.Cleanup(server.Close)

	embClient := analysis.NewEmbeddingClient(analysis.EmbeddingConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "text-embedding-3-small",
	})

	svc := NewService(embClient, embRepo, incRepo, rptRepo)

	// Create 2 incidents with orthogonal vectors (cosine=0.0).
	inc1ID, err := incRepo.Create(ctx, &store.Incident{
		Source:      "prometheus",
		ServiceName: "api-gw",
		Severity:    "critical",
		Status:      "open",
		TraceID:     "trace-1",
	})
	if err != nil {
		t.Fatalf("create incident 1: %v", err)
	}

	inc2ID, err := incRepo.Create(ctx, &store.Incident{
		Source:      "prometheus",
		ServiceName: "api-gw",
		Severity:    "low",
		Status:      "open",
		TraceID:     "trace-2",
	})
	if err != nil {
		t.Fatalf("create incident 2: %v", err)
	}

	_, err = rptRepo.Create(ctx, &store.Report{
		IncidentID: inc1ID,
		Summary:    "Spike",
		RootCause:  "Pool exhaustion",
		Confidence: 0.9,
	})
	if err != nil {
		t.Fatalf("create report 1: %v", err)
	}

	_, err = rptRepo.Create(ctx, &store.Report{
		IncidentID: inc2ID,
		Summary:    "Slow",
		RootCause:  "DB lock",
		Confidence: 0.8,
	})
	if err != nil {
		t.Fatalf("create report 2: %v", err)
	}

	err = embRepo.Create(ctx, inc1ID, "api-gw", EncodeVector(vecA), "text-embedding-3-small")
	if err != nil {
		t.Fatalf("store embedding 1: %v", err)
	}

	err = embRepo.Create(ctx, inc2ID, "api-gw", EncodeVector(vecB), "text-embedding-3-small")
	if err != nil {
		t.Fatalf("store embedding 2: %v", err)
	}

	// Query with threshold 0.5 — orthogonal vectors (similarity=0.0) should be filtered out.
	results, err := svc.FindSimilar(ctx, inc1ID, 5, 0.5)
	if err != nil {
		t.Fatalf("FindSimilar: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results with threshold 0.5, got %d", len(results))
	}
}
