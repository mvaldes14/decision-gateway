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

	"github.com/mvaldes14/decision-gateway/internal/api"
	"github.com/mvaldes14/decision-gateway/internal/config"
	"github.com/mvaldes14/decision-gateway/internal/engine"
	"github.com/mvaldes14/decision-gateway/internal/provider"
	"github.com/mvaldes14/decision-gateway/internal/registry"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	definitions, err := registry.Load(cfg.DefinitionsDir)
	if err != nil {
		return err
	}

	providers := provider.NewSet() // Real adapters added after protocol validation (M0).
	executionEngine := engine.New(providers)
	handler := api.New(definitions, executionEngine, cfg.MaxBodyBytes, cfg.MaxConcurrent, logger)
	server := &http.Server{
		Addr: cfg.ListenAddr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP shutdown failed", "error", err)
		}
	}()

	logger.Info("starting decision gateway", "address", cfg.ListenAddr, "definitions", cfg.DefinitionsDir)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
