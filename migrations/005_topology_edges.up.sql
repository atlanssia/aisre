-- Migration 005: Service topology edges for blast radius analysis
CREATE TABLE IF NOT EXISTS topology_edges (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    source      TEXT NOT NULL,
    target      TEXT NOT NULL,
    relation    TEXT NOT NULL DEFAULT 'calls' CHECK(relation IN ('calls', 'depends_on', 'publishes')),
    metadata    TEXT DEFAULT '{}' CHECK(json_valid(metadata)),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_topology_edge ON topology_edges(source, target, relation);
