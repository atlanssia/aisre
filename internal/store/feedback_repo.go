package store

import (
	"context"
	"database/sql"
	"fmt"
)

// sqliteFeedbackRepo implements the FeedbackRepo interface with SQLite backend.
type sqliteFeedbackRepo struct {
	db *sql.DB
}

// NewFeedbackRepo creates a new FeedbackRepo backed by the given database.
func NewFeedbackRepo(db *sql.DB) FeedbackRepo {
	return &sqliteFeedbackRepo{db: db}
}

// Create inserts a new feedback entry and returns its auto-generated ID.
func (r *sqliteFeedbackRepo) Create(ctx context.Context, fb *Feedback) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO feedback (report_id, user_id, rating, comment, action_taken)
		 VALUES (?, ?, ?, ?, ?)`,
		fb.ReportID, fb.UserID, fb.Rating, fb.Comment, fb.ActionTaken,
	)
	if err != nil {
		return 0, fmt.Errorf("feedback_repo: create: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("feedback_repo: last insert id: %w", err)
	}
	return id, nil
}

// ListByReport retrieves all feedback entries for a given report, ordered by created_at DESC.
func (r *sqliteFeedbackRepo) ListByReport(ctx context.Context, reportID int64) ([]Feedback, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, report_id, user_id, rating, comment, action_taken, created_at
		 FROM feedback WHERE report_id = ? ORDER BY created_at DESC`, reportID,
	)
	if err != nil {
		return nil, fmt.Errorf("feedback_repo: list by report: %w", err)
	}
	defer rows.Close()

	var items []Feedback
	for rows.Next() {
		var fb Feedback
		if err := rows.Scan(&fb.ID, &fb.ReportID, &fb.UserID, &fb.Rating,
			&fb.Comment, &fb.ActionTaken, &fb.CreatedAt); err != nil {
			return nil, fmt.Errorf("feedback_repo: scan: %w", err)
		}
		items = append(items, fb)
	}
	return items, nil
}
