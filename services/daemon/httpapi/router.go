package httpapi

import (
	"net/http"

	"github.com/bzqzheng/zin/services/daemon/httpapi/handler"
	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
)

func NewRouter(
	health *handler.HealthHandler,
	projects *handler.ProjectHandler,
	issues *handler.IssueHandler,
	agents *handler.AgentHandler,
	shutdownCh chan<- struct{},
) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /health", health)

	mux.HandleFunc("GET /api/projects", projects.List)
	mux.HandleFunc("POST /api/projects", projects.Create)
	mux.HandleFunc("GET /api/projects/{id}", projects.Get)
	mux.HandleFunc("PUT /api/projects/{id}", projects.Update)
	mux.HandleFunc("DELETE /api/projects/{id}", projects.Delete)

	mux.HandleFunc("GET /api/projects/{pid}/issues", issues.List)
	mux.HandleFunc("POST /api/projects/{pid}/issues", issues.Create)
	mux.HandleFunc("GET /api/issues/{id}", issues.Get)
	mux.HandleFunc("PUT /api/issues/{id}", issues.Update)
	mux.HandleFunc("DELETE /api/issues/{id}", issues.Delete)
	mux.HandleFunc("PUT /api/issues/{id}/status", issues.UpdateStatus)

	mux.HandleFunc("GET /api/agents", agents.List)
	mux.HandleFunc("POST /api/agents", agents.Create)
	mux.HandleFunc("GET /api/agents/{id}", agents.Get)
	mux.HandleFunc("PUT /api/agents/{id}", agents.Update)
	mux.HandleFunc("DELETE /api/agents/{id}", agents.Delete)

	mux.HandleFunc("POST /shutdown", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"shutting_down"}`))
		go func() {
			shutdownCh <- struct{}{}
		}()
	})

	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{
			"message": "zin daemon API v0.1.0",
		})
	})

	return mux
}
