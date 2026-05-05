package handler

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
	"github.com/bzqzheng/zin/services/daemon/store"
)

type AssignmentHandler struct {
	db             *sql.DB
	issueRepo      *store.IssueRepository
	agentRepo      *store.AgentRepository
	commentRepo    *store.IssueCommentRepository
	assignmentRepo *store.IssueAssignmentRepository
}

func NewAssignmentHandler(
	db *sql.DB,
	issueRepo *store.IssueRepository,
	agentRepo *store.AgentRepository,
	commentRepo *store.IssueCommentRepository,
	assignmentRepo *store.IssueAssignmentRepository,
) *AssignmentHandler {
	return &AssignmentHandler{
		db:             db,
		issueRepo:      issueRepo,
		agentRepo:      agentRepo,
		commentRepo:    commentRepo,
		assignmentRepo: assignmentRepo,
	}
}

type assignmentRequest struct {
	AgentID         string `json:"agent_id"`
	SourceType      string `json:"source_type"`
	SourceID        string `json:"source_id"`
	ClientRequestID string `json:"client_request_id"`
}

type cancelAssignmentRequest struct {
	Reason string `json:"reason"`
}

type transitionAssignmentRequest struct {
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
}

type completeAssignmentRequest struct {
	FromStatus string `json:"from_status"`
	Status     string `json:"status"`
	Output     string `json:"output"`
	Error      string `json:"error"`
}

type assignmentResponse struct {
	Assignment       *store.IssueAssignment `json:"assignment"`
	IdempotentReplay bool                   `json:"idempotent_replay,omitempty"`
}

type assignmentResultResponse struct {
	Assignment *store.IssueAssignment  `json:"assignment"`
	Result     *store.AssignmentResult `json:"result"`
}

type assignmentListResponse struct {
	Assignments []*store.IssueAssignment `json:"assignments"`
}

