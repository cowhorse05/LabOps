package main

import (
	"context"
	"log"
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
			log.Fatalf("open json store: %v", err)
		}
	} else {
		s, err := core.OpenStore(core.Driver(dbDriver), dsn)
		if err != nil {
			log.Fatalf("open store: %v", err)
		}
		store = s
	}
	defer store.Close()
	if err := store.ConfigureEncryptionKey(os.Getenv("LABOPS_ENCRYPTION_KEY")); err != nil {
		log.Fatalf("configure encryption: %v", err)
	}

	if err := store.InitSecure(ctx, os.Getenv("LABOPS_BOOTSTRAP_ADMIN_PASSWORD")); err != nil {
		log.Fatalf("init store: %v", err)
	}
	if err := store.ProtectStoredLLMSecret(ctx); err != nil {
		log.Fatalf("protect stored LLM secret: %v", err)
	}
	environment := env("LABOPS_ENV", "development")
	publicOrigin := env("LABOPS_PUBLIC_ORIGIN", "http://localhost:5173")
	if environment == "production" && os.Getenv("LABOPS_PUBLIC_ORIGIN") == "" {
		log.Fatal("LABOPS_PUBLIC_ORIGIN is required in production")
	}
	if environment == "production" && os.Getenv("LABOPS_ENCRYPTION_KEY") == "" {
		log.Fatal("LABOPS_ENCRYPTION_KEY is required in production")
	}

	app := core.NewApp(store, core.Config{
		Environment:      environment,
		PublicOrigin:     publicOrigin,
		SecureCookies:    environment == "production",
		EncryptionKey:    os.Getenv("LABOPS_ENCRYPTION_KEY"),
		HeartbeatTimeout: envDuration("LABOPS_HEARTBEAT_TIMEOUT", 35*time.Second),
		TaskTimeout:      envDuration("LABOPS_TASK_TIMEOUT", 5*time.Minute),
		LLMURL:           env("LABOPS_LLM_URL", ""),
		LLMAPIKey:        env("LABOPS_LLM_API_KEY", ""),
	})

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
		log.Println("shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		app.Stop()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()

	log.Printf("LabOps server listening on %s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
	log.Println("server stopped")
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		log.Fatalf("invalid %s: %v", key, err)
	}
	return parsed
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
