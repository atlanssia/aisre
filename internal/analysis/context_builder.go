package analysis

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/atlanssia/aisre/internal/contract"
)

// AnalysisContext holds all the information needed for an RCA analysis.
type AnalysisContext struct {
	Incident    *contract.Incident
	ToolResults []contract.ToolResult
}

// ContextBuilder constructs the analysis context from incident data and tool results.
type ContextBuilder struct{}

// NewContextBuilder creates a new ContextBuilder.
func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{}
}

// Build creates an AnalysisContext from an incident and its tool results.
func (cb *ContextBuilder) Build(incident *contract.Incident, toolResults []contract.ToolResult) *AnalysisContext {
	return &AnalysisContext{
		Incident:    incident,
		ToolResults: toolResults,
	}
}

// BuildPrompt generates the LLM prompt string from the analysis context.
func (cb *ContextBuilder) BuildPrompt(ctx *AnalysisContext) string {
	var b strings.Builder

	b.WriteString("You are an expert Site Reliability Engineer performing Root Cause Analysis.\n\n")
	b.WriteString("## Incident Details\n\n")

	if ctx.Incident != nil {
		fmt.Fprintf(&b, "- **Service:** %s\n", ctx.Incident.ServiceName)
		fmt.Fprintf(&b, "- **Severity:** %s\n", ctx.Incident.Severity)
		fmt.Fprintf(&b, "- **Source:** %s\n", ctx.Incident.Source)
		if ctx.Incident.TraceID != "" {
			fmt.Fprintf(&b, "- **Trace ID:** %s\n", ctx.Incident.TraceID)
		}
		fmt.Fprintf(&b, "- **Time:** %s\n", ctx.Incident.CreatedAt)
	}

	if len(ctx.ToolResults) > 0 {
		b.WriteString("\n## Evidence\n\n")
		for i, tr := range ctx.ToolResults {
			fmt.Fprintf(&b, "### Evidence %d: %s (score: %.2f)\n", i+1, tr.Name, tr.Score)
			fmt.Fprintf(&b, "%s\n", tr.Summary)
			if tr.Payload != nil {
				payload, _ := json.MarshalIndent(tr.Payload, "  ", "  ")
				fmt.Fprintf(&b, "  Details: %s\n", string(payload))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n## Instructions\n\n")
	b.WriteString("Based on the incident details and evidence above, provide a Root Cause Analysis.\n")
	b.WriteString("Respond with a JSON object containing: summary, root_cause, confidence (0-1), ")
	b.WriteString("hypotheses (array of {id, description, likelihood, evidence_ids}), ")
	b.WriteString("evidence_ids (array of referenced evidence IDs), ")
	b.WriteString("blast_radius (array of affected services), ")
	b.WriteString("actions ({immediate, fix, prevention} arrays), ")
	b.WriteString("and uncertainties (array of unknowns).\n")

	return b.String()
}

// FormatToolResults formats tool results into a human-readable string.
func (cb *ContextBuilder) FormatToolResults(results []contract.ToolResult) string {
	var b strings.Builder
	for _, tr := range results {
		fmt.Fprintf(&b, "- [%s] %s (score: %.2f)\n", tr.Name, tr.Summary, tr.Score)
	}
	return b.String()
}
