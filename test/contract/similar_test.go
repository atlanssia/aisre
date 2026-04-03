package contract_test

import (
	"encoding/json"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
)

func TestSimilarResult_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
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
