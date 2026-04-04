-- Migration 004: Change correlation events
CREATE TABLE IF NOT EXISTS changes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    service     TEXT NOT NULL,
    change_type TEXT NOT NULL CHECK(change_type IN ('deploy', 'config', 'feature_flag', 'infra')),
    summary     TEXT NOT NULL,
    author      TEXT,
    timestamp   TEXT NOT NULL,
    metadata    TEXT DEFAULT '{}' CHECK(json_valid(metadata)),
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_changes_service_time ON changes(service, timestamp);
CREATE UNIQUE INDEX IF NOT EXISTS idx_changes_dedup ON changes(service, change_type, timestamp, summary);
