package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/terra-clan/sandbox-engine/internal/api"
	"github.com/terra-clan/sandbox-engine/internal/cleanup"
	"github.com/terra-clan/sandbox-engine/internal/config"
	"github.com/terra-clan/sandbox-engine/internal/sandbox"
	"github.com/terra-clan/sandbox-engine/internal/services"
	"github.com/terra-clan/sandbox-engine/internal/storage"
	"github.com/terra-clan/sandbox-engine/internal/templates"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("starting sandbox-engine",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
	)

	// Create context for initialization
	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer initCancel()

	// Run database migrations
	slog.Info("running database migrations", "dir", cfg.Database.MigrationsDir)
	if err := storage.MigrateFromDSN(initCtx, cfg.Database.DSN, cfg.Database.MigrationsDir); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Initialize database repository
	repo, err := storage.NewPostgresRepository(initCtx, storage.PostgresConfig{
		DSN:          cfg.Database.DSN,
		MaxOpenConns: int32(cfg.Database.MaxOpenConns),
		MaxIdleConns: int32(cfg.Database.MaxIdleConns),
	})
	if err != nil {
		slog.Error("failed to create database repository", "error", err)
		os.Exit(1)
	}
	slog.Info("database connected successfully")

	// Initialize service registry
	registry := services.NewRegistry()

	// Register service providers
	postgresProvider, err := services.NewPostgresProvider(cfg.Database.DSN)
	if err != nil {
		slog.Error("failed to create postgres provider", "error", err)
		os.Exit(1)
	}
	registry.Register("postgres", postgresProvider)

	redisProvider, err := services.NewRedisProvider(cfg.Redis.Address, cfg.Redis.Password)
	if err != nil {
		slog.Error("failed to create redis provider", "error", err)
		os.Exit(1)
	}
	registry.Register("redis", redisProvider)

	// Load templates
	templateLoader := templates.NewLoader()
	if err := templateLoader.LoadFromDir(cfg.Templates.Dir); err != nil {
		slog.Warn("failed to load templates from dir", "dir", cfg.Templates.Dir, "error", err)
	}

	// Initialize sandbox manager
	manager, err := sandbox.NewManager(cfg.Docker, registry, templateLoader, repo)
	if err != nil {
		slog.Error("failed to create sandbox manager", "error", err)
		os.Exit(1)
	}

	// Initialize cleanup worker
	cleaner := cleanup.NewCleaner(manager, cfg.Cleanup.Interval)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start cleanup worker
	cleaner.Start(ctx)

	// Setup HTTP server
	server := api.NewServer(cfg.Server, manager, templateLoader)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      server.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("HTTP server starting", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gracefully...")

	// Cancel context to stop background workers
	cancel()

	// Shutdown HTTP server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	// Close manager (cleanup Docker resources)
	if err := manager.Close(); err != nil {
		slog.Error("manager close error", "error", err)
	}

	slog.Info("sandbox-engine stopped")
}
