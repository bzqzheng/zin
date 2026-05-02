package pidfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func Path(dataDir string) string {
	return filepath.Join(dataDir, "daemon.pid")
}

func Write(dataDir string) error {
	path := Path(dataDir)
	pid := os.Getpid()
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	return nil
}

func Remove(dataDir string) error {
	path := Path(dataDir)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove pid file: %w", err)
	}
	return nil
}

func IsStale(dataDir string) bool {
	path := Path(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return true
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return true
	}

	err = process.Signal(syscall.Signal(0))
	return err != nil
}

func CleanupStale(dataDir string) error {
	path := Path(dataDir)
	if !IsStale(dataDir) {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cleanup stale pid file: %w", err)
	}
	return nil
}
