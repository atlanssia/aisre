package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type sqlitePromptTemplateRepo struct {
	db *sql.DB
}

// NewPromptTemplateRepo creates a new PromptTemplateRepo backed by SQLite.
func NewPromptTemplateRepo(db *sql.DB) PromptTemplateRepo {
	return &sqlitePromptTemplateRepo{db: db}
}

func (r *sqlitePromptTemplateRepo) Create(ctx context.Context, tpl *PromptTemplate) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO prompt_templates (name, stage, system_tpl, user_tpl, variables, is_default)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		tpl.Name, tpl.Stage, tpl.SystemTpl, tpl.UserTpl, tpl.Variables, tpl.IsDefault,
	)
	if err != nil {
		return 0, fmt.Errorf("prompt_template_repo: create: %w", err)
	}
	return result.LastInsertId()
}

func (r *sqlitePromptTemplateRepo) GetByID(ctx context.Context, id int64) (*PromptTemplate, error) {
	var tpl PromptTemplate
	var isDefault int
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, stage, system_tpl, user_tpl, variables, is_default, version, created_at, updated_at
		 FROM prompt_templates WHERE id = ?`, id,
	).Scan(&tpl.ID, &tpl.Name, &tpl.Stage, &tpl.SystemTpl, &tpl.UserTpl, &tpl.Variables, &isDefault, &tpl.Version, &tpl.CreatedAt, &tpl.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("prompt_template_repo: template %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("prompt_template_repo: get: %w", err)
	}
	tpl.IsDefault = isDefault == 1
	return &tpl, nil
}

func (r *sqlitePromptTemplateRepo) List(ctx context.Context) ([]PromptTemplate, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, stage, system_tpl, user_tpl, variables, is_default, version, created_at, updated_at
		 FROM prompt_templates ORDER BY stage, name`,
	)
	if err != nil {
		return nil, fmt.Errorf("prompt_template_repo: list: %w", err)
	}
	defer rows.Close()

	var results []PromptTemplate
	for rows.Next() {
		var tpl PromptTemplate
		var isDefault int
		if err := rows.Scan(&tpl.ID, &tpl.Name, &tpl.Stage, &tpl.SystemTpl, &tpl.UserTpl, &tpl.Variables, &isDefault, &tpl.Version, &tpl.CreatedAt, &tpl.UpdatedAt); err != nil {
			return nil, fmt.Errorf("prompt_template_repo: scan: %w", err)
		}
		tpl.IsDefault = isDefault == 1
		results = append(results, tpl)
	}
	return results, rows.Err()
}

func (r *sqlitePromptTemplateRepo) Update(ctx context.Context, tpl *PromptTemplate) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE prompt_templates
		 SET name = ?, stage = ?, system_tpl = ?, user_tpl = ?, variables = ?, is_default = ?,
		     version = version + 1, updated_at = datetime('now')
		 WHERE id = ?`,
		tpl.Name, tpl.Stage, tpl.SystemTpl, tpl.UserTpl, tpl.Variables, tpl.IsDefault, tpl.ID,
	)
	if err != nil {
		return fmt.Errorf("prompt_template_repo: update: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("prompt_template_repo: template %d not found", tpl.ID)
	}
	return nil
}

func (r *sqlitePromptTemplateRepo) GetByStage(ctx context.Context, stage string) (*PromptTemplate, error) {
	var tpl PromptTemplate
	var isDefault int
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, stage, system_tpl, user_tpl, variables, is_default, version, created_at, updated_at
		 FROM prompt_templates WHERE stage = ? AND is_default = 1 LIMIT 1`, stage,
	).Scan(&tpl.ID, &tpl.Name, &tpl.Stage, &tpl.SystemTpl, &tpl.UserTpl, &tpl.Variables, &isDefault, &tpl.Version, &tpl.CreatedAt, &tpl.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // no default template for this stage
	}
	if err != nil {
		return nil, fmt.Errorf("prompt_template_repo: get by stage: %w", err)
	}
	tpl.IsDefault = isDefault == 1
	return &tpl, nil
}
