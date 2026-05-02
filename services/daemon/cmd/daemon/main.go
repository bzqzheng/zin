package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bzqzheng/zin/services/daemon/config"
	"github.com/bzqzheng/zin/services/daemon/db"
	"github.com/bzqzheng/zin/services/daemon/httpapi"
	"github.com/bzqzheng/zin/services/daemon/httpapi/handler"
	"github.com/bzqzheng/zin/services/daemon/migration"
	"github.com/bzqzheng/zin/services/daemon/pidfile"
	"github.com/bzqzheng/zin/services/daemon/store"
)

func main() {
	cfg := config.Parse()

	if err := pidfile.CleanupStale(cfg.DataDir); err != nil {
		log.Fatalf("cleanup stale pid: %v", err)
	}

	database, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	if err := db.IntegrityCheck(database); err != nil {
		log.Fatalf("integrity check: %v", err)
	}

	if err := migration.Run(database); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	projectRepo := store.NewProjectRepository(database)
	issueRepo := store.NewIssueRepository(database)
	agentRepo := store.NewAgentRepository(database)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Port))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	fmt.Printf("%d\n", port)

	if err := pidfile.Write(cfg.DataDir); err != nil {
		log.Fatalf("write pid file: %v", err)
	}

	shutdownCh := make(chan struct{}, 1)
	healthHandler := handler.NewHealthHandler(database, cfg, port)
	router := httpapi.NewRouter(
		healthHandler,
		handler.NewProjectHandler(projectRepo),
		handler.NewIssueHandler(issueRepo),
		handler.NewAgentHandler(agentRepo),
		shutdownCh,
	)

	srv := &http.Server{
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	select {
	case sig := <-sigCh:
		log.Printf("received signal %v, shutting down", sig)
	case <-shutdownCh:
		log.Printf("received shutdown request via API, shutting down")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	srv.Shutdown(ctx)
	pidfile.Remove(cfg.DataDir)
}
