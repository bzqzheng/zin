package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAssignmentValidation       = errors.New("assignment validation")
	ErrAssignmentIssueNotFound    = errors.New("assignment issue not found")
	ErrAssignmentAgentNotFound    = errors.New("assignment agent not found")
	ErrAssignmentAgentNotReady    = errors.New("assignment agent not assignable")
	ErrAssignmentRuntimeNotReady  = errors.New("assignment runtime not healthy")
	ErrAssignmentIdempotencyReuse = errors.New("assignment idempotency key reuse conflict")
	ErrAssignmentNotFound         = errors.New("assignment not found")
	ErrAssignmentTerminal         = errors.New("assignment terminal")
)

type IssueAssignmentRepository struct {
	db *sql.DB
}

type CreateIssueAssignmentInput struct {
	IssueID         string
	AgentID         string
	RequestedBy     string
	SourceType      string
	SourceID        string
	ClientRequestID string
}

type CreateIssueAssignmentResult struct {
	Assignment       *IssueAssignment
	IdempotentReplay bool
}

type CancelIssueAssignmentInput struct {
	AssignmentID string
	Reason       string
}

func NewIssueAssignmentRepository(db *sql.DB) *IssueAssignmentRepository {
	return &IssueAssignmentRepository{db: db}
}

func (r *IssueAssignmentRepository) Create(input CreateIssueAssignmentInput) (*CreateIssueAssignmentResult, error) {
	input.IssueID = strings.TrimSpace(input.IssueID)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.RequestedBy = strings.TrimSpace(input.RequestedBy)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.ClientRequestID = strings.TrimSpace(input.ClientRequestID)

	if input.IssueID == "" || input.AgentID == "" || input.ClientRequestID == "" {
		return nil, fmt.Errorf("%w: issue_id, agent_id, and client_request_id are required", ErrAssignmentValidation)
	}
	if input.SourceType != "issue_detail" && input.SourceType != "comment" {
		return nil, fmt.Errorf("%w: source_type must be issue_detail or comment", ErrAssignmentValidation)
	}
	if input.SourceType == "comment" && input.SourceID == "" {
		return nil, fmt.Errorf("%w: source_id is required for comment assignments", ErrAssignmentValidation)
	}
	if input.SourceType == "issue_detail" {
		input.SourceID = ""
	}

	fingerprint := AssignmentRequestFingerprint(input.IssueID, input.AgentID, input.SourceType, input.SourceID)
	dedupeKey := fingerprint

	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin assignment tx: %w", err)
	}
	defer tx.Rollback()

	if err := requireIssueTx(tx, input.IssueID); err != nil {
		return nil, err
	}
	if input.SourceType == "comment" {
		if err := requireCommentSourceTx(tx, input.IssueID, input.SourceID); err != nil {
			return nil, err
		}
	}
	agentName, err := requireAssignableAgentTx(tx, input.AgentID)
	if err != nil {
		return nil, err
	}

	uid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate assignment uuid: %w", err)
	}
	now := time.Now().UTC()
	agentID := input.AgentID
	assignment := &IssueAssignment{
		ID:                 uid.String(),
		IssueID:            input.IssueID,
		AgentID:            &agentID,
		RequestedBy:        input.RequestedBy,
		SourceType:         input.SourceType,
		SourceID:           input.SourceID,
		ClientRequestID:    input.ClientRequestID,
		RequestFingerprint: fingerprint,
		DedupeKey:          dedupeKey,
		Status:             "queued",
		RequestedAt:        now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	_, err = tx.Exec(`
INSERT INTO issue_assignments (
    id, issue_id, agent_id, requested_by, source_type, source_id,
    client_request_id, request_fingerprint, dedupe_key, status,
    requested_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		assignment.ID,
		assignment.IssueID,
		input.AgentID,
		assignment.RequestedBy,
		assignment.SourceType,
		assignment.SourceID,
		assignment.ClientRequestID,
		assignment.RequestFingerprint,
		assignment.DedupeKey,
		assignment.Status,
		assignment.RequestedAt.Format(time.RFC3339),
		assignment.CreatedAt.Format(time.RFC3339),
		assignment.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueConstraint(err) {
			replay, err := replayAssignmentTx(tx, input.ClientRequestID, fingerprint, dedupeKey)
			if err != nil {
				return nil, err
			}
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit assignment replay tx: %w", err)
			}
			return &CreateIssueAssignmentResult{Assignment: replay, IdempotentReplay: true}, nil
		}
		return nil, fmt.Errorf("insert issue assignment: %w", err)
	}

	if err := createAssignmentActivityTx(tx, assignment, agentName, "assignment.requested", "Assignment requested", ""); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit assignment tx: %w", err)
	}

	return &CreateIssueAssignmentResult{Assignment: assignment, IdempotentReplay: false}, nil
}

func (r *IssueAssignmentRepository) ListByIssue(issueID string) ([]*IssueAssignment, error) {
	issueID = strings.TrimSpace(issueID)
	if issueID == "" {
		return nil, fmt.Errorf("%w: issue_id is required", ErrAssignmentValidation)
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin assignment list tx: %w", err)
	}
	defer tx.Rollback()

	if err := requireIssueTx(tx, issueID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(`
SELECT id, issue_id, agent_id, requested_by, source_type, source_id, client_request_id,
       request_fingerprint, dedupe_key, status, requested_at, accepted_at, completed_at,
       failed_at, cancelled_at, created_at, updated_at
FROM issue_assignments
WHERE issue_id = ?
ORDER BY created_at ASC, id ASC`, issueID)
	if err != nil {
		return nil, fmt.Errorf("list issue assignments: %w", err)
	}
	defer rows.Close()

	assignments, err := scanAssignments(rows)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit assignment list tx: %w", err)
	}
	return assignments, nil
}

func (r *IssueAssignmentRepository) Cancel(input CancelIssueAssignmentInput) (*IssueAssignment, error) {
	input.AssignmentID = strings.TrimSpace(input.AssignmentID)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.AssignmentID == "" {
		return nil, fmt.Errorf("%w: assignment id is required", ErrAssignmentValidation)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin assignment cancel tx: %w", err)
	}
	defer tx.Rollback()

	assignment, err := getAssignmentByIDTx(tx, input.AssignmentID)
	if err != nil {
		return nil, err
	}
	if assignment == nil {
		return nil, ErrAssignmentNotFound
	}
	if assignment.Status == "completed" || assignment.Status == "failed" || assignment.Status == "cancelled" {
		return nil, ErrAssignmentTerminal
	}

	now := time.Now().UTC()
	result, err := tx.Exec(
		`UPDATE issue_assignments
		 SET status = ?, cancelled_at = ?, updated_at = ?
		 WHERE id = ? AND status NOT IN ('completed', 'failed', 'cancelled')`,
		"cancelled", now.Format(time.RFC3339), now.Format(time.RFC3339), input.AssignmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("cancel assignment: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("cancel assignment rows affected: %w", err)
	}
	if affected == 0 {
		return nil, ErrAssignmentTerminal
	}
	assignment, err = getAssignmentByIDTx(tx, input.AssignmentID)
	if err != nil {
		return nil, err
	}
	agentName, err := agentNameForAssignmentTx(tx, assignment)
	if err != nil {
		return nil, err
	}
	if err := createAssignmentActivityTx(tx, assignment, agentName, "assignment.cancelled", "Assignment cancelled", input.Reason); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit assignment cancel tx: %w", err)
	}
	return assignment, nil
}

func AssignmentRequestFingerprint(issueID, agentID, sourceType, sourceID string) string {
	parts := []string{strings.TrimSpace(issueID), strings.TrimSpace(agentID), strings.TrimSpace(sourceType), strings.TrimSpace(sourceID)}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func requireIssueTx(tx *sql.Tx, issueID string) error {
	var id string
	err := tx.QueryRow("SELECT id FROM issues WHERE id = ?", issueID).Scan(&id)
	if err == sql.ErrNoRows {
		return ErrAssignmentIssueNotFound
	}
	if err != nil {
		return fmt.Errorf("verify assignment issue: %w", err)
	}
	return nil
}

func requireCommentSourceTx(tx *sql.Tx, issueID, commentID string) error {
	var id string
	err := tx.QueryRow("SELECT id FROM issue_comments WHERE id = ? AND issue_id = ?", commentID, issueID).Scan(&id)
	if err == sql.ErrNoRows {
		return fmt.Errorf("%w: comment source not found", ErrAssignmentValidation)
	}
	if err != nil {
		return fmt.Errorf("verify assignment comment source: %w", err)
	}
	return nil
}

func requireAssignableAgentTx(tx *sql.Tx, agentID string) (string, error) {
	var name, runtimeID string
	var isAssignable int
	var runtimeHealth sql.NullString
	err := tx.QueryRow(`
SELECT agents.name, agents.runtime_id, agents.is_assignable, runtimes.health_status
FROM agents
LEFT JOIN runtimes ON runtimes.id = agents.runtime_id
WHERE agents.id = ?`, agentID).Scan(&name, &runtimeID, &isAssignable, &runtimeHealth)
	if err == sql.ErrNoRows {
		return "", ErrAssignmentAgentNotFound
	}
	if err != nil {
		return "", fmt.Errorf("verify assignment agent: %w", err)
	}
	if isAssignable == 0 || strings.TrimSpace(runtimeID) == "" {
		return "", ErrAssignmentAgentNotReady
	}
	if !runtimeHealth.Valid || runtimeHealth.String != "healthy" {
		return "", ErrAssignmentRuntimeNotReady
	}
	return name, nil
}

func replayAssignmentTx(tx *sql.Tx, clientRequestID, fingerprint, dedupeKey string) (*IssueAssignment, error) {
	assignment, err := getAssignmentByClientRequestIDTx(tx, clientRequestID)
	if err != nil {
		return nil, err
	}
	if assignment != nil {
		if assignment.RequestFingerprint != fingerprint {
			return nil, ErrAssignmentIdempotencyReuse
		}
		return assignment, nil
	}
	assignment, err = getAssignmentByDedupeKeyTx(tx, dedupeKey)
	if err != nil {
		return nil, err
	}
	if assignment != nil {
		return assignment, nil
	}
	return nil, fmt.Errorf("assignment unique conflict without replay row")
}

func createAssignmentActivityTx(tx *sql.Tx, assignment *IssueAssignment, agentName, activityType, summary, reason string) error {
	uid, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate assignment activity uuid: %w", err)
	}
	agentID := ""
	if assignment.AgentID != nil {
		agentID = *assignment.AgentID
	}
	metadata, err := json.Marshal(map[string]string{
		"assignment_id": assignment.ID,
		"agent_id":      agentID,
		"agent_name":    agentName,
		"source_type":   assignment.SourceType,
		"source_id":     assignment.SourceID,
		"status":        assignment.Status,
		"reason":        reason,
	})
	if err != nil {
		return fmt.Errorf("marshal assignment activity metadata: %w", err)
	}
	_, err = tx.Exec(
		`INSERT INTO issue_activity (id, issue_id, actor_id, type, summary, metadata_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uid.String(),
		assignment.IssueID,
		assignment.RequestedBy,
		activityType,
		summary,
		string(metadata),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert assignment activity: %w", err)
	}
	return nil
}

func getAssignmentByIDTx(tx *sql.Tx, id string) (*IssueAssignment, error) {
	row := tx.QueryRow(assignmentSelectSQL()+" WHERE id = ?", id)
	return scanAssignmentRow(row)
}

func getAssignmentByClientRequestIDTx(tx *sql.Tx, clientRequestID string) (*IssueAssignment, error) {
	row := tx.QueryRow(assignmentSelectSQL()+" WHERE client_request_id = ?", clientRequestID)
	return scanAssignmentRow(row)
}

func getAssignmentByDedupeKeyTx(tx *sql.Tx, dedupeKey string) (*IssueAssignment, error) {
	row := tx.QueryRow(assignmentSelectSQL()+" WHERE dedupe_key = ?", dedupeKey)
	return scanAssignmentRow(row)
}

func agentNameForAssignmentTx(tx *sql.Tx, assignment *IssueAssignment) (string, error) {
	if assignment.AgentID == nil {
		return "", nil
	}
	var name string
	err := tx.QueryRow("SELECT name FROM agents WHERE id = ?", *assignment.AgentID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get assignment agent name: %w", err)
	}
	return name, nil
}

func assignmentSelectSQL() string {
	return `SELECT id, issue_id, agent_id, requested_by, source_type, source_id, client_request_id,
       request_fingerprint, dedupe_key, status, requested_at, accepted_at, completed_at,
       failed_at, cancelled_at, created_at, updated_at
FROM issue_assignments`
}

type assignmentScanner interface {
	Scan(dest ...any) error
}

func scanAssignmentRow(scanner assignmentScanner) (*IssueAssignment, error) {
	assignment := &IssueAssignment{}
	var agentID sql.NullString
	var requestedAt, acceptedAt, completedAt, failedAt, cancelledAt, createdAt, updatedAt sql.NullString
	err := scanner.Scan(
		&assignment.ID,
		&assignment.IssueID,
		&agentID,
		&assignment.RequestedBy,
		&assignment.SourceType,
		&assignment.SourceID,
		&assignment.ClientRequestID,
		&assignment.RequestFingerprint,
		&assignment.DedupeKey,
		&assignment.Status,
		&requestedAt,
		&acceptedAt,
		&completedAt,
		&failedAt,
		&cancelledAt,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan issue assignment: %w", err)
	}
	if agentID.Valid {
		value := agentID.String
		assignment.AgentID = &value
	}
	var errParse error
	assignment.RequestedAt, errParse = parseRequiredTime("requested_at", requestedAt)
	if errParse != nil {
		return nil, errParse
	}
	assignment.AcceptedAt, errParse = parseNullableTime("accepted_at", acceptedAt)
	if errParse != nil {
		return nil, errParse
	}
	assignment.CompletedAt, errParse = parseNullableTime("completed_at", completedAt)
	if errParse != nil {
		return nil, errParse
	}
	assignment.FailedAt, errParse = parseNullableTime("failed_at", failedAt)
	if errParse != nil {
		return nil, errParse
	}
	assignment.CancelledAt, errParse = parseNullableTime("cancelled_at", cancelledAt)
	if errParse != nil {
		return nil, errParse
	}
	assignment.CreatedAt, errParse = parseRequiredTime("created_at", createdAt)
	if errParse != nil {
		return nil, errParse
	}
	assignment.UpdatedAt, errParse = parseRequiredTime("updated_at", updatedAt)
	if errParse != nil {
		return nil, errParse
	}
	return assignment, nil
}

func scanAssignments(rows *sql.Rows) ([]*IssueAssignment, error) {
	var assignments []*IssueAssignment
	for rows.Next() {
		assignment, err := scanAssignmentRow(rows)
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

func parseRequiredTime(field string, value sql.NullString) (time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return time.Time{}, fmt.Errorf("scan assignment %s: missing time", field)
	}
	t, err := parseTime(value.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("scan assignment %s: %w", field, err)
	}
	return t, nil
}

func parseNullableTime(field string, value sql.NullString) (*time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}
	t, err := parseTime(value.String)
	if err != nil {
		return nil, fmt.Errorf("scan assignment %s: %w", field, err)
	}
	return &t, nil
}

func isUniqueConstraint(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}