func (h *AssignmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	issueID := r.PathValue("id")
	var req assignmentRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	req.AgentID = strings.TrimSpace(req.AgentID)
	req.SourceType = strings.TrimSpace(req.SourceType)
	req.SourceID = strings.TrimSpace(req.SourceID)
	req.ClientRequestID = strings.TrimSpace(req.ClientRequestID)
	if req.AgentID == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "agent_id is required", "")
		return
	}
	if req.ClientRequestID == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "client_request_id is required", "")
		return
	}
	if req.SourceType != "issue_detail" && req.SourceType != "comment" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "source_type must be issue_detail or comment", "")
		return
	}
	if req.SourceType == "comment" && req.SourceID == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "source_id is required for comment assignments", "")
		return
	}

	fingerprint := assignmentFingerprint(issueID, req.AgentID, req.SourceType, req.SourceID)
	dedupeKey := assignmentDedupeKey(issueID, req.AgentID, req.SourceType, req.SourceID)
	if existing, err := h.assignmentRepo.GetByClientRequestID(req.ClientRequestID); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to check assignment idempotency", err.Error())
		return
	} else if existing != nil {
		if existing.RequestFingerprint != fingerprint {
			response.Error(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE_CONFLICT", "client_request_id was reused with a different assignment payload", "")
			return
		}
		response.JSON(w, http.StatusOK, assignmentResponse{Assignment: existing, IdempotentReplay: true})
		return
	}
	if existing, err := h.assignmentRepo.GetByDedupeKey(dedupeKey); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to check assignment dedupe", err.Error())
		return
	} else if existing != nil {
		response.JSON(w, http.StatusOK, assignmentResponse{Assignment: existing, IdempotentReplay: true})
		return
	}

	issue, err := h.issueRepo.GetByID(issueID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get issue", err.Error())
		return
	}
	if issue == nil {
		response.Error(w, http.StatusNotFound, "ISSUE_NOT_FOUND", "issue not found", "")
		return
	}
	if req.SourceType == "comment" {
		comment, err := h.commentRepo.GetByID(req.SourceID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get comment", err.Error())
			return
		}
		if comment == nil || comment.IssueID != issueID {
			response.Error(w, http.StatusNotFound, "COMMENT_NOT_FOUND", "comment not found", "")
			return
		}
	}
	agent, err := h.agentRepo.GetByID(req.AgentID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get agent", err.Error())
		return
	}
	if agent == nil {
		response.Error(w, http.StatusNotFound, "AGENT_NOT_FOUND", "agent not found", "")
		return
	}
	if !agent.Assignable {
		if agent.RuntimeID != "" && agent.RuntimeStatus != "healthy" {
			response.Error(w, http.StatusUnprocessableEntity, "RUNTIME_NOT_HEALTHY", agent.AssignableReason, "")
			return
		}
		response.Error(w, http.StatusUnprocessableEntity, "AGENT_NOT_ASSIGNABLE", agent.AssignableReason, "")
		return
	}

	var assignment *store.IssueAssignment
	var replay bool
	if err := store.WithTx(h.db, func(repos store.Repositories) error {
		existing, err := repos.Assignments.GetByClientRequestID(req.ClientRequestID)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint {
				return errIdempotencyReuse
			}
			assignment = existing
			replay = true
			return nil
		}
		existing, err = repos.Assignments.GetByDedupeKey(dedupeKey)
		if err != nil {
			return err
		}
		if existing != nil {
			assignment = existing
			replay = true
			return nil
		}
		assignment, err = repos.Assignments.Create(store.CreateIssueAssignmentInput{
			IssueID:            issueID,
			AgentID:            req.AgentID,
			RequestedBy:        "local-user",
			SourceType:         req.SourceType,
			SourceID:           req.SourceID,
			ClientRequestID:    req.ClientRequestID,
			RequestFingerprint: fingerprint,
			DedupeKey:          dedupeKey,
		})
		if err != nil {
			return err
		}
		metadata, err := json.Marshal(map[string]string{
			"assignment_id": assignment.ID,
			"agent_id":      agent.ID,
			"agent_name":    agent.Name,
			"source_type":   req.SourceType,
			"source_id":     req.SourceID,
		})
		if err != nil {
			return fmt.Errorf("marshal assignment metadata: %w", err)
		}
		_, err = repos.Activity.Create(store.CreateIssueActivityInput{
			IssueID:      issueID,
			ActorID:      "local-user",
			Type:         "assignment.requested",
			Summary:      fmt.Sprintf("Queued assignment for %s", agent.Name),
			MetadataJSON: string(metadata),
		})
		return err
	}); err != nil {
		if errors.Is(err, errIdempotencyReuse) {
			response.Error(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE_CONFLICT", "client_request_id was reused with a different assignment payload", "")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to create assignment", err.Error())
		return
	}
	if replay {
		response.JSON(w, http.StatusOK, assignmentResponse{Assignment: assignment, IdempotentReplay: true})
		return
	}
	response.JSON(w, http.StatusCreated, assignmentResponse{Assignment: assignment})
}

func (h *AssignmentHandler) List(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	issueID := r.PathValue("id")
	issue, err := h.issueRepo.GetByID(issueID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get issue", err.Error())
		return
	}
	if issue == nil {
		response.Error(w, http.StatusNotFound, "ISSUE_NOT_FOUND", "issue not found", "")
		return
	}
	assignments, err := h.assignmentRepo.ListByIssue(issueID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to list assignments", err.Error())
		return
	}
	if assignments == nil {
		assignments = []*store.IssueAssignment{}
	}
	response.JSON(w, http.StatusOK, assignmentListResponse{Assignments: assignments})
}

