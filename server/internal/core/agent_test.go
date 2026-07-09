package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// deviceFromRegister – pure data transform
// ---------------------------------------------------------------------------

func TestDeviceFromRegister(t *testing.T) {
	t.Parallel()

	t.Run("full payload maps all fields", func(t *testing.T) {
		t.Parallel()
		reg := RegisterPayload{
			AgentID:     "dev-1",
			Name:        "my-device",
			GroupName:   "production",
			Profile:     "ubuntu-22.04",
			Version:     "2.1.0",
			Hostname:    "my-device.example.com",
			OS:          "Ubuntu 22.04",
			IP:          "10.0.0.1",
			CPUCores:    8,
			MemoryMB:    16384,
			DiskTotalGB: 512,
		}
		dev := deviceFromRegister(reg)

		if dev.ID != "dev-1" {
			t.Errorf("ID = %q, want %q", dev.ID, "dev-1")
		}
		if dev.Name != "my-device" {
			t.Errorf("Name = %q, want %q", dev.Name, "my-device")
		}
		if dev.GroupName != "production" {
			t.Errorf("GroupName = %q, want %q", dev.GroupName, "production")
		}
		if dev.Profile != "ubuntu-22.04" {
			t.Errorf("Profile = %q, want %q", dev.Profile, "ubuntu-22.04")
		}
		if dev.Version != "2.1.0" {
			t.Errorf("Version = %q, want %q", dev.Version, "2.1.0")
		}
		if dev.Hostname != "my-device.example.com" {
			t.Errorf("Hostname = %q, want %q", dev.Hostname, "my-device.example.com")
		}
		if dev.OS != "Ubuntu 22.04" {
			t.Errorf("OS = %q, want %q", dev.OS, "Ubuntu 22.04")
		}
		if dev.IP != "10.0.0.1" {
			t.Errorf("IP = %q, want %q", dev.IP, "10.0.0.1")
		}
		if dev.CPUCores != 8 {
			t.Errorf("CPUCores = %d, want %d", dev.CPUCores, 8)
		}
		if dev.MemoryMB != 16384 {
			t.Errorf("MemoryMB = %d, want %d", dev.MemoryMB, 16384)
		}
		if dev.DiskTotalGB != 512 {
			t.Errorf("DiskTotalGB = %d, want %d", dev.DiskTotalGB, 512)
		}
		if dev.Status != StatusOnline {
			t.Errorf("Status = %q, want %q", dev.Status, StatusOnline)
		}
		if dev.CreatedAt == "" {
			t.Error("CreatedAt should not be empty")
		}
		if dev.UpdatedAt == "" {
			t.Error("UpdatedAt should not be empty")
		}
		if dev.LastSeen == "" {
			t.Error("LastSeen should not be empty")
		}
	})

	t.Run("empty AgentID yields empty device ID", func(t *testing.T) {
		t.Parallel()
		reg := RegisterPayload{
			AgentID: "",
			Name:    "no-id",
		}
		dev := deviceFromRegister(reg)
		if dev.ID != "" {
			t.Errorf("ID = %q, want empty", dev.ID)
		}
	})

	t.Run("empty Name yields empty device name", func(t *testing.T) {
		t.Parallel()
		reg := RegisterPayload{
			AgentID: "dev-2",
			Name:    "",
		}
		dev := deviceFromRegister(reg)
		if dev.Name != "" {
			t.Errorf("Name = %q, want empty", dev.Name)
		}
	})

	t.Run("GroupName defaults to 'default' when empty", func(t *testing.T) {
		t.Parallel()
		reg := RegisterPayload{AgentID: "d", Name: "d"}
		dev := deviceFromRegister(reg)
		if dev.GroupName != "default" {
			t.Errorf("GroupName = %q, want %q", dev.GroupName, "default")
		}
	})

	t.Run("Profile defaults to 'generic' when empty", func(t *testing.T) {
		t.Parallel()
		reg := RegisterPayload{AgentID: "d", Name: "d"}
		dev := deviceFromRegister(reg)
		if dev.Profile != "generic" {
			t.Errorf("Profile = %q, want %q", dev.Profile, "generic")
		}
	})

	t.Run("Version defaults to 'dev' when empty", func(t *testing.T) {
		t.Parallel()
		reg := RegisterPayload{AgentID: "d", Name: "d"}
		dev := deviceFromRegister(reg)
		if dev.Version != "dev" {
			t.Errorf("Version = %q, want %q", dev.Version, "dev")
		}
	})

	t.Run("whitespace trimming on all string fields", func(t *testing.T) {
		t.Parallel()
		reg := RegisterPayload{
			AgentID:   "  dev-3  ",
			Name:      "  spaced-name  ",
			GroupName: "  group-x  ",
			Profile:   "  custom  ",
			Version:   "  1.0  ",
			Hostname:  "  host  ",
			OS:        "  Linux  ",
			IP:        "  10.0.0.2  ",
		}
		dev := deviceFromRegister(reg)
		if dev.ID != "dev-3" {
			t.Errorf("ID = %q, want %q", dev.ID, "dev-3")
		}
		if dev.Name != "spaced-name" {
			t.Errorf("Name = %q, want %q", dev.Name, "spaced-name")
		}
		if dev.GroupName != "group-x" {
			t.Errorf("GroupName = %q, want %q", dev.GroupName, "group-x")
		}
		if dev.Profile != "custom" {
			t.Errorf("Profile = %q, want %q", dev.Profile, "custom")
		}
		if dev.Version != "1.0" {
			t.Errorf("Version = %q, want %q", dev.Version, "1.0")
		}
		if dev.Hostname != "host" {
			t.Errorf("Hostname = %q, want %q", dev.Hostname, "host")
		}
		if dev.OS != "Linux" {
			t.Errorf("OS = %q, want %q", dev.OS, "Linux")
		}
		if dev.IP != "10.0.0.2" {
			t.Errorf("IP = %q, want %q", dev.IP, "10.0.0.2")
		}
	})

	t.Run("whitespace-only fields fall back to defaults", func(t *testing.T) {
		t.Parallel()
		reg := RegisterPayload{
			AgentID:   "d",
			Name:      "d",
			GroupName: "   ",
			Profile:   "   ",
			Version:   "   ",
		}
		dev := deviceFromRegister(reg)
		if dev.GroupName != "default" {
			t.Errorf("GroupName = %q, want %q", dev.GroupName, "default")
		}
		if dev.Profile != "generic" {
			t.Errorf("Profile = %q, want %q", dev.Profile, "generic")
		}
		if dev.Version != "dev" {
			t.Errorf("Version = %q, want %q", dev.Version, "dev")
		}
	})

	t.Run("zero-valued numeric fields pass through", func(t *testing.T) {
		t.Parallel()
		reg := RegisterPayload{
			AgentID:     "d",
			Name:        "d",
			CPUCores:    0,
			MemoryMB:    0,
			DiskTotalGB: 0,
		}
		dev := deviceFromRegister(reg)
		if dev.CPUCores != 0 {
			t.Errorf("CPUCores = %d, want 0", dev.CPUCores)
		}
		if dev.MemoryMB != 0 {
			t.Errorf("MemoryMB = %d, want 0", dev.MemoryMB)
		}
		if dev.DiskTotalGB != 0 {
			t.Errorf("DiskTotalGB = %d, want 0", dev.DiskTotalGB)
		}
	})
}

