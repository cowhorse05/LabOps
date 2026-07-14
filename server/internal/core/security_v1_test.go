package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func secureTestApp(t *testing.T) (*Store, *App, http.Handler) {
	t.Helper()
	store, err := OpenStore(DriverSQLite, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdatePassword(context.Background(), "admin", "secure-test-password"); err != nil {
		t.Fatal(err)
	}
	app := NewApp(store, Config{Environment: "production", PublicOrigin: "https://labops.test"}, nil)
	t.Cleanup(func() { app.Stop(); store.Close() })
	return store, app, app.Handler()
}

func loginSecure(t *testing.T, handler http.Handler, username, password string) []*http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Origin", "https://labops.test")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", w.Code, w.Body.String())
	}
	return w.Result().Cookies()
}

func authenticatedRequest(method, path string, body any, cookies []*http.Cookie, csrf bool) *http.Request {
	var encoded bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&encoded).Encode(body)
	}
	req := httptest.NewRequest(method, path, &encoded)
	req.Header.Set("Origin", "https://labops.test")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
		if csrf && cookie.Name == csrfCookieName {
			req.Header.Set("X-CSRF-Token", cookie.Value)
		}
	}
	return req
}

func TestSecureSessionCSRFAndRBAC(t *testing.T) {
	store, _, handler := secureTestApp(t)
	adminCookies := loginSecure(t, handler, "admin", "secure-test-password")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, authenticatedRequest(http.MethodPost, "/api/enrollment-codes", map[string]int{}, adminCookies, false))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected CSRF 403, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, authenticatedRequest(http.MethodPost, "/api/enrollment-codes", map[string]int{}, adminCookies, true))
	if w.Code != http.StatusCreated {
		t.Fatalf("admin enrollment status=%d body=%s", w.Code, w.Body.String())
	}

	if _, err := store.CreateUser(context.Background(), "operator1", "Operator", "operator-password-1", RoleOperator); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdatePassword(context.Background(), "operator1", "operator-password-1"); err != nil {
		t.Fatal(err)
	}
	operatorCookies := loginSecure(t, handler, "operator1", "operator-password-1")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, authenticatedRequest(http.MethodPost, "/api/enrollment-codes", map[string]int{}, operatorCookies, true))
	if w.Code != http.StatusForbidden {
		t.Fatalf("operator expected 403, got %d", w.Code)
	}
}

func TestEnrollmentCredentialLifecycle(t *testing.T) {
	store, app, handler := secureTestApp(t)
	code, err := store.CreateEnrollmentCode(context.Background(), "user_admin", 600000000000, 1)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"code": code.Code, "device": map[string]any{"name": "linux-host", "hostname": "host", "os": "ubuntu 22.04"}}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, authenticatedRequest(http.MethodPost, "/api/agent/enroll", body, nil, false))
	if w.Code != http.StatusCreated {
		t.Fatalf("enroll status=%d body=%s", w.Code, w.Body.String())
	}
	var enrolled struct{ DeviceID, DeviceSecret string }
	if err := json.NewDecoder(w.Body).Decode(&enrolled); err != nil {
		t.Fatal(err)
	}
	valid, err := store.ValidateDeviceCredential(context.Background(), enrolled.DeviceID, enrolled.DeviceSecret)
	if err != nil || !valid {
		t.Fatalf("credential valid=%v err=%v", valid, err)
	}

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, authenticatedRequest(http.MethodPost, "/api/agent/enroll", body, nil, false))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("reused code expected 401, got %d", w.Code)
	}

	if err := store.RevokeDeviceCredential(context.Background(), enrolled.DeviceID); err != nil {
		t.Fatal(err)
	}
	valid, _ = store.ValidateDeviceCredential(context.Background(), enrolled.DeviceID, enrolled.DeviceSecret)
	if valid {
		t.Fatal("revoked credential remained valid")
	}
	app.mu.RLock()
	_, connected := app.clients[enrolled.DeviceID]
	app.mu.RUnlock()
	if connected {
		t.Fatal("unexpected connected client")
	}
}

func TestMigrationAndEncryptedLLMSecret(t *testing.T) {
	store, err := OpenStore(DriverSQLite, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	if err := store.ConfigureEncryptionKey(key); err != nil {
		t.Fatal(err)
	}
	if err := store.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("migration is not idempotent: %v", err)
	}
	var versions int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&versions); err != nil || versions != 1 {
		t.Fatalf("migration rows=%d err=%v", versions, err)
	}
	if err := store.SaveLLMConfig(context.Background(), LLMConfig{APIKey: "top-secret", ProviderType: "openai"}); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := store.db.QueryRow("SELECT api_key FROM llm_config WHERE id = 1").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw == "top-secret" {
		t.Fatal("LLM secret stored in plaintext")
	}
	cfg, err := store.GetLLMConfig(context.Background())
	if err != nil || cfg.APIKey != "top-secret" {
		t.Fatalf("decrypted=%q err=%v", cfg.APIKey, err)
	}
}

func TestUpgradeLegacyUsersTable(t *testing.T) {
	store, err := OpenStore(DriverSQLite, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.db.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY NOT NULL, username TEXT NOT NULL UNIQUE, display_name TEXT NOT NULL,
		password TEXT NOT NULL, roles TEXT NOT NULL, must_change_password INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("upgrade legacy schema: %v", err)
	}
	user, ok, err := store.FindUser(context.Background(), "admin", "admin")
	if err != nil || !ok || user.Status != "active" {
		t.Fatalf("migrated user=%+v ok=%v err=%v", user, ok, err)
	}
}

func TestSecureBootstrapRejectsKnownOrMissingPassword(t *testing.T) {
	store, err := OpenStore(DriverSQLite, ":memory:")
	if err != nil { t.Fatal(err) }
	defer store.Close()
	// Empty password on an empty database: now accepted — server starts and
	// the setup API handles first-time admin creation.
	if err := store.InitSecure(context.Background(), ""); err != nil { t.Fatal("empty database with empty password should not error", err) }
	// Verify no user was auto-created (server is in setup-required state)
	count, _ := store.CountUsers(context.Background())
	if count != 0 { t.Fatal("expected 0 users after InitSecure with empty password") }
	// Valid bootstrap password: should auto-create admin
	if err := store.InitSecure(context.Background(), "a-secure-bootstrap-password"); err != nil { t.Fatal(err) }
	if _, ok, err := store.FindUser(context.Background(), "admin", "admin"); err != nil || ok { t.Fatalf("legacy default valid=%v err=%v", ok, err) }
}

func TestRenderTemplateDoesNotBuildShellCommand(t *testing.T) {
	item := CommandTemplate{Enabled: true, Executable: "/bin/echo", Args: []string{"{{value}}"}, Parameters: []TemplateParameter{{Name: "value", Pattern: `[a-z;& ]+`}}}
	args, err := RenderTemplate(item, map[string]any{"value": "hello; uname"})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 || args[0] != "hello; uname" {
		t.Fatalf("unexpected argv: %#v", args)
	}
}
