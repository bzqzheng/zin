package handler

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
	"github.com/bzqzheng/zin/services/daemon/store"
)

type IssueHandler struct {
	db          *sql.DB
	repo        *store.IssueRepository
	projectRepo *store.ProjectRepository
}

func NewIssueHandler(db *sql.DB, repo *store.IssueRepository, projectRepo *store.ProjectRepository) *IssueHandler {
	return &IssueHandler{db: db, repo: repo, projectRepo: projectRepo}
}

type createIssueRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
}

func (h *IssueHandler) List(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	projectID := r.PathValue("pid")
	issues, err := h.repo.ListByProject(projectID, store.IssueFilters{
		Status:   r.URL.Query().Get("status"),
		Priority: r.URL.Query().Get("priority"),
	})
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
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	projectID := r.PathValue("pid")

	project, err := h.projectRepo.GetByID(projectID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to verify project", err.Error())
		return
	}
	if project == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "project not found", "")
		return
	}

	var req createIssueRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.Title == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "title is required", "")
		return
	}
	if req.Status != "" && !validIssueStatus(req.Status) {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid status", "")
		return
	}
	if req.Priority != "" && !validIssuePriority(req.Priority) {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid priority", "")
		return
	}

	var issue *store.Issue
	err = store.WithTx(h.db, func(repos store.Repositories) error {
		var err error
		issue, err = repos.Issues.Create(projectID, req.Title, req.Description, req.Status, req.Priority)
		if err != nil {
			return err
		}
		_, err = repos.Activity.Create(store.CreateIssueActivityInput{
			IssueID:      issue.ID,
			Type:         "issue.created",
			Summary:      "Created work item",
			MetadataJSON: `{"issue_id":"` + issue.ID + `"}`,
		})
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid project reference", "")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to create issue", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, issue)
}

func (h *IssueHandler) Get(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
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
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
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
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.Status != nil {
		if !validIssueStatus(*req.Status) {
			response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid status", "")
			return
		}
	}
	if req.Priority != nil {
		if !validIssuePriority(*req.Priority) {
			response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid priority", "")
			return
		}
	}

	before := *issue
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

	if err := store.WithTx(h.db, func(repos store.Repositories) error {
		if err := repos.Issues.Update(issue); err != nil {
			return err
		}
		inputs, err := store.ActivityInputsForIssueChanges(issue.ID, "", store.MeaningfulIssueChanges(&before, issue))
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
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to update issue", err.Error())
		return
	}
	updated, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get updated issue", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, updated)
}

func (h *IssueHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
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
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.Status == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "status is required", "")
		return
	}
	if !validIssueStatus(req.Status) {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid status", "")
		return
	}

	before := *issue
	issue.Status = req.Status
	if err := store.WithTx(h.db, func(repos store.Repositories) error {
		if err := repos.Issues.UpdateStatus(id, req.Status); err != nil {
			return err
		}
		inputs, err := store.ActivityInputsForIssueChanges(issue.ID, "", store.MeaningfulIssueChanges(&before, issue))
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
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to update issue status", err.Error())
		return
	}
	updated, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get updated issue", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, updated)
}

func (h *IssueHandler) Delete(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
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

func validIssueStatus(status string) bool {
	switch status {
	case "todo", "in_progress", "done", "blocked":
		return true
	default:
		return false
	}
}

func validIssuePriority(priority string) bool {
	switch priority {
	case "low", "medium", "high":
		return true
	default:
		return false
	}
}
