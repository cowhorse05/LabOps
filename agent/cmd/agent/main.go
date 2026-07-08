package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const version = "0.1.0"

type config struct {
	ServerURL   string
	Token       string
	AgentID     string
	Name        string
	GroupName   string
	MockProfile string
}

type envelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type incomingEnvelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type registerPayload struct {
	AgentID     string `json:"agentId"`
	Name        string `json:"name"`
	GroupName   string `json:"groupName"`
	Version     string `json:"version"`
	Profile     string `json:"profile"`
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	IP          string `json:"ip"`
	CPUCores    int    `json:"cpuCores"`
	MemoryMB    int    `json:"memoryMb"`
	DiskTotalGB int    `json:"diskTotalGb"`
}

type heartbeatPayload struct {
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryUsage float64 `json:"memoryUsage"`
	DiskUsage   float64 `json:"diskUsage"`
}

type commandPayload struct {
	TaskID  string `json:"taskId"`
	Command string `json:"command"`
}

type taskResultPayload struct {
	TaskID     string `json:"taskId"`
	Status     string `json:"status"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exitCode"`
	DurationMS int64  `json:"durationMs"`
}

func main() {
	cfg := parseFlags()
	for {
		if err := run(cfg); err != nil {
			log.Printf("agent disconnected: %v", err)
		}
		time.Sleep(3 * time.Second)
	}
}

func parseFlags() config {
	hostname, _ := os.Hostname()
	cfg := config{}
	flag.StringVar(&cfg.ServerURL, "server", env("LABOPS_SERVER_URL", "http://localhost:8080"), "LabOps server URL")
	flag.StringVar(&cfg.Token, "token", env("LABOPS_AGENT_TOKEN", "dev-agent-token"), "agent token")
	flag.StringVar(&cfg.Name, "name", env("LABOPS_AGENT_NAME", hostname), "device name")
	flag.StringVar(&cfg.GroupName, "group", env("LABOPS_AGENT_GROUP", "default"), "device group")
	flag.StringVar(&cfg.MockProfile, "mock-profile", env("LABOPS_MOCK_PROFILE", "ubuntu"), "mock profile")
	flag.StringVar(&cfg.AgentID, "id", env("LABOPS_AGENT_ID", ""), "stable agent id")
	flag.Parse()
	if cfg.AgentID == "" {
		cfg.AgentID = "agent-" + sanitizeID(cfg.Name)
	}
	return cfg
}

func run(cfg config) error {
	wsURL, err := agentWSURL(cfg.ServerURL, cfg.Token)
	if err != nil {
		return err
	}
	log.Printf("connecting to %s as %s", cfg.ServerURL, cfg.Name)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.WriteJSON(envelope{Type: "register", Payload: buildRegister(cfg)}); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var writeMu sync.Mutex
	send := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(v)
	}
	go heartbeatLoop(ctx, send, cancel, cfg.MockProfile)

	for {
		var msg incomingEnvelope
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}
		switch msg.Type {
		case "registered":
			log.Printf("registered with server")
		case "command":
			var cmd commandPayload
			if err := json.Unmarshal(msg.Payload, &cmd); err != nil {
				log.Printf("invalid command payload: %v", err)
				continue
			}
			if cmd.TaskID == "" || cmd.Command == "" {
				_ = send(envelope{Type: "task_result", Payload: taskResultPayload{
					TaskID: cmd.TaskID, Status: "failed", Stderr: "empty taskId or command",
					ExitCode: 1,
				}})
				continue
			}
			go executeAndReport(send, cmd)
		case "error":
			var errPayload struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(msg.Payload, &errPayload); err == nil {
				log.Printf("server error: %s", errPayload.Message)
			}
			return fmt.Errorf("server sent error, reconnecting")
		default:
			log.Printf("unknown message type: %s", msg.Type)
		}
	}
}

func heartbeatLoop(ctx context.Context, send func(any) error, cancel context.CancelFunc, profile string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := send(envelope{Type: "heartbeat", Payload: mockHeartbeat(profile)}); err != nil {
				log.Printf("heartbeat send failed, triggering reconnect: %v", err)
				cancel()
				return
			}
		}
	}
}

