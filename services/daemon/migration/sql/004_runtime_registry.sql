DROP TABLE IF EXISTS runtimes;

CREATE TABLE runtimes (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    display_name TEXT NOT NULL,
    binary_path TEXT NOT NULL DEFAULT '',
    version_raw TEXT NOT NULL DEFAULT '',
    health_status TEXT NOT NULL,
    health_reason TEXT NOT NULL DEFAULT '',
    last_checked_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (kind IN ('codex', 'claude', 'gemini', 'opencode')),
    CHECK (health_status IN ('healthy', 'degraded', 'missing')),
    UNIQUE (kind)
);

CREATE INDEX IF NOT EXISTS idx_runtimes_kind ON runtimes(kind);
CREATE INDEX IF NOT EXISTS idx_runtimes_health_status ON runtimes(health_status);
