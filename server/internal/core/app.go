package core

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Config struct {
	AgentToken       string
	WebToken         string
	HeartbeatTimeout time.Duration
	TaskTimeout      time.Duration
}

type App struct {
	store    *Store
	config   Config
	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[string]*AgentClient
}

func NewApp(store *Store, config Config) *App {
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 35 * time.Second
	}
	if config.TaskTimeout == 0 {
		config.TaskTimeout = 2 * time.Minute
	}
	app := &App{
		store:  store,
		config: config,
		clients: make(map[string]*AgentClient),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	go app.maintenanceLoop()
	return app
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	mux.HandleFunc("GET /api/auth/me", a.handleMe)
	mux.HandleFunc("GET /api/stats", a.handleStats)
	mux.HandleFunc("GET /api/devices", a.handleListDevices)
	mux.HandleFunc("GET /api/devices/{id}", a.handleGetDevice)
	mux.HandleFunc("GET /api/groups", a.handleGroups)
	mux.HandleFunc("GET /api/tasks", a.handleListTasks)
	mux.HandleFunc("POST /api/tasks", a.handleCreateTask)
	mux.HandleFunc("GET /api/tasks/{id}", a.handleGetTask)
	mux.HandleFunc("GET /api/audit-logs", a.handleAuditLogs)
	mux.HandleFunc("GET /api/agent/ws", a.handleAgentWS)
	return a.withCORS(a.withAuth(mux))
}

func (a *App) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || strings.HasPrefix(r.URL.Path, "/api/agent/") || r.URL.Path == "/api/health" || r.URL.Path == "/api/auth/login" {
			next.ServeHTTP(w, r)
			return
		}
		if a.config.WebToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+a.config.WebToken {
			writeError(w, http.StatusUnauthorized, "missing or invalid token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) refreshState(ctx context.Context) {
	cutoff := time.Now().UTC().Add(-a.config.HeartbeatTimeout).Format(time.RFC3339)
	_ = a.store.ExpireDevices(ctx, cutoff)
	taskCutoff := time.Now().UTC().Add(-a.config.TaskTimeout).Format(time.RFC3339)
	_ = a.store.TimeoutTasks(ctx, taskCutoff)
}

func (a *App) maintenanceLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.refreshState(context.Background())
	}
}