func executeAndReport(send func(any) error, command commandPayload) {
	start := time.Now()
	stdout, stderr, exitCode := executeCommand(command.Command)
	status := "success"
	if exitCode != 0 {
		status = "failed"
	}
	result := taskResultPayload{
		TaskID:     command.TaskID,
		Status:     status,
		Stdout:     stdout,
		Stderr:     stderr,
		ExitCode:   exitCode,
		DurationMS: time.Since(start).Milliseconds(),
	}
	if err := send(envelope{Type: "task_result", Payload: result}); err != nil {
		log.Printf("send task result failed: %v", err)
	}
}

func executeCommand(command string) (string, string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), "command timed out after 30s", 124
	}
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stdout.String(), stderr.String(), exitErr.ExitCode()
	}
	return stdout.String(), err.Error(), 1
}

func buildRegister(cfg config) registerPayload {
	profile := profileSpec(cfg.MockProfile)
	hostname := cfg.Name
	if profile.HostnameSuffix != "" {
		hostname = sanitizeID(cfg.Name) + profile.HostnameSuffix
	}
	return registerPayload{
		AgentID:     cfg.AgentID,
		Name:        cfg.Name,
		GroupName:   cfg.GroupName,
		Version:     version,
		Profile:     cfg.MockProfile,
		Hostname:    hostname,
		OS:          profile.OS,
		IP:          profile.IP,
		CPUCores:    profile.CPUCores,
		MemoryMB:    profile.MemoryMB,
		DiskTotalGB: profile.DiskTotalGB,
	}
}

type profile struct {
	OS             string
	IP             string
	CPUCores       int
	MemoryMB       int
	DiskTotalGB    int
	HostnameSuffix string
	CPUBase        float64
	MemBase        float64
	DiskBase       float64
}

func profileSpec(name string) profile {
	switch strings.ToLower(name) {
	case "windows", "windows-lab":
		return profile{OS: "Windows 11 Pro", IP: "10.10.1.21", CPUCores: 8, MemoryMB: 16384, DiskTotalGB: 512, HostnameSuffix: ".win.lab", CPUBase: 22, MemBase: 48, DiskBase: 61}
	case "server", "ubuntu-server":
		return profile{OS: "Ubuntu Server 24.04", IP: "10.10.2.15", CPUCores: 4, MemoryMB: 8192, DiskTotalGB: 128, HostnameSuffix: ".srv.lab", CPUBase: 18, MemBase: 41, DiskBase: 37}
	case "edge", "edge-node":
		return profile{OS: "Debian Edge Node", IP: "10.10.3.33", CPUCores: 2, MemoryMB: 2048, DiskTotalGB: 32, HostnameSuffix: ".edge.lab", CPUBase: 35, MemBase: 55, DiskBase: 44}
	default:
		return profile{OS: "Ubuntu Desktop 24.04", IP: "10.10.0.10", CPUCores: 4, MemoryMB: 4096, DiskTotalGB: 64, HostnameSuffix: ".lab", CPUBase: 15, MemBase: 38, DiskBase: 29}
	}
}

func mockHeartbeat(profileName string) heartbeatPayload {
	p := profileSpec(profileName)
	return heartbeatPayload{
		CPUUsage:    jitter(p.CPUBase),
		MemoryUsage: jitter(p.MemBase),
		DiskUsage:   jitter(p.DiskBase),
	}
}

func jitter(base float64) float64 {
	value := base + rand.Float64()*18 - 6
	if value < 1 {
		return 1
	}
	if value > 99 {
		return 99
	}
	return float64(int(value*10)) / 10
}

func agentWSURL(serverURL, token string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/agent/ws"
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func sanitizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out.WriteRune(r)
			continue
		}
		if out.Len() > 0 && out.String()[out.Len()-1] != '-' {
			out.WriteByte('-')
		}
	}
	result := strings.Trim(out.String(), "-")
	if result == "" {
		return fmt.Sprintf("node-%d", time.Now().Unix())
	}
	return result
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
