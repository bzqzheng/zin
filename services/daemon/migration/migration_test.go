package migration_test

import (
	"testing"

	"github.com/bzqzheng/zin/services/daemon/db"
	"github.com/bzqzheng/zin/services/daemon/migration"
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
