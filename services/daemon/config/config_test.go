package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bzqzheng/zin/services/daemon/config"
)

func TestDefault(t *testing.T) {
	cfg := config.Default()
	if cfg.Port != 0 {
		t.Errorf("expected default port 0, got %d", cfg.Port)
	}
	if cfg.DataDir == "" {
		t.Error("expected non-empty default data dir")
	}
}

func TestParseEnv(t *testing.T) {
	cfg, err := config.ParseArgs(nil, mapEnv(map[string]string{
		"ZIN_PORT":     "9999",
		"ZIN_DATA_DIR": "/tmp/zin-test-env",
	}))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/zin-test-env" {
		t.Errorf("expected data dir /tmp/zin-test-env, got %s", cfg.DataDir)
	}
}

func TestParseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.json")
	writeFile(t, path, `{"port":4321,"data_dir":"/tmp/zin-test-file"}`)

	cfg, err := config.ParseArgs([]string{"--config", path}, mapEnv(nil))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.Port != 4321 {
		t.Errorf("expected port 4321, got %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/zin-test-file" {
		t.Errorf("expected data dir /tmp/zin-test-file, got %s", cfg.DataDir)
	}
}

func TestParsePrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.json")
	writeFile(t, path, `{"port":1111,"data_dir":"/tmp/zin-test-file"}`)

	cfg, err := config.ParseArgs(
		[]string{"--config", path, "--port", "3333", "--data-dir", "/tmp/zin-test-flag"},
		mapEnv(map[string]string{
			"ZIN_PORT":     "2222",
			"ZIN_DATA_DIR": "/tmp/zin-test-env",
		}),
	)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.Port != 3333 {
		t.Errorf("expected flag port 3333, got %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/zin-test-flag" {
		t.Errorf("expected flag data dir /tmp/zin-test-flag, got %s", cfg.DataDir)
	}
}

func TestParseEnvOverridesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.json")
	writeFile(t, path, `{"port":1111,"data_dir":"/tmp/zin-test-file"}`)

	cfg, err := config.ParseArgs(
		[]string{"--config", path},
		mapEnv(map[string]string{
			"ZIN_PORT":     "2222",
			"ZIN_DATA_DIR": "/tmp/zin-test-env",
		}),
	)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.Port != 2222 {
		t.Errorf("expected env port 2222, got %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/zin-test-env" {
		t.Errorf("expected env data dir /tmp/zin-test-env, got %s", cfg.DataDir)
	}
}

func mapEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
}
