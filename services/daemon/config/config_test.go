package config_test

import (
	"os"
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
	os.Setenv("ZIN_PORT", "9999")
	os.Setenv("ZIN_DATA_DIR", "/tmp/zin-test-env")
	defer func() {
		os.Unsetenv("ZIN_PORT")
		os.Unsetenv("ZIN_DATA_DIR")
	}()

	cfg := config.Parse()
	if cfg.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Port)
	}
	if cfg.DataDir != "/tmp/zin-test-env" {
		t.Errorf("expected data dir /tmp/zin-test-env, got %s", cfg.DataDir)
	}
}
