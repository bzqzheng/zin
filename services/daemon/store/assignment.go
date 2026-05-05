package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

type RetrievalOutcomeInput struct {
	AssignmentID      string
	FailureReason     string
	Policy            string
	ContinueOnFailure bool
	AuditMetadataJSON string
}

type MemoryInfluenceEventInput struct {
	AssignmentID  string
	ResultID      string
	MemoryID      string
	InfluenceType string
	ReasonCode    string
	MetadataJSON  string
}

type MemoryInfluenceLogger interface {
	LogMemoryInfluence(MemoryInfluenceEventInput) (*MemoryInfluenceEvent, error)
}

var (
	ErrAssignmentStateConflict     = errors.New("ASSIGNMENT_STATE_CONFLICT")
	ErrAssignmentInvalidTransition = errors.New("ASSIGNMENT_INVALID_TRANSITION")
	ErrRetrievalAuditRequired      = errors.New("RETRIEVAL_AUDIT_REQUIRED")
	ErrMemoryInfluenceInvalid      = errors.New("MEMORY_INFLUENCE_INVALID")
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
		        ia.retrieval_status, ia.retrieval_failure_reason, ia.retrieval_policy, ia.retrieval_audit_metadata,
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

func (r *IssueAssignmentRepository) RecordRetrievalCreationFailure(id, reason string) (*IssueAssignment, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.q.Exec(
		`UPDATE issue_assignments
		 SET status = 'queued',
		     error_code = 'retrieval_failed',
		     error_message = ?,
		     retrieval_status = 'failed',
		     retrieval_failure_reason = ?,
		     retrieval_policy = 'fail_fast',
		     retrieval_audit_metadata = '',
		     updated_at = ?
		 WHERE id = ? AND status = 'queued'`,
		reason,
		reason,
		now,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("record retrieval creation failure: %w", err)
	}
	if err := requireRowsAffected(res); err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *IssueAssignmentRepository) StartRetrieval(id string) (*IssueAssignment, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.q.Exec(
		`UPDATE issue_assignments
		 SET status = 'retrieving_memory',
		     error_code = '',
		     error_message = '',
		     retrieval_status = 'retrieving',
		     retrieval_failure_reason = '',
		     retrieval_policy = 'fail_fast',
		     retrieval_audit_metadata = '',
		     updated_at = ?
		 WHERE id = ? AND status = 'queued'`,
		now,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("start assignment retrieval: %w", err)
	}
	if err := requireRowsAffected(res); err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *IssueAssignmentRepository) CompleteRetrievalEmpty(id string) (*IssueAssignment, error) {
	return r.completeRetrievalReady(id, "empty", "", "fail_fast", "")
}

func (r *IssueAssignmentRepository) CompleteRetrievalFailure(input RetrievalOutcomeInput) (*IssueAssignment, error) {
	reason := strings.TrimSpace(input.FailureReason)
	policy := strings.TrimSpace(input.Policy)
	if policy == "" {
		policy = "fail_fast"
	}
	if !input.ContinueOnFailure {
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := r.q.Exec(
			`UPDATE issue_assignments
			 SET status = 'retrieval_failed',
			     failed_at = ?,
			     error_code = 'retrieval_failed',
			     error_message = ?,
			     retrieval_status = 'failed',
			     retrieval_failure_reason = ?,
			     retrieval_policy = ?,
			     retrieval_audit_metadata = '',
			     updated_at = ?
			 WHERE id = ? AND status = 'retrieving_memory'`,
			now,
			reason,
			reason,
			policy,
			now,
			input.AssignmentID,
		)
		if err != nil {
			return nil, fmt.Errorf("complete retrieval fail-fast: %w", err)
		}
		if err := requireRowsAffected(res); err != nil {
			return nil, err
		}
		return r.GetByID(input.AssignmentID)
	}

	auditMetadata := strings.TrimSpace(input.AuditMetadataJSON)
	if reason == "" || auditMetadata == "" || !json.Valid([]byte(auditMetadata)) {
		return nil, ErrRetrievalAuditRequired
	}
	if _, err := r.LogMemoryInfluence(MemoryInfluenceEventInput{
		AssignmentID:  input.AssignmentID,
		InfluenceType: "retrieval_failed",
		ReasonCode:    reason,
		MetadataJSON:  auditMetadata,
	}); err != nil {
		return nil, err
	}
	return r.completeRetrievalReady(input.AssignmentID, "failed", reason, "continue_without_memory", auditMetadata)
}

func (r *IssueAssignmentRepository) completeRetrievalReady(id, retrievalStatus, reason, policy, metadata string) (*IssueAssignment, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.q.Exec(
		`UPDATE issue_assignments
		 SET status = 'ready',
		     error_code = '',
		     error_message = '',
		     retrieval_status = ?,
		     retrieval_failure_reason = ?,
		     retrieval_policy = ?,
		     retrieval_audit_metadata = ?,
		     updated_at = ?
		 WHERE id = ? AND status = 'retrieving_memory'`,
		retrievalStatus,
		reason,
		policy,
		metadata,
		now,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("complete assignment retrieval: %w", err)
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
		 (result_id, assignment_id, attempt_no, output, status, error, observability_degraded, observability_degraded_reason, observability_degraded_detail, started_at, finished_at)
		 SELECT ?, ?, ?, ?, 'succeeded', '', 0, '', '', ?, ?
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

func (r *IssueAssignmentRepository) CompleteWithResultAndInfluence(input CompleteAssignmentInput, events []MemoryInfluenceEventInput, logger MemoryInfluenceLogger) (*IssueAssignment, *AssignmentResult, error) {
	assignment, result, err := r.CompleteWithResult(input)
	if err != nil {
		return nil, nil, err
	}
	if logger == nil {
		logger = r
	}
	for _, event := range events {
		event.AssignmentID = input.AssignmentID
		event.ResultID = result.ResultID
		if _, err := logger.LogMemoryInfluence(event); err != nil {
			degraded, degradeErr := r.MarkResultObservabilityDegraded(result.ResultID, "influence_logging_failed", err.Error())
			if degradeErr != nil {
				return assignment, result, fmt.Errorf("mark observability degraded after influence log failure: %w", degradeErr)
			}
			return assignment, degraded, nil
		}
	}
	return assignment, result, nil
}

func (r *IssueAssignmentRepository) MarkResultObservabilityDegraded(resultID, reason, detail string) (*AssignmentResult, error) {
	_, err := r.q.Exec(
		`UPDATE assignment_results
		 SET observability_degraded = 1, observability_degraded_reason = ?, observability_degraded_detail = ?
		 WHERE result_id = ?`,
		reason,
		detail,
		resultID,
	)
	if err != nil {
		return nil, fmt.Errorf("mark result observability degraded: %w", err)
	}
	return r.GetResultByID(resultID)
}

func (r *IssueAssignmentRepository) GetResultByID(id string) (*AssignmentResult, error) {
	row := r.q.QueryRow(
		`SELECT result_id, assignment_id, attempt_no, output, status, error, observability_degraded, observability_degraded_reason, observability_degraded_detail, started_at, finished_at
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
		`SELECT result_id, assignment_id, attempt_no, output, status, error, observability_degraded, observability_degraded_reason, observability_degraded_detail, started_at, finished_at
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

func (r *IssueAssignmentRepository) LogMemoryInfluence(input MemoryInfluenceEventInput) (*MemoryInfluenceEvent, error) {
	if strings.TrimSpace(input.InfluenceType) == "" {
		return nil, ErrMemoryInfluenceInvalid
	}
	if input.InfluenceType == "retrieval_failed" && strings.TrimSpace(input.ReasonCode) == "" {
		return nil, ErrMemoryInfluenceInvalid
	}
	metadata := strings.TrimSpace(input.MetadataJSON)
	if metadata == "" {
		metadata = "{}"
	}
	if !json.Valid([]byte(metadata)) {
		return nil, ErrMemoryInfluenceInvalid
	}

	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate memory influence uuid: %w", err)
	}
	appliedAt := time.Now().UTC()
	_, err = r.q.Exec(
		`INSERT INTO memory_influence_events
		 (id, assignment_id, result_id, memory_id, influence_type, reason_code, metadata_json, applied_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uid.String(),
		input.AssignmentID,
		nullString(input.ResultID),
		input.MemoryID,
		input.InfluenceType,
		input.ReasonCode,
		metadata,
		appliedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert memory influence event: %w", err)
	}
	return &MemoryInfluenceEvent{
		ID:            uid.String(),
		AssignmentID:  input.AssignmentID,
		ResultID:      input.ResultID,
		MemoryID:      input.MemoryID,
		InfluenceType: input.InfluenceType,
		ReasonCode:    input.ReasonCode,
		MetadataJSON:  metadata,
		AppliedAt:     appliedAt,
	}, nil
}

func (r *IssueAssignmentRepository) ListMemoryInfluenceEvents(assignmentID string) ([]*MemoryInfluenceEvent, error) {
	rows, err := r.q.Query(
		`SELECT id, assignment_id, COALESCE(result_id, ''), memory_id, influence_type, reason_code, metadata_json, applied_at
		 FROM memory_influence_events
		 WHERE assignment_id = ?
		 ORDER BY applied_at ASC, id ASC`,
		assignmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list memory influence events: %w", err)
	}
	defer rows.Close()

	var events []*MemoryInfluenceEvent
	for rows.Next() {
		event, err := scanMemoryInfluenceEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory influence events: %w", err)
	}
	return events, nil
}

func (r *IssueAssignmentRepository) get(where string, arg string) (*IssueAssignment, error) {
	row := r.q.QueryRow(
		`SELECT ia.id, ia.issue_id, COALESCE(ia.agent_id, ''), COALESCE(a.name, ''), COALESCE(a.runtime_id, ''),
		        COALESCE(rt.display_name, ''), COALESCE(rt.health_status, ''), ia.requested_by, ia.source_type, ia.source_id,
		        ia.client_request_id, ia.request_fingerprint, ia.status, ia.dedupe_key,
		        ia.error_code, ia.error_message,
		        ia.retrieval_status, ia.retrieval_failure_reason, ia.retrieval_policy, ia.retrieval_audit_metadata,
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
		&a.RetrievalStatus,
		&a.RetrievalFailureReason,
		&a.RetrievalPolicy,
		&a.RetrievalAuditMetadata,
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
	var observabilityDegraded int
	if err := scanner.Scan(
		&result.ResultID,
		&result.AssignmentID,
		&result.AttemptNo,
		&result.Output,
		&result.Status,
		&result.Error,
		&observabilityDegraded,
		&result.ObservabilityDegradedReason,
		&result.ObservabilityDegradedDetail,
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
	result.ObservabilityDegraded = observabilityDegraded != 0
	return result, nil
}

func scanMemoryInfluenceEvent(scanner resultScanner) (*MemoryInfluenceEvent, error) {
	event := &MemoryInfluenceEvent{}
	var appliedAt string
	if err := scanner.Scan(
		&event.ID,
		&event.AssignmentID,
		&event.ResultID,
		&event.MemoryID,
		&event.InfluenceType,
		&event.ReasonCode,
		&event.MetadataJSON,
		&appliedAt,
	); err != nil {
		return nil, fmt.Errorf("scan memory influence event: %w", err)
	}
	var err error
	event.AppliedAt, err = parseTime(appliedAt)
	if err != nil {
		return nil, fmt.Errorf("scan memory influence event: %w", err)
	}
	return event, nil
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
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
