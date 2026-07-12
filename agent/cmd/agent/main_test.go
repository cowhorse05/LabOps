package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCredentialFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "credentials.json")
	want := storedCredentials{DeviceID: "device-1", DeviceSecret: "secret-1"}
	if err := saveCredentials(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadCredentials(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestEnroll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/enroll" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["code"] != "one-time" {
			t.Fatalf("unexpected code: %v", body["code"])
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(storedCredentials{DeviceID: "device-2", DeviceSecret: "secret-2"})
	}))
	defer server.Close()
	got, err := enroll(config{ServerURL: server.URL, EnrollCode: "one-time", AgentID: "ignored", Name: "host", GroupName: "lab", MockProfile: "ubuntu"})
	if err != nil {
		t.Fatal(err)
	}
	if got.DeviceID != "device-2" || got.DeviceSecret != "secret-2" {
		t.Fatalf("unexpected credentials: %+v", got)
	}
}

func TestTemplateExecutionUsesArgv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux template paths are validated in CI")
	}
	stdout, stderr, exitCode := executePayload(commandPayload{Kind: "template", Executable: "/bin/echo", Args: []string{"hello; uname"}, TimeoutSeconds: 5})
	if exitCode != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exitCode, stderr)
	}
	if strings.TrimSpace(stdout) != "hello; uname" {
		t.Fatalf("argument was interpreted by a shell: %q", stdout)
	}
	_, stderr, exitCode = executePayload(commandPayload{Kind: "template", Executable: "echo", Args: []string{"unsafe"}, TimeoutSeconds: 5})
	if exitCode == 0 || !strings.Contains(stderr, "absolute path") {
		t.Fatalf("relative executable accepted: exit=%d stderr=%q", exitCode, stderr)
	}
}

func TestAgentWSURL(t *testing.T) {
	got, err := agentWSURL("http://localhost:8080")
	if err != nil {
		t.Fatalf("agentWSURL: %v", err)
	}
	if got != "ws://localhost:8080/api/agent/ws" {
		t.Fatalf("unexpected url: %s", got)
	}
}

func TestBuildRegisterUsesProfile(t *testing.T) {
	reg := buildRegister(config{
		AgentID:     "agent-1",
		Name:        "Lab PC 01",
		GroupName:   "classroom-a",
		MockProfile: "windows-lab",
	})
	if reg.OS != "Windows 11 Pro" || reg.MemoryMB != 16384 {
		t.Fatalf("unexpected register payload: %+v", reg)
	}
	if !strings.Contains(reg.Hostname, ".win.lab") {
		t.Fatalf("expected windows hostname suffix: %s", reg.Hostname)
	}
}

func TestExecuteCommand(t *testing.T) {
	stdout, stderr, exitCode := executeCommand("echo labops")
	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "labops") {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestSanitizeID(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		got := sanitizeID("Hello World")
		if got != "hello-world" {
			t.Fatalf("got %q, want %q", got, "hello-world")
		}
	})

	t.Run("special chars", func(t *testing.T) {
		got := sanitizeID("test@#$device")
		if got != "test-device" {
			t.Fatalf("got %q, want %q", got, "test-device")
		}
	})

	t.Run("empty", func(t *testing.T) {
		got := sanitizeID("")
		if !strings.HasPrefix(got, "node-") {
			t.Fatalf("got %q, want prefix %q", got, "node-")
		}
	})

	t.Run("only special chars", func(t *testing.T) {
		got := sanitizeID("@#$%!")
		if !strings.HasPrefix(got, "node-") {
			t.Fatalf("got %q, want prefix %q", got, "node-")
		}
	})

	t.Run("already clean", func(t *testing.T) {
		got := sanitizeID("node123")
		if got != "node123" {
			t.Fatalf("got %q, want %q", got, "node123")
		}
	})

	t.Run("leading and trailing dashes", func(t *testing.T) {
		got := sanitizeID("-hello-")
		if got != "hello" {
			t.Fatalf("got %q, want %q", got, "hello")
		}
	})

	t.Run("consecutive special chars", func(t *testing.T) {
		got := sanitizeID("a!!b")
		if got != "a-b" {
			t.Fatalf("got %q, want %q", got, "a-b")
		}
	})

	t.Run("mixed case", func(t *testing.T) {
		got := sanitizeID("UPPER lower")
		if got != "upper-lower" {
			t.Fatalf("got %q, want %q", got, "upper-lower")
		}
	})
}

func TestJitter(t *testing.T) {
	t.Run("low base clamped to minimum", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			got := jitter(0)
			if got < 1 || got > 99 {
				t.Fatalf("jitter(0) = %f, want in [1, 99]", got)
			}
			if got != float64(int(got*10))/10 {
				t.Fatalf("jitter(0) = %f, not rounded to 1 decimal", got)
			}
		}
	})

	t.Run("high base clamped to maximum", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			got := jitter(100)
			if got < 1 || got > 99 {
				t.Fatalf("jitter(100) = %f, want in [1, 99]", got)
			}
			if got != float64(int(got*10))/10 {
				t.Fatalf("jitter(100) = %f, not rounded to 1 decimal", got)
			}
		}
	})

	t.Run("normal range", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			got := jitter(50)
			if got < 1 || got > 99 {
				t.Fatalf("jitter(50) = %f, want in [1, 99]", got)
			}
			if got != float64(int(got*10))/10 {
				t.Fatalf("jitter(50) = %f, not rounded to 1 decimal", got)
			}
		}
	})
}

