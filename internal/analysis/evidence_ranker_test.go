package analysis

import (
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
)

func TestEvidenceRanker_Rank(t *testing.T) {
	ranker := NewEvidenceRanker()

	t.Run("sorts by score descending", func(t *testing.T) {
		results := []contract.ToolResult{
			{Name: "metric", Summary: "cpu spike", Score: 0.6},
			{Name: "log", Summary: "error log", Score: 0.95},
			{Name: "trace", Summary: "slow span", Score: 0.8},
		}

		ranked := ranker.Rank(results)
		if len(ranked) != 3 {
			t.Fatalf("expected 3, got %d", len(ranked))
		}
		if ranked[0].Score != 0.95 {
			t.Errorf("expected highest score first (0.95), got %f", ranked[0].Score)
		}
		if ranked[1].Score != 0.8 {
			t.Errorf("expected second score (0.8), got %f", ranked[1].Score)
		}
		if ranked[2].Score != 0.6 {
			t.Errorf("expected third score (0.6), got %f", ranked[2].Score)
		}
	})

	t.Run("returns top N items", func(t *testing.T) {
		results := []contract.ToolResult{
			{Name: "a", Summary: "a", Score: 0.5},
			{Name: "b", Summary: "b", Score: 0.9},
			{Name: "c", Summary: "c", Score: 0.7},
			{Name: "d", Summary: "d", Score: 0.3},
			{Name: "e", Summary: "e", Score: 0.85},
		}

		ranked := ranker.TopN(results, 3)
		if len(ranked) != 3 {
			t.Fatalf("expected 3, got %d", len(ranked))
		}
		if ranked[0].Score != 0.9 {
			t.Errorf("expected 0.9, got %f", ranked[0].Score)
		}
		if ranked[1].Score != 0.85 {
			t.Errorf("expected 0.85, got %f", ranked[1].Score)
		}
		if ranked[2].Score != 0.7 {
			t.Errorf("expected 0.7, got %f", ranked[2].Score)
		}
	})

	t.Run("topN larger than input returns all", func(t *testing.T) {
		results := []contract.ToolResult{
			{Name: "a", Summary: "a", Score: 0.5},
			{Name: "b", Summary: "b", Score: 0.9},
		}

		ranked := ranker.TopN(results, 10)
		if len(ranked) != 2 {
			t.Fatalf("expected 2, got %d", len(ranked))
		}
	})

	t.Run("handles empty results", func(t *testing.T) {
		ranked := ranker.Rank(nil)
		if ranked != nil {
			t.Errorf("expected nil, got %v", ranked)
		}

		topN := ranker.TopN(nil, 5)
		if topN != nil {
			t.Errorf("expected nil, got %v", topN)
		}
	})

	t.Run("does not modify original slice", func(t *testing.T) {
		results := []contract.ToolResult{
			{Name: "a", Summary: "a", Score: 0.3},
			{Name: "b", Summary: "b", Score: 0.9},
			{Name: "c", Summary: "c", Score: 0.5},
		}

		ranker.Rank(results)
		// Original should remain unchanged
		if results[0].Score != 0.3 {
			t.Errorf("original slice was modified: expected 0.3, got %f", results[0].Score)
		}
	})
}

func TestEvidenceRanker_AssignIDs(t *testing.T) {
	ranker := NewEvidenceRanker()

	t.Run("assigns sequential IDs", func(t *testing.T) {
		results := []contract.ToolResult{
			{Name: "log", Summary: "error", Score: 0.95},
			{Name: "trace", Summary: "slow", Score: 0.8},
			{Name: "metric", Summary: "spike", Score: 0.6},
		}

		evidence := ranker.AssignIDs(results)
		if len(evidence) != 3 {
			t.Fatalf("expected 3, got %d", len(evidence))
		}

		expectedIDs := []string{"ev_001", "ev_002", "ev_003"}
		for i, ev := range evidence {
			if ev.ID != expectedIDs[i] {
				t.Errorf("evidence[%d]: expected ID %s, got %s", i, expectedIDs[i], ev.ID)
			}
		}
	})

	t.Run("preserves tool result data", func(t *testing.T) {
		results := []contract.ToolResult{
			{Name: "log", Summary: "connection refused", Score: 0.9, Payload: map[string]any{"service": "db"}},
		}

		evidence := ranker.AssignIDs(results)
		if evidence[0].Type != "log" {
			t.Errorf("expected type log, got %s", evidence[0].Type)
		}
		if evidence[0].Score != 0.9 {
			t.Errorf("expected score 0.9, got %f", evidence[0].Score)
		}
		if evidence[0].Summary != "connection refused" {
			t.Errorf("expected 'connection refused', got %s", evidence[0].Summary)
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		evidence := ranker.AssignIDs(nil)
		if evidence != nil {
			t.Errorf("expected nil, got %v", evidence)
		}
	})

	t.Run("handles large number of items", func(t *testing.T) {
		var results []contract.ToolResult
		for i := 0; i < 15; i++ {
			results = append(results, contract.ToolResult{
				Name: "test", Summary: "test", Score: 0.5,
			})
		}

		evidence := ranker.AssignIDs(results)
		if evidence[9].ID != "ev_010" {
			t.Errorf("expected ev_010, got %s", evidence[9].ID)
		}
		if evidence[14].ID != "ev_015" {
			t.Errorf("expected ev_015, got %s", evidence[14].ID)
		}
	})
}

func TestEvidenceRanker_RankAndAssign(t *testing.T) {
	ranker := NewEvidenceRanker()

	t.Run("full pipeline: rank then assign IDs", func(t *testing.T) {
		results := []contract.ToolResult{
			{Name: "metric", Summary: "cpu spike", Score: 0.6},
			{Name: "log", Summary: "error log", Score: 0.95},
			{Name: "trace", Summary: "slow span", Score: 0.8},
			{Name: "log2", Summary: "another log", Score: 0.3},
		}

		evidence := ranker.RankAndAssign(results, 3)
		if len(evidence) != 3 {
			t.Fatalf("expected 3, got %d", len(evidence))
		}

		// Should be sorted by score and have IDs
		if evidence[0].ID != "ev_001" {
			t.Errorf("expected ev_001, got %s", evidence[0].ID)
		}
		if evidence[0].Score != 0.95 {
			t.Errorf("expected highest score first (0.95), got %f", evidence[0].Score)
		}
		if evidence[2].Score != 0.6 {
			t.Errorf("expected third score (0.6), got %f", evidence[2].Score)
		}
	})
}
