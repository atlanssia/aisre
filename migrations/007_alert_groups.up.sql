-- Migration 007: Alert groups for alert aggregation
CREATE TABLE IF NOT EXISTS alert_groups (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    fingerprint TEXT NOT NULL UNIQUE,
    title       TEXT NOT NULL,
    severity    TEXT NOT NULL DEFAULT 'warning' CHECK(severity IN ('critical', 'high', 'medium', 'low', 'info')),
    labels      TEXT DEFAULT '{}' CHECK(json_valid(labels)),
    incident_id INTEGER,
    count       INTEGER DEFAULT 1,
    first_seen  TEXT NOT NULL,
    last_seen   TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (incident_id) REFERENCES incidents(id)
);
CREATE INDEX IF NOT EXISTS idx_alert_groups_severity_time ON alert_groups(severity, last_seen);
CREATE INDEX IF NOT EXISTS idx_alert_groups_incident ON alert_groups(incident_id);
