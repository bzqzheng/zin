package config

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
)

type DaemonConfig struct {
	Port    int
	DataDir string
}

func Default() *DaemonConfig {
	home, _ := os.UserHomeDir()
	return &DaemonConfig{
		Port:    0,
		DataDir: filepath.Join(home, ".zin"),
	}
}

func Parse() *DaemonConfig {
	cfg := Default()

	port := flag.Int("port", 0, "port to listen on (0 for random)")
	dataDir := flag.String("data-dir", "", "data directory path")
	flag.Parse()

	if *port != 0 {
		cfg.Port = *port
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}

	if v := os.Getenv("ZIN_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := os.Getenv("ZIN_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	return cfg
}
