package analysis

import (
	"fmt"
	"sort"

	"github.com/atlanssia/aisre/internal/contract"
)

// RankedEvidence is a tool result with an assigned evidence ID.
type RankedEvidence struct {
	ID        string
	Type      string
	Score     float64
	Summary   string
	Payload   map[string]any
	SourceURL string
}

// EvidenceRanker sorts tool results by score and assigns evidence IDs.
type EvidenceRanker struct{}

// NewEvidenceRanker creates a new EvidenceRanker.
func NewEvidenceRanker() *EvidenceRanker {
	return &EvidenceRanker{}
}

// Rank sorts tool results by score in descending order.
// It returns a new slice and does not modify the input.
func (er *EvidenceRanker) Rank(results []contract.ToolResult) []contract.ToolResult {
	if len(results) == 0 {
		return nil
	}

	sorted := make([]contract.ToolResult, len(results))
	copy(sorted, results)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	return sorted
}

// TopN returns the top N tool results sorted by score.
func (er *EvidenceRanker) TopN(results []contract.ToolResult, n int) []contract.ToolResult {
	if len(results) == 0 {
		return nil
	}

	ranked := er.Rank(results)
	if n > len(ranked) {
		n = len(ranked)
	}
	return ranked[:n]
}

// AssignIDs assigns sequential evidence IDs (ev_001, ev_002, ...) to tool results.
func (er *EvidenceRanker) AssignIDs(results []contract.ToolResult) []RankedEvidence {
	if len(results) == 0 {
		return nil
	}

	evidence := make([]RankedEvidence, len(results))
	for i, tr := range results {
		evidence[i] = RankedEvidence{
			ID:      fmt.Sprintf("ev_%03d", i+1),
			Type:    tr.Name,
			Score:   tr.Score,
			Summary: tr.Summary,
			Payload: tr.Payload,
		}
	}
	return evidence
}

// RankAndAssign combines ranking, top-N selection, and ID assignment.
func (er *EvidenceRanker) RankAndAssign(results []contract.ToolResult, topN int) []RankedEvidence {
	top := er.TopN(results, topN)
	return er.AssignIDs(top)
}
