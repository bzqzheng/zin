package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type runtimeRecord struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	DisplayName  string `json:"display_name"`
	BinaryPath   string `json:"binary_path"`
	VersionRaw   string `json:"version_raw"`
	HealthStatus string `json:"health_status"`
	HealthReason string `json:"health_reason"`
}

type discoverResponse struct {
	Runtimes []runtimeRecord `json:"runtimes"`
	Summary  struct {
		Healthy  int `json:"healthy"`
		Degraded int `json:"degraded"`
		Missing  int `json:"missing"`
	} `json:"summary"`
}

type runtimeListResponse struct {
	Runtimes []runtimeRecord `json:"runtimes"`
}

func TestE2E(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("ZIN_DATA_DIR", dir)
	defer os.Unsetenv("ZIN_DATA_DIR")

	runtimeDir := t.TempDir()
	writeMockRuntimeBinary(t, runtimeDir, "claude", "#!/bin/sh\necho 'Claude CLI version 1.0.0'\n")
	writeMockRuntimeBinary(t, runtimeDir, "codex", "#!/bin/sh\necho 'codex 1.2.3'\n")
	writeMockRuntimeBinary(t, runtimeDir, "gemini", "#!/bin/sh\necho 'gemini-cli 0.1.0'\n")

	daemonPath := ensureDaemonBinary(t)
	baseURL, stop := startDaemonWithPath(t, daemonPath, dir, runtimeDir)
	defer stop()

	t.Run("health check", func(t *testing.T) {
		var result map[string]interface{}
		if err := apiGet(fmt.Sprintf("%s/health", baseURL), &result); err != nil {
			t.Fatalf("health check failed: %v", err)
		}
		if result["status"] != "ok" {
			t.Errorf("expected status 'ok', got '%v'", result["status"])
		}
		if result["db_status"] != "ok" {
			t.Errorf("expected db_status 'ok', got '%v'", result["db_status"])
		}
		version, _ := result["version"].(string)
		if version != "0.1.0" {
			t.Errorf("expected version '0.1.0', got '%s'", version)
		}
	})

	var projectID string
	var issueID string
	var tagID string
	var commentID string

	t.Run("create project", func(t *testing.T) {
		body := map[string]string{
			"name":        "E2E Test Project",
			"description": "Created during E2E test",
		}
		var result map[string]interface{}
		if err := apiPost(fmt.Sprintf("%s/api/projects", baseURL), body, &result); err != nil {
			t.Fatalf("create project failed: %v", err)
		}
		projectID, _ = result["id"].(string)
		if projectID == "" {
			t.Fatal("expected non-empty project id")
		}
		if result["name"] != "E2E Test Project" {
			t.Errorf("expected name 'E2E Test Project', got '%v'", result["name"])
		}
	})

	t.Run("create issue", func(t *testing.T) {
		body := map[string]string{
			"title":       "E2E Test Issue",
			"description": "Created during E2E test",
			"priority":    "high",
			"status":      "todo",
		}
		var result map[string]interface{}
		url := fmt.Sprintf("%s/api/projects/%s/issues", baseURL, projectID)
		if err := apiPost(url, body, &result); err != nil {
			t.Fatalf("create issue failed: %v", err)
		}
		issueID, _ = result["id"].(string)
		if issueID == "" {
			t.Fatal("expected non-empty issue id")
		}
		if result["title"] != "E2E Test Issue" {
			t.Errorf("expected title 'E2E Test Issue', got '%v'", result["title"])
		}
		if result["identifier"] != "ISSUE-1" {
			t.Errorf("expected generated identifier 'ISSUE-1', got '%v'", result["identifier"])
		}
		if result["position"] != float64(1) {
			t.Errorf("expected generated position 1, got '%v'", result["position"])
		}
	})

	t.Run("create and attach tag", func(t *testing.T) {
		var result map[string]interface{}
		url := fmt.Sprintf("%s/api/projects/%s/tags", baseURL, projectID)
		if err := apiPost(url, map[string]string{"name": "Launch", "color": "#0f766e"}, &result); err != nil {
			t.Fatalf("create tag failed: %v", err)
		}
		tagID, _ = result["id"].(string)
		if tagID == "" {
			t.Fatal("expected non-empty tag id")
		}

		var tags []map[string]interface{}
		url = fmt.Sprintf("%s/api/issues/%s/tags", baseURL, issueID)
		if err := apiPost(url, map[string]string{"tag_id": tagID}, &tags); err != nil {
			t.Fatalf("attach tag failed: %v", err)
		}
		if len(tags) != 1 || tags[0]["id"] != tagID {
			t.Fatalf("expected attached tag, got %#v", tags)
		}
	})

	t.Run("create comment and activity", func(t *testing.T) {
		var result map[string]interface{}
		url := fmt.Sprintf("%s/api/issues/%s/comments", baseURL, issueID)
		if err := apiPost(url, map[string]string{"body": "Persist this comment", "author_name": "You"}, &result); err != nil {
			t.Fatalf("create comment failed: %v", err)
		}
		commentID, _ = result["id"].(string)
		if commentID == "" {
			t.Fatal("expected non-empty comment id")
		}

		var events []map[string]interface{}
		url = fmt.Sprintf("%s/api/issues/%s/activity", baseURL, issueID)
		if err := apiGet(url, &events); err != nil {
			t.Fatalf("list activity failed: %v", err)
		}
		if len(events) < 3 {
			t.Fatalf("expected creation/tag/comment activity, got %#v", events)
		}
	})

	t.Run("verify project via API", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/projects/%s", baseURL, projectID)
		var result map[string]interface{}
		if err := apiGet(url, &result); err != nil {
			t.Fatalf("get project failed: %v", err)
		}
		if result["name"] != "E2E Test Project" {
			t.Errorf("expected name 'E2E Test Project', got '%v'", result["name"])
		}
	})

	t.Run("verify issue list via API", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/projects/%s/issues", baseURL, projectID)
		var results []map[string]interface{}
		if err := apiGet(url, &results); err != nil {
			t.Fatalf("list issues failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 issue, got %d", len(results))
		}
		if results[0]["title"] != "E2E Test Issue" {
			t.Errorf("expected title 'E2E Test Issue', got '%v'", results[0]["title"])
		}
	})

	var runtimeID string
	var codexDegradedState struct {
		status string
		reason string
	}
	var persistedStates map[string]string
	t.Run("discover runtimes and verify health states", func(t *testing.T) {
		var result discoverResponse
		if err := apiPost(fmt.Sprintf("%s/api/runtimes/discover", baseURL), map[string]interface{}{}, &result); err != nil {
			t.Fatalf("discover runtimes failed: %v", err)
		}
		if len(result.Runtimes) != 4 {
			t.Fatalf("expected 4 runtimes, got %d: %#v", len(result.Runtimes), result.Runtimes)
		}
		got := map[string]string{}
		for _, rt := range result.Runtimes {
			got[rt.Kind] = rt.HealthStatus
			if rt.Kind == "codex" {
				runtimeID = rt.ID
			}
		}
		if got["claude"] != "healthy" || got["codex"] != "healthy" || got["gemini"] != "healthy" {
			t.Fatalf("expected mock claude/codex/gemini to be healthy, got: %#v", got)
		}
		if got["opencode"] == "" {
			t.Fatal("expected opencode runtime in discovery results")
		}
		persistedStates = got
	})

	t.Run("force runtime to degraded and verify state persists through restart", func(t *testing.T) {
		if runtimeID == "" {
			t.Skip("no runtime discovered")
		}
		failDir := t.TempDir()
		writeMockRuntimeBinary(t, failDir, "codex", "#!/bin/sh\nexit 9\n")
		failBinary := filepath.Join(failDir, "codex")

		var result struct {
			Runtime runtimeRecord `json:"runtime"`
		}
		payload := map[string]string{"binary_path": failBinary}
		if err := apiPut(fmt.Sprintf("%s/api/runtimes/%s", baseURL, runtimeID), payload, &result); err != nil {
			t.Fatalf("force degraded runtime failed: %v", err)
		}
		if result.Runtime.HealthStatus != "degraded" || result.Runtime.HealthReason != "probe_failed" {
			t.Fatalf("expected degraded/probe_failed from PUT, got %#v", result.Runtime)
		}
		codexDegradedState.status = result.Runtime.HealthStatus
		codexDegradedState.reason = result.Runtime.HealthReason
	})

	stop()

	baseURL, stop = startDaemonWithPath(t, daemonPath, dir, runtimeDir)
	defer stop()

	t.Run("verify persisted state after daemon restart", func(t *testing.T) {
		var project map[string]interface{}
		if err := apiGet(fmt.Sprintf("%s/api/projects/%s", baseURL, projectID), &project); err != nil {
			t.Fatalf("get persisted project failed: %v", err)
		}
		if project["name"] != "E2E Test Project" {
			t.Fatalf("expected persisted project name, got %#v", project)
		}

		var issue map[string]interface{}
		if err := apiGet(fmt.Sprintf("%s/api/issues/%s", baseURL, issueID), &issue); err != nil {
			t.Fatalf("get persisted issue failed: %v", err)
		}
		if issue["title"] != "E2E Test Issue" {
			t.Fatalf("expected persisted issue title, got %#v", issue)
		}

		var tags []map[string]interface{}
		if err := apiGet(fmt.Sprintf("%s/api/issues/%s/tags", baseURL, issueID), &tags); err != nil {
			t.Fatalf("get persisted tags failed: %v", err)
		}
		if len(tags) != 1 || tags[0]["id"] != tagID {
			t.Fatalf("expected persisted tag attachment, got %#v", tags)
		}

		var comments []map[string]interface{}
		if err := apiGet(fmt.Sprintf("%s/api/issues/%s/comments", baseURL, issueID), &comments); err != nil {
			t.Fatalf("get persisted comments failed: %v", err)
		}
		if len(comments) != 1 || comments[0]["id"] != commentID {
			t.Fatalf("expected persisted comment, got %#v", comments)
		}

		var events []map[string]interface{}
		if err := apiGet(fmt.Sprintf("%s/api/issues/%s/activity", baseURL, issueID), &events); err != nil {
			t.Fatalf("get persisted activity failed: %v", err)
		}
		if len(events) < 3 {
			t.Fatalf("expected persisted activity, got %#v", events)
		}
	})

	t.Run("verify runtime state persists after daemon restart", func(t *testing.T) {
		var list runtimeListResponse
		if err := apiGet(fmt.Sprintf("%s/api/runtimes", baseURL), &list); err != nil {
			t.Fatalf("list runtimes after restart failed: %v", err)
		}
		if len(list.Runtimes) != 4 {
			t.Fatalf("expected 4 persisted runtimes, got %d: %#v", len(list.Runtimes), list.Runtimes)
		}
		got := map[string]string{}
		var codexRT runtimeRecord
		for _, rt := range list.Runtimes {
			got[rt.Kind] = rt.HealthStatus
			if rt.Kind == "codex" {
				codexRT = rt
			}
		}
		if got["claude"] != "healthy" || got["gemini"] != "healthy" {
			t.Fatalf("expected mock claude/gemini to persist as healthy, got: %#v", got)
		}
		if got["opencode"] != persistedStates["opencode"] {
			t.Fatalf("opencode state changed from %q to %q after restart", persistedStates["opencode"], got["opencode"])
		}
		if codexRT.HealthStatus != codexDegradedState.status || codexRT.HealthReason != codexDegradedState.reason {
			t.Fatalf("degraded state did not persist across restart: before=%s/%s after=%s/%s",
				codexDegradedState.status, codexDegradedState.reason,
				codexRT.HealthStatus, codexRT.HealthReason)
		}
	})
}

