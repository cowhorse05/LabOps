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

	dbDriver := env("LABOPS_DB_DRIVER", "mysql")
	addr := env("LABOPS_ADDR", ":8080")
	var dsn string
	switch core.Driver(dbDriver) {
	case core.DriverMySQL:
		dsn = env("LABOPS_MYSQL_DSN", "labops:labops@tcp(127.0.0.1:3306)/labops?parseTime=true&charset=utf8mb4")
	default:
		dsn = env("LABOPS_DB_PATH", "data/labops.db")
	}

	store, err := core.OpenStore(core.Driver(dbDriver), dsn)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	if err := store.Init(ctx); err != nil {
		log.Fatalf("init store: %v", err)
	}

	app := core.NewApp(store, core.Config{
		AgentToken:       env("LABOPS_AGENT_TOKEN", "dev-agent-token"),
		WebToken:         env("LABOPS_WEB_TOKEN", "dev-token"),
		HeartbeatTimeout: 35 * time.Second,
		TaskTimeout:      2 * time.Minute,
		LLMURL:           env("LABOPS_LLM_URL", ""),
		LLMAPIKey:        env("LABOPS_LLM_API_KEY", ""),
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: app.Handler(),
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

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
