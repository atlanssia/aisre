package prompt

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
)

func TestBuilder_Build_RendersTemplate(t *testing.T) {
	// Create a temp template directory with a test template
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "rca_system_v1.txt"), []byte("Service: {{.Incident.ServiceName}}, Severity: {{.Incident.Severity}}"), 0644)

	b := NewBuilderWithDir(dir, "rca_system_v1")
	input := PromptInput{
		Incident: contract.Incident{
			ServiceName: "api-gateway",
			Severity:    "critical",
		},
	}

	got, err := b.Build(input)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	want := "Service: api-gateway, Severity: critical"
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func TestBuilder_Build_IncludesEvidence(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "rca_system_v1.txt"), []byte("{{range .Evidence}}- {{.Name}}: {{.Summary}}\n{{end}}"), 0644)

	b := NewBuilderWithDir(dir, "rca_system_v1")
	input := PromptInput{
		Evidence: []contract.ToolResult{
			{Name: "logs", Summary: "Error spike detected"},
			{Name: "traces", Summary: "High latency on /api/v2"},
		},
	}

	got, err := b.Build(input)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(got) == 0 {
		t.Error("Build() returned empty string")
	}
}

func TestBuilder_Build_TemplateNotFound(t *testing.T) {
	dir := t.TempDir()
	// No template file created

	b := NewBuilderWithDir(dir, "nonexistent_template")
	input := PromptInput{}

	_, err := b.Build(input)
	if err == nil {
		t.Error("expected error for missing template")
	}
}

func TestCompressEvidence_SortsByScore(t *testing.T) {
	evidence := []contract.ToolResult{
		{Name: "low", Score: 0.3, Summary: "low score"},
		{Name: "high", Score: 0.9, Summary: "high score"},
		{Name: "mid", Score: 0.6, Summary: "mid score"},
	}

	result := CompressEvidence(evidence, 3)

	if result[0].Name != "high" {
		t.Errorf("first item = %q, want %q", result[0].Name, "high")
	}
	if result[1].Name != "mid" {
		t.Errorf("second item = %q, want %q", result[1].Name, "mid")
	}
	if result[2].Name != "low" {
		t.Errorf("third item = %q, want %q", result[2].Name, "low")
	}
}

func TestCompressEvidence_TruncatesToMaxItems(t *testing.T) {
	evidence := []contract.ToolResult{
		{Name: "a", Score: 0.9},
		{Name: "b", Score: 0.7},
		{Name: "c", Score: 0.5},
		{Name: "d", Score: 0.3},
	}

	result := CompressEvidence(evidence, 2)

	if len(result) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(result))
	}
	if result[0].Name != "a" {
		t.Errorf("first = %q, want %q", result[0].Name, "a")
	}
	if result[1].Name != "b" {
		t.Errorf("second = %q, want %q", result[1].Name, "b")
	}
}

func TestCompressEvidence_LessThanMax(t *testing.T) {
	evidence := []contract.ToolResult{
		{Name: "a", Score: 0.9},
	}

	result := CompressEvidence(evidence, 5)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
}

func TestCompressEvidence_Empty(t *testing.T) {
	result := CompressEvidence(nil, 10)
	if len(result) != 0 {
		t.Errorf("expected empty, got %d items", len(result))
	}
}

func TestCompressEvidence_DoesNotMutateOriginal(t *testing.T) {
	evidence := []contract.ToolResult{
		{Name: "low", Score: 0.3},
		{Name: "high", Score: 0.9},
	}

	CompressEvidence(evidence, 2)

	// Original should not be sorted
	if evidence[0].Name != "low" {
		t.Error("CompressEvidence mutated the original slice")
	}
}

func init() {
	// Ensure sort package is imported (used by implementation)
	_ = sort.Strings
}
