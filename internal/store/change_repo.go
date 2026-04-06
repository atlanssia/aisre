package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type sqliteChangeRepo struct {
	db *sql.DB
}

// NewChangeRepo creates a new ChangeRepo backed by SQLite.
func NewChangeRepo(db *sql.DB) ChangeRepo {
	return &sqliteChangeRepo{db: db}
}

func (r *sqliteChangeRepo) Create(ctx context.Context, ch *Change) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO changes (service, change_type, summary, author, timestamp, metadata)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		ch.Service, ch.ChangeType, ch.Summary, ch.Author, ch.Timestamp, ch.Metadata,
	)
	if err != nil {
		return 0, fmt.Errorf("change_repo: create: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("change_repo: last insert id: %w", err)
	}
	return id, nil
}

func (r *sqliteChangeRepo) GetByID(ctx context.Context, id int64) (*Change, error) {
	var ch Change
	err := r.db.QueryRowContext(ctx,
		`SELECT id, service, change_type, summary, author, timestamp, metadata, created_at
		 FROM changes WHERE id = ?`, id,
	).Scan(&ch.ID, &ch.Service, &ch.ChangeType, &ch.Summary, &ch.Author, &ch.Timestamp, &ch.Metadata, &ch.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("change_repo: change %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("change_repo: get: %w", err)
	}
	return &ch, nil
}

func (r *sqliteChangeRepo) List(ctx context.Context, filter ChangeFilter) ([]Change, error) {
	query := `SELECT id, service, change_type, summary, author, timestamp, metadata, created_at
			  FROM changes WHERE 1=1`
	args := []any{}

	if filter.Service != "" {
		query += " AND service = ?"
		args = append(args, filter.Service)
	}
	if len(filter.ChangeTypes) > 0 {
		placeholders := make([]string, len(filter.ChangeTypes))
		for i, ct := range filter.ChangeTypes {
			placeholders[i] = "?"
			args = append(args, ct)
		}
		query += fmt.Sprintf(" AND change_type IN (%s)", strings.Join(placeholders, ","))
	}
	if filter.StartTime != "" {
		query += " AND timestamp >= ?"
		args = append(args, filter.StartTime)
	}
	if filter.EndTime != "" {
		query += " AND timestamp <= ?"
		args = append(args, filter.EndTime)
	}

	query += " ORDER BY timestamp DESC"

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
		return nil, fmt.Errorf("change_repo: list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []Change
	for rows.Next() {
		var ch Change
		if err := rows.Scan(&ch.ID, &ch.Service, &ch.ChangeType, &ch.Summary, &ch.Author, &ch.Timestamp, &ch.Metadata, &ch.CreatedAt); err != nil {
			return nil, fmt.Errorf("change_repo: scan: %w", err)
		}
		results = append(results, ch)
	}
	return results, rows.Err()
}

func (r *sqliteChangeRepo) ListByService(ctx context.Context, service string, startTime, endTime string) ([]Change, error) {
	return r.List(ctx, ChangeFilter{
		Service:   service,
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     50,
	})
}
