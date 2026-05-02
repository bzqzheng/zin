DROP TABLE IF EXISTS issues_upgrade;

CREATE TABLE issues_upgrade (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    identifier TEXT NOT NULL,
    position INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'todo',
    priority TEXT NOT NULL DEFAULT 'medium',
    assignee_id TEXT NOT NULL DEFAULT '',
    creator_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    UNIQUE (project_id, identifier),
    UNIQUE (project_id, position)
);

INSERT INTO issues_upgrade (
    id,
    project_id,
    identifier,
    position,
    title,
    description,
    status,
    priority,
    assignee_id,
    creator_id,
    created_at,
    updated_at
)
SELECT
    id,
    project_id,
    'ISSUE-' || rn,
    rn,
    title,
    description,
    COALESCE(NULLIF(status, ''), 'todo'),
    COALESCE(NULLIF(priority, ''), 'medium'),
    assignee_id,
    creator_id,
    created_at,
    updated_at
FROM (
    SELECT
        id,
        project_id,
        title,
        description,
        status,
        priority,
        assignee_id,
        creator_id,
        created_at,
        updated_at,
        ROW_NUMBER() OVER (
            PARTITION BY project_id
            ORDER BY created_at ASC, id ASC
        ) AS rn
    FROM issues
);

DROP TABLE issues;
ALTER TABLE issues_upgrade RENAME TO issues;

CREATE INDEX IF NOT EXISTS idx_issues_project_id ON issues(project_id);
CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(status);
CREATE INDEX IF NOT EXISTS idx_issues_priority ON issues(priority);
CREATE INDEX IF NOT EXISTS idx_issues_project_position ON issues(project_id, position);
