package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	Output       string
	StartedAt    time.Time
	FinishedAt   time.Time
}

var (
	ErrAssignmentStateConflict     = errors.New("ASSIGNMENT_STATE_CONFLICT")
	ErrAssignmentInvalidTransition = errors.New("ASSIGNMENT_INVALID_TRANSITION")
)

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
		Status:             "queued",
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
		        ia.error_code, ia.error_message,
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
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.q.Exec(
		`UPDATE issue_assignments
		 SET status = 'cancelled', cancelled_at = ?, error_code = '', error_message = '', updated_at = ?
		 WHERE id = ? AND status NOT IN ('succeeded', 'failed', 'cancelled', 'retrieval_failed')`,
		now,
		now,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("cancel issue assignment: %w", err)
	}
	if err := requireRowsAffected(res); err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *IssueAssignmentRepository) TransitionCAS(id, fromStatus, toStatus string) (*IssueAssignment, error) {
	if !validAssignmentTransition(fromStatus, toStatus) {
		return nil, ErrAssignmentInvalidTransition
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.q.Exec(
		`UPDATE issue_assignments
		 SET status = ?, error_code = '', error_message = '', updated_at = ?
		 WHERE id = ? AND status = ?`,
		toStatus,
		now,
		id,
		fromStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("transition issue assignment: %w", err)
	}
	if err := requireRowsAffected(res); err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func validAssignmentTransition(fromStatus, toStatus string) bool {
	switch fromStatus {
	case "queued":
		return toStatus == "retrieving_memory"
	case "retrieving_memory":
		return toStatus == "ready" || toStatus == "retrieval_failed"
	case "ready":
		return toStatus == "running"
	default:
		return false
	}
}

func (r *IssueAssignmentRepository) FailCAS(id, fromStatus, code, message string) (*IssueAssignment, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.q.Exec(
		`UPDATE issue_assignments
		 SET status = 'failed', failed_at = ?, error_code = ?, error_message = ?, updated_at = ?
		 WHERE id = ? AND status = ?`,
		now,
		code,
		message,
		now,
		id,
		fromStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("fail issue assignment: %w", err)
	}
	if err := requireRowsAffected(res); err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *IssueAssignmentRepository) CompleteWithResult(input CompleteAssignmentInput) (*IssueAssignment, *AssignmentResult, error) {
	startedAt := input.StartedAt.UTC()
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	finishedAt := input.FinishedAt.UTC()
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}

	resultID, err := uuid.NewV7()
	if err != nil {
		return nil, nil, fmt.Errorf("generate result uuid: %w", err)
	}
	var attemptNo int
	if err := r.q.QueryRow(
		`SELECT COALESCE(MAX(attempt_no), 0) + 1 FROM assignment_results WHERE assignment_id = ?`,
		input.AssignmentID,
	).Scan(&attemptNo); err != nil {
		return nil, nil, fmt.Errorf("select assignment result attempt: %w", err)
	}

	res, err := r.q.Exec(
		`INSERT INTO assignment_results
		 (result_id, assignment_id, attempt_no, output, status, error, started_at, finished_at)
		 SELECT ?, ?, ?, ?, 'succeeded', '', ?, ?
		 WHERE EXISTS (SELECT 1 FROM issue_assignments WHERE id = ? AND status = 'running')`,
		resultID.String(),
		input.AssignmentID,
		attemptNo,
		input.Output,
		startedAt.Format(time.RFC3339),
		finishedAt.Format(time.RFC3339),
		input.AssignmentID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("insert assignment result: %w", err)
	}
	if err := requireRowsAffected(res); err != nil {
		return nil, nil, err
	}

	res, err = r.q.Exec(
		`UPDATE issue_assignments
		 SET status = 'succeeded', completed_at = ?, error_code = '', error_message = '', updated_at = ?
		 WHERE id = ? AND status = 'running'`,
		finishedAt.Format(time.RFC3339),
		finishedAt.Format(time.RFC3339),
		input.AssignmentID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("mark assignment succeeded: %w", err)
	}
	if err := requireRowsAffected(res); err != nil {
		return nil, nil, err
	}

	assignment, err := r.GetByID(input.AssignmentID)
	if err != nil {
		return nil, nil, err
	}
	result, err := r.GetResultByID(resultID.String())
	if err != nil {
		return nil, nil, err
	}
	return assignment, result, nil
}

func (r *IssueAssignmentRepository) GetResultByID(id string) (*AssignmentResult, error) {
	row := r.q.QueryRow(
		`SELECT result_id, assignment_id, attempt_no, output, status, error, started_at, finished_at
		 FROM assignment_results
		 WHERE result_id = ?`,
		id,
	)
	result, err := scanAssignmentResult(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *IssueAssignmentRepository) ListResults(assignmentID string) ([]*AssignmentResult, error) {
	rows, err := r.q.Query(
		`SELECT result_id, assignment_id, attempt_no, output, status, error, started_at, finished_at
		 FROM assignment_results
		 WHERE assignment_id = ?
		 ORDER BY attempt_no ASC`,
		assignmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list assignment results: %w", err)
	}
	defer rows.Close()

	var results []*AssignmentResult
	for rows.Next() {
		result, err := scanAssignmentResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assignment results: %w", err)
	}
	return results, nil
}

func (r *IssueAssignmentRepository) get(where string, arg string) (*IssueAssignment, error) {
	row := r.q.QueryRow(
		`SELECT ia.id, ia.issue_id, COALESCE(ia.agent_id, ''), COALESCE(a.name, ''), COALESCE(a.runtime_id, ''),
		        COALESCE(rt.display_name, ''), COALESCE(rt.health_status, ''), ia.requested_by, ia.source_type, ia.source_id,
		        ia.client_request_id, ia.request_fingerprint, ia.status, ia.dedupe_key,
		        ia.error_code, ia.error_message,
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
		&a.ErrorCode,
		&a.ErrorMessage,
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

type resultScanner interface {
	Scan(dest ...any) error
}

func scanAssignmentResult(scanner resultScanner) (*AssignmentResult, error) {
	result := &AssignmentResult{}
	var startedAt, finishedAt string
	if err := scanner.Scan(
		&result.ResultID,
		&result.AssignmentID,
		&result.AttemptNo,
		&result.Output,
		&result.Status,
		&result.Error,
		&startedAt,
		&finishedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("scan assignment result: %w", err)
	}

	var err error
	result.StartedAt, err = parseTime(startedAt)
	if err != nil {
		return nil, fmt.Errorf("scan assignment result: %w", err)
	}
	result.FinishedAt, err = parseTime(finishedAt)
	if err != nil {
		return nil, fmt.Errorf("scan assignment result: %w", err)
	}
	return result, nil
}

func requireRowsAffected(res sql.Result) error {
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check assignment rows affected: %w", err)
	}
	if rows == 0 {
		return ErrAssignmentStateConflict
	}
	return nil
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
