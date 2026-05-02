package migration

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

func Run(db *sql.DB) error {
	entries, err := sqlFiles.ReadDir("sql")
	if err != nil {
		return fmt.Errorf("read migration dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		version := entry.Name()

		applied, err := migrationApplied(tx, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		alreadyCurrent, err := migrationAlreadyCurrent(tx, version)
		if err != nil {
			return err
		}
		if alreadyCurrent {
			if version == "002_upgrade_issues_contract.sql" {
				if _, err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_issues_project_position ON issues(project_id, position)"); err != nil {
					return fmt.Errorf("create issue position index: %w", err)
				}
			}
			if err := markMigrationApplied(tx, version); err != nil {
				return err
			}
			continue
		}

		content, err := sqlFiles.ReadFile("sql/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		if _, err := tx.Exec(string(content)); err != nil {
			return fmt.Errorf("exec %s: %w", entry.Name(), err)
		}
		if err := markMigrationApplied(tx, version); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func migrationApplied(tx *sql.Tx, version string) (bool, error) {
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return count > 0, nil
}

func migrationAlreadyCurrent(tx *sql.Tx, version string) (bool, error) {
	if version != "002_upgrade_issues_contract.sql" {
		return false, nil
	}

	var count int
	if err := tx.QueryRow(`
SELECT COUNT(*)
FROM pragma_table_info('issues')
WHERE name IN ('identifier', 'position') AND [notnull] = 1
`).Scan(&count); err != nil {
		return false, fmt.Errorf("check issue contract schema: %w", err)
	}

	return count == 2, nil
}

func markMigrationApplied(tx *sql.Tx, version string) error {
	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		return fmt.Errorf("mark migration %s applied: %w", version, err)
	}
	return nil
}
