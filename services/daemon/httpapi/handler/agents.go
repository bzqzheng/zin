package handler

import (
	"net/http"
	"strings"

	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
	"github.com/bzqzheng/zin/services/daemon/store"
)

type AgentHandler struct {
	repo        *store.AgentRepository
	runtimeRepo *store.RuntimeRepository
}

func NewAgentHandler(repo *store.AgentRepository, runtimeRepo *store.RuntimeRepository) *AgentHandler {
	return &AgentHandler{repo: repo, runtimeRepo: runtimeRepo}
}

type createAgentRequest struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	RuntimeID    string `json:"runtime_id"`
	ModelHint    string `json:"model_hint"`
	Instructions string `json:"instructions"`
	IsAssignable bool   `json:"is_assignable"`
}

func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var filters store.AgentFilters
	if raw := r.URL.Query().Get("assignable"); raw != "" {
		switch raw {
		case "true":
			value := true
			filters.Assignable = &value
		case "false":
			value := false
			filters.Assignable = &value
		default:
			response.Error(w, http.StatusBadRequest, "INVALID_QUERY_PARAM", "assignable must be true or false", "")
			return
		}
	}
	agents, err := h.repo.ListWithFilters(filters)
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
	if !h.validateAgentRuntime(w, req.RuntimeID, req.IsAssignable) {
		return
	}
	agent, err := h.repo.CreateWithInput(store.CreateAgentInput{
		Name:         req.Name,
		Role:         req.Role,
		RuntimeID:    strings.TrimSpace(req.RuntimeID),
		ModelHint:    req.ModelHint,
		Instructions: req.Instructions,
		IsAssignable: req.IsAssignable,
	})
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
	Name         *string `json:"name"`
	Role         *string `json:"role"`
	Status       *string `json:"status"`
	RuntimeID    *string `json:"runtime_id"`
	ModelHint    *string `json:"model_hint"`
	Instructions *string `json:"instructions"`
	IsAssignable *bool   `json:"is_assignable"`
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
	if req.RuntimeID != nil {
		agent.RuntimeID = strings.TrimSpace(*req.RuntimeID)
	}
	if req.ModelHint != nil {
		agent.ModelHint = *req.ModelHint
	}
	if req.Instructions != nil {
		agent.Instructions = *req.Instructions
	}
	if req.IsAssignable != nil {
		agent.IsAssignable = *req.IsAssignable
	}
	if !h.validateAgentRuntime(w, agent.RuntimeID, agent.IsAssignable) {
		return
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

func (h *AgentHandler) validateAgentRuntime(w http.ResponseWriter, runtimeID string, isAssignable bool) bool {
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		if isAssignable {
			response.Error(w, http.StatusUnprocessableEntity, "RUNTIME_NOT_HEALTHY", "assignable agents require a healthy runtime", "")
			return false
		}
		return true
	}
	runtime, err := h.runtimeRepo.GetByID(runtimeID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get runtime", err.Error())
		return false
	}
	if runtime == nil {
		response.Error(w, http.StatusNotFound, "RUNTIME_NOT_FOUND", "runtime not found", "")
		return false
	}
	if isAssignable && runtime.HealthStatus != "healthy" {
		response.Error(w, http.StatusUnprocessableEntity, "RUNTIME_NOT_HEALTHY", "assignable agents require a healthy runtime", "")
		return false
	}
	return true
}
