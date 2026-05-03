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
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Parse()
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if err := pidfile.CleanupStale(cfg.DataDir); err != nil {
		return fmt.Errorf("cleanup stale pid: %w", err)
	}

	database, err := db.Open(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	if err := db.IntegrityCheck(database); err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}

	if err := migration.Run(database); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	projectRepo := store.NewProjectRepository(database)
	issueRepo := store.NewIssueRepository(database)
	tagRepo := store.NewTagRepository(database)
	commentRepo := store.NewIssueCommentRepository(database)
	activityRepo := store.NewIssueActivityRepository(database)
	agentRepo := store.NewAgentRepository(database)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	fmt.Printf("%d\n", port)

	if err := pidfile.Write(cfg.DataDir); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	defer func() {
		if err := pidfile.Remove(cfg.DataDir); err != nil {
			log.Printf("remove pid file: %v", err)
		}
	}()

	shutdownCh := make(chan struct{}, 1)
	healthHandler := handler.NewHealthHandler(database, cfg, port)
	router := httpapi.NewRouter(
		healthHandler,
		handler.NewProjectHandler(projectRepo),
		handler.NewIssueHandler(database, issueRepo, projectRepo),
		handler.NewInteractionHandler(database, projectRepo, issueRepo, tagRepo, commentRepo, activityRepo),
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
	defer signal.Stop(sigCh)

	serveErrCh := make(chan error, 1)

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			serveErrCh <- err
		}
	}()

	var serveErr error
	select {
	case sig := <-sigCh:
		log.Printf("received signal %v, shutting down", sig)
	case <-shutdownCh:
		log.Printf("received shutdown request via API, shutting down")
	case serveErr = <-serveErrCh:
		log.Printf("server failed, shutting down: %v", serveErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown server: %v", err)
	}

	if serveErr != nil {
		return fmt.Errorf("serve: %w", serveErr)
	}

	return nil
}
