package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bzqzheng/zin/services/daemon/config"
	"github.com/bzqzheng/zin/services/daemon/db"
	"github.com/bzqzheng/zin/services/daemon/httpapi"
	"github.com/bzqzheng/zin/services/daemon/httpapi/handler"
	"github.com/bzqzheng/zin/services/daemon/migration"
	"github.com/bzqzheng/zin/services/daemon/store"
)

func setupRouter(t *testing.T) (http.Handler, *store.ProjectRepository, *store.IssueRepository, func()) {
	t.Helper()

	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := migration.Run(database); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	projectRepo := store.NewProjectRepository(database)
	issueRepo := store.NewIssueRepository(database)
	tagRepo := store.NewTagRepository(database)
	commentRepo := store.NewIssueCommentRepository(database)
	activityRepo := store.NewIssueActivityRepository(database)
	agentRepo := store.NewAgentRepository(database)
	runtimeRepo := store.NewRuntimeRepository(database)
	router := httpapi.NewRouter(
		handler.NewHealthHandler(database, &config.DaemonConfig{}, 0),
		handler.NewProjectHandler(projectRepo),
		handler.NewIssueHandler(database, issueRepo, projectRepo),
		handler.NewInteractionHandler(database, projectRepo, issueRepo, tagRepo, commentRepo, activityRepo),
		handler.NewAgentHandler(agentRepo, runtimeRepo),
		handler.NewRuntimeHandler(runtimeRepo),
		make(chan struct{}, 1),
	)

	return router, projectRepo, issueRepo, func() {
		database.Close()
	}
}

func TestRouterCORSAllowsDesktopOrigins(t *testing.T) {
	router, _, _, cleanup := setupRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected desktop origin to be allowed, got %q", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary: Origin, got %q", got)
	}
}

func TestRouterCORSPreflight(t *testing.T) {
	router, _, _, cleanup := setupRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodOptions, "/api/projects", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected desktop origin to be allowed, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Fatalf("expected Content-Type to be allowed, got %q", got)
	}
}

func TestRouterCORSRejectsRemoteOrigins(t *testing.T) {
	router, _, _, cleanup := setupRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected remote origin to be rejected, got %q", got)
	}
}

