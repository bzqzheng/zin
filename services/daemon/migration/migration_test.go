package migration_test

import (
	"testing"

	"github.com/bzqzheng/zin/services/daemon/db"
	"github.com/bzqzheng/zin/services/daemon/migration"
	"github.com/bzqzheng/zin/services/daemon/store"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if err := migration.Run(database); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	var tableCount int
	if err := database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('projects','agents','issues')").Scan(&tableCount); err != nil {
		t.Fatalf("verify tables: %v", err)
	}
	if tableCount != 3 {
		t.Errorf("expected 3 tables, got %d", tableCount)
	}
}

func TestRunIdempotent(t *testing.T) {
	dir := t.TempDir()

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if err := migration.Run(database); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := migration.Run(database); err != nil {
		t.Fatalf("second run: %v", err)
	}
}

func TestRunUpgradesPreIssueContractSchema(t *testing.T) {
	dir := t.TempDir()

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if _, err := database.Exec(`
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'offline',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE issues (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    identifier TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'todo',
    priority TEXT NOT NULL DEFAULT 'medium',
    assignee_id TEXT NOT NULL DEFAULT '',
    creator_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

INSERT INTO projects (id, name, description, created_at, updated_at)
VALUES ('project-1', 'Legacy Project', '', '2026-05-02T20:00:00Z', '2026-05-02T20:00:00Z');

INSERT INTO issues (id, project_id, identifier, title, description, status, priority, created_at, updated_at)
VALUES
    ('issue-2', 'project-1', 'CALLER-2', 'Second', '', 'in_progress', 'high', '2026-05-02T20:02:00Z', '2026-05-02T20:02:00Z'),
    ('issue-1', 'project-1', '', 'First', '', 'todo', 'medium', '2026-05-02T20:01:00Z', '2026-05-02T20:01:00Z');
`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	if err := migration.Run(database); err != nil {
		t.Fatalf("run upgrade migrations: %v", err)
	}
	if err := migration.Run(database); err != nil {
		t.Fatalf("rerun upgrade migrations: %v", err)
	}

	rows, err := database.Query("SELECT identifier, position FROM issues WHERE project_id = 'project-1' ORDER BY position ASC")
	if err != nil {
		t.Fatalf("query upgraded issues: %v", err)
	}

	got := map[int]string{}
	for rows.Next() {
		var identifier string
		var position int
		if err := rows.Scan(&identifier, &position); err != nil {
			t.Fatalf("scan upgraded issue: %v", err)
		}
		got[position] = identifier
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate upgraded issues: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close upgraded issues rows: %v", err)
	}
	if got[1] != "ISSUE-1" || got[2] != "ISSUE-2" {
		t.Fatalf("expected backfilled identifiers by position, got %#v", got)
	}

	projectRepo := store.NewProjectRepository(database)
	issueRepo := store.NewIssueRepository(database)
	created, err := issueRepo.Create("project-1", "Third", "", "", "")
	if err != nil {
		t.Fatalf("create issue after upgrade: %v", err)
	}
	if created.Identifier != "ISSUE-3" || created.Position != 3 {
		t.Fatalf("expected next issue ISSUE-3/3, got %s/%d", created.Identifier, created.Position)
	}

	if err := projectRepo.Delete("project-1"); err != nil {
		t.Fatalf("delete project after upgrade: %v", err)
	}
	remaining, err := issueRepo.ListByProject("project-1", store.IssueFilters{})
	if err != nil {
		t.Fatalf("list issues after cascade delete: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected cascade delete after upgrade, got %d remaining issues", len(remaining))
	}
}
