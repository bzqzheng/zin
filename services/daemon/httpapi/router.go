package httpapi

import (
	"net"
	"net/http"
	"net/url"

	"github.com/bzqzheng/zin/services/daemon/httpapi/handler"
	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
)

func NewRouter(
	health *handler.HealthHandler,
	projects *handler.ProjectHandler,
	issues *handler.IssueHandler,
	interactions *handler.InteractionHandler,
	agents *handler.AgentHandler,
	runtimes *handler.RuntimeHandler,
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
	mux.HandleFunc("GET /api/projects/{pid}/tags", interactions.ListProjectTags)
	mux.HandleFunc("POST /api/projects/{pid}/tags", interactions.CreateProjectTag)
	mux.HandleFunc("PUT /api/tags/{id}", interactions.UpdateTag)
	mux.HandleFunc("DELETE /api/tags/{id}", interactions.DeleteTag)
	mux.HandleFunc("GET /api/issues/{id}/tags", interactions.ListIssueTags)
	mux.HandleFunc("POST /api/issues/{id}/tags", interactions.AttachIssueTag)
	mux.HandleFunc("DELETE /api/issues/{id}/tags/{tag_id}", interactions.DetachIssueTag)
	mux.HandleFunc("GET /api/issues/{id}/comments", interactions.ListComments)
	mux.HandleFunc("POST /api/issues/{id}/comments", interactions.CreateComment)
	mux.HandleFunc("PUT /api/comments/{id}", interactions.UpdateComment)
	mux.HandleFunc("DELETE /api/comments/{id}", interactions.DeleteComment)
	mux.HandleFunc("GET /api/issues/{id}/activity", interactions.ListActivity)

	mux.HandleFunc("GET /api/agents", agents.List)
	mux.HandleFunc("POST /api/agents", agents.Create)
	mux.HandleFunc("GET /api/agents/{id}", agents.Get)
	mux.HandleFunc("PUT /api/agents/{id}", agents.Update)
	mux.HandleFunc("DELETE /api/agents/{id}", agents.Delete)
	mux.HandleFunc("GET /api/runtimes", runtimes.List)
	mux.HandleFunc("POST /api/runtimes/discover", runtimes.Discover)
	mux.HandleFunc("POST /api/runtimes/validate", runtimes.Validate)
	mux.HandleFunc("PUT /api/runtimes/{id}", runtimes.Update)

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

	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.Header().Add("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	switch parsed.Scheme {
	case "tauri":
		return parsed.Hostname() == "localhost"
	case "http", "https":
		host := parsed.Hostname()
		if host == "" {
			host, _, err = net.SplitHostPort(parsed.Host)
			if err != nil {
				return false
			}
		}
		return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "tauri.localhost"
	default:
		return false
	}
}
