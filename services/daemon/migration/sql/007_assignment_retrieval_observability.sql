ALTER TABLE issue_assignments ADD COLUMN retrieval_status TEXT NOT NULL DEFAULT 'not_started';
ALTER TABLE issue_assignments ADD COLUMN retrieval_failure_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE issue_assignments ADD COLUMN retrieval_policy TEXT NOT NULL DEFAULT 'fail_fast';
ALTER TABLE issue_assignments ADD COLUMN retrieval_audit_metadata TEXT NOT NULL DEFAULT '';

ALTER TABLE assignment_results ADD COLUMN observability_degraded INTEGER NOT NULL DEFAULT 0;
ALTER TABLE assignment_results ADD COLUMN observability_degraded_reason TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS memory_influence_events (
    id TEXT PRIMARY KEY,
    assignment_id TEXT NOT NULL,
    result_id TEXT,
    memory_id TEXT NOT NULL DEFAULT '',
    influence_type TEXT NOT NULL,
    reason_code TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    applied_at TEXT NOT NULL,
    FOREIGN KEY (assignment_id) REFERENCES issue_assignments(id) ON DELETE CASCADE,
    FOREIGN KEY (result_id) REFERENCES assignment_results(result_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_memory_influence_events_assignment ON memory_influence_events(assignment_id, applied_at);
CREATE INDEX IF NOT EXISTS idx_memory_influence_events_result ON memory_influence_events(result_id);
