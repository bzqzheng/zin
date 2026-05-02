package handler

import (
	"net/http"
	"strings"

	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
	"github.com/bzqzheng/zin/services/daemon/store"
)

type AgentHandler struct {
	repo *store.AgentRepository
}

func NewAgentHandler(repo *store.AgentRepository) *AgentHandler {
	return &AgentHandler{repo: repo}
}

type createAgentRequest struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	agents, err := h.repo.List()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to list agents", err.Error())
		return
	}
	if agents == nil {
		agents = []*store.Agent{}
	}
	response.JSON(w, http.StatusOK, agents)
}

func (h *AgentHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req createAgentRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.Name == "" {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "name is required", "")
		return
	}
	agent, err := h.repo.Create(req.Name, req.Role)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to create agent", err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, agent)
}

func (h *AgentHandler) Get(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	id := r.PathValue("id")
	agent, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get agent", err.Error())
		return
	}
	if agent == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "agent not found", "")
		return
	}
	response.JSON(w, http.StatusOK, agent)
}

type updateAgentRequest struct {
	Name   *string `json:"name"`
	Role   *string `json:"role"`
	Status *string `json:"status"`
}

func (h *AgentHandler) Update(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	id := r.PathValue("id")
	agent, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get agent", err.Error())
		return
	}
	if agent == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "agent not found", "")
		return
	}

	var req updateAgentRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.Name != nil {
		agent.Name = *req.Name
	}
	if req.Role != nil {
		agent.Role = *req.Role
	}
	if req.Status != nil {
		agent.Status = *req.Status
	}

	if err := h.repo.Update(agent); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to update agent", err.Error())
		return
	}
	response.JSON(w, http.StatusOK, agent)
}

func (h *AgentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	id := r.PathValue("id")
	agent, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get agent", err.Error())
		return
	}
	if agent == nil {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "agent not found", "")
		return
	}
	if err := h.repo.Delete(id); err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			response.Error(w, http.StatusConflict, "CONFLICT", "cannot delete agent with active references", "")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to delete agent", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
