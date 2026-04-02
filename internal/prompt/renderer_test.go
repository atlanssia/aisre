package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderer_LoadTemplates(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test_v1.txt"), []byte("Hello {{.Name}}!"), 0644)
	os.WriteFile(filepath.Join(dir, "test_v2.txt"), []byte("Bye {{.Name}}!"), 0644)

	r, err := NewRenderer(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.templates) != 2 {
		t.Errorf("expected 2 templates, got %d", len(r.templates))
	}
}

func TestRenderer_Render(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "greet_v1.txt"), []byte("Hello {{.Name}}, you have {{.Count}} alerts!"), 0644)

	r, _ := NewRenderer(dir)
	out, err := r.Render("greet_v1", map[string]any{"Name": "SRE", "Count": 5})
	if err != nil {
		t.Fatal(err)
	}
	if out != "Hello SRE, you have 5 alerts!" {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestRenderer_Render_NotFound(t *testing.T) {
	dir := t.TempDir()
	r, _ := NewRenderer(dir)

	_, err := r.Render("nonexistent", nil)
	if err == nil {
		t.Error("expected error for missing template")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention template name: %v", err)
	}
}

func TestRenderer_LoadFromRealTemplates(t *testing.T) {
	// Load the actual project templates
	r, err := NewRenderer("templates")
	if err != nil {
		t.Skip("templates directory not found")
	}
	if len(r.templates) == 0 {
		t.Error("expected at least one template")
	}

	t.Run("rca_system_v1 renders", func(t *testing.T) {
		out, err := r.Render("rca_system_v1", map[string]any{
			"Incident":     "test incident",
			"Evidence":     []map[string]any{},
			"SimilarRCA":   []map[string]any{},
			"Environment":  "production",
			"TimeWindow":   "1h",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "Root Cause Analyst") {
			t.Error("expected system prompt content")
		}
	})
}

func TestNewRenderer_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRenderer(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.templates) != 0 {
		t.Errorf("expected 0 templates for empty dir, got %d", len(r.templates))
	}
}

func TestNewRenderer_NonexistentDir(t *testing.T) {
	_, err := NewRenderer("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

func TestRenderer_TemplateNameParsing(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "my_prompt_v2.txt"), []byte("test"), 0644)

	r, _ := NewRenderer(dir)
	if _, ok := r.templates["my_prompt_v2"]; !ok {
		t.Error("expected template named 'my_prompt_v2'")
	}
	// Also check without extension
	if _, ok := r.templates["my_prompt_v2.txt"]; ok {
		t.Error("should not include .txt extension in name")
	}
}
