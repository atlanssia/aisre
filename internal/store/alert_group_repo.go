package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type sqliteAlertGroupRepo struct {
	db *sql.DB
}

// NewAlertGroupRepo creates a new AlertGroupRepo backed by SQLite.
func NewAlertGroupRepo(db *sql.DB) AlertGroupRepo {
	return &sqliteAlertGroupRepo{db: db}
}

func (r *sqliteAlertGroupRepo) Create(ctx context.Context, ag *AlertGroup) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO alert_groups (fingerprint, title, severity, labels, incident_id, count, first_seen, last_seen)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ag.Fingerprint, ag.Title, ag.Severity, ag.Labels, ag.IncidentID, ag.Count, ag.FirstSeen, ag.LastSeen,
	)
	if err != nil {
		return 0, fmt.Errorf("alert_group_repo: create: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("alert_group_repo: last insert id: %w", err)
	}
	return id, nil
}

func (r *sqliteAlertGroupRepo) GetByID(ctx context.Context, id int64) (*AlertGroup, error) {
	var ag AlertGroup
	var incidentID sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT id, fingerprint, title, severity, labels, incident_id, count, first_seen, last_seen, created_at
		 FROM alert_groups WHERE id = ?`, id,
	).Scan(&ag.ID, &ag.Fingerprint, &ag.Title, &ag.Severity, &ag.Labels, &incidentID, &ag.Count, &ag.FirstSeen, &ag.LastSeen, &ag.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("alert_group_repo: %d not found: %w", id, sql.ErrNoRows)
	}
	if err != nil {
		return nil, fmt.Errorf("alert_group_repo: get: %w", err)
	}
	if incidentID.Valid {
		val := incidentID.Int64
		ag.IncidentID = &val
	}
	return &ag, nil
}

func (r *sqliteAlertGroupRepo) GetByFingerprint(ctx context.Context, fp string) (*AlertGroup, error) {
	var ag AlertGroup
	var incidentID sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT id, fingerprint, title, severity, labels, incident_id, count, first_seen, last_seen, created_at
		 FROM alert_groups WHERE fingerprint = ?`, fp,
	).Scan(&ag.ID, &ag.Fingerprint, &ag.Title, &ag.Severity, &ag.Labels, &incidentID, &ag.Count, &ag.FirstSeen, &ag.LastSeen, &ag.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("alert_group_repo: fingerprint %s not found: %w", fp, sql.ErrNoRows)
	}
	if err != nil {
		return nil, fmt.Errorf("alert_group_repo: get by fingerprint: %w", err)
	}
	if incidentID.Valid {
		val := incidentID.Int64
		ag.IncidentID = &val
	}
	return &ag, nil
}

func (r *sqliteAlertGroupRepo) Update(ctx context.Context, ag *AlertGroup) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE alert_groups SET title=?, severity=?, labels=?, incident_id=?, count=?, first_seen=?, last_seen=?, updated_at=datetime('now') WHERE id = ?`,
		ag.Title, ag.Severity, ag.Labels, ag.IncidentID, ag.Count, ag.FirstSeen, ag.LastSeen, ag.ID,
	)
	if err != nil {
		return fmt.Errorf("alert_group_repo: update: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("alert_group_repo: %d not found for update", ag.ID)
	}
	return nil
}

func (r *sqliteAlertGroupRepo) List(ctx context.Context, filter AlertGroupFilter) ([]AlertGroup, error) {
	query := `SELECT id, fingerprint, title, severity, labels, incident_id, count, first_seen, last_seen, created_at
		 FROM alert_groups WHERE 1=1`
	args := []any{}

	if filter.Severity != "" {
		query += " AND severity = ?"
		args = append(args, filter.Severity)
	}
	if filter.StartTime != "" {
		query += " AND last_seen >= ?"
		args = append(args, filter.StartTime)
	}
	if filter.EndTime != "" {
		query += " AND last_seen <= ?"
		args = append(args, filter.EndTime)
	}

	query += " ORDER BY last_seen DESC"

	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	query += " LIMIT ?"
	args = append(args, filter.Limit)
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("alert_group_repo: list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []AlertGroup
	for rows.Next() {
		var ag AlertGroup
		var incidentID sql.NullInt64
		if err := rows.Scan(&ag.ID, &ag.Fingerprint, &ag.Title, &ag.Severity, &ag.Labels, &incidentID, &ag.Count, &ag.FirstSeen, &ag.LastSeen, &ag.CreatedAt); err != nil {
			return nil, fmt.Errorf("alert_group_repo: scan: %w", err)
		}
		if incidentID.Valid {
			val := incidentID.Int64
			ag.IncidentID = &val
		}
		results = append(results, ag)
	}
	return results, rows.Err()
}
