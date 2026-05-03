CREATE TABLE IF NOT EXISTS issue_assignments (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL,
    agent_id TEXT,
    requested_by TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL DEFAULT '',
    client_request_id TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL,
    status TEXT NOT NULL,
    dedupe_key TEXT NOT NULL,
    requested_at TEXT NOT NULL,
    accepted_at TEXT,
    completed_at TEXT,
    failed_at TEXT,
    cancelled_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (issue_id) REFERENCES issues(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL,
    UNIQUE (client_request_id),
    UNIQUE (dedupe_key)
);

CREATE INDEX IF NOT EXISTS idx_issue_assignments_issue_status ON issue_assignments(issue_id, status);
CREATE INDEX IF NOT EXISTS idx_issue_assignments_agent ON issue_assignments(agent_id);
