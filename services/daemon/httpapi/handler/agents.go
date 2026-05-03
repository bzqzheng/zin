package handler

import (
	"fmt"
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
	Model        string `json:"model"`
	Instructions string `json:"instructions"`
}

func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	assignableOnly := r.URL.Query().Get("assignable") == "true"
	agents, err := h.repo.List(assignableOnly)
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
	if err := h.validateRuntime(req.RuntimeID); err != nil {
		response.Error(w, http.StatusBadRequest, "RUNTIME_NOT_FOUND", "runtime_id does not reference a known runtime", err.Error())
		return
	}
	agent, err := h.repo.Create(req.Name, req.Role, req.RuntimeID, req.Model, req.Instructions)
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
	Model        *string `json:"model"`
	Instructions *string `json:"instructions"`
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
		if err := h.validateRuntime(*req.RuntimeID); err != nil {
			response.Error(w, http.StatusBadRequest, "RUNTIME_NOT_FOUND", "runtime_id does not reference a known runtime", err.Error())
			return
		}
		agent.RuntimeID = *req.RuntimeID
	}
	if req.Model != nil {
		agent.Model = *req.Model
	}
	if req.Instructions != nil {
		agent.Instructions = *req.Instructions
	}

	if err := h.repo.Update(agent); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to update agent", err.Error())
		return
	}
	agent, err = h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL", "failed to get agent", err.Error())
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

func (h *AgentHandler) validateRuntime(runtimeID string) error {
	if runtimeID == "" {
		return nil
	}
	runtime, err := h.runtimeRepo.GetByID(runtimeID)
	if err != nil {
		return err
	}
	if runtime == nil {
		return fmt.Errorf("runtime %q not found", runtimeID)
	}
	return nil
}
