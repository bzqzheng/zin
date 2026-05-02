package handler

import (
	"net/http"

	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
	"github.com/bzqzheng/zin/services/daemon/store"
)

type IssueHandler struct {
	repo *store.IssueRepository
}

func NewIssueHandler(repo *store.IssueRepository) *IssueHandler {
	return &IssueHandler{repo: repo}
}

type createIssueRequest struct {
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
}

func (h *IssueHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("pid")
	issues, err := h.repo.ListByProject(projectID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to list issues", err.Error())
		return
	}
	if issues == nil {
		issues = []*store.Issue{}
	}
	response.JSON(w, http.StatusOK, issues)
}

func (h *IssueHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("pid")

	var req createIssueRequest
	if err := 	response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.Title == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "title is required", "")
		return
	}

	issue, err := h.repo.Create(projectID, req.Identifier, req.Title, req.Description, req.Status, req.Priority)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to create issue", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, issue)
}

func (h *IssueHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get issue", err.Error())
		return
	}
	if issue == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "issue not found", "")
		return
	}
	response.JSON(w, http.StatusOK, issue)
}

type updateIssueRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	CreatorID   *string `json:"creator_id"`
}

func (h *IssueHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get issue", err.Error())
		return
	}
	if issue == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "issue not found", "")
		return
	}

	var req updateIssueRequest
	if err := 	response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.Title != nil {
		issue.Title = *req.Title
	}
	if req.Description != nil {
		issue.Description = *req.Description
	}
	if req.Status != nil {
		issue.Status = *req.Status
	}
	if req.Priority != nil {
		issue.Priority = *req.Priority
	}
	if req.AssigneeID != nil {
		issue.AssigneeID = *req.AssigneeID
	}
	if req.CreatorID != nil {
		issue.CreatorID = *req.CreatorID
	}

	if err := h.repo.Update(issue); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to update issue", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, issue)
}

func (h *IssueHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get issue", err.Error())
		return
	}
	if issue == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "issue not found", "")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := 	response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.Status == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "status is required", "")
		return
	}

	if err := h.repo.UpdateStatus(id, req.Status); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to update issue status", err.Error())
		return
	}
	issue.Status = req.Status
	response.JSON(w, http.StatusOK, issue)
}

func (h *IssueHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get issue", err.Error())
		return
	}
	if issue == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "issue not found", "")
		return
	}
	if err := h.repo.Delete(id); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to delete issue", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
