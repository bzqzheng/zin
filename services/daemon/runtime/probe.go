package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	StatusHealthy  = "healthy"
	StatusDegraded = "degraded"
	StatusMissing  = "missing"

	ReasonNotFound          = "not_found"
	ReasonInvalidBinaryPath = "invalid_binary_path"
	ReasonProbeTimeout      = "probe_timeout"
	ReasonProbeFailed       = "probe_failed"
	ReasonOutputUnparseable = "output_unparseable"
	ReasonRuntimeMismatch   = "runtime_kind_mismatch"
)

const outputLimit = 4 * 1024

var SupportedKinds = []string{"claude", "codex", "gemini", "opencode"}

type ProbeResult struct {
	Kind         string
	BinaryPath   string
	VersionRaw   string
	HealthStatus string
	HealthReason string
}

func IsSupportedKind(kind string) bool {
	for _, supported := range SupportedKinds {
		if kind == supported {
			return true
		}
	}
	return false
}

func DisplayName(kind string) string {
	switch kind {
	case "claude":
		return "Claude CLI"
	case "codex":
		return "Codex CLI"
	case "gemini":
		return "Gemini CLI"
	case "opencode":
		return "OpenCode CLI"
	default:
		return kind
	}
}

func Probe(kind, binaryPath string) ProbeResult {
	result := ProbeResult{
		Kind:         kind,
		BinaryPath:   binaryPath,
		HealthStatus: StatusMissing,
		HealthReason: ReasonNotFound,
	}
	if binaryPath == "" {
		return result
	}
	if err := ValidateBinaryPath(binaryPath); err != nil {
		result.HealthStatus = StatusDegraded
		result.HealthReason = ReasonInvalidBinaryPath
		return result
	}

	stdout, stderr, timedOut, err := runVersionProbe(binaryPath)
	output := strings.TrimSpace(strings.ToValidUTF8(joinOutput(stdout, stderr), ""))
	result.VersionRaw = output

	switch {
	case timedOut:
		result.HealthStatus = StatusDegraded
		result.HealthReason = ReasonProbeTimeout
	case err != nil:
		result.HealthStatus = StatusDegraded
		result.HealthReason = ReasonProbeFailed
	case !parseableOutput(output):
		result.HealthStatus = StatusDegraded
		result.HealthReason = ReasonOutputUnparseable
	case mismatchedRuntimeKind(kind, output):
		result.HealthStatus = StatusDegraded
		result.HealthReason = ReasonRuntimeMismatch
	default:
		result.HealthStatus = StatusHealthy
		result.HealthReason = ""
	}

	return result
}

func ValidateBinaryPath(binaryPath string) error {
	if binaryPath == "" {
		return errors.New("binary path is required")
	}
	if strings.ContainsRune(binaryPath, '\x00') {
		return errors.New("binary path contains null byte")
	}
	if !filepath.IsAbs(binaryPath) {
		return errors.New("binary path must be absolute")
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("stat binary path: %w", err)
	}
	if info.IsDir() {
		return errors.New("binary path is a directory")
	}
	if info.Mode().Perm()&0111 == 0 {
		return errors.New("binary path is not executable")
	}
	return nil
}

func LookPath(kind string) string {
	if !IsSupportedKind(kind) {
		return ""
	}
	path, err := exec.LookPath(kind)
	if err != nil {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absolute
}

func runVersionProbe(binaryPath string) (stdout string, stderr string, timedOut bool, err error) {
	cmd := exec.Command(binaryPath, "--version")
	cmd.Env = append(os.Environ(), "CI=1", "TERM=dumb", "NO_COLOR=1")
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdoutBuf cappedBuffer
	var stderrBuf cappedBuffer
	stdoutBuf.limit = outputLimit
	stderrBuf.limit = outputLimit
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return "", "", false, err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()

	select {
	case err := <-done:
		return stdoutBuf.String(), stderrBuf.String(), false, err
	case <-timer.C:
		terminateProcessGroup(cmd.Process.Pid)
		select {
		case err := <-done:
			return stdoutBuf.String(), stderrBuf.String(), true, err
		case <-time.After(200 * time.Millisecond):
			killProcessGroup(cmd.Process.Pid)
			err := <-done
			return stdoutBuf.String(), stderrBuf.String(), true, err
		}
	}
}

func terminateProcessGroup(pid int) {
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

func killProcessGroup(pid int) {
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func joinOutput(stdout, stderr string) string {
	switch {
	case stdout != "" && stderr != "":
		return stdout + "\n" + stderr
	case stdout != "":
		return stdout
	default:
		return stderr
	}
}

func parseableOutput(output string) bool {
	return output != "" && utf8.ValidString(output)
}

func mismatchedRuntimeKind(kind, output string) bool {
	lower := strings.ToLower(output)
	foundOther := ""
	for _, supported := range SupportedKinds {
		if supported == kind {
			continue
		}
		if strings.Contains(lower, supported) {
			foundOther = supported
			break
		}
	}
	return foundOther != "" && !strings.Contains(lower, kind)
}

type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) <= remaining {
			b.buf.Write(p)
		} else {
			b.buf.Write(p[:remaining])
			b.truncated = true
		}
	} else if len(p) > 0 {
		b.truncated = true
	}
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	if b.truncated {
		return b.buf.String() + "...[truncated]"
	}
	return b.buf.String()
}
