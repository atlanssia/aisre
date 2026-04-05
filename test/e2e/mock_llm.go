package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

// mockRCAOutput returns a valid RCA JSON output used by the mock LLM server.
var mockRCAOutput = map[string]any{
	"summary":     "Test analysis summary",
	"root_cause":  "Test root cause",
	"confidence":  0.85,
	"hypotheses":  []any{},
	"evidence_ids": []any{},
	"blast_radius": []any{},
	"actions": map[string]any{
		"immediate":  []string{"Check service health"},
		"fix":        []string{"Review recent deployments"},
		"prevention": []string{"Add monitoring"},
	},
	"uncertainties": []any{},
}

// newMockLLMServer creates an httptest.Server that simulates an OpenAI-compatible
// chat completions endpoint. It returns mockRCAOutput as the LLM response content.
func newMockLLMServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request structure
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		rcaJSON, _ := json.Marshal(mockRCAOutput)

		resp := map[string]any{
			"id": "chatcmpl-e2e-mock",
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": string(rcaJSON),
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

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}
