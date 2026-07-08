package core

import (
	"context"
	"log"
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
	analyzer *Analyzer

	mu           sync.RWMutex
	clients      map[string]*AgentClient
	rateLimiters map[string]*rateLimiter
	rlMu         sync.Mutex
}

type rateLimiter struct {
	tokens    int
	maxTokens int
	interval  time.Duration
	last      time.Time
}

func (rl *rateLimiter) allow() bool {
	now := time.Now()
	elapsed := now.Sub(rl.last)
	rl.last = now
	rl.tokens += int(elapsed / rl.interval)
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	if rl.tokens <= 0 {
		return false
	}
	rl.tokens--
	return true
}

func newRateLimiter(maxTokens int, interval time.Duration) *rateLimiter {
	return &rateLimiter{
		tokens:    maxTokens,
		maxTokens: maxTokens,
		interval:  interval,
		last:      time.Now(),
	}
}

func NewApp(store *Store, config Config) *App {
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 35 * time.Second
	}
	if config.TaskTimeout == 0 {
		config.TaskTimeout = 2 * time.Minute
	}
	app := &App{
		store:        store,
		config:       config,
		clients:      make(map[string]*AgentClient),
		rateLimiters: make(map[string]*rateLimiter),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	app.analyzer = NewAnalyzer(store, config)
	go app.maintenanceLoop()
	return app
}

// Stop gracefully shuts down background loops (analyzer, maintenance) and
// closes all agent WebSocket connections so in-flight writes can complete.
func (a *App) Stop() {
	a.analyzer.Stop()
	a.mu.Lock()
	for id, client := range a.clients {
		client.Close()
		delete(a.clients, id)
	}
	a.mu.Unlock()
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
	mux.HandleFunc("GET /api/aiops/report", a.handleAiOpsReport)
	mux.HandleFunc("GET /api/agent/ws", a.handleAgentWS)
	return a.withCORS(a.withRateLimit(a.withAuth(mux)))
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
		w.Header().Set("Vary", "Origin")
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

func (a *App) withRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		// Strip port if present.
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
		a.rlMu.Lock()
		rl, exists := a.rateLimiters[ip]
		if !exists {
			rl = newRateLimiter(60, time.Second)
			a.rateLimiters[ip] = rl
		}
		// Hold rlMu through allow() to prevent data races on the rateLimiter fields
		// when concurrent requests arrive from the same IP.
		ok := rl.allow()
		a.rlMu.Unlock()
		if !ok {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) refreshState(ctx context.Context) {
	cutoff := time.Now().UTC().Add(-a.config.HeartbeatTimeout).Format(time.RFC3339)
	if err := a.store.ExpireDevices(ctx, cutoff); err != nil {
		log.Printf("expire devices error: %v", err)
	}
	taskCutoff := time.Now().UTC().Add(-a.config.TaskTimeout).Format(time.RFC3339)
	if err := a.store.TimeoutTasks(ctx, taskCutoff); err != nil {
		log.Printf("timeout tasks error: %v", err)
	}
}

func (a *App) maintenanceLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.analyzer.done:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			a.refreshState(ctx)
			cancel()
		}
	}
}
