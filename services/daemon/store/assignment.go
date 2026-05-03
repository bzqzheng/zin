package store

import (
	"database/sql"
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
	_, err := r.q.Exec(
		`UPDATE issue_assignments
		 SET status = 'cancelled', cancelled_at = ?, updated_at = ?
		 WHERE id = ? AND status NOT IN ('completed', 'failed', 'cancelled')`,
		now,
		now,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("cancel issue assignment: %w", err)
	}
	return r.GetByID(id)
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
