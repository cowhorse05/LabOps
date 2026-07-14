package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSystemBootstrapSQLiteLifecycle(t *testing.T) {
	store, err := OpenStore(DriverSQLite, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InitSecure(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	app := NewApp(store, Config{Environment: "production", PublicOrigin: "https://labops.test"}, nil)
	t.Cleanup(func() { app.Stop(); store.Close() })
	handler := app.Handler()

	status := httptest.NewRecorder()
	handler.ServeHTTP(status, jsonRequest(http.MethodGet, "/api/v1/system/status", nil, nil))
	if status.Code != http.StatusOK {
		t.Fatalf("status code=%d body=%s", status.Code, status.Body.String())
	}
	var before SetupStatus
	if err := json.NewDecoder(status.Body).Decode(&before); err != nil {
		t.Fatal(err)
	}
	if before.Initialized || before.ActiveAdminExists || !before.RegistrationAllowed {
		t.Fatalf("unexpected initial status: %+v", before)
	}

	body := map[string]string{"username": "admin", "displayName": "Administrator", "password": "bootstrap-password-1", "confirmPassword": "bootstrap-password-1"}
	create := httptest.NewRecorder()
	handler.ServeHTTP(create, jsonRequest(http.MethodPost, "/api/v1/system/bootstrap", body, nil))
	if create.Code != http.StatusCreated {
		t.Fatalf("bootstrap code=%d body=%s", create.Code, create.Body.String())
	}
	loginSecure(t, handler, "admin", "bootstrap-password-1")

	status = httptest.NewRecorder()
	handler.ServeHTTP(status, jsonRequest(http.MethodGet, "/api/v1/system/status", nil, nil))
	var after SetupStatus
	if err := json.NewDecoder(status.Body).Decode(&after); err != nil {
		t.Fatal(err)
	}
	if !after.Initialized || !after.ActiveAdminExists || after.RegistrationAllowed {
		t.Fatalf("unexpected initialized status: %+v", after)
	}

	again := httptest.NewRecorder()
	handler.ServeHTTP(again, jsonRequest(http.MethodPost, "/api/v1/system/bootstrap", body, nil))
	if again.Code != http.StatusConflict || !strings.Contains(again.Body.String(), "BOOTSTRAP_CLOSED") {
		t.Fatalf("second bootstrap code=%d body=%s", again.Code, again.Body.String())
	}
}

func TestSystemBootstrapValidationAndRecovery(t *testing.T) {
	store, err := OpenStore(DriverSQLite, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InitSecure(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateUser(context.Background(), "viewer1", "Viewer", "viewer-password-1", RoleViewer); err != nil {
		t.Fatal(err)
	}
	app := NewApp(store, Config{Environment: "production", PublicOrigin: "https://labops.test"}, nil)
	t.Cleanup(func() { app.Stop(); store.Close() })
	handler := app.Handler()

	status := httptest.NewRecorder()
	handler.ServeHTTP(status, jsonRequest(http.MethodGet, "/api/v1/system/status", nil, nil))
	var current SetupStatus
	if err := json.NewDecoder(status.Body).Decode(&current); err != nil {
		t.Fatal(err)
	}
	if !current.RecoveryRequired || !current.RegistrationAllowed {
		t.Fatalf("expected recovery status, got %+v", current)
	}

	cases := []struct {
		name string
		body map[string]string
		code int
	}{
		{"empty username", map[string]string{"username": "", "password": "bootstrap-password-1", "confirmPassword": "bootstrap-password-1"}, http.StatusBadRequest},
		{"short password", map[string]string{"username": "admin", "password": "short", "confirmPassword": "short"}, http.StatusBadRequest},
		{"mismatch", map[string]string{"username": "admin", "password": "bootstrap-password-1", "confirmPassword": "bootstrap-password-2"}, http.StatusBadRequest},
		{"duplicate username", map[string]string{"username": "viewer1", "password": "bootstrap-password-1", "confirmPassword": "bootstrap-password-1"}, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/v1/system/bootstrap", tc.body, nil))
			if w.Code != tc.code {
				t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
			}
		})
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, jsonRequest(http.MethodPost, "/api/v1/system/bootstrap", map[string]string{"username": "admin", "password": "bootstrap-password-1", "confirmPassword": "bootstrap-password-1"}, nil))
	if w.Code != http.StatusCreated {
		t.Fatalf("recovery bootstrap code=%d body=%s", w.Code, w.Body.String())
	}
	loginSecure(t, handler, "admin", "bootstrap-password-1")
}

func TestJSONBootstrapPersistenceCorruptionAndConcurrency(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenJSONStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InitSecure(context.Background(), ""); err != nil {
		t.Fatal(err)
	}

	const workers = 8
	var wg sync.WaitGroup
	results := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.BootstrapFirstAdmin(context.Background(), BootstrapAdminInput{
				Username:        "admin" + string(rune('a'+i)),
				DisplayName:     "Admin",
				Password:        "json-bootstrap-password",
				ConfirmPassword: "json-bootstrap-password",
			})
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one bootstrap success, got %d", successes)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenJSONStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	status, err := reopened.SetupStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.ActiveAdminExists || !status.Initialized {
		t.Fatalf("expected persisted active admin, got %+v", status)
	}
	data, err := os.ReadFile(filepath.Join(dir, "users.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "json-bootstrap-password") {
		t.Fatal("users.json contains plaintext password")
	}

	corruptDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(corruptDir, "users.json"), []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenJSONStore(corruptDir); err == nil || !strings.Contains(err.Error(), "parse users.json") {
		t.Fatalf("expected clear corrupt json error, got %v", err)
	}
}
