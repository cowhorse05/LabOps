package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cowhorse05/labops/server/internal/core"
)

func main() {
	ctx := context.Background()
	dbPath := env("LABOPS_DB_PATH", "data/labops.db")
	addr := env("LABOPS_ADDR", ":8080")

	store, err := core.OpenStore(dbPath)
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
	})

	log.Printf("LabOps server listening on %s", addr)
	if err := http.ListenAndServe(addr, app.Handler()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
