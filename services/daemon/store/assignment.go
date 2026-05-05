package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	AssignmentStatusQueued           = "queued"
	AssignmentStatusRetrievingMemory = "retrieving_memory"
	AssignmentStatusReady            = "ready"
	AssignmentStatusRunning          = "running"
	AssignmentStatusSucceeded        = "succeeded"
	AssignmentStatusFailed           = "failed"
	AssignmentStatusCancelled        = "cancelled"
	AssignmentStatusRetrievalFailed  = "retrieval_failed"
)

var (
	ErrAssignmentStateConflict     = errors.New("assignment state conflict")
	ErrInvalidAssignmentTransition = errors.New("invalid assignment transition")
	ErrAssignmentResultStatus      = errors.New("invalid assignment result status")
	ErrAssignmentResultTxRequired  = errors.New("assignment result transaction required")
)

type IssueAssignmentRepository struct {
	db *sql.DB
	q  sqlRunner
}

type CreateIssueAssignmentInput struct {
	IssueID            string
	AgentID            string
	RequestedBy        string
	SourceType         string
	SourceID           string
	ClientRequestID    string
	RequestFingerprint string
	DedupeKey          string
}

type CompleteAssignmentInput struct {
	AssignmentID string
	FromStatus   string
	Status       string
	Output       string
	Error        string
	StartedAt    time.Time
	FinishedAt   time.Time
}

func NewIssueAssignmentRepository(db *sql.DB) *IssueAssignmentRepository {
	return newIssueAssignmentRepository(db, db)
}

func newIssueAssignmentRepository(db *sql.DB, q sqlRunner) *IssueAssignmentRepository {
	return &IssueAssignmentRepository{db: db, q: q}
}

