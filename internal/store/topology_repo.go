package store

import (
	"context"
	"database/sql"
	"fmt"
)

type sqliteTopologyRepo struct {
	db *sql.DB
}

// NewTopologyRepo creates a new TopologyRepo backed by SQLite.
func NewTopologyRepo(db *sql.DB) TopologyRepo {
	return &sqliteTopologyRepo{db: db}
}

func (r *sqliteTopologyRepo) Create(ctx context.Context, edge *TopologyEdge) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO topology_edges (source, target, relation, metadata)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(source, target, relation) DO UPDATE SET updated_at = datetime('now'), metadata = excluded.metadata`,
		edge.Source, edge.Target, edge.Relation, edge.Metadata,
	)
	if err != nil {
		return 0, fmt.Errorf("topology_repo: create: %w", err)
	}
	return result.LastInsertId()
}

func (r *sqliteTopologyRepo) List(ctx context.Context) ([]TopologyEdge, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, source, target, relation, metadata, created_at, updated_at
		 FROM topology_edges ORDER BY source, target`,
	)
	if err != nil {
		return nil, fmt.Errorf("topology_repo: list: %w", err)
	}
	defer rows.Close()

	var results []TopologyEdge
	for rows.Next() {
		var e TopologyEdge
		if err := rows.Scan(&e.ID, &e.Source, &e.Target, &e.Relation, &e.Metadata, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("topology_repo: scan: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (r *sqliteTopologyRepo) ListBySource(ctx context.Context, source string) ([]TopologyEdge, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, source, target, relation, metadata, created_at, updated_at
		 FROM topology_edges WHERE source = ? ORDER BY target`,
		source,
	)
	if err != nil {
		return nil, fmt.Errorf("topology_repo: list by source: %w", err)
	}
	defer rows.Close()

	var results []TopologyEdge
	for rows.Next() {
		var e TopologyEdge
		if err := rows.Scan(&e.ID, &e.Source, &e.Target, &e.Relation, &e.Metadata, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("topology_repo: scan: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (r *sqliteTopologyRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM topology_edges WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("topology_repo: delete: %w", err)
	}
	return nil
}
