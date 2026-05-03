package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
	runtimeprobe "github.com/bzqzheng/zin/services/daemon/runtime"
	"github.com/bzqzheng/zin/services/daemon/store"
)

type RuntimeHandler struct {
	repo *store.RuntimeRepository
}

func NewRuntimeHandler(repo *store.RuntimeRepository) *RuntimeHandler {
	return &RuntimeHandler{repo: repo}
}

type runtimesResponse struct {
	Runtimes []*store.Runtime `json:"runtimes"`
}

type discoverRuntimesRequest struct {
	PathOverrides map[string]string `json:"path_overrides"`
}

type runtimeSummary struct {
	Healthy  int `json:"healthy"`
	Degraded int `json:"degraded"`
	Missing  int `json:"missing"`
}

type discoverRuntimesResponse struct {
	Runtimes []*store.Runtime `json:"runtimes"`
	Summary  runtimeSummary   `json:"summary"`
}

type validateRuntimeRequest struct {
	RuntimeID string `json:"runtime_id"`
}

type runtimeResponse struct {
	Runtime *store.Runtime `json:"runtime"`
}

type updateRuntimeRequest struct {
	DisplayName *string `json:"display_name"`
	BinaryPath  *string `json:"binary_path"`
}

func (h *RuntimeHandler) List(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	runtimes, err := h.repo.List()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list runtimes", err.Error())
		return
	}
	if runtimes == nil {
		runtimes = []*store.Runtime{}
	}
	response.JSON(w, http.StatusOK, runtimesResponse{Runtimes: runtimes})
}

func (h *RuntimeHandler) Discover(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req discoverRuntimesRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := response.DecodeJSON(r, &req); err != nil {
			response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
			return
		}
	}

	for kind, override := range req.PathOverrides {
		if !runtimeprobe.IsSupportedKind(kind) {
			response.Error(w, http.StatusBadRequest, "INVALID_RUNTIME_KIND", "unknown runtime kind", kind)
			return
		}
		if strings.TrimSpace(override) == "" {
			continue
		}
		if err := runtimeprobe.ValidateBinaryPath(override); err != nil {
			response.Error(w, http.StatusUnprocessableEntity, "INVALID_BINARY_PATH", "invalid binary path", err.Error())
			return
		}
	}

	var runtimes []*store.Runtime
	var summary runtimeSummary
	for _, kind := range runtimeprobe.SupportedKinds {
		binaryPath := ""
		if override := strings.TrimSpace(req.PathOverrides[kind]); override != "" {
			binaryPath = override
		} else {
			binaryPath = runtimeprobe.LookPath(kind)
		}
		probe := runtimeprobe.Probe(kind, binaryPath)
		runtime, err := h.repo.Upsert(runtimeFromProbe(probe, runtimeprobe.DisplayName(kind)))
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist runtime discovery", err.Error())
			return
		}
		runtimes = append(runtimes, runtime)
		addRuntimeSummary(&summary, runtime.HealthStatus)
	}

	response.JSON(w, http.StatusOK, discoverRuntimesResponse{Runtimes: runtimes, Summary: summary})
}

func (h *RuntimeHandler) Validate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req validateRuntimeRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if strings.TrimSpace(req.RuntimeID) == "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "runtime_id is required", "")
		return
	}

	runtime, err := h.repo.GetByID(req.RuntimeID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get runtime", err.Error())
		return
	}
	if runtime == nil {
		response.Error(w, http.StatusNotFound, "RUNTIME_NOT_FOUND", "runtime not found", "")
		return
	}

	updated := applyProbeToRuntime(runtime, runtimeprobe.Probe(runtime.Kind, runtime.BinaryPath))
	if err := h.repo.Update(updated); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update runtime validation", err.Error())
		return
	}
	if updated.HealthReason == runtimeprobe.ReasonRuntimeMismatch {
		response.Error(w, http.StatusConflict, "RUNTIME_KIND_MISMATCH", "runtime kind does not match binary output", "")
		return
	}
	response.JSON(w, http.StatusOK, runtimeResponse{Runtime: updated})
}

func (h *RuntimeHandler) Update(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	id := r.PathValue("id")
	runtime, err := h.repo.GetByID(id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get runtime", err.Error())
		return
	}
	if runtime == nil {
		response.Error(w, http.StatusNotFound, "RUNTIME_NOT_FOUND", "runtime not found", "")
		return
	}

	var req updateRuntimeRequest
	if err := response.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body", err.Error())
		return
	}
	if req.DisplayName != nil {
		runtime.DisplayName = strings.TrimSpace(*req.DisplayName)
		if runtime.DisplayName == "" {
			runtime.DisplayName = runtimeprobe.DisplayName(runtime.Kind)
		}
	}
	if req.BinaryPath != nil {
		binaryPath := strings.TrimSpace(*req.BinaryPath)
		if err := runtimeprobe.ValidateBinaryPath(binaryPath); err != nil {
			response.Error(w, http.StatusUnprocessableEntity, "INVALID_BINARY_PATH", "invalid binary path", err.Error())
			return
		}
		runtime.BinaryPath = binaryPath
		runtime = applyProbeToRuntime(runtime, runtimeprobe.Probe(runtime.Kind, binaryPath))
	}

	if err := h.repo.Update(runtime); err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update runtime", err.Error())
		return
	}
	if runtime.HealthReason == runtimeprobe.ReasonRuntimeMismatch {
		response.Error(w, http.StatusConflict, "RUNTIME_KIND_MISMATCH", "runtime kind does not match binary output", "")
		return
	}
	response.JSON(w, http.StatusOK, runtimeResponse{Runtime: runtime})
}

func runtimeFromProbe(probe runtimeprobe.ProbeResult, displayName string) *store.Runtime {
	now := time.Now().UTC()
	return &store.Runtime{
		Kind:          probe.Kind,
		DisplayName:   displayName,
		BinaryPath:    probe.BinaryPath,
		VersionRaw:    probe.VersionRaw,
		HealthStatus:  probe.HealthStatus,
		HealthReason:  probe.HealthReason,
		LastCheckedAt: now,
	}
}

func applyProbeToRuntime(runtime *store.Runtime, probe runtimeprobe.ProbeResult) *store.Runtime {
	runtime.BinaryPath = probe.BinaryPath
	runtime.VersionRaw = probe.VersionRaw
	runtime.HealthStatus = probe.HealthStatus
	runtime.HealthReason = probe.HealthReason
	runtime.LastCheckedAt = time.Now().UTC()
	return runtime
}

func addRuntimeSummary(summary *runtimeSummary, status string) {
	switch status {
	case runtimeprobe.StatusHealthy:
		summary.Healthy++
	case runtimeprobe.StatusDegraded:
		summary.Degraded++
	default:
		summary.Missing++
	}
}
