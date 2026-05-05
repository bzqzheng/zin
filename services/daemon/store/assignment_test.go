package store_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/bzqzheng/zin/services/daemon/db"
	"github.com/bzqzheng/zin/services/daemon/migration"
	"github.com/bzqzheng/zin/services/daemon/store"
)

func setupAssignmentStore(t *testing.T) (*sqlStoreFixture, func()) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := migration.Run(database); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	fixture := &sqlStoreFixture{
		db:          database,
		projects:    store.NewProjectRepository(database),
		issues:      store.NewIssueRepository(database),
		agents:      store.NewAgentRepository(database),
		runtimes:    store.NewRuntimeRepository(database),
		assignments: store.NewIssueAssignmentRepository(database),
	}
	return fixture, func() {
		database.Close()
	}
}

type sqlStoreFixture struct {
	db          *sql.DB
	projects    *store.ProjectRepository
	issues      *store.IssueRepository
	agents      *store.AgentRepository
	runtimes    *store.RuntimeRepository
	assignments *store.IssueAssignmentRepository
}

func createAssignmentFixture(t *testing.T, f *sqlStoreFixture) *store.IssueAssignment {
	t.Helper()
	project, err := f.projects.Create("Assignments", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	issue, err := f.issues.Create(project.ID, "Run work", "", "", "")
	if err != nil {
		t.Fatalf("create issue: %v", err)
	}
	runtime, err := f.runtimes.Upsert(&store.Runtime{
		Kind:         "codex",
		DisplayName:  "Codex",
		BinaryPath:   "/usr/local/bin/codex",
		VersionRaw:   "codex 1.0.0",
		HealthStatus: "healthy",
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	agent, err := f.agents.Create("Trinity", "builder", runtime.ID, "gpt-5", "Complete the work.")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	assignment, err := f.assignments.Create(store.CreateIssueAssignmentInput{
		IssueID:            issue.ID,
		AgentID:            agent.ID,
		RequestedBy:        "local-user",
		SourceType:         "issue_detail",
		ClientRequestID:    "req-1",
		RequestFingerprint: "fingerprint",
		DedupeKey:          "dedupe",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	return assignment
}

func advanceToRunning(t *testing.T, repo *store.IssueAssignmentRepository, id string) {
	t.Helper()
	for _, step := range []struct {
		from string
		to   string
	}{
		{store.AssignmentStatusQueued, store.AssignmentStatusRetrievingMemory},
		{store.AssignmentStatusRetrievingMemory, store.AssignmentStatusReady},
		{store.AssignmentStatusReady, store.AssignmentStatusRunning},
	} {
		if _, err := repo.Transition(id, step.from, step.to); err != nil {
			t.Fatalf("transition %s -> %s: %v", step.from, step.to, err)
		}
	}
}

func TestAssignmentStateMachineTransitionsAndCASConflict(t *testing.T) {
	f, cleanup := setupAssignmentStore(t)
	defer cleanup()
	assignment := createAssignmentFixture(t, f)

	if _, err := f.assignments.Transition(assignment.ID, store.AssignmentStatusQueued, store.AssignmentStatusRunning); !errors.Is(err, store.ErrInvalidAssignmentTransition) {
		t.Fatalf("expected invalid queued -> running transition, got %v", err)
	}
	retrieving, err := f.assignments.Transition(assignment.ID, store.AssignmentStatusQueued, store.AssignmentStatusRetrievingMemory)
	if err != nil {
		t.Fatalf("transition to retrieving_memory: %v", err)
	}
	if retrieving.Status != store.AssignmentStatusRetrievingMemory {
		t.Fatalf("expected retrieving_memory, got %#v", retrieving)
	}
	if _, err := f.assignments.Transition(assignment.ID, store.AssignmentStatusQueued, store.AssignmentStatusRetrievingMemory); !errors.Is(err, store.ErrAssignmentStateConflict) {
		t.Fatalf("expected stale queued CAS conflict, got %v", err)
	}
	ready, err := f.assignments.Transition(assignment.ID, store.AssignmentStatusRetrievingMemory, store.AssignmentStatusReady)
	if err != nil {
		t.Fatalf("transition to ready: %v", err)
	}
	running, err := f.assignments.Transition(assignment.ID, store.AssignmentStatusReady, store.AssignmentStatusRunning)
	if err != nil {
		t.Fatalf("transition to running: %v", err)
	}
	if ready.Status != store.AssignmentStatusReady || running.Status != store.AssignmentStatusRunning || running.AcceptedAt == nil {
		t.Fatalf("unexpected ready/running state: %#v %#v", ready, running)
	}
}

func TestAssignmentSucceededRequiresPersistedResult(t *testing.T) {
	f, cleanup := setupAssignmentStore(t)
	defer cleanup()
	assignment := createAssignmentFixture(t, f)
	advanceToRunning(t, f.assignments, assignment.ID)

	if _, err := f.assignments.Transition(assignment.ID, store.AssignmentStatusRunning, store.AssignmentStatusSucceeded); !errors.Is(err, store.ErrInvalidAssignmentTransition) {
		t.Fatalf("expected direct succeeded transition rejection, got %v", err)
	}
	assertAssignmentResultCount(t, f.db, assignment.ID, 0)

	updated, result, err := f.assignments.CompleteWithResult(store.CompleteAssignmentInput{
		AssignmentID: assignment.ID,
		FromStatus:   store.AssignmentStatusRunning,
		Status:       store.AssignmentStatusSucceeded,
		Output:       "done",
		StartedAt:    time.Now().UTC().Add(-time.Second),
		FinishedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("complete with result: %v", err)
	}
	if updated.Status != store.AssignmentStatusSucceeded || updated.CompletedAt == nil {
		t.Fatalf("expected succeeded assignment with completed_at, got %#v", updated)
	}
	if result.AttemptNo != 1 || result.Output != "done" || result.Status != store.AssignmentStatusSucceeded {
		t.Fatalf("unexpected result: %#v", result)
	}
	assertAssignmentResultCount(t, f.db, assignment.ID, 1)
}

func TestAssignmentResultsCascadeWithIssueDelete(t *testing.T) {
	f, cleanup := setupAssignmentStore(t)
	defer cleanup()
	assignment := createAssignmentFixture(t, f)
	advanceToRunning(t, f.assignments, assignment.ID)
	if _, _, err := f.assignments.CompleteWithResult(store.CompleteAssignmentInput{
		AssignmentID: assignment.ID,
		FromStatus:   store.AssignmentStatusRunning,
		Status:       store.AssignmentStatusSucceeded,
		Output:       "done",
	}); err != nil {
		t.Fatalf("complete with result: %v", err)
	}

	if err := f.issues.Delete(assignment.IssueID); err != nil {
		t.Fatalf("delete issue: %v", err)
	}
	assertAssignmentResultCount(t, f.db, assignment.ID, 0)
}

func TestAssignmentResultRetriesCreateIncrementedAttempts(t *testing.T) {
	f, cleanup := setupAssignmentStore(t)
	defer cleanup()
	assignment := createAssignmentFixture(t, f)
	advanceToRunning(t, f.assignments, assignment.ID)

	result, err := f.assignments.RecordAttemptResult(store.CompleteAssignmentInput{
		AssignmentID: assignment.ID,
		FromStatus:   store.AssignmentStatusRunning,
		Status:       store.AssignmentStatusFailed,
		Output:       "partial",
		Error:        "tool failed",
	})
	if err != nil {
		t.Fatalf("first failed result: %v", err)
	}
	if result.AttemptNo != 1 {
		t.Fatalf("unexpected first result: %#v", result)
	}
	running, err := f.assignments.GetByID(assignment.ID)
	if err != nil {
		t.Fatalf("get assignment after retry result: %v", err)
	}
	if running.Status != store.AssignmentStatusRunning {
		t.Fatalf("expected retryable assignment to remain running, got %#v", running)
	}
	second, err := f.assignments.RecordAttemptResult(store.CompleteAssignmentInput{
		AssignmentID: assignment.ID,
		FromStatus:   store.AssignmentStatusRunning,
		Status:       store.AssignmentStatusFailed,
		Output:       "second partial",
		Error:        "still failed",
	})
	if err != nil {
		t.Fatalf("second failed result: %v", err)
	}
	if second.AttemptNo != 2 {
		t.Fatalf("expected second attempt_no 2, got %#v", second)
	}
	assertAssignmentResultCount(t, f.db, assignment.ID, 2)
}

func assertAssignmentResultCount(t *testing.T, database *sql.DB, assignmentID string, expected int) {
	t.Helper()
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM assignment_results WHERE assignment_id = ?", assignmentID).Scan(&count); err != nil {
		t.Fatalf("count assignment results: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d assignment results, got %d", expected, count)
	}
}
