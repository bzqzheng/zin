CREATE TABLE IF NOT EXISTS assignment_results (
    result_id TEXT PRIMARY KEY,
    assignment_id TEXT NOT NULL,
    attempt_no INTEGER NOT NULL,
    output TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    error TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    finished_at TEXT NOT NULL,
    FOREIGN KEY (assignment_id) REFERENCES issue_assignments(id) ON DELETE CASCADE,
    UNIQUE (assignment_id, attempt_no)
);

CREATE INDEX IF NOT EXISTS idx_assignment_results_assignment ON assignment_results(assignment_id, attempt_no);