// ---------------------------------------------------------------------------
// registerClient / unregisterClient – client map lifecycle
// ---------------------------------------------------------------------------

func TestRegisterClient(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	app := NewApp(store, Config{})
	defer app.Stop()

	c1 := &AgentClient{deviceID: "dev-a", sessionID: 1, closed: true}
	c2 := &AgentClient{deviceID: "dev-a", sessionID: 2, closed: true}

	t.Run("registers a new client", func(t *testing.T) {
		app.registerClient(c1)
		app.mu.RLock()
		got := app.clients["dev-a"]
		app.mu.RUnlock()
		if got != c1 {
			t.Fatalf("clients[dev-a] = %v, want c1", got)
		}
	})

	t.Run("replaces existing client and closes the old one", func(t *testing.T) {
		app.registerClient(c2)
		app.mu.RLock()
		got := app.clients["dev-a"]
		app.mu.RUnlock()
		if got != c2 {
			t.Fatalf("clients[dev-a] = %v, want c2", got)
		}
		if !c1.closed {
			t.Error("old client should have been closed")
		}
	})
}

func TestUnregisterClient(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	app := NewApp(store, Config{})
	defer app.Stop()

	client := &AgentClient{deviceID: "dev-b", sessionID: 10, closed: true}
	app.registerClient(client)

	t.Run("removes client from map", func(t *testing.T) {
		app.unregisterClient(client)
		app.mu.RLock()
		_, exists := app.clients["dev-b"]
		app.mu.RUnlock()
		if exists {
			t.Error("client should have been removed from map")
		}
	})

	t.Run("no-op when current differs from client", func(t *testing.T) {
		other := &AgentClient{deviceID: "dev-c", sessionID: 20, closed: true}
		app.registerClient(other)

		different := &AgentClient{deviceID: "dev-c", sessionID: 30, closed: true}
		app.unregisterClient(different) // different pointer, should not remove

		app.mu.RLock()
		got := app.clients["dev-c"]
		app.mu.RUnlock()
		if got == nil {
			t.Error("client should not have been removed when current differs")
		}
	})
}

