package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLLMClient_Complete(t *testing.T) {
	t.Run("successful completion", func(t *testing.T) {
		mockResp := map[string]any{
			"id": "chatcmpl-123",
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"summary":"test summary","root_cause":"test cause","confidence":0.85}`,
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     100,
				"completion_tokens": 50,
				"total_tokens":      150,
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/chat/completions" {
				t.Errorf("expected /v1/chat/completions, got %s", r.URL.Path)
			}
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("expected Bearer test-key, got %s", r.Header.Get("Authorization"))
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected application/json content type")
			}

			// Verify request body
			var reqBody map[string]any
			_ = json.NewDecoder(r.Body).Decode(&reqBody) //nolint:errcheck // test: validate field below
			if reqBody["model"] != "gpt-4" {
				t.Errorf("expected model gpt-4, got %v", reqBody["model"])
			}

			_ = json.NewEncoder(w).Encode(mockResp)
		}))
		defer server.Close()

		client := NewLLMClient(LLMConfig{
			BaseURL:   server.URL,
			APIKey:    "test-key",
			Model:     "gpt-4",
			MaxTokens: 4096,
		})

		messages := []Message{
			{Role: "system", Content: "You are an RCA expert."},
			{Role: "user", Content: "Analyze this incident."},
		}

		resp, err := client.Complete(context.Background(), messages)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Content == "" {
			t.Error("expected non-empty content")
		}
		if resp.Usage.TotalTokens != 150 {
			t.Errorf("expected 150 total tokens, got %d", resp.Usage.TotalTokens)
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"internal server error"}}`))
		}))
		defer server.Close()

		client := NewLLMClient(LLMConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-4",
		})

		_, err := client.Complete(context.Background(), []Message{
			{Role: "user", Content: "test"},
		})
		if err == nil {
			t.Error("expected error for 500 response")
		}
	})

	t.Run("rate limit error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limit exceeded"}}`))
		}))
		defer server.Close()

		client := NewLLMClient(LLMConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-4",
		})

		_, err := client.Complete(context.Background(), []Message{
			{Role: "user", Content: "test"},
		})
		if err == nil {
			t.Error("expected error for 429 response")
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow response
			select {}
		}))
		defer server.Close()

		client := NewLLMClient(LLMConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-4",
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := client.Complete(ctx, []Message{
			{Role: "user", Content: "test"},
		})
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})

	t.Run("empty choices", func(t *testing.T) {
		mockResp := map[string]any{
			"id":      "chatcmpl-123",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 0,
				"total_tokens":      10,
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(mockResp)
		}))
		defer server.Close()

		client := NewLLMClient(LLMConfig{
			BaseURL: server.URL,
			APIKey:  "test-key",
			Model:   "gpt-4",
		})

		_, err := client.Complete(context.Background(), []Message{
			{Role: "user", Content: "test"},
		})
		if err == nil {
			t.Error("expected error for empty choices")
		}
	})
}

func TestLLMClient_ParseRCAOutput(t *testing.T) {
	client := NewLLMClient(LLMConfig{
		BaseURL: "http://localhost",
		APIKey:  "test",
		Model:   "gpt-4",
	})

	t.Run("valid RCA output", func(t *testing.T) {
		content := `{
			"summary": "Database connection pool exhaustion causing service degradation",
			"root_cause": "Connection leak in the user-service module",
			"confidence": 0.92,
			"hypotheses": [
				{"id": "h1", "description": "Connection leak", "likelihood": 0.9, "evidence_ids": ["ev_001"]},
				{"id": "h2", "description": "Spike in traffic", "likelihood": 0.3, "evidence_ids": ["ev_002"]}
			],
			"evidence_ids": ["ev_001", "ev_002"],
			"blast_radius": ["user-service", "api-gateway"],
			"actions": {
				"immediate": ["Restart user-service pods", "Increase connection pool size"],
				"fix": ["Fix connection leak in auth module"],
				"prevention": ["Add connection pool monitoring"]
			},
			"uncertainties": ["Cannot confirm if traffic spike is correlated"]
		}`

		output, err := client.ParseRCAOutput(content)
		if err != nil {
			t.Fatal(err)
		}
		if output.Summary == "" {
			t.Error("expected non-empty summary")
		}
		if output.RootCause == "" {
			t.Error("expected non-empty root cause")
		}
		if output.Confidence < 0.9 {
			t.Errorf("expected confidence >= 0.9, got %f", output.Confidence)
		}
		if len(output.Hypotheses) != 2 {
			t.Errorf("expected 2 hypotheses, got %d", len(output.Hypotheses))
		}
		if len(output.BlastRadius) != 2 {
			t.Errorf("expected 2 blast radius items, got %d", len(output.BlastRadius))
		}
		if len(output.Actions.Immediate) != 2 {
			t.Errorf("expected 2 immediate actions, got %d", len(output.Actions.Immediate))
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := client.ParseRCAOutput("not json")
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}