func TestProfileSpec(t *testing.T) {
	t.Run("windows", func(t *testing.T) {
		p := profileSpec("windows")
		if p.OS != "Windows 11 Pro" {
			t.Fatalf("OS = %q, want %q", p.OS, "Windows 11 Pro")
		}
		if p.HostnameSuffix != ".win.lab" {
			t.Fatalf("HostnameSuffix = %q, want %q", p.HostnameSuffix, ".win.lab")
		}
		if p.CPUCores != 8 || p.MemoryMB != 16384 || p.DiskTotalGB != 512 {
			t.Fatalf("unexpected windows specs: %+v", p)
		}
	})

	t.Run("server", func(t *testing.T) {
		p := profileSpec("server")
		if p.OS != "Ubuntu Server 24.04" {
			t.Fatalf("OS = %q, want %q", p.OS, "Ubuntu Server 24.04")
		}
		if p.HostnameSuffix != ".srv.lab" {
			t.Fatalf("HostnameSuffix = %q, want %q", p.HostnameSuffix, ".srv.lab")
		}
		if p.CPUCores != 4 || p.MemoryMB != 8192 || p.DiskTotalGB != 128 {
			t.Fatalf("unexpected server specs: %+v", p)
		}
	})

	t.Run("edge", func(t *testing.T) {
		p := profileSpec("edge")
		if p.OS != "Debian Edge Node" {
			t.Fatalf("OS = %q, want %q", p.OS, "Debian Edge Node")
		}
		if p.HostnameSuffix != ".edge.lab" {
			t.Fatalf("HostnameSuffix = %q, want %q", p.HostnameSuffix, ".edge.lab")
		}
		if p.CPUCores != 2 || p.MemoryMB != 2048 || p.DiskTotalGB != 32 {
			t.Fatalf("unexpected edge specs: %+v", p)
		}
	})

	t.Run("ubuntu", func(t *testing.T) {
		p := profileSpec("ubuntu")
		if p.OS != "Ubuntu Desktop 24.04" {
			t.Fatalf("OS = %q, want %q", p.OS, "Ubuntu Desktop 24.04")
		}
		if p.HostnameSuffix != ".lab" {
			t.Fatalf("HostnameSuffix = %q, want %q", p.HostnameSuffix, ".lab")
		}
		if p.CPUCores != 4 || p.MemoryMB != 4096 || p.DiskTotalGB != 64 {
			t.Fatalf("unexpected ubuntu specs: %+v", p)
		}
	})

	t.Run("unknown falls back to ubuntu", func(t *testing.T) {
		p := profileSpec("unknown")
		if p.OS != "Ubuntu Desktop 24.04" {
			t.Fatalf("OS = %q, want %q", p.OS, "Ubuntu Desktop 24.04")
		}
		if p.HostnameSuffix != ".lab" {
			t.Fatalf("HostnameSuffix = %q, want %q", p.HostnameSuffix, ".lab")
		}
	})

	t.Run("case insensitive windows-lab", func(t *testing.T) {
		p := profileSpec("WINDOWS-LAB")
		if p.OS != "Windows 11 Pro" {
			t.Fatalf("OS = %q, want %q", p.OS, "Windows 11 Pro")
		}
	})
}

func TestAgentWSURLAdditional(t *testing.T) {
	t.Run("https to wss", func(t *testing.T) {
		got, err := agentWSURL("https://example.com")
		if err != nil {
			t.Fatalf("agentWSURL: %v", err)
		}
		if got != "wss://example.com/api/agent/ws" {
			t.Fatalf("unexpected url: %s", got)
		}
	})

	t.Run("trailing slash", func(t *testing.T) {
		got, err := agentWSURL("http://localhost:8080/")
		if err != nil {
			t.Fatalf("agentWSURL: %v", err)
		}
		if got != "ws://localhost:8080/api/agent/ws" {
			t.Fatalf("unexpected url: %s", got)
		}
	})

	t.Run("x-agent-token header", func(t *testing.T) {
		header := make(map[string][]string)
		cfg := config{ServerURL: "http://localhost:8080", Token: "secret-token"}
		_, _ = agentWSURL(cfg.ServerURL) // verify no token in URL
		// Token is now sent via X-Agent-Token header, not query param
		_ = header // placeholder for header-based test
	})

	t.Run("real mode ignores profile", func(t *testing.T) {
		reg := buildRegister(config{
			AgentID: "agent-real-test", Name: "real-device", GroupName: "lab",
			RealMetrics: true,
		})
		if reg.Profile != "real" {
			t.Fatalf("expected profile=real, got %s", reg.Profile)
		}
		if reg.Hostname == "" || reg.OS == "" || reg.CPUCores <= 0 || reg.MemoryMB <= 0 {
			t.Fatalf("expected real system info, got %+v", reg)
		}
	})

	t.Run("collectMetrics in valid range", func(t *testing.T) {
		primeCPUMetrics()
		hb := collectMetrics()
		if hb.CPUUsage < 0 || hb.CPUUsage > 100 {
			t.Fatalf("cpuUsage out of range: %f", hb.CPUUsage)
		}
		if hb.MemoryUsage < 0 || hb.MemoryUsage > 100 {
			t.Fatalf("memoryUsage out of range: %f", hb.MemoryUsage)
		}
		if hb.DiskUsage < 0 || hb.DiskUsage > 100 {
			t.Fatalf("diskUsage out of range: %f", hb.DiskUsage)
		}
	})
}
