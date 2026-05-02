package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type DaemonConfig struct {
	Port       int
	DataDir    string
	ConfigPath string
}

type fileConfig struct {
	Port        *int   `json:"port"`
	DataDir     string `json:"data_dir"`
	DataDirName string `json:"dataDir"`
}

func Default() *DaemonConfig {
	home, _ := os.UserHomeDir()
	return &DaemonConfig{
		Port:       0,
		DataDir:    filepath.Join(home, ".zin"),
		ConfigPath: filepath.Join(home, ".zin", "daemon.json"),
	}
}

func Parse() (*DaemonConfig, error) {
	return ParseArgs(os.Args[1:], os.Getenv)
}

func ParseArgs(args []string, getenv func(string) string) (*DaemonConfig, error) {
	cfg := Default()

	flags := flag.NewFlagSet("daemon", flag.ContinueOnError)
	port := flags.Int("port", 0, "port to listen on (0 for random)")
	dataDir := flags.String("data-dir", "", "data directory path")
	configPath := flags.String("config", "", "config file path")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	explicit := map[string]bool{}
	flags.Visit(func(f *flag.Flag) {
		explicit[f.Name] = true
	})

	if v := getenv("ZIN_CONFIG"); v != "" {
		cfg.ConfigPath = v
	}
	if explicit["config"] {
		cfg.ConfigPath = *configPath
	}

	if err := applyConfigFile(cfg, explicit["config"]); err != nil {
		return nil, err
	}

	if v := getenv("ZIN_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := getenv("ZIN_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	if explicit["port"] {
		cfg.Port = *port
	}
	if explicit["data-dir"] {
		cfg.DataDir = *dataDir
	}

	return cfg, nil
}

func applyConfigFile(cfg *DaemonConfig, explicit bool) error {
	data, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}

	var parsed fileConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}

	if parsed.Port != nil {
		cfg.Port = *parsed.Port
	}
	if parsed.DataDir != "" {
		cfg.DataDir = parsed.DataDir
	}
	if parsed.DataDirName != "" {
		cfg.DataDir = parsed.DataDirName
	}

	return nil
}
