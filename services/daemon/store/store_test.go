package store_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bzqzheng/zin/services/daemon/db"
	"github.com/bzqzheng/zin/services/daemon/migration"
	"github.com/bzqzheng/zin/services/daemon/store"
)

func setupStore(t *testing.T) (*store.ProjectRepository, *store.IssueRepository, *store.AgentRepository, *store.RuntimeRepository, func()) {
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
	rr := store.NewRuntimeRepository(database)

	cleanup := func() {
		database.Close()
	}

	return pr, ir, ar, rr, cleanup
}

func setupAssignmentExecutionStore(t *testing.T) (*sql.DB, *store.ProjectRepository, *store.IssueRepository, *store.AgentRepository, *store.IssueAssignmentRepository, func()) {
	t.Helper()
	dir := t.TempDir()

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := migration.Run(database); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	cleanup := func() {
		database.Close()
	}
	return database,
		store.NewProjectRepository(database),
		store.NewIssueRepository(database),
		store.NewAgentRepository(database),
		store.NewIssueAssignmentRepository(database),
		cleanup
}

func createExecutionAssignment(t *testing.T, projectRepo *store.ProjectRepository, issueRepo *store.IssueRepository, agentRepo *store.AgentRepository, assignmentRepo *store.IssueAssignmentRepository) *store.IssueAssignment {
	t.Helper()
	project, err := projectRepo.Create("Execution", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	issue, err := issueRepo.Create(project.ID, "Run assignment", "", "todo", "high")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	agent, err := agentRepo.Create("Trinity", "builder", "", "", "")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	assignment, err := assignmentRepo.Create(store.CreateIssueAssignmentInput{
		IssueID:            issue.ID,
		AgentID:            agent.ID,
		RequestedBy:        "local-user",
		SourceType:         "issue_detail",
		ClientRequestID:    "req-" + issue.ID,
		RequestFingerprint: "fingerprint-" + issue.ID,
		DedupeKey:          "dedupe-" + issue.ID,
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	return assignment
}

func TestAssignmentExecutionTransitionsAreCASGuarded(t *testing.T) {
	_, projectRepo, issueRepo, agentRepo, assignmentRepo, cleanup := setupAssignmentExecutionStore(t)
	defer cleanup()

	assignment := createExecutionAssignment(t, projectRepo, issueRepo, agentRepo, assignmentRepo)

	retrieving, err := assignmentRepo.TransitionCAS(assignment.ID, "queued", "retrieving_memory")
	if err != nil {
		t.Fatalf("transition queued to retrieving_memory: %v", err)
	}
	if retrieving.Status != "retrieving_memory" {
		t.Fatalf("expected retrieving_memory, got %#v", retrieving)
	}

	if _, err := assignmentRepo.TransitionCAS(assignment.ID, "queued", "retrieving_memory"); !errors.Is(err, store.ErrAssignmentStateConflict) {
		t.Fatalf("expected stale worker CAS conflict, got %v", err)
	}
	if _, err := assignmentRepo.TransitionCAS(assignment.ID, "retrieving_memory", "running"); !errors.Is(err, store.ErrAssignmentInvalidTransition) {
		t.Fatalf("expected invalid transition error, got %v", err)
	}

	ready, err := assignmentRepo.TransitionCAS(assignment.ID, "retrieving_memory", "ready")
	if err != nil {
		t.Fatalf("transition retrieving_memory to ready: %v", err)
	}
	if ready.Status != "ready" {
		t.Fatalf("expected ready, got %#v", ready)
	}
	running, err := assignmentRepo.TransitionCAS(assignment.ID, "ready", "running")
	if err != nil {
		t.Fatalf("transition ready to running: %v", err)
	}
	if running.Status != "running" {
		t.Fatalf("expected running, got %#v", running)
	}
}

func TestAssignmentCompletionPersistsResultAtomically(t *testing.T) {
	_, projectRepo, issueRepo, agentRepo, assignmentRepo, cleanup := setupAssignmentExecutionStore(t)
	defer cleanup()

	assignment := createExecutionAssignment(t, projectRepo, issueRepo, agentRepo, assignmentRepo)
	if _, err := assignmentRepo.TransitionCAS(assignment.ID, "queued", "retrieving_memory"); err != nil {
		t.Fatalf("transition to retrieving_memory: %v", err)
	}
	if _, err := assignmentRepo.TransitionCAS(assignment.ID, "retrieving_memory", "ready"); err != nil {
		t.Fatalf("transition to ready: %v", err)
	}
	if _, err := assignmentRepo.TransitionCAS(assignment.ID, "ready", "running"); err != nil {
		t.Fatalf("transition to running: %v", err)
	}

	startedAt := time.Date(2026, 5, 5, 20, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Second)
	completed, result, err := assignmentRepo.CompleteWithResult(store.CompleteAssignmentInput{
		AssignmentID: assignment.ID,
		Output:       "done",
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
	})
	if err != nil {
		t.Fatalf("complete with result: %v", err)
	}
	if completed.Status != "succeeded" || completed.CompletedAt == nil {
		t.Fatalf("expected succeeded assignment with completed_at, got %#v", completed)
	}
	if result.AssignmentID != assignment.ID || result.AttemptNo != 1 || result.Output != "done" || result.Status != "succeeded" {
		t.Fatalf("unexpected result row: %#v", result)
	}

	if _, _, err := assignmentRepo.CompleteWithResult(store.CompleteAssignmentInput{AssignmentID: assignment.ID, Output: "duplicate"}); !errors.Is(err, store.ErrAssignmentStateConflict) {
		t.Fatalf("expected duplicate completion conflict, got %v", err)
	}
	results, err := assignmentRepo.ListResults(assignment.ID)
	if err != nil {
		t.Fatalf("list results: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one immutable result row, got %d", len(results))
	}
}

func TestIssueDeleteCascadesAssignmentResults(t *testing.T) {
	database, projectRepo, issueRepo, agentRepo, assignmentRepo, cleanup := setupAssignmentExecutionStore(t)
	defer cleanup()

	assignment := createExecutionAssignment(t, projectRepo, issueRepo, agentRepo, assignmentRepo)
	if _, err := assignmentRepo.TransitionCAS(assignment.ID, "queued", "retrieving_memory"); err != nil {
		t.Fatalf("transition to retrieving_memory: %v", err)
	}
	if _, err := assignmentRepo.TransitionCAS(assignment.ID, "retrieving_memory", "ready"); err != nil {
		t.Fatalf("transition to ready: %v", err)
	}
	if _, err := assignmentRepo.TransitionCAS(assignment.ID, "ready", "running"); err != nil {
		t.Fatalf("transition to running: %v", err)
	}
	if _, _, err := assignmentRepo.CompleteWithResult(store.CompleteAssignmentInput{AssignmentID: assignment.ID, Output: "done"}); err != nil {
		t.Fatalf("complete with result: %v", err)
	}
	if err := issueRepo.Delete(assignment.IssueID); err != nil {
		t.Fatalf("delete issue: %v", err)
	}

	var assignmentCount, resultCount int
	if err := database.QueryRow("SELECT COUNT(*) FROM issue_assignments WHERE issue_id = ?", assignment.IssueID).Scan(&assignmentCount); err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	if err := database.QueryRow("SELECT COUNT(*) FROM assignment_results WHERE assignment_id = ?", assignment.ID).Scan(&resultCount); err != nil {
		t.Fatalf("count assignment results: %v", err)
	}
	if assignmentCount != 0 || resultCount != 0 {
		t.Fatalf("expected issue delete to cascade assignment/results, got assignments=%d results=%d", assignmentCount, resultCount)
	}
}

func TestRetrievalFailureFailFastDefault(t *testing.T) {
	_, projectRepo, issueRepo, agentRepo, assignmentRepo, cleanup := setupAssignmentExecutionStore(t)
	defer cleanup()

	assignment := createExecutionAssignment(t, projectRepo, issueRepo, agentRepo, assignmentRepo)
	if _, err := assignmentRepo.StartRetrieval(assignment.ID); err != nil {
		t.Fatalf("start retrieval: %v", err)
	}
	failed, err := assignmentRepo.CompleteRetrievalFailure(store.RetrievalOutcomeInput{
		AssignmentID:  assignment.ID,
		FailureReason: "retriever_unavailable",
	})
	if err != nil {
		t.Fatalf("complete retrieval failure: %v", err)
	}
	if failed.Status != "retrieval_failed" || failed.RetrievalFailureReason != "retriever_unavailable" || failed.RetrievalPolicy != "fail_fast" {
		t.Fatalf("expected fail-fast retrieval_failed state, got %#v", failed)
	}
	events, err := assignmentRepo.ListMemoryInfluenceEvents(assignment.ID)
	if err != nil {
		t.Fatalf("list influence events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("fail-fast retrieval failure should emit no memory event, got %#v", events)
	}
}

func TestRetrievalCreationFailureLeavesQueuedWithoutMemoryEvent(t *testing.T) {
	_, projectRepo, issueRepo, agentRepo, assignmentRepo, cleanup := setupAssignmentExecutionStore(t)
	defer cleanup()

	assignment := createExecutionAssignment(t, projectRepo, issueRepo, agentRepo, assignmentRepo)
	queued, err := assignmentRepo.RecordRetrievalCreationFailure(assignment.ID, "retriever_boot_failed")
	if err != nil {
		t.Fatalf("record creation retrieval failure: %v", err)
	}
	if queued.Status != "queued" || queued.ErrorCode != "retrieval_failed" || queued.RetrievalStatus != "failed" {
		t.Fatalf("expected queued retriable retrieval failure, got %#v", queued)
	}
	events, err := assignmentRepo.ListMemoryInfluenceEvents(assignment.ID)
	if err != nil {
		t.Fatalf("list influence events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("creation retrieval failure should emit no memory event, got %#v", events)
	}
}

func TestContinueWithoutMemoryRequiresRetrievalFailedAudit(t *testing.T) {
	_, projectRepo, issueRepo, agentRepo, assignmentRepo, cleanup := setupAssignmentExecutionStore(t)
	defer cleanup()

	assignment := createExecutionAssignment(t, projectRepo, issueRepo, agentRepo, assignmentRepo)
	if _, err := assignmentRepo.StartRetrieval(assignment.ID); err != nil {
		t.Fatalf("start retrieval: %v", err)
	}
	if _, err := assignmentRepo.CompleteRetrievalFailure(store.RetrievalOutcomeInput{
		AssignmentID:      assignment.ID,
		ContinueOnFailure: true,
	}); !errors.Is(err, store.ErrRetrievalAuditRequired) {
		t.Fatalf("expected audit-required error, got %v", err)
	}
	ready, err := assignmentRepo.CompleteRetrievalFailure(store.RetrievalOutcomeInput{
		AssignmentID:      assignment.ID,
		FailureReason:     "retriever_timeout",
		ContinueOnFailure: true,
		AuditMetadataJSON: `{"source":"AssignmentDispatched"}`,
	})
	if err != nil {
		t.Fatalf("continue without memory with audit: %v", err)
	}
	if ready.Status != "ready" || ready.RetrievalStatus != "failed" || ready.RetrievalFailureReason != "retriever_timeout" {
		t.Fatalf("expected ready assignment with retrieval_failed audit metadata, got %#v", ready)
	}
	events, err := assignmentRepo.ListMemoryInfluenceEvents(assignment.ID)
	if err != nil {
		t.Fatalf("list influence events: %v", err)
	}
	if len(events) != 1 || events[0].InfluenceType != "retrieval_failed" || events[0].ReasonCode != "retriever_timeout" {
		t.Fatalf("expected retrieval_failed influence event, got %#v", events)
	}
}

func TestInfluenceLoggingFailureDoesNotBlockResultPersistence(t *testing.T) {
	_, projectRepo, issueRepo, agentRepo, assignmentRepo, cleanup := setupAssignmentExecutionStore(t)
	defer cleanup()

	assignment := createExecutionAssignment(t, projectRepo, issueRepo, agentRepo, assignmentRepo)
	if _, err := assignmentRepo.StartRetrieval(assignment.ID); err != nil {
		t.Fatalf("start retrieval: %v", err)
	}
	if _, err := assignmentRepo.CompleteRetrievalEmpty(assignment.ID); err != nil {
		t.Fatalf("complete empty retrieval: %v", err)
	}
	if _, err := assignmentRepo.TransitionCAS(assignment.ID, "ready", "running"); err != nil {
		t.Fatalf("transition to running: %v", err)
	}

	completed, result, err := assignmentRepo.CompleteWithResultAndInfluence(
		store.CompleteAssignmentInput{AssignmentID: assignment.ID, Output: "finished"},
		[]store.MemoryInfluenceEventInput{{InfluenceType: "memory_applied", MemoryID: "memory-1"}},
		failingInfluenceLogger{},
	)
	if err != nil {
		t.Fatalf("complete with failing influence logger: %v", err)
	}
	if completed.Status != "succeeded" {
		t.Fatalf("expected succeeded assignment despite influence log failure, got %#v", completed)
	}
	if !result.ObservabilityDegraded || result.ObservabilityDegradedReason != "influence_logging_failed" || result.ObservabilityDegradedDetail != "influence log failed" {
		t.Fatalf("expected degraded observability flag, got %#v", result)
	}
	results, err := assignmentRepo.ListResults(assignment.ID)
	if err != nil {
		t.Fatalf("list results: %v", err)
	}
	if len(results) != 1 || results[0].Output != "finished" {
		t.Fatalf("expected persisted assignment result, got %#v", results)
	}
}

type failingInfluenceLogger struct{}

func (failingInfluenceLogger) LogMemoryInfluence(store.MemoryInfluenceEventInput) (*store.MemoryInfluenceEvent, error) {
	return nil, errors.New("influence log failed")
}

func setupInteractionStore(t *testing.T) (*sql.DB, *store.ProjectRepository, *store.IssueRepository, *store.TagRepository, *store.IssueCommentRepository, *store.IssueActivityRepository, func()) {
	t.Helper()
	dir := t.TempDir()

	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := migration.Run(database); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	cleanup := func() {
		database.Close()
	}

	return database,
		store.NewProjectRepository(database),
		store.NewIssueRepository(database),
		store.NewTagRepository(database),
		store.NewIssueCommentRepository(database),
		store.NewIssueActivityRepository(database),
		cleanup
}

func TestProjectCRUD(t *testing.T) {
	pr, _, _, _, cleanup := setupStore(t)
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
	pr, ir, _, _, cleanup := setupStore(t)
	defer cleanup()

	project, err := pr.Create("Test Project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	issue, err := ir.Create(project.ID, "First Issue", "Description", "todo", "high")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	if issue.ID == "" {
		t.Error("expected non-empty issue ID")
	}
	if issue.Identifier != "ISSUE-1" {
		t.Errorf("expected generated identifier 'ISSUE-1', got '%s'", issue.Identifier)
	}
	if issue.Position != 1 {
		t.Errorf("expected generated position 1, got %d", issue.Position)
	}

	got, err := ir.GetByID(issue.ID)
	if err != nil {
		t.Fatalf("get issue: %v", err)
	}
	if got.Title != "First Issue" {
		t.Errorf("expected title 'First Issue', got '%s'", got.Title)
	}

	issues, err := ir.ListByProject(project.ID, store.IssueFilters{})
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

	issues, _ = ir.ListByProject(project.ID, store.IssueFilters{})
	if len(issues) != 0 {
		t.Errorf("expected 0 issues after delete, got %d", len(issues))
	}
}

func TestIssueFiltersGeneratedFieldsAndCascade(t *testing.T) {
	pr, ir, _, _, cleanup := setupStore(t)
	defer cleanup()

	project, err := pr.Create("Filtered Project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	first, err := ir.Create(project.ID, "First", "", "todo", "high")
	if err != nil {
		t.Fatalf("create first issue: %v", err)
	}
	second, err := ir.Create(project.ID, "Second", "", "in_progress", "high")
	if err != nil {
		t.Fatalf("create second issue: %v", err)
	}
	third, err := ir.Create(project.ID, "Third", "", "todo", "low")
	if err != nil {
		t.Fatalf("create third issue: %v", err)
	}

	for _, tc := range []struct {
		name       string
		issue      *store.Issue
		identifier string
		position   int
	}{
		{name: "first", issue: first, identifier: "ISSUE-1", position: 1},
		{name: "second", issue: second, identifier: "ISSUE-2", position: 2},
		{name: "third", issue: third, identifier: "ISSUE-3", position: 3},
	} {
		if tc.issue.Identifier != tc.identifier {
			t.Errorf("%s identifier: expected %s, got %s", tc.name, tc.identifier, tc.issue.Identifier)
		}
		if tc.issue.Position != tc.position {
			t.Errorf("%s position: expected %d, got %d", tc.name, tc.position, tc.issue.Position)
		}
	}

	filtered, err := ir.ListByProject(project.ID, store.IssueFilters{Status: "todo", Priority: "high"})
	if err != nil {
		t.Fatalf("filter issues: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != first.ID {
		t.Fatalf("expected only first todo/high issue, got %#v", filtered)
	}

	statusOnly, err := ir.ListByProject(project.ID, store.IssueFilters{Status: "todo"})
	if err != nil {
		t.Fatalf("filter by status: %v", err)
	}
	if len(statusOnly) != 2 || statusOnly[0].ID != first.ID || statusOnly[1].ID != third.ID {
		t.Fatalf("expected todo issues in position order, got %#v", statusOnly)
	}

	if err := pr.Delete(project.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	remaining, err := ir.ListByProject(project.ID, store.IssueFilters{})
	if err != nil {
		t.Fatalf("list after cascade delete: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected cascade delete to remove issues, got %d", len(remaining))
	}
}

func TestAgentCRUD(t *testing.T) {
	_, _, ar, rr, cleanup := setupStore(t)
	defer cleanup()

	runtime, err := rr.Upsert(&store.Runtime{
		Kind:         "codex",
		DisplayName:  "Codex",
		BinaryPath:   "/usr/local/bin/codex",
		VersionRaw:   "1.0.0",
		HealthStatus: "healthy",
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	agent, err := ar.Create("Build Agent", "craftsperson", runtime.ID, "gpt-5", "Ship complete work.")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if agent.ID == "" {
		t.Error("expected non-empty agent ID")
	}
	if agent.Status != "offline" {
		t.Errorf("expected status 'offline', got '%s'", agent.Status)
	}
	if !agent.Assignable {
		t.Errorf("expected agent with healthy runtime to be assignable: %s", agent.AssignableReason)
	}

	got, err := ar.GetByID(agent.ID)
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if got.Name != "Build Agent" {
		t.Errorf("expected name 'Build Agent', got '%s'", got.Name)
	}

	agent.Name = "Oracle"
	agent.Role = "advisor"
	agent.Status = "online"
	agent.Model = "gpt-5-mini"
	agent.Instructions = "Advise carefully."
	if err := ar.Update(agent); err != nil {
		t.Fatalf("update agent: %v", err)
	}

	agents, err := ar.List(false)
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

	agents, _ = ar.List(false)
	if len(agents) != 0 {
		t.Errorf("expected 0 agents after delete, got %d", len(agents))
	}
}

func TestRuntimeUpsertAndList(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()
	if err := migration.Run(database); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	repo := store.NewRuntimeRepository(database)
	created, err := repo.Upsert(&store.Runtime{
		Kind:         "codex",
		DisplayName:  "Codex CLI",
		BinaryPath:   "/usr/local/bin/codex",
		VersionRaw:   "codex 1.0.0",
		HealthStatus: "healthy",
		HealthReason: "",
	})
	if err != nil {
		t.Fatalf("upsert runtime: %v", err)
	}
	if created.ID == "" || created.Kind != "codex" || created.HealthStatus != "healthy" {
		t.Fatalf("unexpected created runtime: %#v", created)
	}

	updated, err := repo.Upsert(&store.Runtime{
		Kind:         "codex",
		DisplayName:  "Codex Stable",
		BinaryPath:   "/opt/homebrew/bin/codex",
		VersionRaw:   "",
		HealthStatus: "degraded",
		HealthReason: "probe_failed",
	})
	if err != nil {
		t.Fatalf("upsert runtime update: %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("expected kind upsert to preserve id %s, got %s", created.ID, updated.ID)
	}
	if updated.DisplayName != "Codex Stable" || updated.HealthReason != "probe_failed" {
		t.Fatalf("unexpected updated runtime: %#v", updated)
	}

	runtimes, err := repo.List()
	if err != nil {
		t.Fatalf("list runtimes: %v", err)
	}
	if len(runtimes) != 1 || runtimes[0].ID != created.ID {
		t.Fatalf("expected one runtime, got %#v", runtimes)
	}
}

func TestTagCRUDAndIssueAttachIdempotency(t *testing.T) {
	_, pr, ir, tr, _, _, cleanup := setupInteractionStore(t)
	defer cleanup()

	project, err := pr.Create("Tagged Project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	issue, err := ir.Create(project.ID, "Tagged Issue", "", "", "")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}

	tag, err := tr.Create(project.ID, "Bug", "")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if tag.Color != "#71717a" {
		t.Fatalf("expected default tag color, got %s", tag.Color)
	}

	_, err = tr.Create(project.ID, "bug", "#ef4444")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Fatalf("expected case-insensitive unique tag error, got %v", err)
	}

	got, err := tr.GetByID(tag.ID)
	if err != nil {
		t.Fatalf("get tag: %v", err)
	}
	if got == nil || got.Name != "Bug" {
		t.Fatalf("expected Bug tag, got %#v", got)
	}

	tag.Name = "Defect"
	tag.Color = "#dc2626"
	if err := tr.Update(tag); err != nil {
		t.Fatalf("update tag: %v", err)
	}

	if err := tr.AttachToIssue(issue.ID, tag.ID); err != nil {
		t.Fatalf("attach tag: %v", err)
	}
	if err := tr.AttachToIssue(issue.ID, tag.ID); err != nil {
		t.Fatalf("duplicate attach should be idempotent: %v", err)
	}

	issueTags, err := tr.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list issue tags: %v", err)
	}
	if len(issueTags) != 1 || issueTags[0].Name != "Defect" {
		t.Fatalf("expected one attached Defect tag, got %#v", issueTags)
	}

	projectTags, err := tr.ListByProject(project.ID)
	if err != nil {
		t.Fatalf("list project tags: %v", err)
	}
	if len(projectTags) != 1 {
		t.Fatalf("expected one project tag, got %d", len(projectTags))
	}

	if err := tr.DetachFromIssue(issue.ID, tag.ID); err != nil {
		t.Fatalf("detach tag: %v", err)
	}
	issueTags, err = tr.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list issue tags after detach: %v", err)
	}
	if len(issueTags) != 0 {
		t.Fatalf("expected no issue tags after detach, got %d", len(issueTags))
	}

	if err := tr.Delete(tag.ID); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	projectTags, err = tr.ListByProject(project.ID)
	if err != nil {
		t.Fatalf("list project tags after delete: %v", err)
	}
	if len(projectTags) != 0 {
		t.Fatalf("expected no project tags after delete, got %d", len(projectTags))
	}
}

func TestTagAttachRejectsCrossProjectTags(t *testing.T) {
	_, pr, ir, tr, _, _, cleanup := setupInteractionStore(t)
	defer cleanup()

	firstProject, err := pr.Create("First Project", "")
	if err != nil {
		t.Fatalf("create first project: %v", err)
	}
	secondProject, err := pr.Create("Second Project", "")
	if err != nil {
		t.Fatalf("create second project: %v", err)
	}
	issue, err := ir.Create(firstProject.ID, "Scoped Issue", "", "", "")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	tag, err := tr.Create(secondProject.ID, "Foreign Tag", "")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	err = tr.AttachToIssue(issue.ID, tag.ID)
	if err == nil || !strings.Contains(err.Error(), "same project") {
		t.Fatalf("expected cross-project attach rejection, got %v", err)
	}

	tags, err := tr.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list issue tags: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected no cross-project tags attached, got %#v", tags)
	}
}

func TestIssueCommentCRUD(t *testing.T) {
	_, pr, ir, _, cr, _, cleanup := setupInteractionStore(t)
	defer cleanup()

	project, err := pr.Create("Comments Project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	issue, err := ir.Create(project.ID, "Commented Issue", "", "", "")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}

	comment, err := cr.Create(issue.ID, "agent-1", "first note")
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if comment.ID == "" {
		t.Fatal("expected comment ID")
	}

	got, err := cr.GetByID(comment.ID)
	if err != nil {
		t.Fatalf("get comment: %v", err)
	}
	if got == nil || got.Body != "first note" {
		t.Fatalf("expected first note comment, got %#v", got)
	}

	comment.Body = "updated note"
	if err := cr.Update(comment); err != nil {
		t.Fatalf("update comment: %v", err)
	}

	comments, err := cr.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list comments: %v", err)
	}
	if len(comments) != 1 || comments[0].Body != "updated note" {
		t.Fatalf("expected updated comment, got %#v", comments)
	}

	if err := cr.Delete(comment.ID); err != nil {
		t.Fatalf("delete comment: %v", err)
	}
	comments, err = cr.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list comments after delete: %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("expected no comments after delete, got %d", len(comments))
	}
}

func TestActivityCreateListAndIssueChangeInputs(t *testing.T) {
	_, pr, ir, _, _, ar, cleanup := setupInteractionStore(t)
	defer cleanup()

	if changes := store.MeaningfulIssueChanges(nil, &store.Issue{}); changes != nil {
		t.Fatalf("expected nil changes for nil before issue, got %#v", changes)
	}
	if changes := store.MeaningfulIssueChanges(&store.Issue{}, nil); changes != nil {
		t.Fatalf("expected nil changes for nil after issue, got %#v", changes)
	}

	project, err := pr.Create("Activity Project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	issue, err := ir.Create(project.ID, "Original", "  description  ", "todo", "medium")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}

	after := *issue
	after.Title = "Original "
	after.Description = "changed"
	after.Status = "in_progress"
	after.CreatorID = "creator-2"
	changes := store.MeaningfulIssueChanges(issue, &after)
	if len(changes) != 3 {
		t.Fatalf("expected description, status, and creator changes only, got %#v", changes)
	}
	if changes[2].Field != "creator_id" || changes[2].ActivityType != "creator.changed" {
		t.Fatalf("expected creator_id to emit creator.changed, got %#v", changes[2])
	}

	inputs, err := store.ActivityInputsForIssueChanges(issue.ID, "agent-1", changes)
	if err != nil {
		t.Fatalf("build activity inputs: %v", err)
	}
	var firstActivity *store.IssueActivity
	for _, input := range inputs {
		activity, err := ar.Create(input)
		if err != nil {
			t.Fatalf("create activity: %v", err)
		}
		if firstActivity == nil {
			firstActivity = activity
		}
	}

	got, err := ar.GetByID(firstActivity.ID)
	if err != nil {
		t.Fatalf("get activity: %v", err)
	}
	if got == nil || got.Type != "issue.updated" {
		t.Fatalf("expected issue.updated activity, got %#v", got)
	}
	got.Summary = "description changed"
	if err := ar.Update(got); err != nil {
		t.Fatalf("update activity: %v", err)
	}

	activities, err := ar.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	if len(activities) != 3 {
		t.Fatalf("expected three activity rows, got %d", len(activities))
	}
	if activities[0].Type != "issue.updated" || activities[1].Type != "status.changed" || activities[2].Type != "creator.changed" {
		t.Fatalf("unexpected activity types: %#v", activities)
	}
	if !strings.Contains(activities[0].MetadataJSON, `"field":"description"`) {
		t.Fatalf("expected metadata to include changed field, got %s", activities[0].MetadataJSON)
	}

	if err := ar.Delete(activities[0].ID); err != nil {
		t.Fatalf("delete activity: %v", err)
	}
	activities, err = ar.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list activity after delete: %v", err)
	}
	if len(activities) != 2 {
		t.Fatalf("expected two activities after delete, got %d", len(activities))
	}
}

func TestWithTxCommitsMutationAndActivityAtomically(t *testing.T) {
	database, pr, ir, tr, _, ar, cleanup := setupInteractionStore(t)
	defer cleanup()

	project, err := pr.Create("Tx Project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	issue, err := ir.Create(project.ID, "Tx Issue", "", "", "")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}

	if err := store.WithTx(database, func(repos store.Repositories) error {
		tag, err := repos.Tags.Create(project.ID, "Ready", "#22c55e")
		if err != nil {
			return err
		}
		if err := repos.Tags.AttachToIssue(issue.ID, tag.ID); err != nil {
			return err
		}
		_, err = repos.Activity.Create(store.CreateIssueActivityInput{
			IssueID:      issue.ID,
			ActorID:      "agent-1",
			Type:         "tag.attached",
			Summary:      "tag attached",
			MetadataJSON: `{"tag_id":"` + tag.ID + `"}`,
		})
		return err
	}); err != nil {
		t.Fatalf("commit transaction: %v", err)
	}

	issueTags, err := tr.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list committed issue tags: %v", err)
	}
	activities, err := ar.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list committed activity: %v", err)
	}
	if len(issueTags) != 1 || len(activities) != 1 {
		t.Fatalf("expected committed tag and activity, got %d tags/%d activities", len(issueTags), len(activities))
	}

	err = store.WithTx(database, func(repos store.Repositories) error {
		tag, err := repos.Tags.Create(project.ID, "Rolled Back", "#ef4444")
		if err != nil {
			return err
		}
		if err := repos.Tags.AttachToIssue(issue.ID, tag.ID); err != nil {
			return err
		}
		_, err = repos.Activity.Create(store.CreateIssueActivityInput{
			IssueID: issue.ID,
			Type:    "tag.attached",
			Summary: "tag attached",
		})
		if err != nil {
			return err
		}
		return sql.ErrTxDone
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}

	issueTags, err = tr.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list issue tags after rollback: %v", err)
	}
	activities, err = ar.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list activity after rollback: %v", err)
	}
	if len(issueTags) != 1 || len(activities) != 1 {
		t.Fatalf("expected rollback to keep counts at 1/1, got %d tags/%d activities", len(issueTags), len(activities))
	}
}

func TestWithTxCommitsIssueUpdateAndActivityAtomically(t *testing.T) {
	database, pr, ir, _, _, ar, cleanup := setupInteractionStore(t)
	defer cleanup()

	project, err := pr.Create("Issue Tx Project", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	issue, err := ir.Create(project.ID, "Before", "old", "todo", "medium")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}

	if err := store.WithTx(database, func(repos store.Repositories) error {
		before, err := repos.Issues.GetByID(issue.ID)
		if err != nil {
			return err
		}
		after := *before
		after.Title = "After"
		after.Status = "in_progress"
		if err := repos.Issues.Update(&after); err != nil {
			return err
		}
		inputs, err := store.ActivityInputsForIssueChanges(after.ID, "agent-1", store.MeaningfulIssueChanges(before, &after))
		if err != nil {
			return err
		}
		for _, input := range inputs {
			if _, err := repos.Activity.Create(input); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("commit issue update transaction: %v", err)
	}

	updated, err := ir.GetByID(issue.ID)
	if err != nil {
		t.Fatalf("get updated issue: %v", err)
	}
	activities, err := ar.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list update activity: %v", err)
	}
	if updated.Title != "After" || updated.Status != "in_progress" {
		t.Fatalf("expected committed issue update, got %#v", updated)
	}
	if len(activities) != 2 || activities[0].Type != "issue.updated" || activities[1].Type != "status.changed" {
		t.Fatalf("expected issue.updated and status.changed activity rows, got %#v", activities)
	}

	err = store.WithTx(database, func(repos store.Repositories) error {
		before, err := repos.Issues.GetByID(issue.ID)
		if err != nil {
			return err
		}
		after := *before
		after.Priority = "high"
		if err := repos.Issues.Update(&after); err != nil {
			return err
		}
		inputs, err := store.ActivityInputsForIssueChanges(after.ID, "agent-1", store.MeaningfulIssueChanges(before, &after))
		if err != nil {
			return err
		}
		for _, input := range inputs {
			if _, err := repos.Activity.Create(input); err != nil {
				return err
			}
		}
		return sql.ErrTxDone
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}

	updated, err = ir.GetByID(issue.ID)
	if err != nil {
		t.Fatalf("get issue after rollback: %v", err)
	}
	activities, err = ar.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list activity after rollback: %v", err)
	}
	if updated.Priority != "medium" || len(activities) != 2 {
		t.Fatalf("expected rolled back priority/activity, got priority %s and %d activities", updated.Priority, len(activities))
	}
}

func TestInteractionCascadeOnProjectDelete(t *testing.T) {
	_, pr, ir, tr, cr, ar, cleanup := setupInteractionStore(t)
	defer cleanup()

	project, err := pr.Create("Cascade Interactions", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	issue, err := ir.Create(project.ID, "Cascade Issue", "", "", "")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	tag, err := tr.Create(project.ID, "Cleanup", "")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := tr.AttachToIssue(issue.ID, tag.ID); err != nil {
		t.Fatalf("attach tag: %v", err)
	}
	if _, err := cr.Create(issue.ID, "", "delete me"); err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if _, err := ar.Create(store.CreateIssueActivityInput{IssueID: issue.ID, Type: "issue.created", Summary: "issue created"}); err != nil {
		t.Fatalf("create activity: %v", err)
	}

	if err := pr.Delete(project.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}

	projectTags, err := tr.ListByProject(project.ID)
	if err != nil {
		t.Fatalf("list tags after project delete: %v", err)
	}
	comments, err := cr.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list comments after project delete: %v", err)
	}
	activities, err := ar.ListByIssue(issue.ID)
	if err != nil {
		t.Fatalf("list activity after project delete: %v", err)
	}
	if len(projectTags) != 0 || len(comments) != 0 || len(activities) != 0 {
		t.Fatalf("expected interaction rows to cascade, got %d tags/%d comments/%d activities", len(projectTags), len(comments), len(activities))
	}
}
