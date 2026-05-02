package db_test

import (
	"testing"

	"github.com/bzqzheng/zin/services/daemon/db"
)

func TestOpenAndIntegrity(t *testing.T) {
	dir := t.TempDir()

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	if err := db.IntegrityCheck(database); err != nil {
		t.Fatalf("integrity check: %v", err)
	}
}