// ---------------------------------------------------------------------------
// dispatchTask – nil client path (no WebSocket connection)
// ---------------------------------------------------------------------------

func TestDispatchTask_NoClient(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	app := NewApp(store, Config{})
	defer app.Stop()

	// Seed a device so the audit log can resolve a name later.
	device := Device{
		ID: "orphan-device", Name: "orphan", GroupName: "lab",
		Profile: "generic", Version: "dev", Hostname: "orphan",
		OS: "Linux", IP: "10.0.0.1", CPUCores: 2, MemoryMB: 2048,
		DiskTotalGB: 32, Status: StatusOnline,
	}
	if err := store.UpsertDevice(ctx, device); err != nil {
		t.Fatalf("UpsertDevice: %v", err)
	}

	task := Task{
		ID:          "task-1",
		DeviceID:    "orphan-device",
		Command:     "echo hello",
		RequestedBy: "admin",
		Status:      StatusPending,
	}

	// dispatchTask with no connected client for this device should create a
	// "command.queue" audit log rather than trying to send over WebSocket.
	if err := app.dispatchTask(ctx, task); err != nil {
		t.Fatalf("dispatchTask: %v", err)
	}

	logs, err := store.ListAudit(ctx)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(logs) == 0 {
		t.Fatal("expected at least one audit log")
	}
	found := false
	for _, l := range logs {
		if l.Action == "command.queue" && l.TaskID == "task-1" {
			found = true
			if l.Status != StatusPending {
				t.Errorf("audit status = %q, want %q", l.Status, StatusPending)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected a command.queue audit log for task-1")
	}
}

// ---------------------------------------------------------------------------
// dispatchPendingTasks – fan-out dispatch for pending tasks
// ---------------------------------------------------------------------------

func TestDispatchPendingTasks(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	app := NewApp(store, Config{})
	defer app.Stop()

	device := Device{
		ID: "pending-dev", Name: "pending-agent", GroupName: "lab",
		Profile: "generic", Version: "dev", Hostname: "pending-agent",
		OS: "Linux", IP: "10.0.0.2", CPUCores: 2, MemoryMB: 2048,
		DiskTotalGB: 32, Status: StatusOnline,
	}
	if err := store.UpsertDevice(ctx, device); err != nil {
		t.Fatalf("UpsertDevice: %v", err)
	}

	// Create pending tasks directly in the store.
	for i := 0; i < 3; i++ {
		id := string(rune('a' + i))
		_, err := store.CreateTask(ctx, "pending-dev", "lab", "echo task-"+id, "admin")
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
	}

	t.Run("dispatches all pending tasks for device", func(t *testing.T) {
		if err := app.dispatchPendingTasks(ctx, "pending-dev"); err != nil {
			t.Fatalf("dispatchPendingTasks: %v", err)
		}

		logs, err := store.ListAudit(ctx)
		if err != nil {
			t.Fatalf("ListAudit: %v", err)
		}
		var queueLogs int
		for _, l := range logs {
			if l.Action == "command.queue" {
				queueLogs++
			}
		}
		if queueLogs != 3 {
			t.Fatalf("expected 3 command.queue audit logs, got %d", queueLogs)
		}
	})

	t.Run("no pending tasks returns nil", func(t *testing.T) {
		if err := app.dispatchPendingTasks(ctx, "unknown-device"); err != nil {
			t.Fatalf("dispatchPendingTasks for unknown device: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// refreshState – maintenance loop helper
// ---------------------------------------------------------------------------

func TestRefreshState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	app := NewApp(store, Config{HeartbeatTimeout: 1 * time.Nanosecond, TaskTimeout: 1 * time.Nanosecond})
	defer app.Stop()

	// Seed an online device that should be expired.
	old := Device{
		ID: "stale-device", Name: "stale", GroupName: "lab",
		Profile: "generic", Version: "dev", Hostname: "stale",
		OS: "Linux", IP: "10.0.0.3", CPUCores: 2, MemoryMB: 2048,
		DiskTotalGB: 32, Status: StatusOnline,
		LastSeen: "2000-01-01T00:00:00Z",
	}
	if err := store.UpsertDevice(ctx, old); err != nil {
		t.Fatalf("UpsertDevice: %v", err)
	}

	app.refreshState(ctx)

	d, found, err := store.GetDevice(ctx, "stale-device")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if !found {
		t.Fatal("device should still exist")
	}
	if d.Status != StatusOffline {
		t.Errorf("device status = %q, want %q (should have been expired)", d.Status, StatusOffline)
	}
}

// ---------------------------------------------------------------------------
// AgentClient.Send – error path when client is closed
// ---------------------------------------------------------------------------

func TestAgentClientSend_Closed(t *testing.T) {
	t.Parallel()
	client := &AgentClient{closed: true}
	err := client.Send("hello")
	if err == nil {
		t.Fatal("expected error when sending on closed client")
	}
	if !strings.Contains(err.Error(), "client is closed") {
		t.Errorf("error = %q, want substring %q", err.Error(), "client is closed")
	}
}

// ---------------------------------------------------------------------------
// AgentClient.Close – idempotent double close
// ---------------------------------------------------------------------------

func TestAgentClientClose_Idempotent(t *testing.T) {
	t.Parallel()
	client := &AgentClient{closed: true}
	// Close on an already-closed client should not panic.
	client.Close()
	client.Close()
	// If we got here without a nil pointer dereference, the test passes.
}

// ---------------------------------------------------------------------------
// rateLimiter – used by withRateLimit which protects /api/agent/ws
// ---------------------------------------------------------------------------

func TestRateLimiter(t *testing.T) {
	t.Parallel()

	t.Run("allows up to max tokens", func(t *testing.T) {
		rl := newRateLimiter(3, time.Hour) // very long interval so tokens don't refill
		if !rl.allow() {
			t.Error("expected allow #1")
		}
		if !rl.allow() {
			t.Error("expected allow #2")
		}
		if !rl.allow() {
			t.Error("expected allow #3")
		}
		if rl.allow() {
			t.Error("expected deny #4 (exhausted)")
		}
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		rl := newRateLimiter(1, 50*time.Millisecond)
		if !rl.allow() {
			t.Error("expected allow (initial token)")
		}
		if rl.allow() {
			t.Error("expected deny (no refill yet)")
		}
		time.Sleep(60 * time.Millisecond)
		if !rl.allow() {
			t.Error("expected allow after refill")
		}
	})

	t.Run("caps tokens at max", func(t *testing.T) {
		rl := newRateLimiter(5, 10*time.Millisecond)
		time.Sleep(100 * time.Millisecond) // enough to accumulate many tokens
		for i := 0; i < 5; i++ {
			if !rl.allow() {
				t.Fatalf("expected allow #%d", i+1)
			}
		}
		if rl.allow() {
			t.Error("expected deny after cap exhausted")
		}
	})
}

// ---------------------------------------------------------------------------
// NewApp defaults
// ---------------------------------------------------------------------------

func TestNewAppDefaults(t *testing.T) {
	t.Parallel()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	app := NewApp(store, Config{})
	defer app.Stop()

	if app.config.HeartbeatTimeout != 35*time.Second {
		t.Errorf("HeartbeatTimeout = %v, want 35s", app.config.HeartbeatTimeout)
	}
	if app.config.TaskTimeout != 2*time.Minute {
		t.Errorf("TaskTimeout = %v, want 2m", app.config.TaskTimeout)
	}
	if app.clients == nil {
		t.Error("clients map should be initialized")
	}
	if app.rateLimiters == nil {
		t.Error("rateLimiters map should be initialized")
	}
	if app.upgrader.CheckOrigin == nil {
		t.Error("upgrader.CheckOrigin should be set")
	}
	if !app.upgrader.CheckOrigin(nil) {
		t.Error("CheckOrigin should return true")
	}
}

// ---------------------------------------------------------------------------
// handleAgentWS – WebSocket integration tests
// ---------------------------------------------------------------------------

func TestHandleAgentWS_TokenAuth(t *testing.T) {
	t.Parallel()

	t.Run("missing token returns 401", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		store, err := OpenStore(":memory:")
		if err != nil {
			t.Fatalf("OpenStore: %v", err)
		}
		defer store.Close()
		if err := store.Init(ctx); err != nil {
			t.Fatalf("Init: %v", err)
		}
		app := NewApp(store, Config{AgentToken: "test-token"})
		defer app.Stop()

		srv := httptest.NewServer(app.Handler())
		defer srv.Close()

		req, _ := http.NewRequest("GET", srv.URL+"/api/agent/ws", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("wrong token returns 401", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		store, err := OpenStore(":memory:")
		if err != nil {
			t.Fatalf("OpenStore: %v", err)
		}
		defer store.Close()
		if err := store.Init(ctx); err != nil {
			t.Fatalf("Init: %v", err)
		}
		app := NewApp(store, Config{AgentToken: "correct-token"})
		defer app.Stop()

		srv := httptest.NewServer(app.Handler())
		defer srv.Close()

		req, _ := http.NewRequest("GET", srv.URL+"/api/agent/ws", nil)
		req.Header.Set("X-Agent-Token", "wrong-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("valid token upgrades successfully", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		store, err := OpenStore(":memory:")
		if err != nil {
			t.Fatalf("OpenStore: %v", err)
		}
		defer store.Close()
		if err := store.Init(ctx); err != nil {
			t.Fatalf("Init: %v", err)
		}
		app := NewApp(store, Config{AgentToken: "valid-token"})
		defer app.Stop()

		srv := httptest.NewServer(app.Handler())
		defer srv.Close()

		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/agent/ws"
		header := http.Header{}
		header.Set("X-Agent-Token", "valid-token")
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			t.Fatalf("dial: %v (status: %d)", err, resp.StatusCode)
		}
		defer conn.Close()

		if resp.StatusCode != http.StatusSwitchingProtocols {
			t.Fatalf("expected 101 Switching Protocols, got %d", resp.StatusCode)
		}
	})
}

func TestHandleAgentWS_Register(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	app := NewApp(store, Config{AgentToken: "reg-token"})
	defer app.Stop()

	srv := httptest.NewServer(app.Handler())
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/agent/ws"
	header := http.Header{}
	header.Set("X-Agent-Token", "reg-token")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send register message.
	registerMsg := map[string]any{
		"type": "register",
		"payload": map[string]any{
			"agentId":   "dev-ws-1",
			"name":      "ws-agent-1",
			"groupName": "ws-group",
			"hostname":  "ws-agent-1.local",
			"os":        "linux",
			"ip":        "10.0.0.100",
		},
	}
	if err := conn.WriteJSON(registerMsg); err != nil {
		t.Fatalf("write register: %v", err)
	}

	// Read registered response.
	var response map[string]any
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatalf("read response: %v", err)
	}
	if response["type"] != "registered" {
		t.Fatalf("expected type 'registered', got %q", response["type"])
	}
	payload, ok := response["payload"].(map[string]any)
	if !ok {
		t.Fatal("payload is missing or not an object")
	}
	deviceID, ok := payload["deviceId"].(string)
	if !ok || deviceID == "" {
		t.Fatalf("expected non-empty deviceId in payload, got %v", payload["deviceId"])
	}
	if deviceID != "dev-ws-1" {
		t.Fatalf("expected deviceId 'dev-ws-1', got %q", deviceID)
	}

	// Verify device was created in store.
	dev, found, err := store.GetDevice(ctx, "dev-ws-1")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if !found {
		t.Fatal("device not found in store after register")
	}
	if dev.Name != "ws-agent-1" {
		t.Errorf("device name = %q, want %q", dev.Name, "ws-agent-1")
	}
	if dev.Status != StatusOnline {
		t.Errorf("device status = %q, want %q", dev.Status, StatusOnline)
	}
	if dev.GroupName != "ws-group" {
		t.Errorf("device group = %q, want %q", dev.GroupName, "ws-group")
	}

	// Verify audit log was created.
	logs, err := store.ListAudit(ctx)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	foundRegister := false
	for _, l := range logs {
		if l.Action == "agent.register" && l.DeviceID == "dev-ws-1" && l.Status == StatusSuccess {
			foundRegister = true
			break
		}
	}
	if !foundRegister {
		t.Fatal("expected agent.register audit log entry")
	}
}

func TestHandleAgentWS_InvalidRegister(t *testing.T) {
	t.Parallel()

	t.Run("first message wrong type", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		store, err := OpenStore(":memory:")
		if err != nil {
			t.Fatalf("OpenStore: %v", err)
		}
		defer store.Close()
		if err := store.Init(ctx); err != nil {
			t.Fatalf("Init: %v", err)
		}
		app := NewApp(store, Config{AgentToken: "inv-token"})
		defer app.Stop()

		srv := httptest.NewServer(app.Handler())
		defer srv.Close()

		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/agent/ws"
		header := http.Header{}
		header.Set("X-Agent-Token", "inv-token")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close()

		// Send heartbeat as first message — should be rejected.
		heartbeatMsg := map[string]any{
			"type":    "heartbeat",
			"payload": map[string]any{},
		}
		if err := conn.WriteJSON(heartbeatMsg); err != nil {
			t.Fatalf("write heartbeat: %v", err)
		}

		var response map[string]any
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatalf("read response: %v", err)
		}
		if response["type"] != "error" {
			t.Fatalf("expected type 'error', got %q", response["type"])
		}
		msg, _ := response["message"].(string)
		if !strings.Contains(msg, "first message must be register") {
			t.Errorf("expected error about register, got %q", msg)
		}
	})

	t.Run("missing agentId and name", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		store, err := OpenStore(":memory:")
		if err != nil {
			t.Fatalf("OpenStore: %v", err)
		}
		defer store.Close()
		if err := store.Init(ctx); err != nil {
			t.Fatalf("Init: %v", err)
		}
		app := NewApp(store, Config{AgentToken: "inv-token"})
		defer app.Stop()

		srv := httptest.NewServer(app.Handler())
		defer srv.Close()

		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/agent/ws"
		header := http.Header{}
		header.Set("X-Agent-Token", "inv-token")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close()

		// Send register with empty agentId and name.
		registerMsg := map[string]any{
			"type": "register",
			"payload": map[string]any{
				"agentId": "",
				"name":    "",
			},
		}
		if err := conn.WriteJSON(registerMsg); err != nil {
			t.Fatalf("write register: %v", err)
		}

		var response map[string]any
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatalf("read response: %v", err)
		}
		if response["type"] != "error" {
			t.Fatalf("expected type 'error', got %q", response["type"])
		}
		msg, _ := response["message"].(string)
		if !strings.Contains(msg, "agentId and name are required") {
			t.Errorf("expected error about required fields, got %q", msg)
		}
	})
}

func TestHandleAgentWS_Heartbeat(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	app := NewApp(store, Config{AgentToken: "hb-token", HeartbeatTimeout: 35 * time.Second})
	defer app.Stop()

	srv := httptest.NewServer(app.Handler())
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/agent/ws"
	header := http.Header{}
	header.Set("X-Agent-Token", "hb-token")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Register.
	registerMsg := map[string]any{
		"type": "register",
		"payload": map[string]any{
			"agentId": "dev-hb-1",
			"name":    "hb-agent",
		},
	}
	if err := conn.WriteJSON(registerMsg); err != nil {
		t.Fatalf("write register: %v", err)
	}
	var regResponse map[string]any
	if err := conn.ReadJSON(&regResponse); err != nil {
		t.Fatalf("read register response: %v", err)
	}
	if regResponse["type"] != "registered" {
		t.Fatalf("expected registered, got %q", regResponse["type"])
	}

	// Send heartbeat.
	heartbeatMsg := map[string]any{
		"type": "heartbeat",
		"payload": map[string]any{
			"cpuUsage":    45.5,
			"memoryUsage": 60.0,
		},
	}
	if err := conn.WriteJSON(heartbeatMsg); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}

	// Give the handler time to process the heartbeat.
	time.Sleep(50 * time.Millisecond)

	// Verify device heartbeat was updated.
	dev, found, err := store.GetDevice(ctx, "dev-hb-1")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if !found {
		t.Fatal("device not found after heartbeat")
	}
	if dev.LastSeen == "" {
		t.Error("lastSeen should not be empty after heartbeat")
	}
	if dev.CPUUsage != 45.5 {
		t.Errorf("cpuUsage = %f, want 45.5", dev.CPUUsage)
	}
	if dev.MemoryUsage != 60.0 {
		t.Errorf("memoryUsage = %f, want 60.0", dev.MemoryUsage)
	}
}

func TestHandleAgentWS_Disconnect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	app := NewApp(store, Config{AgentToken: "dc-token"})
	defer app.Stop()

	srv := httptest.NewServer(app.Handler())
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/agent/ws"
	header := http.Header{}
	header.Set("X-Agent-Token", "dc-token")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Register.
	registerMsg := map[string]any{
		"type": "register",
		"payload": map[string]any{
			"agentId": "dev-dc-1",
			"name":    "dc-agent",
		},
	}
	if err := conn.WriteJSON(registerMsg); err != nil {
		t.Fatalf("write register: %v", err)
	}
	var regResponse map[string]any
	if err := conn.ReadJSON(&regResponse); err != nil {
		t.Fatalf("read register response: %v", err)
	}
	if regResponse["type"] != "registered" {
		t.Fatalf("expected registered, got %q", regResponse["type"])
	}

	// Close the connection.
	if err := conn.Close(); err != nil {
		t.Fatalf("close connection: %v", err)
	}

	// Wait briefly for cleanup goroutines to run.
	time.Sleep(100 * time.Millisecond)

	// Verify device is offline.
	dev, found, err := store.GetDevice(ctx, "dev-dc-1")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if !found {
		t.Fatal("device not found after disconnect")
	}
	if dev.Status != StatusOffline {
		t.Errorf("device status = %q, want %q", dev.Status, StatusOffline)
	}

	// Verify audit log has disconnect entry.
	logs, err := store.ListAudit(ctx)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	foundDisconnect := false
	for _, l := range logs {
		if l.Action == "agent.disconnect" && l.DeviceID == "dev-dc-1" && l.Status == StatusOffline {
			foundDisconnect = true
			break
		}
	}
	if !foundDisconnect {
		t.Fatal("expected agent.disconnect audit log entry")
	}
}
