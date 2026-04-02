package prompt

import (
	"github.com/atlanssia/aisre/internal/contract"
)

// PromptInput holds all data needed to build an RCA prompt.
type PromptInput struct {
	Incident    contract.Incident
	Evidence    []contract.ToolResult
	SimilarRCA  []contract.RCAReport
	TimeWindow  string
	Environment string
}

// Builder constructs prompts from templates and input data.
type Builder struct {
	version  string
	template string
}

// NewBuilder creates a prompt builder for the given template version.
func NewBuilder(version string) *Builder {
	return &Builder{
		version: version,
	}
}

// Build renders the prompt template with the given input.
func (b *Builder) Build(input PromptInput) (string, error) {
	// TODO: implement template rendering
	return "", nil
}

// CompressEvidence reduces raw evidence to the most relevant items.
// This is the key step that determines RCA quality.
func CompressEvidence(evidence []contract.ToolResult, maxItems int) []contract.ToolResult {
	if len(evidence) <= maxItems {
		return evidence
	}
	// Sort by score descending, take top N
	sorted := make([]contract.ToolResult, len(evidence))
	copy(sorted, evidence)
	// TODO: implement sort by score
	return sorted[:maxItems]
}
