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

func TestValidateBinaryPathRejectsUnsafePaths(t *testing.T) {
	for _, path := range []string{"codex", "/tmp/codex\x00bad"} {
		if err := ValidateBinaryPath(path); err == nil {
			t.Fatalf("expected invalid path %q to fail", path)
		}
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
