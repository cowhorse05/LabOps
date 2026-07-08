package core

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealth(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %q", body["status"])
	}
}

func TestHandleLogin_Valid(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	body := loginRequest{Username: "admin", Password: "admin"}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	req := httptest.NewRequest("POST", "/api/auth/login", &buf)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp loginResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.User.Username != "admin" {
		t.Fatalf("expected username admin, got %s", resp.User.Username)
	}
	if len(resp.User.Roles) == 0 {
		t.Fatal("expected user to have roles")
	}
}

func TestHandleLogin_Invalid(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	body := loginRequest{Username: "admin", Password: "wrong"}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	req := httptest.NewRequest("POST", "/api/auth/login", &buf)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	var errResp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if errResp["error"] == "" {
		t.Fatal("expected error message in response")
	}
}

func TestHandleMe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var user User
	if err := json.NewDecoder(w.Body).Decode(&user); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("expected admin, got %s", user.Username)
	}
	if user.DisplayName != "LabOps Admin" {
		t.Fatalf("expected LabOps Admin, got %s", user.DisplayName)
	}
}

func TestHandleStats_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/stats", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var stats DeviceStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if stats.Total != 0 {
		t.Fatalf("expected total=0, got %d", stats.Total)
	}
	if stats.Online != 0 {
		t.Fatalf("expected online=0, got %d", stats.Online)
	}
	if stats.Offline != 0 {
		t.Fatalf("expected offline=0, got %d", stats.Offline)
	}
}

func TestHandleDevices_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/devices", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var devices []Device
	if err := json.NewDecoder(w.Body).Decode(&devices); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(devices) != 0 {
		t.Fatalf("expected 0 devices, got %d", len(devices))
	}
}

func TestHandleDevices_WithData(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)

	device := Device{
		ID:          "test-device",
		Name:        "test-agent",
		GroupName:   "lab",
		Profile:     "ubuntu",
		Version:     "1.0",
		Hostname:    "test-agent",
		OS:          "Ubuntu",
		IP:          "10.10.0.10",
		CPUCores:    4,
		MemoryMB:    4096,
		DiskTotalGB: 64,
		Status:      StatusOnline,
	}
	if err := store.UpsertDevice(ctx, device); err != nil {
		t.Fatalf("upsert device: %v", err)
	}

	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/devices", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var devices []Device
	if err := json.NewDecoder(w.Body).Decode(&devices); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].ID != "test-device" {
		t.Fatalf("expected id test-device, got %s", devices[0].ID)
	}
	if devices[0].Name != "test-agent" {
		t.Fatalf("expected name test-agent, got %s", devices[0].Name)
	}
	if devices[0].Status != StatusOnline {
		t.Fatalf("expected status online, got %s", devices[0].Status)
	}
}

func TestHandleGetDevice_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/devices/nonexistent", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	var errResp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if errResp["error"] == "" {
		t.Fatal("expected error message in response")
	}
}

func TestHandleGroups(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)

	devices := []Device{
		{
			ID: "g1-a", Name: "g1-a", GroupName: "group-a",
			Profile: "ubuntu", Version: "1.0", Hostname: "g1-a",
			OS: "Ubuntu", IP: "10.0.1.1", CPUCores: 2, MemoryMB: 2048,
			DiskTotalGB: 32, Status: StatusOnline,
		},
		{
			ID: "g1-b", Name: "g1-b", GroupName: "group-a",
			Profile: "ubuntu", Version: "1.0", Hostname: "g1-b",
			OS: "Ubuntu", IP: "10.0.1.2", CPUCores: 2, MemoryMB: 2048,
			DiskTotalGB: 32, Status: StatusOffline,
		},
		{
			ID: "g2-a", Name: "g2-a", GroupName: "group-b",
			Profile: "ubuntu", Version: "1.0", Hostname: "g2-a",
			OS: "Ubuntu", IP: "10.0.2.1", CPUCores: 4, MemoryMB: 4096,
			DiskTotalGB: 64, Status: StatusOnline,
		},
	}
	for _, d := range devices {
		if err := store.UpsertDevice(ctx, d); err != nil {
			t.Fatalf("upsert device %s: %v", d.ID, err)
		}
	}

	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/groups", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var groups []DeviceGroup
	if err := json.NewDecoder(w.Body).Decode(&groups); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	for _, g := range groups {
		switch g.Name {
		case "group-a":
			if g.Total != 2 {
				t.Fatalf("expected group-a total=2, got %d", g.Total)
			}
			if g.Online != 1 {
				t.Fatalf("expected group-a online=1, got %d", g.Online)
			}
		case "group-b":
			if g.Total != 1 {
				t.Fatalf("expected group-b total=1, got %d", g.Total)
			}
			if g.Online != 1 {
				t.Fatalf("expected group-b online=1, got %d", g.Online)
			}
		default:
			t.Fatalf("unexpected group name: %s", g.Name)
		}
	}
}

