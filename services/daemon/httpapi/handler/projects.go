package handler

import (
	"net/http"

	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
	"github.com/bzqzheng/zin/services/daemon/store"
)

type ProjectHandler struct {
	repo *store.ProjectRepository
}

func NewProjectHandler(repo *store.ProjectRepository) *ProjectHandler {
	return &ProjectHandler{repo: repo}
}

type createProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.repo.List()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to list projects", err.Error())
		return
	}
	if projects == nil {
		projects = []*store.Project{}
	}
	response.JSON(w, http.StatusOK, projects)
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := 	response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.Name == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "name is required", "")
		return
	}
	project, err := h.repo.Create(req.Name, req.Description)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to create project", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, project)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get project", err.Error())
		return
	}
	if project == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "project not found", "")
		return
	}
	response.JSON(w, http.StatusOK, project)
}

type updateProjectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get project", err.Error())
		return
	}
	if project == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "project not found", "")
		return
	}

	var req updateProjectRequest
	if err := 	response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = *req.Description
	}

	if err := h.repo.Update(project); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to update project", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, project)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get project", err.Error())
		return
	}
	if project == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "project not found", "")
		return
	}
	if err := h.repo.Delete(id); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to delete project", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
