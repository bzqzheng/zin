package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
	"github.com/bzqzheng/zin/services/daemon/store"
)

var (
	errInteractionBadRequest = errors.New("bad request")
	errInteractionConflict   = errors.New("conflict")
	errInteractionNotFound   = errors.New("not found")
)

type InteractionHandler struct {
	db           *sql.DB
	projectRepo  *store.ProjectRepository
	issueRepo    *store.IssueRepository
	tagRepo      *store.TagRepository
	commentRepo  *store.IssueCommentRepository
	activityRepo *store.IssueActivityRepository
}

func NewInteractionHandler(
	db *sql.DB,
	projectRepo *store.ProjectRepository,
	issueRepo *store.IssueRepository,
	tagRepo *store.TagRepository,
	commentRepo *store.IssueCommentRepository,
	activityRepo *store.IssueActivityRepository,
) *InteractionHandler {
	return &InteractionHandler{
		db:           db,
		projectRepo:  projectRepo,
		issueRepo:    issueRepo,
		tagRepo:      tagRepo,
		commentRepo:  commentRepo,
		activityRepo: activityRepo,
	}
}

type tagRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (h *InteractionHandler) ListProjectTags(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	projectID := r.PathValue("pid")
	if err := h.requireProject(projectID); err != nil {
		writeInteractionError(w, err, "failed to verify project")
		return
	}
	tags, err := h.tagRepo.ListByProject(projectID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to list tags", err.Error())
		return
	}
	if tags == nil {
		tags = []*store.Tag{}
	}
	response.JSON(w, http.StatusOK, tags)
}

func (h *InteractionHandler) CreateProjectTag(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	projectID := r.PathValue("pid")
	if err := h.requireProject(projectID); err != nil {
		writeInteractionError(w, err, "failed to verify project")
		return
	}
	var req tagRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "tag name is required", "")
		return
	}
	tag, err := h.tagRepo.Create(projectID, req.Name, req.Color)
	if err != nil {
		writeInteractionError(w, mapStoreError(err), "failed to create tag")
		return
	}
	response.JSON(w, http.StatusCreated, tag)
}

func (h *InteractionHandler) UpdateTag(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	id := r.PathValue("id")
	tag, err := h.tagRepo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get tag", err.Error())
		return
	}
	if tag == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "tag not found", "")
		return
	}
	var req tagRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "tag name is required", "")
		return
	}
	tag.Name = req.Name
	tag.Color = req.Color
	if err := h.tagRepo.Update(tag); err != nil {
		writeInteractionError(w, mapStoreError(err), "failed to update tag")
		return
	}
	updated, err := h.tagRepo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get updated tag", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, updated)
}

