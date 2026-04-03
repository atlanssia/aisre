package prompt

import (
	"bytes"
	"fmt"
	"sort"

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
	version      string
	templateDir  string
	templateName string
	renderer     *Renderer
}

// NewBuilder creates a prompt builder for the given template version.
func NewBuilder(version string) *Builder {
	return &Builder{
		version:      version,
		templateName: "rca_system_" + version,
	}
}

// NewBuilderWithDir creates a prompt builder that loads templates from a specific directory.
func NewBuilderWithDir(templateDir, templateName string) *Builder {
	return &Builder{
		templateDir:  templateDir,
		templateName: templateName,
	}
}

// initRenderer lazily initializes the template renderer.
func (b *Builder) initRenderer() error {
	if b.renderer != nil {
		return nil
	}

	dir := b.templateDir
	if dir == "" {
		dir = "internal/prompt/templates"
	}

	r, err := NewRenderer(dir)
	if err != nil {
		return fmt.Errorf("prompt: init renderer from %s: %w", dir, err)
	}
	b.renderer = r
	return nil
}

// Build renders the prompt template with the given input.
func (b *Builder) Build(input PromptInput) (string, error) {
	if err := b.initRenderer(); err != nil {
		return "", err
	}

	// Prepare template data from PromptInput
	data := map[string]any{
		"Incident":     input.Incident,
		"Evidence":     input.Evidence,
		"SimilarRCA":   input.SimilarRCA,
		"Environment":  input.Environment,
		"TimeWindow":   input.TimeWindow,
	}

	tmpl, ok := b.renderer.templates[b.templateName]
	if !ok {
		return "", &TemplateError{Name: b.templateName}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("prompt: render template %s: %w", b.templateName, err)
	}

	return buf.String(), nil
}

// CompressEvidence reduces raw evidence to the most relevant items.
// Sorts by score descending, then takes top maxItems.
func CompressEvidence(evidence []contract.ToolResult, maxItems int) []contract.ToolResult {
	if len(evidence) == 0 {
		return nil
	}

	// Always sort by score descending (copy to avoid mutating original)
	sorted := make([]contract.ToolResult, len(evidence))
	copy(sorted, evidence)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	if len(sorted) > maxItems {
		return sorted[:maxItems]
	}
	return sorted
}
