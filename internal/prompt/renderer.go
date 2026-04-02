package prompt

import (
	"fmt"
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
	// TODO: load templates from templateDir/templates/*.txt
	return &Renderer{
		templates: make(map[string]*template.Template),
	}, nil
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
