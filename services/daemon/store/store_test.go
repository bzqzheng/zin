package store_test

import (
	"testing"

	"github.com/bzqzheng/zin/services/daemon/db"
	"github.com/bzqzheng/zin/services/daemon/migration"
	"github.com/bzqzheng/zin/services/daemon/store"
)

func setupStore(t *testing.T) (*store.ProjectRepository, *store.IssueRepository, *store.AgentRepository, func()) {
	t.Helper()
	dir := t.TempDir()

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := migration.Run(database); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	pr := store.NewProjectRepository(database)
	ir := store.NewIssueRepository(database)
	ar := store.NewAgentRepository(database)

	cleanup := func() {
		database.Close()
	}

	return pr, ir, ar, cleanup
}

func TestProjectCRUD(t *testing.T) {
	pr, _, _, cleanup := setupStore(t)
	defer cleanup()

	project, err := pr.Create("Test Project", "A test project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if project.ID == "" {
		t.Error("expected non-empty project ID")
	}

	got, err := pr.GetByID(project.ID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if got.Name != "Test Project" {
		t.Errorf("expected name 'Test Project', got '%s'", got.Name)
	}

	project.Name = "Updated Project"
	if err := pr.Update(project); err != nil {
		t.Fatalf("update project: %v", err)
	}

	projects, err := pr.List()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}

	missing, err := pr.GetByID("nonexistent")
	if err != nil {
		t.Fatalf("get missing project: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for missing project")
	}

	if err := pr.Delete(project.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}

	projects, _ = pr.List()
	if len(projects) != 0 {
		t.Errorf("expected 0 projects after delete, got %d", len(projects))
	}
}

func TestIssueCRUD(t *testing.T) {
	pr, ir, _, cleanup := setupStore(t)
	defer cleanup()

	project, err := pr.Create("Test Project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	issue, err := ir.Create(project.ID, "ISSUE-1", "First Issue", "Description", "todo", "high")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	if issue.ID == "" {
		t.Error("expected non-empty issue ID")
	}

	got, err := ir.GetByID(issue.ID)
	if err != nil {
		t.Fatalf("get issue: %v", err)
	}
	if got.Title != "First Issue" {
		t.Errorf("expected title 'First Issue', got '%s'", got.Title)
	}

	issues, err := ir.ListByProject(project.ID)
	if err != nil {
		t.Fatalf("list issues: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}

	if err := ir.UpdateStatus(issue.ID, "in_progress"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, _ = ir.GetByID(issue.ID)
	if got.Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got '%s'", got.Status)
	}

	issue.Title = "Updated Issue"
	if err := ir.Update(issue); err != nil {
		t.Fatalf("update issue: %v", err)
	}

	missing, err := ir.GetByID("nonexistent")
	if err != nil {
		t.Fatalf("get missing issue: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for missing issue")
	}

	if err := ir.Delete(issue.ID); err != nil {
		t.Fatalf("delete issue: %v", err)
	}

	issues, _ = ir.ListByProject(project.ID)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues after delete, got %d", len(issues))
	}
}

func TestAgentCRUD(t *testing.T) {
	_, _, ar, cleanup := setupStore(t)
	defer cleanup()

	agent, err := ar.Create("Trinity", "craftsperson")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if agent.ID == "" {
		t.Error("expected non-empty agent ID")
	}
	if agent.Status != "offline" {
		t.Errorf("expected status 'offline', got '%s'", agent.Status)
	}

	got, err := ar.GetByID(agent.ID)
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if got.Name != "Trinity" {
		t.Errorf("expected name 'Trinity', got '%s'", got.Name)
	}

	agent.Name = "Oracle"
	agent.Role = "advisor"
	agent.Status = "online"
	if err := ar.Update(agent); err != nil {
		t.Fatalf("update agent: %v", err)
	}

	agents, err := ar.List()
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}

	missing, err := ar.GetByID("nonexistent")
	if err != nil {
		t.Fatalf("get missing agent: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for missing agent")
	}

	if err := ar.Delete(agent.ID); err != nil {
		t.Fatalf("delete agent: %v", err)
	}

	agents, _ = ar.List()
	if len(agents) != 0 {
		t.Errorf("expected 0 agents after delete, got %d", len(agents))
	}
}
