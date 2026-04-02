CREATE TABLE IF NOT EXISTS incidents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    severity TEXT NOT NULL CHECK(severity IN ('critical','high','medium','low','info')),
    service_name TEXT NOT NULL,
    title TEXT,
    description TEXT,
    labels TEXT,
    trace_id TEXT,
    started_at DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','analyzing','resolved','closed')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rca_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    incident_id INTEGER NOT NULL REFERENCES incidents(id),
    summary TEXT,
    root_cause TEXT,
    confidence REAL,
    report_json TEXT,
    status TEXT NOT NULL DEFAULT 'generated',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS evidence_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL REFERENCES rca_reports(id),
    evidence_type TEXT NOT NULL,
    score REAL,
    payload TEXT,
    source_url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS recommendations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL REFERENCES rca_reports(id),
    category TEXT NOT NULL,
    description TEXT,
    priority INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id INTEGER NOT NULL REFERENCES rca_reports(id),
    user_id TEXT,
    rating INTEGER CHECK(rating BETWEEN 1 AND 5),
    comment TEXT,
    action_taken TEXT CHECK(action_taken IN ('accepted','partial','rejected')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_incidents_service ON incidents(service_name);
CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_reports_incident ON rca_reports(incident_id);
CREATE INDEX idx_evidence_report ON evidence_items(report_id);
CREATE INDEX idx_feedback_report ON feedback(report_id);
