package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cowhorse05/labops/server/internal/core"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	environment := env("LABOPS_ENV", "development")

	// Initialize structured logger (Go 1.21+ slog).
	// Text handler for development (human-readable), JSON for production.
	var level slog.Level
	switch environment {
	case "production":
		level = slog.LevelInfo
	default:
		level = slog.LevelDebug
	}
	handlerOpts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if environment == "production" {
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	dbDriver := env("LABOPS_DB_DRIVER", "sqlite")
	addr := env("LABOPS_ADDR", ":8080")
	var dsn string
	switch core.Driver(dbDriver) {
	case core.DriverMySQL:
		dsn = env("LABOPS_MYSQL_DSN", "labops:labops@tcp(127.0.0.1:3306)/labops?parseTime=true&charset=utf8mb4")
	case core.DriverJSON:
		dsn = env("LABOPS_DB_PATH", "data")
	default:
		dsn = env("LABOPS_DB_PATH", "data/labops.db")
	}

	var store core.DataStore
	if core.Driver(dbDriver) == core.DriverJSON {
		var err error
		store, err = core.OpenJSONStore(dsn)
		if err != nil {
			logger.Error("open json store", "error", err)
			os.Exit(1)
		}
	} else {
		s, err := core.OpenStore(core.Driver(dbDriver), dsn)
		if err != nil {
			logger.Error("open store", "error", err)
			os.Exit(1)
		}
		store = s
	}
	defer store.Close()
	if err := store.ConfigureEncryptionKey(os.Getenv("LABOPS_ENCRYPTION_KEY")); err != nil {
		logger.Error("configure encryption", "error", err)
		os.Exit(1)
	}

	if err := store.InitSecure(ctx, os.Getenv("LABOPS_BOOTSTRAP_ADMIN_PASSWORD")); err != nil {
		logger.Error("init store", "error", err)
		os.Exit(1)
	}
	if err := store.ProtectStoredLLMSecret(ctx); err != nil {
		logger.Error("protect stored LLM secret", "error", err)
		os.Exit(1)
	}

	publicOrigin := env("LABOPS_PUBLIC_ORIGIN", "http://localhost:5173")
	if environment == "production" && os.Getenv("LABOPS_PUBLIC_ORIGIN") == "" {
		logger.Error("LABOPS_PUBLIC_ORIGIN is required in production")
		os.Exit(1)
	}
	if environment == "production" && os.Getenv("LABOPS_ENCRYPTION_KEY") == "" {
		logger.Error("LABOPS_ENCRYPTION_KEY is required in production")
		os.Exit(1)
	}

	app := core.NewApp(store, core.Config{
		Environment:      environment,
		PublicOrigin:     publicOrigin,
		SecureCookies:    environment == "production",
		EncryptionKey:    os.Getenv("LABOPS_ENCRYPTION_KEY"),
		OpenRegistration: os.Getenv("LABOPS_OPEN_REGISTRATION") == "true",
		HeartbeatTimeout: envDuration("LABOPS_HEARTBEAT_TIMEOUT", 35*time.Second, logger),
		TaskTimeout:      envDuration("LABOPS_TASK_TIMEOUT", 5*time.Minute, logger),
		LLMURL:           env("LABOPS_LLM_URL", ""),
		LLMAPIKey:        env("LABOPS_LLM_API_KEY", ""),
	}, logger)

	srv := &http.Server{
		Addr:              addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	go func() {
		<-ctx.Done()
		logger.Info("shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		app.Stop()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown", "error", err)
		}
	}()

	logger.Info("LabOps server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("listen", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}

func envDuration(key string, fallback time.Duration, logger *slog.Logger) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		logger.Error("invalid duration", "key", key, "error", err)
		os.Exit(1)
	}
	return parsed
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