func (h *AssignmentHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	id := r.PathValue("id")
	var req cancelAssignmentRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}

	var assignment *store.IssueAssignment
	if err := store.WithTx(h.db, func(repos store.Repositories) error {
		current, err := repos.Assignments.GetByID(id)
		if err != nil {
			return err
		}
		if current == nil {
			return errAssignmentNotFound
		}
		if store.AssignmentStatusTerminal(current.Status) {
			return errAssignmentTerminal
		}
		assignment, err = repos.Assignments.Cancel(id)
		if err != nil {
			return err
		}
		metadata, err := json.Marshal(map[string]string{
			"assignment_id": id,
			"agent_id":      current.AgentID,
			"agent_name":    current.AgentName,
			"reason":        strings.TrimSpace(req.Reason),
		})
		if err != nil {
			return fmt.Errorf("marshal assignment cancel metadata: %w", err)
		}
		_, err = repos.Activity.Create(store.CreateIssueActivityInput{
			IssueID:      current.IssueID,
			ActorID:      "local-user",
			Type:         "assignment.cancelled",
			Summary:      fmt.Sprintf("Cancelled assignment for %s", current.AgentName),
			MetadataJSON: string(metadata),
		})
		return err
	}); err != nil {
		switch {
		case errors.Is(err, errAssignmentNotFound):
			response.Error(w, http.StatusNotFound, "ASSIGNMENT_NOT_FOUND", "assignment not found", "")
		case errors.Is(err, errAssignmentTerminal):
			response.Error(w, http.StatusConflict, "ASSIGNMENT_TERMINAL", "assignment is already terminal", "")
		case errors.Is(err, store.ErrAssignmentStateConflict):
			response.Error(w, http.StatusConflict, "ASSIGNMENT_STATE_CONFLICT", "assignment state changed before cancellation could commit", "")
		case errors.Is(err, store.ErrInvalidAssignmentTransition):
			response.Error(w, http.StatusConflict, "ASSIGNMENT_TERMINAL", "assignment cannot be cancelled from its current state", "")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to cancel assignment", err.Error())
		}
		return
	}
	response.JSON(w, http.StatusOK, assignmentResponse{Assignment: assignment})
}

func (h *AssignmentHandler) Transition(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	id := r.PathValue("id")
	var req transitionAssignmentRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	req.FromStatus = strings.TrimSpace(req.FromStatus)
	req.ToStatus = strings.TrimSpace(req.ToStatus)
	assignment, err := h.assignmentRepo.Transition(id, req.FromStatus, req.ToStatus)
	if err != nil {
		writeAssignmentExecutionError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, assignmentResponse{Assignment: assignment})
}

func (h *AssignmentHandler) CompleteWithResult(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	id := r.PathValue("id")
	var req completeAssignmentRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	assignment, result, err := h.assignmentRepo.CompleteWithResult(store.CompleteAssignmentInput{
		AssignmentID: id,
		FromStatus:   strings.TrimSpace(req.FromStatus),
		Status:       strings.TrimSpace(req.Status),
		Output:       req.Output,
		Error:        req.Error,
	})
	if err != nil {
		writeAssignmentExecutionError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, assignmentResultResponse{Assignment: assignment, Result: result})
}

var (
	errAssignmentNotFound = errors.New("assignment not found")
	errAssignmentTerminal = errors.New("assignment terminal")
	errIdempotencyReuse   = errors.New("idempotency key reused")
)

func writeAssignmentExecutionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrAssignmentStateConflict):
		response.Error(w, http.StatusConflict, "ASSIGNMENT_STATE_CONFLICT", "assignment state changed before this transition could commit", "")
	case errors.Is(err, store.ErrInvalidAssignmentTransition):
		response.Error(w, http.StatusBadRequest, "INVALID_ASSIGNMENT_TRANSITION", "assignment transition is not valid", "")
	case errors.Is(err, store.ErrAssignmentResultStatus):
		response.Error(w, http.StatusBadRequest, "INVALID_ASSIGNMENT_RESULT_STATUS", "assignment result status must be succeeded or failed", "")
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to update assignment execution state", err.Error())
	}
}

func assignmentFingerprint(issueID, agentID, sourceType, sourceID string) string {
	return sha256Hex(issueID + "\x00" + agentID + "\x00" + sourceType + "\x00" + sourceID)
}

func assignmentDedupeKey(issueID, agentID, sourceType, sourceID string) string {
	return sha256Hex("assignment" + "\x00" + issueID + "\x00" + agentID + "\x00" + sourceType + "\x00" + sourceID)
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
