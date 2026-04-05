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

			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode request: %v", err)
			}
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
			_ = json.NewEncoder(w).Encode(resp)
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

	t.Run("Model() returns config model", func(t *testing.T) {
		cfg := EmbeddingConfig{BaseURL: "http://localhost", APIKey: "key", Model: "text-embedding-3-small"}
		client := NewEmbeddingClient(cfg)
		if client.Model() != "text-embedding-3-small" {
			t.Errorf("Model(): got %q, want %q", client.Model(), "text-embedding-3-small")
		}
	})
}