func (h *InteractionHandler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	id := r.PathValue("id")
	if err := store.WithTx(h.db, func(repos store.Repositories) error {
		tag, err := repos.Tags.GetByID(id)
		if err != nil {
			return err
		}
		if tag == nil {
			return fmt.Errorf("%w: tag not found", errInteractionNotFound)
		}
		issueIDs, err := repos.Tags.AttachedIssueIDs(id)
		if err != nil {
			return err
		}
		for _, issueID := range issueIDs {
			metadata, err := json.Marshal(map[string]string{"tag_id": tag.ID, "tag_name": tag.Name})
			if err != nil {
				return fmt.Errorf("marshal tag removal metadata: %w", err)
			}
			if _, err := repos.Activity.Create(store.CreateIssueActivityInput{
				IssueID:      issueID,
				Type:         "tag.removed",
				Summary:      fmt.Sprintf("Removed tag %s", tag.Name),
				MetadataJSON: string(metadata),
			}); err != nil {
				return err
			}
		}
		return repos.Tags.Delete(id)
	}); err != nil {
		writeInteractionError(w, err, "failed to delete tag")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *InteractionHandler) ListIssueTags(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	issueID := r.PathValue("id")
	if err := h.requireIssue(issueID); err != nil {
		writeInteractionError(w, err, "failed to verify issue")
		return
	}
	tags, err := h.tagRepo.ListByIssue(issueID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to list issue tags", err.Error())
		return
	}
	if tags == nil {
		tags = []*store.Tag{}
	}
	response.JSON(w, http.StatusOK, tags)
}

type attachTagRequest struct {
	TagID string `json:"tag_id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (h *InteractionHandler) AttachIssueTag(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	issueID := r.PathValue("id")
	var req attachTagRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}

	var tags []*store.Tag
	if err := store.WithTx(h.db, func(repos store.Repositories) error {
		issue, err := repos.Issues.GetByID(issueID)
		if err != nil {
			return err
		}
		if issue == nil {
			return fmt.Errorf("%w: issue not found", errInteractionNotFound)
		}

		var tag *store.Tag
		if req.TagID != "" {
			tag, err = repos.Tags.GetByID(req.TagID)
			if err != nil {
				return err
			}
			if tag == nil || tag.ProjectID != issue.ProjectID {
				return fmt.Errorf("%w: tag not found", errInteractionNotFound)
			}
		} else {
			name := strings.TrimSpace(req.Name)
			if name == "" {
				return fmt.Errorf("%w: tag_id or tag name is required", errInteractionBadRequest)
			}
			tag, err = repos.Tags.GetByProjectName(issue.ProjectID, name)
			if err != nil {
				return err
			}
			if tag == nil {
				tag, err = repos.Tags.Create(issue.ProjectID, name, req.Color)
				if err != nil {
					return err
				}
			}
		}

		attached, err := repos.Tags.IsAttachedToIssue(issueID, tag.ID)
		if err != nil {
			return err
		}
		if err := repos.Tags.AttachToIssue(issueID, tag.ID); err != nil {
			return err
		}
		if !attached {
			metadata, err := json.Marshal(map[string]string{"tag_id": tag.ID, "tag_name": tag.Name})
			if err != nil {
				return fmt.Errorf("marshal tag add metadata: %w", err)
			}
			if _, err := repos.Activity.Create(store.CreateIssueActivityInput{
				IssueID:      issueID,
				Type:         "tag.added",
				Summary:      fmt.Sprintf("Added tag %s", tag.Name),
				MetadataJSON: string(metadata),
			}); err != nil {
				return err
			}
		}
		tags, err = repos.Tags.ListByIssue(issueID)
		return err
	}); err != nil {
		writeInteractionError(w, mapStoreError(err), "failed to attach tag")
		return
	}
	if tags == nil {
		tags = []*store.Tag{}
	}
	response.JSON(w, http.StatusOK, tags)
}

func (h *InteractionHandler) DetachIssueTag(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	issueID := r.PathValue("id")
	tagID := r.PathValue("tag_id")
	if err := store.WithTx(h.db, func(repos store.Repositories) error {
		issue, err := repos.Issues.GetByID(issueID)
		if err != nil {
			return err
		}
		if issue == nil {
			return fmt.Errorf("%w: issue not found", errInteractionNotFound)
		}
		tag, err := repos.Tags.GetByID(tagID)
		if err != nil {
			return err
		}
		if tag == nil || tag.ProjectID != issue.ProjectID {
			return fmt.Errorf("%w: tag attachment not found", errInteractionNotFound)
		}
		attached, err := repos.Tags.IsAttachedToIssue(issueID, tagID)
		if err != nil {
			return err
		}
		if !attached {
			return fmt.Errorf("%w: tag attachment not found", errInteractionNotFound)
		}
		if err := repos.Tags.DetachFromIssue(issueID, tagID); err != nil {
			return err
		}
		metadata, err := json.Marshal(map[string]string{"tag_id": tag.ID, "tag_name": tag.Name})
		if err != nil {
			return fmt.Errorf("marshal tag removal metadata: %w", err)
		}
		_, err = repos.Activity.Create(store.CreateIssueActivityInput{
			IssueID:      issueID,
			Type:         "tag.removed",
			Summary:      fmt.Sprintf("Removed tag %s", tag.Name),
			MetadataJSON: string(metadata),
		})
		return err
	}); err != nil {
		writeInteractionError(w, err, "failed to detach tag")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *InteractionHandler) ListComments(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	issueID := r.PathValue("id")
	if err := h.requireIssue(issueID); err != nil {
		writeInteractionError(w, err, "failed to verify issue")
		return
	}
	comments, err := h.commentRepo.ListByIssue(issueID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to list comments", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, commentResponses(comments))
}

type commentRequest struct {
	Body       string `json:"body"`
	AuthorID   string `json:"author_id"`
	AuthorName string `json:"author_name"`
}

type commentResponse struct {
	ID         string `json:"id"`
	IssueID    string `json:"issue_id"`
	AuthorID   string `json:"author_id"`
	AuthorName string `json:"author_name"`
	Body       string `json:"body"`
	CreatedAt  any    `json:"created_at"`
	UpdatedAt  any    `json:"updated_at"`
}

func (h *InteractionHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	issueID := r.PathValue("id")
	var req commentRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "comment body is required", "")
		return
	}

	var comment *store.IssueComment
	if err := store.WithTx(h.db, func(repos store.Repositories) error {
		issue, err := repos.Issues.GetByID(issueID)
		if err != nil {
			return err
		}
		if issue == nil {
			return fmt.Errorf("%w: issue not found", errInteractionNotFound)
		}
		comment, err = repos.Comments.Create(issueID, req.AuthorID, body)
		if err != nil {
			return err
		}
		metadata, err := json.Marshal(map[string]string{"comment_id": comment.ID})
		if err != nil {
			return fmt.Errorf("marshal comment add metadata: %w", err)
		}
		_, err = repos.Activity.Create(store.CreateIssueActivityInput{
			IssueID:      issueID,
			ActorID:      req.AuthorID,
			Type:         "comment.added",
			Summary:      "Comment added",
			MetadataJSON: string(metadata),
		})
		return err
	}); err != nil {
		writeInteractionError(w, err, "failed to create comment")
		return
	}
	response.JSON(w, http.StatusCreated, commentToResponse(comment))
}

func (h *InteractionHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	id := r.PathValue("id")
	var req commentRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "comment body is required", "")
		return
	}

	var comment *store.IssueComment
	if err := store.WithTx(h.db, func(repos store.Repositories) error {
		var err error
		comment, err = repos.Comments.GetByID(id)
		if err != nil {
			return err
		}
		if comment == nil {
			return fmt.Errorf("%w: comment not found", errInteractionNotFound)
		}
		if strings.TrimSpace(comment.Body) != body {
			comment.Body = body
			if err := repos.Comments.Update(comment); err != nil {
				return err
			}
			metadata, err := json.Marshal(map[string]string{"comment_id": comment.ID})
			if err != nil {
				return fmt.Errorf("marshal comment update metadata: %w", err)
			}
			if _, err := repos.Activity.Create(store.CreateIssueActivityInput{
				IssueID:      comment.IssueID,
				ActorID:      comment.AuthorID,
				Type:         "comment.updated",
				Summary:      "Comment updated",
				MetadataJSON: string(metadata),
			}); err != nil {
				return err
			}
		}
		comment, err = repos.Comments.GetByID(id)
		return err
	}); err != nil {
		writeInteractionError(w, err, "failed to update comment")
		return
	}
	response.JSON(w, http.StatusOK, commentToResponse(comment))
}

func (h *InteractionHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	id := r.PathValue("id")
	if err := store.WithTx(h.db, func(repos store.Repositories) error {
		comment, err := repos.Comments.GetByID(id)
		if err != nil {
			return err
		}
		if comment == nil {
			return fmt.Errorf("%w: comment not found", errInteractionNotFound)
		}
		if err := repos.Comments.Delete(id); err != nil {
			return err
		}
		metadata, err := json.Marshal(map[string]string{"comment_id": comment.ID})
		if err != nil {
			return fmt.Errorf("marshal comment delete metadata: %w", err)
		}
		_, err = repos.Activity.Create(store.CreateIssueActivityInput{
			IssueID:      comment.IssueID,
			ActorID:      comment.AuthorID,
			Type:         "comment.deleted",
			Summary:      "Comment deleted",
			MetadataJSON: string(metadata),
		})
		return err
	}); err != nil {
		writeInteractionError(w, err, "failed to delete comment")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type activityResponse struct {
	ID        string                 `json:"id"`
	IssueID   string                 `json:"issue_id"`
	ActorID   string                 `json:"actor_id"`
	Type      string                 `json:"type"`
	Summary   string                 `json:"summary"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt any                    `json:"created_at"`
}

func (h *InteractionHandler) ListActivity(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	issueID := r.PathValue("id")
	if err := h.requireIssue(issueID); err != nil {
		writeInteractionError(w, err, "failed to verify issue")
		return
	}
	activities, err := h.activityRepo.ListByIssue(issueID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to list activity", err.Error())
		return
	}
	result := make([]activityResponse, 0, len(activities))
	for _, activity := range activities {
		metadata := map[string]interface{}{}
		if strings.TrimSpace(activity.MetadataJSON) != "" {
			if err := json.Unmarshal([]byte(activity.MetadataJSON), &metadata); err != nil {
				response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to decode activity metadata", err.Error())
				return
			}
			if metadata == nil {
				metadata = map[string]interface{}{}
			}
		}
		result = append(result, activityResponse{
			ID:        activity.ID,
			IssueID:   activity.IssueID,
			ActorID:   activity.ActorID,
			Type:      activity.Type,
			Summary:   activity.Summary,
			Metadata:  metadata,
			CreatedAt: activity.CreatedAt,
		})
	}
	response.JSON(w, http.StatusOK, result)
}

func (h *InteractionHandler) requireProject(projectID string) error {
	project, err := h.projectRepo.GetByID(projectID)
	if err != nil {
		return err
	}
	if project == nil {
		return fmt.Errorf("%w: project not found", errInteractionNotFound)
	}
	return nil
}

func (h *InteractionHandler) requireIssue(issueID string) error {
	issue, err := h.issueRepo.GetByID(issueID)
	if err != nil {
		return err
	}
	if issue == nil {
		return fmt.Errorf("%w: issue not found", errInteractionNotFound)
	}
	return nil
}

func commentResponses(comments []*store.IssueComment) []commentResponse {
	if comments == nil {
		return []commentResponse{}
	}
	result := make([]commentResponse, 0, len(comments))
	for _, comment := range comments {
		result = append(result, commentToResponse(comment))
	}
	return result
}

func commentToResponse(comment *store.IssueComment) commentResponse {
	return commentResponse{
		ID:         comment.ID,
		IssueID:    comment.IssueID,
		AuthorID:   comment.AuthorID,
		AuthorName: "You",
		Body:       comment.Body,
		CreatedAt:  comment.CreatedAt,
		UpdatedAt:  comment.UpdatedAt,
	}
}

func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "unique"):
		return fmt.Errorf("%w: %v", errInteractionConflict, err)
	case strings.Contains(lower, "same project"):
		return fmt.Errorf("%w: tag not found", errInteractionNotFound)
	default:
		return err
	}
}

func writeInteractionError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, errInteractionBadRequest):
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "")
	case errors.Is(err, errInteractionNotFound):
		response.Error(w, http.StatusNotFound, "NOT_FOUND", err.Error(), "")
	case errors.Is(err, errInteractionConflict):
		response.Error(w, http.StatusConflict, "CONFLICT", err.Error(), "")
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL", fallback, err.Error())
	}
}
