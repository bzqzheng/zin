CREATE TABLE IF NOT EXISTS runtimes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    command TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'unknown',
    status_message TEXT NOT NULL DEFAULT '',
    last_checked_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_runtimes_status ON runtimes(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_runtimes_kind_command ON runtimes(kind, command);

ALTER TABLE agents ADD COLUMN runtime_id TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN model TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN instructions TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_agents_runtime_id ON agents(runtime_id);
