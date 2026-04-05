package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type sqliteEmbeddingRepo struct {
	db *sql.DB
}

// NewEmbeddingRepo creates a new EmbeddingRepo backed by SQLite.
func NewEmbeddingRepo(db *sql.DB) EmbeddingRepo {
	return &sqliteEmbeddingRepo{db: db}
}

func (r *sqliteEmbeddingRepo) Create(ctx context.Context, incidentID int64, service string, embedding []byte, model string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO incident_embeddings (incident_id, service, embedding, model)
		 VALUES (?, ?, ?, ?)`,
		incidentID, service, embedding, model,
	)
	if err != nil {
		return fmt.Errorf("embedding_repo: create: %w", err)
	}
	return nil
}

func (r *sqliteEmbeddingRepo) GetByIncidentID(ctx context.Context, incidentID int64) (*Embedding, error) {
	var e Embedding
	err := r.db.QueryRowContext(ctx,
		`SELECT incident_id, service, embedding, model, created_at
		 FROM incident_embeddings WHERE incident_id = ?`, incidentID,
	).Scan(&e.IncidentID, &e.Service, &e.Embedding, &e.Model, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("embedding_repo: embedding for incident %d not found", incidentID)
	}
	if err != nil {
		return nil, fmt.Errorf("embedding_repo: get: %w", err)
	}
	return &e, nil
}

func (r *sqliteEmbeddingRepo) ListByService(ctx context.Context, service string) ([]Embedding, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT incident_id, service, embedding, model, created_at
		 FROM incident_embeddings WHERE service = ?`, service,
	)
	if err != nil {
		return nil, fmt.Errorf("embedding_repo: list by service: %w", err)
	}
	defer rows.Close()

	var results []Embedding
	for rows.Next() {
		var e Embedding
		if err := rows.Scan(&e.IncidentID, &e.Service, &e.Embedding, &e.Model, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("embedding_repo: scan: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}
