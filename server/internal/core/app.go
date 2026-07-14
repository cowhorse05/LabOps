package core

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Config struct {
	Environment      string
	PublicOrigin     string
	SecureCookies    bool
	EncryptionKey    string
	AgentToken       string
	WebToken         string
	OpenRegistration bool
	HeartbeatTimeout time.Duration
	TaskTimeout      time.Duration
	LLMURL           string
	LLMAPIKey        string
}

type App struct {
	store    DataStore
	config   Config
	logger   *slog.Logger
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

func NewApp(store DataStore, config Config, logger *slog.Logger) *App {
	if config.HeartbeatTimeout == 0 {
		config.HeartbeatTimeout = 35 * time.Second
	}
	if config.TaskTimeout == 0 {
		config.TaskTimeout = 2 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	app := &App{
		store:        store,
		config:       config,
		logger:       logger,
		clients:      make(map[string]*AgentClient),
		rateLimiters: make(map[string]*rateLimiter),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				if r == nil {
					return config.Environment == ""
				}
				origin := r.Header.Get("Origin")
				if origin == "" || config.PublicOrigin == "" || origin == config.PublicOrigin {
					return true
				}
				// Allow agents on the same machine to connect via loopback.
				if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
					return true
				}
				return false
			},
		},
	}
	app.analyzer = NewAnalyzer(store, config, logger)
	app.analyzer.OnDispatch = app.dispatchTask
	app.analyzer.Start()
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
	mux.HandleFunc("POST /api/auth/register", a.handleSelfRegister)
	mux.HandleFunc("POST /api/auth/change-password", a.handleChangePassword)
	mux.HandleFunc("POST /api/auth/logout", a.handleLogout)
	mux.HandleFunc("GET /api/auth/me", a.handleMe)
	mux.HandleFunc("GET /api/users", a.handleListUsers)
	mux.HandleFunc("POST /api/users", a.handleCreateUser)
	mux.HandleFunc("PUT /api/users/{id}", a.handleUpdateUser)
	mux.HandleFunc("GET /api/stats", a.handleStats)
	mux.HandleFunc("GET /api/devices", a.handleListDevices)
	mux.HandleFunc("GET /api/devices/{id}", a.handleGetDevice)
	mux.HandleFunc("GET /api/devices/{id}/tasks", a.handleListDeviceTasks)
	mux.HandleFunc("POST /api/devices", a.handleCreateDevice)
	mux.HandleFunc("DELETE /api/devices/{id}", a.handleDeleteDevice)
	mux.HandleFunc("POST /api/devices/{id}/revoke", a.handleRevokeDevice)
	mux.HandleFunc("GET /api/enrollment-codes", a.handleListEnrollmentCodes)
	mux.HandleFunc("POST /api/enrollment-codes", a.handleCreateEnrollmentCode)
	mux.HandleFunc("DELETE /api/enrollment-codes/{id}", a.handleRevokeEnrollmentCode)
	mux.HandleFunc("POST /api/agent/enroll", a.handleAgentEnroll)
	mux.HandleFunc("GET /api/groups", a.handleGroups)
	mux.HandleFunc("GET /api/tasks", a.handleListTasks)
	mux.HandleFunc("POST /api/tasks", a.handleCreateTask)
	mux.HandleFunc("GET /api/tasks/{id}", a.handleGetTask)
	mux.HandleFunc("GET /api/command-templates", a.handleListCommandTemplates)
	mux.HandleFunc("POST /api/command-templates", a.handleCreateCommandTemplate)
	mux.HandleFunc("PUT /api/command-templates/{id}", a.handleUpdateCommandTemplate)
	mux.HandleFunc("GET /api/audit-logs", a.handleAuditLogs)
	mux.HandleFunc("GET /api/aiops/report", a.handleAiOpsReport)
	mux.HandleFunc("GET /api/aiops/llm-config", a.handleGetLLMConfig)
	mux.HandleFunc("PUT /api/aiops/llm-config", a.handleSaveLLMConfig)
	mux.HandleFunc("POST /api/aiops/llm-test", a.handleTestLLM)
	mux.HandleFunc("POST /api/aiops/recommendations/execute", a.handleExecuteRecommendation)
	mux.HandleFunc("GET /api/aiops/auto-mode", a.handleGetAutoMode)
	mux.HandleFunc("PUT /api/aiops/auto-mode", a.handleSaveAutoMode)
	mux.HandleFunc("GET /api/agent/ws", a.handleAgentWS)
	mux.HandleFunc("GET /api/v1/system/status", a.handleSystemStatus)
	mux.HandleFunc("POST /api/v1/system/bootstrap", a.handleSystemBootstrap)
	mux.HandleFunc("GET /api/setup/status", a.handleSetupStatus)
	mux.HandleFunc("POST /api/setup/admin", a.handleSetupAdmin)
	return a.withRequestID(a.withRequestLogging(a.withCORS(a.withRateLimit(a.withAuth(mux)))))
}

func (a *App) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigin := a.config.PublicOrigin
		if a.config.Environment == "" && allowedOrigin == "" {
			allowedOrigin = origin
			if allowedOrigin == "" {
				allowedOrigin = "*"
			}
		}
		if origin != "" && allowedOrigin != "*" && !originsMatch(origin, allowedOrigin) {
			a.logger.Warn("CORS blocked", "origin", origin, "allowed", allowedOrigin)
			writeAPIError(w, http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "origin is not allowed")
			return
		}
		if allowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-CSRF-Token,X-Request-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// originsMatch compares two HTTP origins, normalizing default ports and trailing slashes.
// "http://47.80.31.24:80" == "http://47.80.31.24" and "http://47.80.31.24/" == "http://47.80.31.24".
func originsMatch(a, b string) bool {
	na := normalizeOrigin(a)
	nb := normalizeOrigin(b)
	return na == nb
}

