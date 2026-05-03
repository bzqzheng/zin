package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProbeMapsHealthyVersionCommand(t *testing.T) {
	binary := writeExecutable(t, "codex", "#!/bin/sh\necho 'codex 1.2.3'\n")

	result := Probe("codex", binary)

	if result.HealthStatus != StatusHealthy {
		t.Fatalf("expected healthy, got %#v", result)
	}
	if result.HealthReason != "" {
		t.Fatalf("expected empty health reason, got %q", result.HealthReason)
	}
	if result.VersionRaw != "codex 1.2.3" {
		t.Fatalf("expected captured version, got %q", result.VersionRaw)
	}
}

func TestProbeMapsTimeoutToDegraded(t *testing.T) {
	binary := writeExecutable(t, "codex", "#!/bin/sh\nsleep 10\n")

	start := time.Now()
	result := Probe("codex", binary)
	elapsed := time.Since(start)

	if result.HealthStatus != StatusDegraded || result.HealthReason != ReasonProbeTimeout {
		t.Fatalf("expected timeout degraded, got %#v", result)
	}
	if elapsed > 4*time.Second {
		t.Fatalf("expected hard timeout near 3s, took %s", elapsed)
	}
}

func TestProbeTruncatesOutputAndSanitizesInvalidUTF8(t *testing.T) {
	binary := writeExecutable(t, "codex", "#!/bin/sh\nprintf 'codex '\ni=0\nwhile [ \"$i\" -lt 6000 ]; do printf x; i=$((i + 1)); done\nprintf '\\377'\n")

	result := Probe("codex", binary)

	if result.HealthStatus != StatusHealthy {
		t.Fatalf("expected long output to remain healthy after truncation, got %#v", result)
	}
	if !strings.Contains(result.VersionRaw, "...[truncated]") {
		t.Fatalf("expected truncated marker, got %q", result.VersionRaw)
	}
	if len(result.VersionRaw) > outputLimit+len("...[truncated]") {
		t.Fatalf("expected output cap, got %d bytes", len(result.VersionRaw))
	}
}

func TestProbeMissingBinaryReturnsDegraded(t *testing.T) {
	result := Probe("codex", "/nonexistent/codex")

	if result.HealthStatus != StatusDegraded || result.HealthReason != ReasonInvalidBinaryPath {
		t.Fatalf("expected degraded for nonexistent path, got %#v", result)
	}
}

func TestProbeEmptyBinaryPathReturnsMissing(t *testing.T) {
	result := Probe("codex", "")

	if result.HealthStatus != StatusMissing || result.HealthReason != ReasonNotFound {
		t.Fatalf("expected missing status for empty path, got %#v", result)
	}
}

func TestProbeExitErrorReturnsDegraded(t *testing.T) {
	binary := writeExecutable(t, "codex", "#!/bin/sh\nexit 9\n")

	result := Probe("codex", binary)

	if result.HealthStatus != StatusDegraded || result.HealthReason != ReasonProbeFailed {
		t.Fatalf("expected degraded on probe exit error, got %#v", result)
	}
}

func TestProbeNonExecutablePathReturnsDegraded(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "not_executable")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	result := Probe("codex", path)

	if result.HealthStatus != StatusDegraded || result.HealthReason != ReasonInvalidBinaryPath {
		t.Fatalf("expected degraded for non-executable, got %#v", result)
	}
}

func TestProbeDirectoryPathReturnsDegraded(t *testing.T) {
	result := Probe("codex", t.TempDir())

	if result.HealthStatus != StatusDegraded || result.HealthReason != ReasonInvalidBinaryPath {
		t.Fatalf("expected degraded for directory path, got %#v", result)
	}
}

func TestValidateBinaryPathRejectsUnsafePaths(t *testing.T) {
	for _, path := range []string{"codex", "/tmp/codex\x00bad"} {
		if err := ValidateBinaryPath(path); err == nil {
			t.Fatalf("expected invalid path %q to fail", path)
		}
	}
}

func TestValidateBinaryPathRejectsMissingAndDirectory(t *testing.T) {
	if err := ValidateBinaryPath("/nonexistent/binary"); err == nil {
		t.Fatal("expected error for missing binary path")
	}
	if err := ValidateBinaryPath(t.TempDir()); err == nil {
		t.Fatal("expected error for directory path")
	}
}

func TestLookPathReturnsEmptyForUnsupportedKind(t *testing.T) {
	if path := LookPath("unsupported"); path != "" {
		t.Fatalf("expected empty path for unsupported kind, got %q", path)
	}
}

func TestLookPathReturnsEmptyForMissingKind(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir)

	if path := LookPath("codex"); path != "" {
		t.Fatalf("expected empty path for missing binary, got %q", path)
	}
}

func writeExecutable(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	return path
}
