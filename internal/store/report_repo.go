package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// sqliteReportRepo implements the ReportRepo interface with SQLite backend.
type sqliteReportRepo struct {
	db *sql.DB
}

// NewReportRepo creates a new ReportRepo backed by the given database.
func NewReportRepo(db *sql.DB) ReportRepo {
	return &sqliteReportRepo{db: db}
}

// Create inserts a new RCA report and returns its auto-generated ID.
func (r *sqliteReportRepo) Create(ctx context.Context, report *Report) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO rca_reports (incident_id, summary, root_cause, confidence, report_json, status)
		 VALUES (?, ?, ?, ?, ?, 'generated')`,
		report.IncidentID, report.Summary, report.RootCause, report.Confidence, report.ReportJSON,
	)
	if err != nil {
		return 0, fmt.Errorf("report_repo: create: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("report_repo: last insert id: %w", err)
	}
	return id, nil
}

// GetByID retrieves a single report by its primary key.
func (r *sqliteReportRepo) GetByID(ctx context.Context, id int64) (*Report, error) {
	var report Report
	err := r.db.QueryRowContext(ctx,
		`SELECT id, incident_id, summary, root_cause, confidence, report_json, status, created_at
		 FROM rca_reports WHERE id = ?`, id,
	).Scan(&report.ID, &report.IncidentID, &report.Summary, &report.RootCause,
		&report.Confidence, &report.ReportJSON, &report.Status, &report.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("report_repo: report %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("report_repo: get by id: %w", err)
	}
	return &report, nil
}

// List returns reports matching the given filter, ordered by created_at DESC.
func (r *sqliteReportRepo) List(ctx context.Context, filter ReportFilter) ([]Report, error) {
	query := `SELECT r.id, r.incident_id, r.summary, r.root_cause, r.confidence, r.report_json, r.status, r.created_at
			  FROM rca_reports r
			  JOIN incidents i ON r.incident_id = i.id
			  WHERE 1=1`
	args := []any{}

	if filter.IncidentID != 0 {
		query += " AND r.incident_id = ?"
		args = append(args, filter.IncidentID)
	}
	if filter.Service != "" {
		query += " AND i.service_name = ?"
		args = append(args, filter.Service)
	}
	if filter.Severity != "" {
		query += " AND i.severity = ?"
		args = append(args, filter.Severity)
	}

	query += " ORDER BY r.created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("report_repo: list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []Report
	for rows.Next() {
		var report Report
		if err := rows.Scan(&report.ID, &report.IncidentID, &report.Summary,
			&report.RootCause, &report.Confidence, &report.ReportJSON, &report.Status, &report.CreatedAt); err != nil {
			return nil, fmt.Errorf("report_repo: scan: %w", err)
		}
		items = append(items, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("report_repo: list rows: %w", err)
	}
	return items, nil
}

// Search performs a full-text search on report summary/root_cause with optional filters.
func (r *sqliteReportRepo) Search(ctx context.Context, query string, filter ReportFilter) ([]Report, error) {
	q := `SELECT r.id, r.incident_id, r.summary, r.root_cause, r.confidence, r.report_json, r.status, r.created_at
		  FROM rca_reports r
		  JOIN incidents i ON r.incident_id = i.id
		  WHERE (r.summary LIKE ? OR r.root_cause LIKE ?)`
	escaped := strings.ReplaceAll(query, "%", "\\%")
	escaped = strings.ReplaceAll(escaped, "_", "\\_")
	args := []any{"%" + escaped + "%", "%" + escaped + "%"}

	if filter.Service != "" {
		q += " AND i.service_name = ?"
		args = append(args, filter.Service)
	}
	if filter.Severity != "" {
		q += " AND i.severity = ?"
		args = append(args, filter.Severity)
	}

	q += " ORDER BY r.created_at DESC"

	if filter.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		q += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("report_repo: search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []Report
	for rows.Next() {
		var report Report
		if err := rows.Scan(&report.ID, &report.IncidentID, &report.Summary,
			&report.RootCause, &report.Confidence, &report.ReportJSON, &report.Status, &report.CreatedAt); err != nil {
			return nil, fmt.Errorf("report_repo: search scan: %w", err)
		}
		items = append(items, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("report_repo: search rows: %w", err)
	}
	return items, nil
}

// sqliteEvidenceRepo implements the EvidenceRepo interface with SQLite backend.
type sqliteEvidenceRepo struct {
	db *sql.DB
}

// NewEvidenceRepo creates a new EvidenceRepo backed by the given database.
func NewEvidenceRepo(db *sql.DB) EvidenceRepo {
	return &sqliteEvidenceRepo{db: db}
}

// Create inserts a new evidence item and returns its auto-generated ID.
func (r *sqliteEvidenceRepo) Create(ctx context.Context, evidence *Evidence) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO evidence_items (report_id, evidence_type, score, payload, source_url)
		 VALUES (?, ?, ?, ?, ?)`,
		evidence.ReportID, evidence.EvidenceType, evidence.Score, evidence.Payload, evidence.SourceURL,
	)
	if err != nil {
		return 0, fmt.Errorf("evidence_repo: create: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("evidence_repo: last insert id: %w", err)
	}
	return id, nil
}

// ListByReport retrieves all evidence items for a given report.
func (r *sqliteEvidenceRepo) ListByReport(ctx context.Context, reportID int64) ([]Evidence, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, report_id, evidence_type, score, payload, source_url, created_at
		 FROM evidence_items WHERE report_id = ? ORDER BY score DESC`, reportID,
	)
	if err != nil {
		return nil, fmt.Errorf("evidence_repo: list by report: %w", err)
	}
	defer rows.Close()

	var items []Evidence
	for rows.Next() {
		var ev Evidence
		if err := rows.Scan(&ev.ID, &ev.ReportID, &ev.EvidenceType,
			&ev.Score, &ev.Payload, &ev.SourceURL, &ev.CreatedAt); err != nil {
			return nil, fmt.Errorf("evidence_repo: scan: %w", err)
		}
		items = append(items, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("evidence_repo: list rows: %w", err)
	}
	return items, nil
}
