package promptstudio

import (
	"context"
	"fmt"
	"testing"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

type mockPromptRepo struct {
	templates []store.PromptTemplate
	nextID    int64
}

func (m *mockPromptRepo) Create(_ context.Context, tpl *store.PromptTemplate) (int64, error) {
	m.nextID++
	tpl.ID = m.nextID
	m.templates = append(m.templates, *tpl)
	return m.nextID, nil
}

func (m *mockPromptRepo) GetByID(_ context.Context, id int64) (*store.PromptTemplate, error) {
	for _, t := range m.templates {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockPromptRepo) List(_ context.Context) ([]store.PromptTemplate, error) {
	return m.templates, nil
}

func (m *mockPromptRepo) Update(_ context.Context, tpl *store.PromptTemplate) error {
	for i, t := range m.templates {
		if t.ID == tpl.ID {
			m.templates[i] = *tpl
			return nil
		}
	}
	return fmt.Errorf("not found")
}

func (m *mockPromptRepo) GetByStage(_ context.Context, stage string) (*store.PromptTemplate, error) {
	for _, t := range m.templates {
		if t.Stage == stage && t.IsDefault {
			return &t, nil
		}
	}
	return nil, nil
}

func TestCreateAndGet(t *testing.T) {
	svc := NewService(&mockPromptRepo{})

	created, err := svc.Create(context.Background(), contract.CreatePromptTemplateRequest{
		Name:      "test-rca",
		Stage:     "rca",
		SystemTpl: "You are an SRE analyst.",
		UserTpl:   "Analyze {{.service}} with severity {{.severity}}.",
		Variables: []string{"service", "severity"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if created.Name != "test-rca" {
		t.Errorf("name = %q, want %q", created.Name, "test-rca")
	}

	got, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "test-rca" {
		t.Errorf("got name = %q", got.Name)
	}
}

func TestList(t *testing.T) {
	svc := NewService(&mockPromptRepo{})
	svc.Create(context.Background(), contract.CreatePromptTemplateRequest{
		Name: "a", Stage: "rca", SystemTpl: "sys", UserTpl: "usr", Variables: []string{},
	})
	svc.Create(context.Background(), contract.CreatePromptTemplateRequest{
		Name: "b", Stage: "context", SystemTpl: "sys", UserTpl: "usr", Variables: []string{},
	})

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 templates, got %d", len(list))
	}
}

func TestDryRun(t *testing.T) {
	repo := &mockPromptRepo{}
	svc := NewService(repo)

	tpl, _ := svc.Create(context.Background(), contract.CreatePromptTemplateRequest{
		Name:      "dryrun-test",
		Stage:     "rca",
		SystemTpl: "You are an analyst.",
		UserTpl:   "Analyze {{.service}} with severity {{.severity}}.",
		Variables: []string{"service", "severity"},
	})

	result, err := svc.DryRun(context.Background(), tpl.ID, map[string]string{
		"service":   "payment-svc",
		"severity":  "critical",
		"forbidden": "should be ignored",
	})
	if err != nil {
		t.Fatalf("dryrun: %v", err)
	}

	expected := "Analyze payment-svc with severity critical."
	if result != expected {
		t.Errorf("result = %q, want %q", result, expected)
	}
}

func TestRenderTemplate_Limit(t *testing.T) {
	tpl := ""
	for i := 0; i < 101; i++ {
		tpl += "{{.var}} "
	}
	_, err := RenderTemplate(tpl, map[string]string{"var": "x"})
	if err == nil {
		t.Error("expected error for > 100 replacements")
	}
}

func TestValidateTemplate_ForbidsGoTemplate(t *testing.T) {
	err := validateTemplate("{{range .Items}}{{.}}{{end}}", "")
	if err == nil {
		t.Error("expected error for Go template directive")
	}
}

func TestValidateTemplate_AllowsDotPrefix(t *testing.T) {
	err := validateTemplate("{{.service}} is down", "")
	if err != nil {
		t.Errorf("expected no error for dot-prefix variable: %v", err)
	}
}

func TestValidateStage(t *testing.T) {
	for _, stage := range []string{"context", "evidence", "rca", "summary"} {
		if err := validateStage(stage); err != nil {
			t.Errorf("stage %q should be valid: %v", stage, err)
		}
	}
	if err := validateStage("invalid"); err == nil {
		t.Error("expected error for invalid stage")
	}
}