func (r *IssueAssignmentRepository) Create(input CreateIssueAssignmentInput) (*IssueAssignment, error) {
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}
	now := time.Now().UTC()
	assignment := &IssueAssignment{
		ID:                 uid.String(),
		IssueID:            input.IssueID,
		AgentID:            input.AgentID,
		RequestedBy:        input.RequestedBy,
		SourceType:         input.SourceType,
		SourceID:           input.SourceID,
		ClientRequestID:    input.ClientRequestID,
		RequestFingerprint: input.RequestFingerprint,
		Status:             AssignmentStatusQueued,
		DedupeKey:          input.DedupeKey,
		RequestedAt:        now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	_, err = r.q.Exec(
		`INSERT INTO issue_assignments
		 (id, issue_id, agent_id, requested_by, source_type, source_id, client_request_id, request_fingerprint, status, dedupe_key, requested_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		assignment.ID,
		assignment.IssueID,
		assignment.AgentID,
		assignment.RequestedBy,
		assignment.SourceType,
		assignment.SourceID,
		assignment.ClientRequestID,
		assignment.RequestFingerprint,
		assignment.Status,
		assignment.DedupeKey,
		assignment.RequestedAt.Format(time.RFC3339),
		assignment.CreatedAt.Format(time.RFC3339),
		assignment.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert issue assignment: %w", err)
	}
	return r.GetByID(assignment.ID)
}

func (r *IssueAssignmentRepository) GetByID(id string) (*IssueAssignment, error) {
	return r.get("ia.id = ?", id)
}

func (r *IssueAssignmentRepository) GetByClientRequestID(id string) (*IssueAssignment, error) {
	return r.get("ia.client_request_id = ?", id)
}

func (r *IssueAssignmentRepository) GetByDedupeKey(key string) (*IssueAssignment, error) {
	return r.get("ia.dedupe_key = ?", key)
}

func (r *IssueAssignmentRepository) ListByIssue(issueID string) ([]*IssueAssignment, error) {
	rows, err := r.q.Query(
		`SELECT ia.id, ia.issue_id, COALESCE(ia.agent_id, ''), COALESCE(a.name, ''), COALESCE(a.runtime_id, ''),
		        COALESCE(rt.display_name, ''), COALESCE(rt.health_status, ''), ia.requested_by, ia.source_type, ia.source_id,
		        ia.client_request_id, ia.request_fingerprint, ia.status, ia.dedupe_key,
		        ia.requested_at, ia.accepted_at, ia.completed_at, ia.failed_at, ia.cancelled_at, ia.created_at, ia.updated_at
		 FROM issue_assignments ia
		 LEFT JOIN agents a ON a.id = ia.agent_id
		 LEFT JOIN runtimes rt ON rt.id = a.runtime_id
		 WHERE ia.issue_id = ?
		 ORDER BY ia.created_at ASC, ia.id ASC`,
		issueID,
	)
	if err != nil {
		return nil, fmt.Errorf("list issue assignments: %w", err)
	}
	defer rows.Close()

	var assignments []*IssueAssignment
	for rows.Next() {
		assignment, err := scanIssueAssignment(rows)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue assignments: %w", err)
	}
	return assignments, nil
}

func (r *IssueAssignmentRepository) Cancel(id string) (*IssueAssignment, error) {
	current, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, nil
	}
	return r.Transition(id, current.Status, AssignmentStatusCancelled)
}

func (r *IssueAssignmentRepository) Transition(id, fromStatus, toStatus string) (*IssueAssignment, error) {
	if !validAssignmentTransition(fromStatus, toStatus, false) {
		return nil, ErrInvalidAssignmentTransition
	}
	if err := r.transitionCAS(id, fromStatus, toStatus); err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *IssueAssignmentRepository) CompleteWithResult(input CompleteAssignmentInput) (*IssueAssignment, *AssignmentResult, error) {
	if r.db == nil {
		return nil, nil, ErrAssignmentResultTxRequired
	}

	var assignment *IssueAssignment
	var result *AssignmentResult
	err := WithTx(r.db, func(repos Repositories) error {
		created, err := repos.Assignments.createResult(input)
		if err != nil {
			return err
		}
		if err := repos.Assignments.transitionCAS(input.AssignmentID, input.FromStatus, input.Status); err != nil {
			return err
		}
		updated, err := repos.Assignments.GetByID(input.AssignmentID)
		if err != nil {
			return err
		}
		result = created
		assignment = updated
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return assignment, result, nil
}

func (r *IssueAssignmentRepository) RecordAttemptResult(input CompleteAssignmentInput) (*AssignmentResult, error) {
	if r.db == nil {
		return nil, ErrAssignmentResultTxRequired
	}
	if input.Status != AssignmentStatusFailed {
		return nil, ErrAssignmentResultStatus
	}

	var result *AssignmentResult
	err := WithTx(r.db, func(repos Repositories) error {
		if err := repos.Assignments.requireStatus(input.AssignmentID, input.FromStatus); err != nil {
			return err
		}
		created, err := repos.Assignments.createResult(input)
		if err != nil {
			return err
		}
		result = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *IssueAssignmentRepository) requireStatus(id, status string) error {
	var count int
	if err := r.q.QueryRow(
		`SELECT COUNT(*) FROM issue_assignments WHERE id = ? AND status = ?`,
		id,
		status,
	).Scan(&count); err != nil {
		return fmt.Errorf("check assignment status: %w", err)
	}
	if count == 0 {
		return ErrAssignmentStateConflict
	}
	return nil
}

func (r *IssueAssignmentRepository) transitionCAS(id, fromStatus, toStatus string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	setClause := "status = ?, updated_at = ?"
	args := []any{toStatus, now}
	switch toStatus {
	case AssignmentStatusRunning:
		setClause = "status = ?, accepted_at = COALESCE(accepted_at, ?), updated_at = ?"
		args = []any{toStatus, now, now}
	case AssignmentStatusSucceeded:
		setClause = "status = ?, completed_at = ?, updated_at = ?"
		args = []any{toStatus, now, now}
	case AssignmentStatusFailed, AssignmentStatusRetrievalFailed:
		setClause = "status = ?, failed_at = ?, updated_at = ?"
		args = []any{toStatus, now, now}
	case AssignmentStatusCancelled:
		setClause = "status = ?, cancelled_at = ?, updated_at = ?"
		args = []any{toStatus, now, now}
	}
	args = append(args, id, fromStatus)
	res, err := r.q.Exec(
		`UPDATE issue_assignments SET `+setClause+` WHERE id = ? AND status = ?`,
		args...,
	)
	if err != nil {
		return fmt.Errorf("transition issue assignment: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("transition issue assignment rows affected: %w", err)
	}
	if affected == 0 {
		return ErrAssignmentStateConflict
	}
	return nil
}

func (r *IssueAssignmentRepository) createResult(input CompleteAssignmentInput) (*AssignmentResult, error) {
	if !validAssignmentTransition(input.FromStatus, input.Status, true) {
		return nil, ErrInvalidAssignmentTransition
	}
	if input.Status != AssignmentStatusSucceeded && input.Status != AssignmentStatusFailed {
		return nil, ErrAssignmentResultStatus
	}
	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}
	startedAt := input.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	finishedAt := input.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}
	var attemptNo int
	if err := r.q.QueryRow(
		`SELECT COALESCE(MAX(attempt_no), 0) + 1 FROM assignment_results WHERE assignment_id = ?`,
		input.AssignmentID,
	).Scan(&attemptNo); err != nil {
		return nil, fmt.Errorf("next assignment result attempt: %w", err)
	}
	result := &AssignmentResult{
		ResultID:     uid.String(),
		AssignmentID: input.AssignmentID,
		AttemptNo:    attemptNo,
		Output:       input.Output,
		Status:       input.Status,
		Error:        input.Error,
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
	}
	_, err = r.q.Exec(
		`INSERT INTO assignment_results
		 (result_id, assignment_id, attempt_no, output, status, error, started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		result.ResultID,
		result.AssignmentID,
		result.AttemptNo,
		result.Output,
		result.Status,
		result.Error,
		result.StartedAt.Format(time.RFC3339),
		result.FinishedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert assignment result: %w", err)
	}
	return result, nil
}

func (r *IssueAssignmentRepository) get(where string, arg string) (*IssueAssignment, error) {
	row := r.q.QueryRow(
		`SELECT ia.id, ia.issue_id, COALESCE(ia.agent_id, ''), COALESCE(a.name, ''), COALESCE(a.runtime_id, ''),
		        COALESCE(rt.display_name, ''), COALESCE(rt.health_status, ''), ia.requested_by, ia.source_type, ia.source_id,
		        ia.client_request_id, ia.request_fingerprint, ia.status, ia.dedupe_key,
		        ia.requested_at, ia.accepted_at, ia.completed_at, ia.failed_at, ia.cancelled_at, ia.created_at, ia.updated_at
		 FROM issue_assignments ia
		 LEFT JOIN agents a ON a.id = ia.agent_id
		 LEFT JOIN runtimes rt ON rt.id = a.runtime_id
		 WHERE `+where,
		arg,
	)
	assignment, err := scanIssueAssignment(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return assignment, nil
}

func validAssignmentTransition(fromStatus, toStatus string, withResult bool) bool {
	switch fromStatus {
	case AssignmentStatusQueued:
		return toStatus == AssignmentStatusRetrievingMemory || toStatus == AssignmentStatusCancelled
	case AssignmentStatusRetrievingMemory:
		return toStatus == AssignmentStatusReady || toStatus == AssignmentStatusRetrievalFailed || toStatus == AssignmentStatusCancelled
	case AssignmentStatusReady:
		return toStatus == AssignmentStatusRunning || toStatus == AssignmentStatusCancelled
	case AssignmentStatusRunning:
		return (withResult && (toStatus == AssignmentStatusSucceeded || toStatus == AssignmentStatusFailed)) || toStatus == AssignmentStatusCancelled
	default:
		return false
	}
}

func AssignmentStatusTerminal(status string) bool {
	switch status {
	case AssignmentStatusSucceeded, AssignmentStatusFailed, AssignmentStatusCancelled, AssignmentStatusRetrievalFailed:
		return true
	default:
		return false
	}
}

type assignmentScanner interface {
	Scan(dest ...any) error
}

func scanIssueAssignment(scanner assignmentScanner) (*IssueAssignment, error) {
	a := &IssueAssignment{}
	var requestedAt, createdAt, updatedAt string
	var acceptedAt, completedAt, failedAt, cancelledAt sql.NullString
	if err := scanner.Scan(
		&a.ID,
		&a.IssueID,
		&a.AgentID,
		&a.AgentName,
		&a.RuntimeID,
		&a.RuntimeName,
		&a.RuntimeStatus,
		&a.RequestedBy,
		&a.SourceType,
		&a.SourceID,
		&a.ClientRequestID,
		&a.RequestFingerprint,
		&a.Status,
		&a.DedupeKey,
		&requestedAt,
		&acceptedAt,
		&completedAt,
		&failedAt,
		&cancelledAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("scan issue assignment: %w", err)
	}

	var err error
	a.RequestedAt, err = parseTime(requestedAt)
	if err != nil {
		return nil, fmt.Errorf("scan issue assignment: %w", err)
	}
	a.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("scan issue assignment: %w", err)
	}
	a.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan issue assignment: %w", err)
	}
	if a.AcceptedAt, err = parseNullTime(acceptedAt); err != nil {
		return nil, err
	}
	if a.CompletedAt, err = parseNullTime(completedAt); err != nil {
		return nil, err
	}
	if a.FailedAt, err = parseNullTime(failedAt); err != nil {
		return nil, err
	}
	if a.CancelledAt, err = parseNullTime(cancelledAt); err != nil {
		return nil, err
	}
	return a, nil
}

func parseNullTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, fmt.Errorf("scan issue assignment: %w", err)
	}
	return &parsed, nil
}
