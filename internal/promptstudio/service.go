package promptstudio

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/atlanssia/aisre/internal/contract"
	"github.com/atlanssia/aisre/internal/store"
)

// Service manages prompt templates.
type Service struct {
	repo   store.PromptTemplateRepo
	logger *slog.Logger
}

// NewService creates a new prompt studio service.
func NewService(repo store.PromptTemplateRepo) *Service {
	return &Service{repo: repo, logger: slog.Default()}
}

// List returns all prompt templates.
func (s *Service) List(ctx context.Context) ([]contract.PromptTemplate, error) {
	templates, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("promptstudio: list: %w", err)
	}
	result := make([]contract.PromptTemplate, len(templates))
	for i, tpl := range templates {
		result[i] = storeToContract(tpl)
	}
	return result, nil
}

// Get returns a single prompt template by ID.
func (s *Service) Get(ctx context.Context, id int64) (*contract.PromptTemplate, error) {
	tpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("promptstudio: get: %w", err)
	}
	ct := storeToContract(*tpl)
	return &ct, nil
}

// Create creates a new prompt template.
func (s *Service) Create(ctx context.Context, req contract.CreatePromptTemplateRequest) (*contract.PromptTemplate, error) {
	if err := validateStage(req.Stage); err != nil {
		return nil, err
	}
	if err := validateTemplate(req.SystemTpl, req.UserTpl); err != nil {
		return nil, err
	}
	varsJSON, _ := json.Marshal(req.Variables)
	storeTpl := &store.PromptTemplate{
		Name:      req.Name,
		Stage:     req.Stage,
		SystemTpl: req.SystemTpl,
		UserTpl:   req.UserTpl,
		Variables: string(varsJSON),
		IsDefault: req.IsDefault,
	}
	id, err := s.repo.Create(ctx, storeTpl)
	if err != nil {
		return nil, fmt.Errorf("promptstudio: create: %w", err)
	}
	storeTpl.ID = id
	ct := storeToContract(*storeTpl)
	s.logger.Info("prompt template created", "id", id, "name", req.Name)
	return &ct, nil
}

// Update updates an existing prompt template.
func (s *Service) Update(ctx context.Context, id int64, req contract.UpdatePromptTemplateRequest) (*contract.PromptTemplate, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("promptstudio: update: %w", err)
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Stage != "" {
		if err := validateStage(req.Stage); err != nil {
			return nil, err
		}
		existing.Stage = req.Stage
	}
	if req.SystemTpl != "" {
		if err := validateTemplate(req.SystemTpl, req.UserTpl); err != nil {
			return nil, err
		}
		existing.SystemTpl = req.SystemTpl
	}
	if req.UserTpl != "" {
		existing.UserTpl = req.UserTpl
	}
	varsJSON, _ := json.Marshal(req.Variables)
	existing.Variables = string(varsJSON)
	existing.IsDefault = req.IsDefault

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("promptstudio: update: %w", err)
	}
	updated, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("promptstudio: update: reload: %w", err)
	}
	ct := storeToContract(*updated)
	s.logger.Info("prompt template updated", "id", id, "version", ct.Version)
	return &ct, nil
}

// DryRun renders both system and user templates with provided variables.
// Only variables declared in the template's Variables field are used.
func (s *Service) DryRun(ctx context.Context, id int64, vars map[string]string) (*contract.PromptDryRunResponse, error) {
	tpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("promptstudio: dryrun: %w", err)
	}

	// Filter vars to only those declared in the template
	allowed := allowedVars(tpl.Variables)
	filtered := make(map[string]string, len(allowed))
	for k, v := range vars {
		if allowed[k] {
			filtered[k] = v
		}
	}

	systemRendered, err := RenderTemplate(tpl.SystemTpl, filtered)
	if err != nil {
		return nil, fmt.Errorf("promptstudio: dryrun: render system: %w", err)
	}
	userRendered, err := RenderTemplate(tpl.UserTpl, filtered)
	if err != nil {
		return nil, fmt.Errorf("promptstudio: dryrun: render user: %w", err)
	}
	return &contract.PromptDryRunResponse{
		SystemPrompt: systemRendered,
		UserPrompt:   userRendered,
	}, nil
}

func allowedVars(varsJSON string) map[string]bool {
	var vars []string
	if varsJSON != "" {
		_ = json.Unmarshal([]byte(varsJSON), &vars)
	}
	m := make(map[string]bool, len(vars))
	for _, v := range vars {
		m[v] = true
	}
	return m
}

// RenderTemplate performs safe variable interpolation using strings.Replace.
// Security: does NOT use text/template. Only replaces {{.variable_name}} patterns.
func RenderTemplate(tpl string, vars map[string]string) (string, error) {
	result := tpl
	replacements := 0
	for key, val := range vars {
		placeholder := "{{." + key + "}}"
		count := strings.Count(result, placeholder)
		replacements += count
		if replacements > 100 {
			return "", fmt.Errorf("promptstudio: too many variable replacements (max 100)")
		}
		result = strings.ReplaceAll(result, placeholder, val)
	}
	return result, nil
}

func validateStage(stage string) error {
	validStages := map[string]bool{"context": true, "evidence": true, "rca": true, "summary": true}
	if !validStages[stage] {
		return fmt.Errorf("promptstudio: invalid stage %q, must be one of: context, evidence, rca, summary", stage)
	}
	return nil
}

// validateTemplate checks for Go template injection patterns.
func validateTemplate(systemTpl, userTpl string) error {
	for _, tpl := range []string{systemTpl, userTpl} {
		if idx := strings.Index(tpl, "{{"); idx != -1 {
			rest := tpl[idx+2:]
			if len(rest) > 0 && rest[0] != '.' {
				return fmt.Errorf("promptstudio: template contains forbidden Go template directive at {{%s...}}", string(rest[0]))
			}
		}
	}
	return nil
}

func storeToContract(tpl store.PromptTemplate) contract.PromptTemplate {
	var vars []string
	if tpl.Variables != "" {
		_ = json.Unmarshal([]byte(tpl.Variables), &vars)
	}
	return contract.PromptTemplate{
		ID:        tpl.ID,
		Name:      tpl.Name,
		Stage:     tpl.Stage,
		SystemTpl: tpl.SystemTpl,
		UserTpl:   tpl.UserTpl,
		Variables: vars,
		IsDefault: tpl.IsDefault,
		Version:   tpl.Version,
		CreatedAt: tpl.CreatedAt,
		UpdatedAt: tpl.UpdatedAt,
	}
}