func TestHandleCreateTask_SingleDevice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)

	device := Device{
		ID: "task-device", Name: "task-agent", GroupName: "lab",
		Profile: "ubuntu", Version: "1.0", Hostname: "task-agent",
		OS: "Ubuntu", IP: "10.10.0.20", CPUCores: 4, MemoryMB: 4096,
		DiskTotalGB: 64, Status: StatusOnline,
	}
	if err := store.UpsertDevice(ctx, device); err != nil {
		t.Fatalf("upsert device: %v", err)
	}

	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	reqBody := createTaskRequest{DeviceID: "task-device", Command: "echo hello"}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(reqBody)

	req := httptest.NewRequest("POST", "/api/tasks", &buf)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp createTaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(resp.Tasks))
	}
	task := resp.Tasks[0]
	if task.DeviceID != "task-device" {
		t.Fatalf("expected deviceId task-device, got %s", task.DeviceID)
	}
	if task.DeviceName != "task-agent" {
		t.Fatalf("expected deviceName task-agent, got %s", task.DeviceName)
	}
	if task.Command != "echo hello" {
		t.Fatalf("expected command 'echo hello', got %s", task.Command)
	}
	if task.Status != StatusPending {
		t.Fatalf("expected status pending, got %s", task.Status)
	}
	if task.RequestedBy != "admin" {
		t.Fatalf("expected requestedBy admin, got %s", task.RequestedBy)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty task ID")
	}
}

func TestHandleCreateTask_MissingCommand(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	reqBody := createTaskRequest{DeviceID: "some-device"}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(reqBody)

	req := httptest.NewRequest("POST", "/api/tasks", &buf)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var errResp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if errResp["error"] == "" {
		t.Fatal("expected error message in response")
	}
}

func TestHandleCreateTask_MissingTarget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	reqBody := createTaskRequest{Command: "echo hello"}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(reqBody)

	req := httptest.NewRequest("POST", "/api/tasks", &buf)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var errResp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if errResp["error"] == "" {
		t.Fatal("expected error message in response")
	}
}

func TestHandleCreateTask_DeviceNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	reqBody := createTaskRequest{DeviceID: "nonexistent-device", Command: "echo hello"}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(reqBody)

	req := httptest.NewRequest("POST", "/api/tasks", &buf)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	var errResp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if errResp["error"] == "" {
		t.Fatal("expected error message in response")
	}
}

func TestHandleTasks_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var tasks []Task
	if err := json.NewDecoder(w.Body).Decode(&tasks); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestHandleAudit_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/audit-logs", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var logs []AuditLog
	if err := json.NewDecoder(w.Body).Decode(&logs); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 audit logs, got %d", len(logs))
	}
}

func TestWithAuth_NoToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "secret-token", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/devices", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestWithAuth_ValidToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "secret-token", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/devices", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestWithAuth_InvalidToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "secret-token", AgentToken: ""})
	handler := app.Handler()

	req := httptest.NewRequest("GET", "/api/devices", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestWithAuth_SkipPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := OpenStore(":memory:")
	defer store.Close()
	store.Init(ctx)
	app := NewApp(store, Config{WebToken: "secret-token", AgentToken: ""})
	handler := app.Handler()

	// /api/health should be accessible without token
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/health, got %d", w.Code)
	}

	// /api/auth/login should be accessible without token
	loginBody := loginRequest{Username: "admin", Password: "admin"}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(loginBody)
	req2 := httptest.NewRequest("POST", "/api/auth/login", &buf)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/auth/login, got %d", w2.Code)
	}
}
