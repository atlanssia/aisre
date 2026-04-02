package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// TemplateError is returned when a template is not found.
type TemplateError struct {
	Name string
}

func (e *TemplateError) Error() string {
	return fmt.Sprintf("template not found: %s", e.Name)
}

// Renderer loads and executes prompt templates.
type Renderer struct {
	templates map[string]*template.Template
}

// NewRenderer creates a renderer that loads templates from the given directory.
func NewRenderer(templateDir string) (*Renderer, error) {
	r := &Renderer{
		templates: make(map[string]*template.Template),
	}

	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, fmt.Errorf("prompt: read template dir %s: %w", templateDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".txt")
		content, err := os.ReadFile(filepath.Join(templateDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("prompt: read template %s: %w", entry.Name(), err)
		}

		tmpl, err := template.New(name).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("prompt: parse template %s: %w", entry.Name(), err)
		}

		r.templates[name] = tmpl
	}

	return r, nil
}

// Render executes a named template with the given data.
func (r *Renderer) Render(name string, data any) (string, error) {
	tmpl, ok := r.templates[name]
	if !ok {
		return "", &TemplateError{Name: name}
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
