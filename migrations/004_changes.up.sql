CREATE TABLE IF NOT EXISTS changes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    service TEXT NOT NULL,
    change_type TEXT NOT NULL,
    summary TEXT,
    author TEXT,
    timestamp TEXT NOT NULL,
    metadata TEXT DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_changes_service ON changes(service);
CREATE INDEX idx_changes_timestamp ON changes(timestamp);
CREATE INDEX idx_changes_change_type ON changes(change_type);
