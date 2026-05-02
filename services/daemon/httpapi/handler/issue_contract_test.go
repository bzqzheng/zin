package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	agentRepo := store.NewAgentRepository(database)
	router := httpapi.NewRouter(
		handler.NewHealthHandler(database, &config.DaemonConfig{}, 0),
		handler.NewProjectHandler(projectRepo),
		handler.NewIssueHandler(issueRepo, projectRepo),
		handler.NewAgentHandler(agentRepo),
		make(chan struct{}, 1),
	)

	return router, projectRepo, issueRepo, func() {
		database.Close()
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
