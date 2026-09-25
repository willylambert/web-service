package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/willylambert/web-service/internal/config"
	"github.com/willylambert/web-service/internal/handlers"
	"github.com/willylambert/web-service/internal/sncf"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()
	client := sncf.NewClient(cfg.SNCFAPIToken)
	if !client.Configured() {
		logger.Warn("SNCF_API_TOKEN not set; timetable API will return 503 until configured")
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handlers.NewRouter(client),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("server listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("shutting down")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