func TestIssueListFiltersAndGeneratedFields(t *testing.T) {
	router, projectRepo, _, cleanup := setupRouter(t)
	defer cleanup()

	project, err := projectRepo.Create("Contract Project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	created := []store.Issue{
		postIssue(t, router, project.ID, map[string]string{
			"identifier": "CLIENT-SUPPLIED",
			"title":      "First",
			"status":     "todo",
			"priority":   "high",
		}),
		postIssue(t, router, project.ID, map[string]string{
			"title":    "Second",
			"status":   "in_progress",
			"priority": "high",
		}),
		postIssue(t, router, project.ID, map[string]string{
			"title":    "Third",
			"status":   "todo",
			"priority": "low",
		}),
	}

	if created[0].Identifier != "ISSUE-1" || created[0].Position != 1 {
		t.Fatalf("expected first issue to be server-generated ISSUE-1/1, got %s/%d", created[0].Identifier, created[0].Position)
	}
	if created[1].Identifier != "ISSUE-2" || created[1].Position != 2 {
		t.Fatalf("expected second issue to be server-generated ISSUE-2/2, got %s/%d", created[1].Identifier, created[1].Position)
	}
	if created[2].Identifier != "ISSUE-3" || created[2].Position != 3 {
		t.Fatalf("expected third issue to be server-generated ISSUE-3/3, got %s/%d", created[2].Identifier, created[2].Position)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+project.ID+"/issues?status=todo&priority=high", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var filtered []store.Issue
	if err := json.NewDecoder(rec.Body).Decode(&filtered); err != nil {
		t.Fatalf("decode filtered issues: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != created[0].ID {
		t.Fatalf("expected only first todo/high issue, got %#v", filtered)
	}
}

func TestProjectDeleteCascadesIssues(t *testing.T) {
	router, projectRepo, issueRepo, cleanup := setupRouter(t)
	defer cleanup()

	project, err := projectRepo.Create("Cascade Project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := issueRepo.Create(project.ID, "Cascaded issue", "", "todo", "medium"); err != nil {
		t.Fatalf("create issue: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/projects/"+project.ID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 cascade delete, got %d: %s", rec.Code, rec.Body.String())
	}

	remaining, err := issueRepo.ListByProject(project.ID, store.IssueFilters{})
	if err != nil {
		t.Fatalf("list issues after delete: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected no issues after project cascade delete, got %d", len(remaining))
	}
}

func TestErrorEnvelopeAlwaysIncludesDetails(t *testing.T) {
	router, _, _, cleanup := setupRouter(t)
	defer cleanup()

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "validation", method: http.MethodPost, path: "/api/projects", body: `{}`},
		{name: "not found", method: http.MethodGet, path: "/api/issues/missing", body: ``},
		{name: "decode", method: http.MethodPost, path: "/api/projects", body: `{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code < 400 {
				t.Fatalf("expected error response, got %d: %s", rec.Code, rec.Body.String())
			}

			var payload map[string]map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			detail, ok := payload["error"]
			if !ok {
				t.Fatalf("missing error key in response: %#v", payload)
			}
			if _, ok := detail["code"]; !ok {
				t.Fatalf("missing error.code: %#v", detail)
			}
			if _, ok := detail["message"]; !ok {
				t.Fatalf("missing error.message: %#v", detail)
			}
			if _, ok := detail["details"]; !ok {
				t.Fatalf("missing error.details: %#v", detail)
			}
		})
	}
}

func TestAgentRuntimeBindingAndAssignableFilter(t *testing.T) {
	router, _, _, cleanup := setupRouter(t)
	defer cleanup()
	t.Setenv("PATH", t.TempDir())

	codex := writeHandlerExecutable(t, "codex", "#!/bin/sh\necho 'codex 1.2.3'\n")
	claude := writeHandlerExecutable(t, "claude", "#!/bin/sh\nexit 9\n")
	discovered := requestJSON[struct {
		Runtimes []store.Runtime `json:"runtimes"`
	}](t, router, http.MethodPost, "/api/runtimes/discover", `{"path_overrides":{"codex":`+mustJSONQuote(t, codex)+`,"claude":`+mustJSONQuote(t, claude)+`}}`, http.StatusOK)

	var healthy, degraded store.Runtime
	for _, runtime := range discovered.Runtimes {
		switch runtime.Kind {
		case "codex":
			healthy = runtime
		case "claude":
			degraded = runtime
		}
	}
	if healthy.ID == "" || degraded.ID == "" {
		t.Fatalf("expected healthy and degraded runtimes, got %#v", discovered.Runtimes)
	}

	assignable := requestJSON[store.Agent](t, router, http.MethodPost, "/api/agents", `{"name":"Trinity","role":"builder","runtime_id":"`+healthy.ID+`","model":"gpt-5","instructions":"Complete the work."}`, http.StatusCreated)
	if !assignable.Assignable || assignable.RuntimeStatus != "healthy" || assignable.RuntimeName == "" || assignable.Model != "gpt-5" {
		t.Fatalf("expected healthy runtime agent to be assignable, got %#v", assignable)
	}

	blocked := requestJSON[store.Agent](t, router, http.MethodPost, "/api/agents", `{"name":"Smith","role":"reviewer","runtime_id":"`+degraded.ID+`"}`, http.StatusCreated)
	if blocked.Assignable || !strings.Contains(blocked.AssignableReason, "degraded") {
		t.Fatalf("expected degraded runtime agent to be gated with explicit reason, got %#v", blocked)
	}

	filtered := requestJSON[[]store.Agent](t, router, http.MethodGet, "/api/agents?assignable=true", ``, http.StatusOK)
	if len(filtered) != 1 || filtered[0].ID != assignable.ID {
		t.Fatalf("expected assignable filter to return only healthy runtime agent, got %#v", filtered)
	}

	updated := requestJSON[store.Agent](t, router, http.MethodPut, "/api/agents/"+blocked.ID, `{"runtime_id":"`+healthy.ID+`","model":"claude-sonnet","instructions":"Review carefully."}`, http.StatusOK)
	if !updated.Assignable || updated.Model != "claude-sonnet" || updated.Instructions != "Review carefully." {
		t.Fatalf("expected update to bind healthy runtime and persist config, got %#v", updated)
	}

	assertStatus(t, router, http.MethodPost, "/api/agents", `{"name":"Missing","runtime_id":"missing"}`, http.StatusBadRequest)
}

func TestRuntimeDiscoveryContracts(t *testing.T) {
	router, _, _, cleanup := setupRouter(t)
	defer cleanup()
	t.Setenv("PATH", t.TempDir())

	codex := writeHandlerExecutable(t, "codex", "#!/bin/sh\necho 'codex 1.2.3'\n")
	body := `{"path_overrides":{"codex":` + mustJSONQuote(t, codex) + `}}`
	req := httptest.NewRequest(http.MethodPost, "/api/runtimes/discover", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Runtimes []store.Runtime `json:"runtimes"`
		Summary  struct {
			Healthy  int `json:"healthy"`
			Degraded int `json:"degraded"`
			Missing  int `json:"missing"`
		} `json:"summary"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode discover response: %v", err)
	}
	if len(payload.Runtimes) != 4 {
		t.Fatalf("expected four supported runtimes, got %#v", payload.Runtimes)
	}
	if payload.Summary.Healthy != 1 || payload.Summary.Missing != 3 {
		t.Fatalf("unexpected discovery summary: %#v", payload.Summary)
	}

	runtimes := requestJSON[struct {
		Runtimes []store.Runtime `json:"runtimes"`
	}](t, router, http.MethodGet, "/api/runtimes", ``, http.StatusOK)
	if len(runtimes.Runtimes) != 4 {
		t.Fatalf("expected persisted runtime list, got %#v", runtimes)
	}
}

func TestRuntimeDiscoveryRejectsInvalidOverrideWithEnvelope(t *testing.T) {
	router, _, _, cleanup := setupRouter(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/runtimes/discover", bytes.NewBufferString(`{"path_overrides":{"unknown":"/bin/echo"}}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload["error"]["code"] != "INVALID_RUNTIME_KIND" || payload["error"]["details"] == "" {
		t.Fatalf("unexpected error envelope: %#v", payload)
	}
}

func TestRuntimeValidatePersistsStatus(t *testing.T) {
	router, _, _, cleanup := setupRouter(t)
	defer cleanup()
	t.Setenv("PATH", t.TempDir())

	codex := writeHandlerExecutable(t, "codex", "#!/bin/sh\necho 'codex 1.2.3'\n")
	discovered := requestJSON[struct {
		Runtimes []store.Runtime `json:"runtimes"`
	}](t, router, http.MethodPost, "/api/runtimes/discover", `{"path_overrides":{"codex":`+mustJSONQuote(t, codex)+`}}`, http.StatusOK)

	var codexRuntime store.Runtime
	for _, runtime := range discovered.Runtimes {
		if runtime.Kind == "codex" {
			codexRuntime = runtime
		}
	}
	if codexRuntime.ID == "" {
		t.Fatalf("expected codex runtime in discovery response: %#v", discovered)
	}

	degraded := writeHandlerExecutable(t, "codex", "#!/bin/sh\nexit 9\n")
	updated := requestJSON[struct {
		Runtime store.Runtime `json:"runtime"`
	}](t, router, http.MethodPut, "/api/runtimes/"+codexRuntime.ID, `{"binary_path":`+mustJSONQuote(t, degraded)+`}`, http.StatusOK)
	if updated.Runtime.HealthStatus != "degraded" || updated.Runtime.HealthReason != "probe_failed" {
		t.Fatalf("expected degraded update, got %#v", updated.Runtime)
	}

	validated := requestJSON[struct {
		Runtime store.Runtime `json:"runtime"`
	}](t, router, http.MethodPost, "/api/runtimes/validate", `{"runtime_id":"`+codexRuntime.ID+`"}`, http.StatusOK)
	if validated.Runtime.HealthStatus != "degraded" || validated.Runtime.HealthReason != "probe_failed" {
		t.Fatalf("expected degraded validation to persist, got %#v", validated.Runtime)
	}
}

func TestRuntimeUpdatePersistsKindMismatch(t *testing.T) {
	router, _, _, cleanup := setupRouter(t)
	defer cleanup()
	t.Setenv("PATH", t.TempDir())

	codex := writeHandlerExecutable(t, "codex", "#!/bin/sh\necho 'codex 1.2.3'\n")
	discovered := requestJSON[struct {
		Runtimes []store.Runtime `json:"runtimes"`
	}](t, router, http.MethodPost, "/api/runtimes/discover", `{"path_overrides":{"codex":`+mustJSONQuote(t, codex)+`}}`, http.StatusOK)

	var codexRuntime store.Runtime
	for _, runtime := range discovered.Runtimes {
		if runtime.Kind == "codex" {
			codexRuntime = runtime
		}
	}
	if codexRuntime.ID == "" {
		t.Fatalf("expected codex runtime in discovery response: %#v", discovered)
	}

	claude := writeHandlerExecutable(t, "claude", "#!/bin/sh\necho 'claude 2.0.0'\n")
	assertStatus(t, router, http.MethodPut, "/api/runtimes/"+codexRuntime.ID, `{"binary_path":`+mustJSONQuote(t, claude)+`}`, http.StatusConflict)

	listed := requestJSON[struct {
		Runtimes []store.Runtime `json:"runtimes"`
	}](t, router, http.MethodGet, "/api/runtimes", ``, http.StatusOK)
	for _, runtime := range listed.Runtimes {
		if runtime.ID == codexRuntime.ID {
			if runtime.BinaryPath != claude || runtime.HealthStatus != "degraded" || runtime.HealthReason != "runtime_kind_mismatch" {
				t.Fatalf("expected mismatch update to persist degraded state, got %#v", runtime)
			}
			return
		}
	}
	t.Fatalf("expected updated runtime in list: %#v", listed.Runtimes)
}

func TestInteractionEndpointContracts(t *testing.T) {
	router, projectRepo, _, cleanup := setupRouter(t)
	defer cleanup()

	project, err := projectRepo.Create("Interactions", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	issue := postIssue(t, router, project.ID, map[string]string{"title": "Work item"})

	assertStatus(t, router, http.MethodPost, "/api/projects/"+project.ID+"/tags", `{"name":"   "}`, http.StatusBadRequest)

	tag := requestJSON[store.Tag](t, router, http.MethodPost, "/api/projects/"+project.ID+"/tags", `{"name":"Launch","color":"#0f766e"}`, http.StatusCreated)
	if tag.Name != "Launch" || tag.Color != "#0f766e" {
		t.Fatalf("unexpected created tag: %#v", tag)
	}
	assertStatus(t, router, http.MethodPost, "/api/projects/"+project.ID+"/tags", `{"name":" launch "}`, http.StatusConflict)

	tags := requestJSON[[]store.Tag](t, router, http.MethodGet, "/api/projects/"+project.ID+"/tags", ``, http.StatusOK)
	if len(tags) != 1 || tags[0].ID != tag.ID {
		t.Fatalf("expected project tag list to contain created tag, got %#v", tags)
	}

	updated := requestJSON[store.Tag](t, router, http.MethodPut, "/api/tags/"+tag.ID, `{"name":"Release","color":"#155e75"}`, http.StatusOK)
	if updated.Name != "Release" || updated.Color != "#155e75" {
		t.Fatalf("unexpected updated tag: %#v", updated)
	}

	attached := requestJSON[[]store.Tag](t, router, http.MethodPost, "/api/issues/"+issue.ID+"/tags", `{"tag_id":"`+tag.ID+`"}`, http.StatusOK)
	if len(attached) != 1 || attached[0].ID != tag.ID {
		t.Fatalf("expected attached tag, got %#v", attached)
	}
	duplicateAttach := requestJSON[[]store.Tag](t, router, http.MethodPost, "/api/issues/"+issue.ID+"/tags", `{"tag_id":"`+tag.ID+`"}`, http.StatusOK)
	if len(duplicateAttach) != 1 {
		t.Fatalf("duplicate attach should be idempotent, got %#v", duplicateAttach)
	}
	createdAttach := requestJSON[[]store.Tag](t, router, http.MethodPost, "/api/issues/"+issue.ID+"/tags", `{"name":"Design"}`, http.StatusOK)
	if len(createdAttach) != 2 {
		t.Fatalf("expected create-and-attach to return two tags, got %#v", createdAttach)
	}
	var designTagID string
	for _, tag := range createdAttach {
		if tag.Name == "Design" {
			designTagID = tag.ID
		}
	}
	if designTagID == "" {
		t.Fatalf("expected create-and-attach response to include Design tag, got %#v", createdAttach)
	}
	assertStatus(t, router, http.MethodDelete, "/api/tags/"+designTagID, ``, http.StatusNoContent)
	assertStatus(t, router, http.MethodDelete, "/api/tags/"+designTagID, ``, http.StatusNotFound)

	assertStatus(t, router, http.MethodDelete, "/api/issues/"+issue.ID+"/tags/"+tag.ID, ``, http.StatusNoContent)
	assertStatus(t, router, http.MethodDelete, "/api/issues/"+issue.ID+"/tags/"+tag.ID, ``, http.StatusNotFound)

	assertStatus(t, router, http.MethodPost, "/api/issues/"+issue.ID+"/comments", `{"body":"   "}`, http.StatusBadRequest)
	comment := requestJSON[map[string]interface{}](t, router, http.MethodPost, "/api/issues/"+issue.ID+"/comments", `{"body":"First comment","author_name":"Bright"}`, http.StatusCreated)
	commentID, _ := comment["id"].(string)
	if commentID == "" || comment["body"] != "First comment" || comment["author_name"] != "You" {
		t.Fatalf("unexpected comment: %#v", comment)
	}
	comments := requestJSON[[]map[string]interface{}](t, router, http.MethodGet, "/api/issues/"+issue.ID+"/comments", ``, http.StatusOK)
	if len(comments) != 1 || comments[0]["id"] != commentID {
		t.Fatalf("expected one listed comment, got %#v", comments)
	}
	edited := requestJSON[map[string]interface{}](t, router, http.MethodPut, "/api/comments/"+commentID, `{"body":"Edited comment"}`, http.StatusOK)
	if edited["body"] != "Edited comment" {
		t.Fatalf("expected edited comment body, got %#v", edited)
	}
	assertStatus(t, router, http.MethodDelete, "/api/comments/"+commentID, ``, http.StatusNoContent)
	assertStatus(t, router, http.MethodDelete, "/api/comments/"+commentID, ``, http.StatusNotFound)

	events := requestJSON[[]map[string]interface{}](t, router, http.MethodGet, "/api/issues/"+issue.ID+"/activity", ``, http.StatusOK)
	gotTypes := map[string]bool{}
	for _, event := range events {
		typ, _ := event["type"].(string)
		gotTypes[typ] = true
		if _, ok := event["metadata"].(map[string]interface{}); !ok {
			t.Fatalf("expected metadata object for event %#v", event)
		}
	}
	for _, typ := range []string{"issue.created", "tag.added", "tag.removed", "comment.added", "comment.updated", "comment.deleted"} {
		if !gotTypes[typ] {
			t.Fatalf("missing activity type %s in %#v", typ, gotTypes)
		}
	}
}

func TestInteractionNotFoundAndValidationPaths(t *testing.T) {
	router, projectRepo, _, cleanup := setupRouter(t)
	defer cleanup()

	project, err := projectRepo.Create("Validation", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	issue := postIssue(t, router, project.ID, map[string]string{"title": "Work item"})

	assertStatus(t, router, http.MethodGet, "/api/projects/missing/tags", ``, http.StatusNotFound)
	assertStatus(t, router, http.MethodGet, "/api/issues/missing/tags", ``, http.StatusNotFound)
	assertStatus(t, router, http.MethodPost, "/api/issues/"+issue.ID+"/tags", `{"tag_id":"missing"}`, http.StatusNotFound)
	assertStatus(t, router, http.MethodPost, "/api/issues/"+issue.ID+"/tags", `{`, http.StatusBadRequest)
	assertStatus(t, router, http.MethodPut, "/api/tags/missing", `{"name":"Missing"}`, http.StatusNotFound)
	assertStatus(t, router, http.MethodDelete, "/api/tags/missing", ``, http.StatusNotFound)
	assertStatus(t, router, http.MethodGet, "/api/issues/missing/comments", ``, http.StatusNotFound)
	assertStatus(t, router, http.MethodPost, "/api/issues/missing/comments", `{"body":"x"}`, http.StatusNotFound)
	assertStatus(t, router, http.MethodPut, "/api/comments/missing", `{"body":"x"}`, http.StatusNotFound)
	assertStatus(t, router, http.MethodGet, "/api/issues/missing/activity", ``, http.StatusNotFound)
	assertStatus(t, router, http.MethodPost, "/api/projects/"+project.ID+"/issues", `{"title":"Bad","status":"later"}`, http.StatusBadRequest)
	assertStatus(t, router, http.MethodPost, "/api/projects/"+project.ID+"/issues", `{"title":"Bad","priority":"urgent"}`, http.StatusBadRequest)
	assertStatus(t, router, http.MethodPut, "/api/issues/"+issue.ID, `{"priority":"urgent"}`, http.StatusBadRequest)
	assertStatus(t, router, http.MethodPut, "/api/issues/"+issue.ID+"/status", `{"status":"later"}`, http.StatusBadRequest)
}

func postIssue(t *testing.T, router http.Handler, projectID string, body map[string]string) store.Issue {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal issue body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/issues", bytes.NewReader(data))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var issue store.Issue
	if err := json.NewDecoder(rec.Body).Decode(&issue); err != nil {
		t.Fatalf("decode issue response: %v", err)
	}
	return issue
}

func requestJSON[T any](t *testing.T, router http.Handler, method, path, body string, want int) T {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s: expected %d, got %d: %s", method, path, want, rec.Code, rec.Body.String())
	}
	var result T
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode %s %s: %v; body=%s", method, path, err, rec.Body.String())
	}
	return result
}

func assertStatus(t *testing.T, router http.Handler, method, path, body string, want int) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s: expected %d, got %d: %s", method, path, want, rec.Code, rec.Body.String())
	}
}

func writeHandlerExecutable(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	return path
}

func mustJSONQuote(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("quote value: %v", err)
	}
	return string(encoded)
}
