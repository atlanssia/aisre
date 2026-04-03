CREATE TABLE IF NOT EXISTS incident_embeddings (
    incident_id INTEGER NOT NULL,
    service     TEXT NOT NULL DEFAULT '',
    embedding   BLOB NOT NULL,
    model       TEXT NOT NULL DEFAULT 'text-embedding-3-small',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (incident_id),
    FOREIGN KEY (incident_id) REFERENCES incidents(id)
);
CREATE INDEX IF NOT EXISTS idx_embeddings_service ON incident_embeddings(service);
