package handler

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/bzqzheng/zin/services/daemon/config"
	"github.com/bzqzheng/zin/services/daemon/httpapi/response"
)

var startTime time.Time

func init() {
	startTime = time.Now()
}

type HealthHandler struct {
	db   *sql.DB
	cfg  *config.DaemonConfig
	port int
}

func NewHealthHandler(db *sql.DB, cfg *config.DaemonConfig, port int) *HealthHandler {
	return &HealthHandler{db: db, cfg: cfg, port: port}
}

type HealthResponse struct {
	Status        string  `json:"status"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	DBStatus      string  `json:"db_status"`
	Version       string  `json:"version"`
	Port          int     `json:"port"`
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := h.db.Ping(); err != nil {
		dbStatus = "error: " + err.Error()
	}

	uptime := time.Since(startTime).Seconds()

	response.JSON(w, http.StatusOK, HealthResponse{
		Status:        "ok",
		UptimeSeconds: uptime,
		DBStatus:      dbStatus,
		Version:       "0.1.0",
		Port:          h.port,
	})
}
