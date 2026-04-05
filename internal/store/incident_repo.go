package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// sqliteIncidentRepo implements the IncidentRepo interface with SQLite backend.
type sqliteIncidentRepo struct {
	db *sql.DB
}

// NewIncidentRepo creates a new IncidentRepo backed by the given database.
func NewIncidentRepo(db *sql.DB) IncidentRepo {
	return &sqliteIncidentRepo{db: db}
}

// Create inserts a new incident and returns its auto-generated ID.
func (r *sqliteIncidentRepo) Create(ctx context.Context, inc *Incident) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO incidents (source, service_name, severity, status, trace_id, started_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'))`,
		inc.Source, inc.ServiceName, inc.Severity, inc.Status, inc.TraceID,
	)
	if err != nil {
		return 0, fmt.Errorf("incident_repo: create: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("incident_repo: last insert id: %w", err)
	}
	return id, nil
}

// GetByID retrieves a single incident by its primary key.
func (r *sqliteIncidentRepo) GetByID(ctx context.Context, id int64) (*Incident, error) {
	var inc Incident
	err := r.db.QueryRowContext(ctx,
		`SELECT id, source, service_name, severity, status, trace_id, created_at
		 FROM incidents WHERE id = ?`, id,
	).Scan(&inc.ID, &inc.Source, &inc.ServiceName, &inc.Severity, &inc.Status, &inc.TraceID, &inc.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("incident_repo: incident %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("incident_repo: get by id: %w", err)
	}
	return &inc, nil
}

// List returns incidents matching the given filter, ordered by created_at DESC.
func (r *sqliteIncidentRepo) List(ctx context.Context, filter IncidentFilter) ([]Incident, error) {
	query := `SELECT id, source, service_name, severity, status, trace_id, created_at
			  FROM incidents WHERE 1=1`
	args := []any{}

	if filter.Service != "" {
		query += " AND service_name = ?"
		args = append(args, filter.Service)
	}
	if filter.Severity != "" {
		query += " AND severity = ?"
		args = append(args, filter.Severity)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		if filter.Limit <= 0 {
			query += " LIMIT 100"
		}
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("incident_repo: list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []Incident
	for rows.Next() {
		var inc Incident
		if err := rows.Scan(&inc.ID, &inc.Source, &inc.ServiceName, &inc.Severity, &inc.Status, &inc.TraceID, &inc.CreatedAt); err != nil {
			return nil, fmt.Errorf("incident_repo: scan: %w", err)
		}
		items = append(items, inc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("incident_repo: list rows: %w", err)
	}
	return items, nil
}

// UpdateStatus changes the status of an existing incident.
func (r *sqliteIncidentRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE incidents SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("incident_repo: update status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("incident_repo: incident %d not found", id)
	}
	return nil
}
