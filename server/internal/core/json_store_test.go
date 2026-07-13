package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONStoreFirstSetupEnrollmentAndSecretProtection(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenJSONStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	if err := store.ConfigureEncryptionKey(key); err != nil {
		t.Fatal(err)
	}
	if err := store.InitSecure(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	app := NewApp(store, Config{Environment: "production", PublicOrigin: "https://labops.test"})
	t.Cleanup(func() { app.Stop(); store.Close() })
	handler := app.Handler()

	status := httptest.NewRecorder()
	handler.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/setup/status", nil))
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"setupRequired":true`) {
		t.Fatalf("setup status=%d body=%s", status.Code, status.Body.String())
	}

	setupBody := map[string]string{"username": "admin", "password": "json-store-password", "confirmPassword": "json-store-password"}
	setup := httptest.NewRecorder()
	handler.ServeHTTP(setup, jsonRequest(http.MethodPost, "/api/setup/admin", setupBody, nil))
	if setup.Code != http.StatusCreated {
		t.Fatalf("setup admin status=%d body=%s", setup.Code, setup.Body.String())
	}
	cookies := setup.Result().Cookies()

	changeBody := map[string]string{"oldPassword": "json-store-password", "newPassword": "json-store-password-2"}
	change := httptest.NewRecorder()
	handler.ServeHTTP(change, authenticatedRequest(http.MethodPost, "/api/auth/change-password", changeBody, cookies, true))
	if change.Code != http.StatusOK {
		t.Fatalf("change password status=%d body=%s", change.Code, change.Body.String())
	}

	loginCookies := loginSecure(t, handler, "admin", "json-store-password-2")
	enroll := httptest.NewRecorder()
	handler.ServeHTTP(enroll, authenticatedRequest(http.MethodPost, "/api/enrollment-codes", map[string]int{"expiresInSeconds": 600, "maxUses": 1}, loginCookies, true))
	if enroll.Code != http.StatusCreated {
		t.Fatalf("create enrollment status=%d body=%s", enroll.Code, enroll.Body.String())
	}
	var code EnrollmentCode
	if err := json.NewDecoder(enroll.Body).Decode(&code); err != nil {
		t.Fatal(err)
	}
	if code.Code == "" {
		t.Fatal("enrollment response did not include one-time code")
	}

	agentBody := map[string]any{"code": code.Code, "device": map[string]any{"name": "aliyun-host", "groupName": "aliyun", "hostname": "iZmj7hc1iuj5wcbyl6tjgcZ", "os": "Alibaba Cloud Linux 3"}}
	agentEnroll := httptest.NewRecorder()
	handler.ServeHTTP(agentEnroll, jsonRequest(http.MethodPost, "/api/agent/enroll", agentBody, nil))
	if agentEnroll.Code != http.StatusCreated {
		t.Fatalf("agent enroll status=%d body=%s", agentEnroll.Code, agentEnroll.Body.String())
	}
	var enrolled struct {
		DeviceID     string `json:"deviceId"`
		DeviceSecret string `json:"deviceSecret"`
	}
	if err := json.NewDecoder(agentEnroll.Body).Decode(&enrolled); err != nil {
		t.Fatal(err)
	}
	valid, err := store.ValidateDeviceCredential(context.Background(), enrolled.DeviceID, enrolled.DeviceSecret)
	if err != nil || !valid {
		t.Fatalf("device credential valid=%v err=%v", valid, err)
	}

	templates, err := store.ListCommandTemplates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) < 5 {
		t.Fatalf("expected built-in command templates, got %d", len(templates))
	}

	if err := store.SaveLLMConfig(context.Background(), LLMConfig{ProviderType: "openai", APIKey: "json-secret-key"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := store.GetLLMConfig(context.Background())
	if err != nil || cfg.APIKey != "json-secret-key" {
		t.Fatalf("decrypted api key=%q err=%v", cfg.APIKey, err)
	}

	assertFileDoesNotContain(t, filepath.Join(dir, "enrollment_codes.json"), code.Code)
	assertFileDoesNotContain(t, filepath.Join(dir, "device_credentials.json"), enrolled.DeviceSecret)
	assertFileDoesNotContain(t, filepath.Join(dir, "llm_config.json"), "json-secret-key")
}

func jsonRequest(method, path string, body any, cookies []*http.Cookie) *http.Request {
	var encoded bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&encoded).Encode(body)
	}
	req := httptest.NewRequest(method, path, &encoded)
	req.Header.Set("Origin", "https://labops.test")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	return req
}

func assertFileDoesNotContain(t *testing.T, path, needle string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if needle != "" && strings.Contains(string(data), needle) {
		t.Fatalf("%s contains plaintext secret %q", path, needle)
	}
}
