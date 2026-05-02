package pidfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bzqzheng/zin/services/daemon/pidfile"
)

func TestWriteAndRemove(t *testing.T) {
	dir := t.TempDir()

	if err := pidfile.Write(dir); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	path := pidfile.Path(dir)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("pid file was not created")
	}

	if err := pidfile.Remove(dir); err != nil {
		t.Fatalf("remove pid file: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("pid file was not removed")
	}
}

func TestPath(t *testing.T) {
	path := pidfile.Path("/tmp/zin")
	expected := filepath.Join("/tmp/zin", "daemon.pid")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestStaleDetection(t *testing.T) {
	dir := t.TempDir()

	if pidfile.IsStale(dir) {
		t.Error("expected no stale pid file in empty dir")
	}

	pidfile.Write(dir)
	if pidfile.IsStale(dir) {
		t.Error("own pid should not be stale")
	}

	pidfile.Remove(dir)

	pidPath := pidfile.Path(dir)
	os.WriteFile(pidPath, []byte("999999\n"), 0644)
	if !pidfile.IsStale(dir) {
		t.Error("non-existent pid should be stale")
	}

	os.WriteFile(pidPath, []byte("not-a-number\n"), 0644)
	if !pidfile.IsStale(dir) {
		t.Error("non-numeric pid should be stale")
	}
}

func TestCleanupStale(t *testing.T) {
	dir := t.TempDir()

	pidPath := pidfile.Path(dir)
	os.WriteFile(pidPath, []byte("999999\n"), 0644)

	if err := pidfile.CleanupStale(dir); err != nil {
		t.Fatalf("cleanup stale: %v", err)
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("stale pid file was not cleaned up")
	}
}
