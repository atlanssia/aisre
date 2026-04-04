-- Migration 006: Prompt templates for Prompt Studio
CREATE TABLE IF NOT EXISTS prompt_templates (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    stage       TEXT NOT NULL CHECK(stage IN ('context', 'evidence', 'rca', 'summary')),
    system_tpl  TEXT NOT NULL,
    user_tpl    TEXT NOT NULL,
    variables   TEXT DEFAULT '[]' CHECK(json_valid(variables)),
    is_default  INTEGER DEFAULT 0,
    version     INTEGER DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
