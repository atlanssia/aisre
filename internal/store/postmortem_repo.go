package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type sqlitePostmortemRepo struct {
	db *sql.DB
}

// NewPostmortemRepo creates a new PostmortemRepo backed by SQLite.
func NewPostmortemRepo(db *sql.DB) PostmortemRepo {
	return &sqlitePostmortemRepo{db: db}
}

func (r *sqlitePostmortemRepo) Create(ctx context.Context, pm *Postmortem) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO postmortems (incident_id, content, status)
		 VALUES (?, ?, ?)`,
		pm.IncidentID, pm.Content, pm.Status,
	)
	if err != nil {
		return 0, fmt.Errorf("postmortem_repo: create: %w", err)
	}
	return result.LastInsertId()
}

func (r *sqlitePostmortemRepo) GetByID(ctx context.Context, id int64) (*Postmortem, error) {
	var pm Postmortem
	err := r.db.QueryRowContext(ctx,
		`SELECT id, incident_id, content, status, created_at, updated_at
		 FROM postmortems WHERE id = ?`, id,
	).Scan(&pm.ID, &pm.IncidentID, &pm.Content, &pm.Status, &pm.CreatedAt, &pm.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("postmortem_repo: %d not found: %w", id, sql.ErrNoRows)
	}
	if err != nil {
		return nil, fmt.Errorf("postmortem_repo: get: %w", err)
	}
	return &pm, nil
}

func (r *sqlitePostmortemRepo) GetByIncidentID(ctx context.Context, incidentID int64) (*Postmortem, error) {
	var pm Postmortem
	err := r.db.QueryRowContext(ctx,
		`SELECT id, incident_id, content, status, created_at, updated_at
		 FROM postmortems WHERE incident_id = ?`, incidentID,
	).Scan(&pm.ID, &pm.IncidentID, &pm.Content, &pm.Status, &pm.CreatedAt, &pm.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("postmortem_repo: incident %d not found: %w", incidentID, sql.ErrNoRows)
	}
	if err != nil {
		return nil, fmt.Errorf("postmortem_repo: get by incident: %w", err)
	}
	return &pm, nil
}

func (r *sqlitePostmortemRepo) List(ctx context.Context) ([]Postmortem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, incident_id, content, status, created_at, updated_at
		 FROM postmortems ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("postmortem_repo: list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []Postmortem
	for rows.Next() {
		var pm Postmortem
		if err := rows.Scan(&pm.ID, &pm.IncidentID, &pm.Content, &pm.Status, &pm.CreatedAt, &pm.UpdatedAt); err != nil {
			return nil, fmt.Errorf("postmortem_repo: scan: %w", err)
		}
		results = append(results, pm)
	}
	return results, rows.Err()
}

func (r *sqlitePostmortemRepo) Update(ctx context.Context, pm *Postmortem) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE postmortems SET content=?, status=?, updated_at=datetime('now') WHERE id = ?`,
		pm.Content, pm.Status, pm.ID,
	)
	if err != nil {
		return fmt.Errorf("postmortem_repo: update: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("postmortem_repo: %d not found for update", pm.ID)
	}
	return nil
}
