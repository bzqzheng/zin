ALTER TABLE agents ADD COLUMN runtime_id TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN model_hint TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN instructions TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN is_assignable INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_agents_runtime_id ON agents(runtime_id);
CREATE INDEX IF NOT EXISTS idx_agents_assignable ON agents(is_assignable);

CREATE TABLE IF NOT EXISTS issue_assignments (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL,
    agent_id TEXT,
    requested_by TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL DEFAULT '',
    client_request_id TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL,
    dedupe_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    requested_at TEXT NOT NULL,
    accepted_at TEXT,
    completed_at TEXT,
    failed_at TEXT,
    cancelled_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (source_type IN ('issue_detail', 'comment')),
    CHECK (status IN ('queued', 'accepted', 'completed', 'failed', 'cancelled')),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL,
    UNIQUE (client_request_id),
    UNIQUE (dedupe_key)
);

CREATE INDEX IF NOT EXISTS idx_issue_assignments_issue_created ON issue_assignments(issue_id, created_at);
CREATE INDEX IF NOT EXISTS idx_issue_assignments_agent_status ON issue_assignments(agent_id, status);
CREATE INDEX IF NOT EXISTS idx_issue_assignments_fingerprint ON issue_assignments(request_fingerprint);
