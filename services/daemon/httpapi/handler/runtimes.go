package handler

import (
	"net/http"

	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
	"github.com/bzqzheng/zin/services/daemon/store"
)

type RuntimeHandler struct {
	repo *store.RuntimeRepository
}

func NewRuntimeHandler(repo *store.RuntimeRepository) *RuntimeHandler {
	return &RuntimeHandler{repo: repo}
}

type createRuntimeRequest struct {
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Command       string `json:"command"`
	Path          string `json:"path"`
	Version       string `json:"version"`
	Status        string `json:"status"`
	StatusMessage string `json:"status_message"`
}

func (h *RuntimeHandler) List(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	runtimes, err := h.repo.List()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to list runtimes", err.Error())
		return
	}
	if runtimes == nil {
		runtimes = []*store.Runtime{}
	}
	response.JSON(w, http.StatusOK, runtimes)
}

func (h *RuntimeHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req createRuntimeRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.Name == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "name is required", "")
		return
	}
	if req.Kind == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "kind is required", "")
		return
	}
	if !validRuntimeStatus(req.Status) {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "status must be healthy, degraded, unhealthy, or unknown", "")
		return
	}
	runtime, err := h.repo.Create(req.Name, req.Kind, req.Command, req.Path, req.Version, req.Status, req.StatusMessage)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to create runtime", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, runtime)
}

func validRuntimeStatus(status string) bool {
	switch status {
	case "", "healthy", "degraded", "unhealthy", "unknown":
		return true
	default:
		return false
	}
}
