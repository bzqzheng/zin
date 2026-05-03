package handler

import (
	"errors"
	"net/http"

	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
	"github.com/bzqzheng/zin/services/daemon/store"
)

type AssignmentHandler struct {
	repo *store.IssueAssignmentRepository
}

func NewAssignmentHandler(repo *store.IssueAssignmentRepository) *AssignmentHandler {
	return &AssignmentHandler{repo: repo}
}

type createAssignmentRequest struct {
	AgentID         string `json:"agent_id"`
	RequestedBy     string `json:"requested_by"`
	SourceType      string `json:"source_type"`
	SourceID        string `json:"source_id"`
	ClientRequestID string `json:"client_request_id"`
}

type cancelAssignmentRequest struct {
	Reason string `json:"reason"`
}

type assignmentResponse struct {
	ID          string  `json:"id"`
	IssueID     string  `json:"issue_id"`
	AgentID     *string `json:"agent_id"`
	RequestedBy string  `json:"requested_by"`
	Status      string  `json:"status"`
	SourceType  string  `json:"source_type"`
	SourceID    string  `json:"source_id"`
	DedupeKey   string  `json:"dedupe_key"`
	RequestedAt any     `json:"requested_at"`
	AcceptedAt  any     `json:"accepted_at,omitempty"`
	CompletedAt any     `json:"completed_at,omitempty"`
	FailedAt    any     `json:"failed_at,omitempty"`
	CancelledAt any     `json:"cancelled_at,omitempty"`
	CreatedAt   any     `json:"created_at"`
	UpdatedAt   any     `json:"updated_at"`
}

type createAssignmentResponse struct {
	Assignment       assignmentResponse `json:"assignment"`
	IdempotentReplay bool               `json:"idempotent_replay"`
}

type listAssignmentsResponse struct {
	Assignments []assignmentResponse `json:"assignments"`
}

func (h *AssignmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	issueID := r.PathValue("id")
	var req createAssignmentRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	result, err := h.repo.Create(store.CreateIssueAssignmentInput{
		IssueID:         issueID,
		AgentID:         req.AgentID,
		RequestedBy:     req.RequestedBy,
		SourceType:      req.SourceType,
		SourceID:        req.SourceID,
		ClientRequestID: req.ClientRequestID,
	})
	if err != nil {
		writeAssignmentError(w, err)
		return
	}
	status := http.StatusCreated
	if result.IdempotentReplay {
		status = http.StatusOK
	}
	response.JSON(w, status, createAssignmentResponse{
		Assignment:       assignmentToResponse(result.Assignment),
		IdempotentReplay: result.IdempotentReplay,
	})
}

func (h *AssignmentHandler) ListByIssue(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	assignments, err := h.repo.ListByIssue(r.PathValue("id"))
	if err != nil {
		writeAssignmentError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, listAssignmentsResponse{Assignments: assignmentsToResponse(assignments)})
}

func (h *AssignmentHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req cancelAssignmentRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := response.DecodeJSON(r, &req); err != nil {
			response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
			return
		}
	}
	assignment, err := h.repo.Cancel(store.CancelIssueAssignmentInput{
		AssignmentID: r.PathValue("id"),
		Reason:       req.Reason,
	})
	if err != nil {
		writeAssignmentError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]assignmentResponse{"assignment": assignmentToResponse(assignment)})
}

func assignmentToResponse(assignment *store.IssueAssignment) assignmentResponse {
	if assignment == nil {
		return assignmentResponse{}
	}
	return assignmentResponse{
		ID:          assignment.ID,
		IssueID:     assignment.IssueID,
		AgentID:     assignment.AgentID,
		RequestedBy: assignment.RequestedBy,
		Status:      assignment.Status,
		SourceType:  assignment.SourceType,
		SourceID:    assignment.SourceID,
		DedupeKey:   assignment.DedupeKey,
		RequestedAt: assignment.RequestedAt,
		AcceptedAt:  assignment.AcceptedAt,
		CompletedAt: assignment.CompletedAt,
		FailedAt:    assignment.FailedAt,
		CancelledAt: assignment.CancelledAt,
		CreatedAt:   assignment.CreatedAt,
		UpdatedAt:   assignment.UpdatedAt,
	}
}

func assignmentsToResponse(assignments []*store.IssueAssignment) []assignmentResponse {
	if assignments == nil {
		return []assignmentResponse{}
	}
	result := make([]assignmentResponse, 0, len(assignments))
	for _, assignment := range assignments {
		result = append(result, assignmentToResponse(assignment))
	}
	return result
}

func writeAssignmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrAssignmentValidation):
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), "")
	case errors.Is(err, store.ErrAssignmentIssueNotFound):
		response.Error(w, http.StatusNotFound, "ISSUE_NOT_FOUND", "issue not found", "")
	case errors.Is(err, store.ErrAssignmentAgentNotFound):
		response.Error(w, http.StatusNotFound, "AGENT_NOT_FOUND", "agent not found", "")
	case errors.Is(err, store.ErrAssignmentAgentNotReady):
		response.Error(w, http.StatusUnprocessableEntity, "AGENT_NOT_ASSIGNABLE", "agent is not assignable", "")
	case errors.Is(err, store.ErrAssignmentRuntimeNotReady):
		response.Error(w, http.StatusUnprocessableEntity, "RUNTIME_NOT_HEALTHY", "agent runtime is not healthy", "")
	case errors.Is(err, store.ErrAssignmentIdempotencyReuse):
		response.Error(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE_CONFLICT", "client_request_id was reused with a different request", "")
	case errors.Is(err, store.ErrAssignmentNotFound):
		response.Error(w, http.StatusNotFound, "ASSIGNMENT_NOT_FOUND", "assignment not found", "")
	case errors.Is(err, store.ErrAssignmentTerminal):
		response.Error(w, http.StatusConflict, "ASSIGNMENT_TERMINAL", "assignment is already terminal", "")
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "assignment request failed", err.Error())
	}
}