func startDaemon(t *testing.T, daemonPath, dir string) (string, func()) {
	t.Helper()
	return startDaemonWithPath(t, daemonPath, dir, "")
}

func startDaemonWithPath(t *testing.T, daemonPath, dir, runtimeDir string) (string, func()) {
	t.Helper()

	cmd := exec.Command(daemonPath, "--port", "0", "--data-dir", dir)
	env := append(os.Environ(), "ZIN_DATA_DIR="+dir)
	if runtimeDir != "" {
		env = append(env, "PATH="+runtimeDir+":"+os.Getenv("PATH"))
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start daemon: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	var port string
	deadline := time.After(5 * time.Second)
	for port == "" {
		select {
		case <-deadline:
			cmd.Process.Signal(os.Interrupt)
			cmd.Wait()
			t.Fatalf("daemon did not emit port within 5s\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
		default:
		}
		output := stdout.String()
		if idx := strings.Index(output, "\n"); idx != -1 {
			port = strings.TrimSpace(output[:idx])
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		shutdownURL := fmt.Sprintf("http://127.0.0.1:%s/shutdown", port)
		resp, err := http.Post(shutdownURL, "application/json", bytes.NewReader([]byte(`{}`)))
		if err == nil {
			resp.Body.Close()
		}
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()
		select {
		case <-time.After(3 * time.Second):
			cmd.Process.Signal(os.Interrupt)
			cmd.Wait()
		case err := <-done:
			if err != nil {
				t.Fatalf("daemon exited with error: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
			}
		}
	}

	return fmt.Sprintf("http://127.0.0.1:%s", port), stop
}

func apiGet(url string, result interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("unmarshal: %w (body: %s)", err, string(body))
	}
	return nil
}

func apiPost(url string, payload interface{}, result interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("unmarshal: %w (body: %s)", err, string(body))
	}
	return nil
}

func apiPut(url string, payload interface{}, result interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create PUT request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("PUT %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("unmarshal: %w (body: %s)", err, string(body))
	}
	return nil
}

func ensureDaemonBinary(t *testing.T) string {
	t.Helper()

	if path := findDaemonBinary(t); path != "" {
		return path
	}

	root := repoRoot(t)
	path := filepath.Join(t.TempDir(), "daemon")
	cmd := exec.Command("go", "build", "-o", path, "./services/daemon/cmd/daemon")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build daemon binary: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	return path
}

func findDaemonBinary(t *testing.T) string {
	t.Helper()

	candidates := []string{
		"./bin/daemon",
		"./daemon",
		"./services/daemon/cmd/daemon/daemon",
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for _, c := range candidates {
		path := filepath.Join(wd, c)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return ""
		}
		wd = parent
	}

	for _, c := range candidates {
		path := filepath.Join(wd, c)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func repoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("repo root with go.mod not found")
		}
		wd = parent
	}
}

func writeMockRuntimeBinary(t *testing.T, runtimeDir, name, content string) {
	t.Helper()
	path := filepath.Join(runtimeDir, name)
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write mock runtime binary %s: %v", name, err)
	}
}
