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

func TestE2E(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("ZIN_DATA_DIR", dir)
	defer os.Unsetenv("ZIN_DATA_DIR")

	daemonPath := findDaemonBinary(t)
	if daemonPath == "" {
		t.Skip("daemon binary not found, run: go build -o ./bin/daemon ./services/daemon/cmd/daemon/")
	}

	cmd := exec.Command(daemonPath, "--port", "0", "--data-dir", dir)
	cmd.Env = append(os.Environ(), "ZIN_DATA_DIR="+dir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start daemon: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	defer func() {
		cmd.Process.Signal(os.Interrupt)
		cmd.Wait()
	}()

	var port string
	deadline := time.After(5 * time.Second)
	for port == "" {
		select {
		case <-deadline:
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

	if port == "" {
		t.Fatalf("no port output from daemon\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%s", port)

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
		issueID, _ := result["id"].(string)
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