func normalizeOrigin(raw string) string {
	s := strings.TrimRight(raw, "/")
	// Strip default port: 80 for http, 443 for https
	if strings.HasPrefix(s, "http://") {
		s = strings.TrimSuffix(s, ":80")
	} else if strings.HasPrefix(s, "https://") {
		s = strings.TrimSuffix(s, ":443")
	}
	return strings.ToLower(s)
}

func (a *App) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions ||
			r.URL.Path == "/api/agent/enroll" ||
			r.URL.Path == "/api/agent/ws" ||
			r.URL.Path == "/api/health" ||
			r.URL.Path == "/api/auth/login" ||
			r.URL.Path == "/api/auth/register" ||
			r.URL.Path == "/api/v1/system/status" ||
			r.URL.Path == "/api/v1/system/bootstrap" ||
			r.URL.Path == "/api/setup/status" ||
			r.URL.Path == "/api/setup/admin" {
			next.ServeHTTP(w, r)
			return
		}

		// Legacy auth is intentionally reachable only from explicit internal test
		// configuration. Production main never sets WebToken.
		if a.config.WebToken != "" && r.Header.Get("Authorization") == "Bearer "+a.config.WebToken {
			ctx := context.WithValue(r.Context(), authContextKey{}, authContext{User: a.store.AdminUser(), Legacy: true})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if a.config.Environment == "" && a.config.WebToken == "" {
			ctx := context.WithValue(r.Context(), authContextKey{}, authContext{User: a.store.AdminUser(), Legacy: true})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeAPIError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "authentication required")
			return
		}
		user, sessionID, csrfHash, ok, err := a.store.AuthenticateWebSession(r.Context(), cookie.Value)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "AUTH_SESSION_ERROR", "unable to validate session")
			return
		}
		if !ok {
			clearAuthCookies(w, a.config.SecureCookies)
			writeAPIError(w, http.StatusUnauthorized, "SESSION_EXPIRED", "session expired")
			return
		}
		if isStateChanging(r.Method) && !isCSRFExempt(r.URL.Path) {
			csrfCookie, cookieErr := r.Cookie(csrfCookieName)
			headerToken := r.Header.Get("X-CSRF-Token")
			if cookieErr != nil || headerToken == "" || csrfCookie.Value != headerToken ||
				subtle.ConstantTimeCompare([]byte(tokenHash(headerToken)), []byte(csrfHash)) != 1 {
				writeAPIError(w, http.StatusForbidden, "CSRF_INVALID", "invalid CSRF token")
				return
			}
		}
		if a.store.MustChangePassword(r.Context(), user.Username) && r.URL.Path != "/api/auth/change-password" && r.URL.Path != "/api/auth/me" && r.URL.Path != "/api/auth/logout" {
			writeAPIError(w, http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED", "password change required")
			return
		}
		if permission := requiredPermission(r); permission != "" && !HasPermission(user, permission) {
			writeAPIError(w, http.StatusForbidden, "PERMISSION_DENIED", "permission denied")
			return
		}
		ctx := context.WithValue(r.Context(), authContextKey{}, authContext{User: user, SessionID: sessionID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requiredPermission(r *http.Request) string {
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/users") {
		return PermissionUserManage
	}
	if strings.HasPrefix(path, "/api/enrollment-codes") {
		return PermissionEnrollManage
	}
	if strings.HasSuffix(path, "/revoke") && strings.HasPrefix(path, "/api/devices/") {
		return PermissionDeviceRevoke
	}
	if strings.HasPrefix(path, "/api/devices") && r.Method != http.MethodGet {
		return PermissionDeviceRevoke
	}
	if strings.HasPrefix(path, "/api/command-templates") && r.Method != http.MethodGet {
		return PermissionTemplateManage
	}
	if strings.Contains(path, "/aiops/llm-") || strings.Contains(path, "/aiops/auto-mode") || strings.Contains(path, "/aiops/recommendations/") {
		return PermissionLLMManage
	}
	if r.Method == http.MethodGet || path == "/api/auth/change-password" || path == "/api/auth/logout" {
		return PermissionRead
	}
	return ""
}

func (a *App) withRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		key := "general:" + ip
		maxTokens := 60
		interval := time.Second
		if r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/register" {
			key, maxTokens, interval = "login:"+ip, 5, 3*time.Minute
		} else if r.URL.Path == "/api/v1/system/bootstrap" || r.URL.Path == "/api/setup/admin" {
			key, maxTokens, interval = "bootstrap:"+ip, 5, 3*time.Minute
		} else if r.URL.Path == "/api/agent/enroll" {
			key, maxTokens, interval = "enroll:"+ip, 10, time.Minute
		}
		a.rlMu.Lock()
		rl, exists := a.rateLimiters[key]
		if !exists {
			rl = newRateLimiter(maxTokens, interval)
			a.rateLimiters[key] = rl
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
		a.logger.Error("expire devices", "error", err)
	}
	taskCutoff := time.Now().UTC().Add(-a.config.TaskTimeout).Format(time.RFC3339)
	if err := a.store.TimeoutTasks(ctx, taskCutoff); err != nil {
		a.logger.Error("timeout tasks", "error", err)
	}
	if err := a.store.PruneExpiredWebSessions(ctx); err != nil {
		a.logger.Error("prune web sessions", "error", err)
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
			a.pruneRateLimiters()
		}
	}
}

func (a *App) pruneRateLimiters() {
	cutoff := time.Now().Add(-30 * time.Minute)
	a.rlMu.Lock()
	for key, limiter := range a.rateLimiters {
		if limiter.last.Before(cutoff) {
			delete(a.rateLimiters, key)
		}
	}
	a.rlMu.Unlock()
}
