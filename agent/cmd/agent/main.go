package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	goNet "net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	psnet "github.com/shirou/gopsutil/v4/net"
)

const version = "0.1.0"

type config struct {
	ServerURL       string
	Token           string
	DeviceSecret    string
	EnrollCode      string
	CredentialsPath string
	AgentID         string
	Name            string
	GroupName       string
	MockProfile     string
	RealMetrics     bool
	EnrollOnly      bool
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
	ProtocolVersion int      `json:"protocolVersion"`
	TaskID          string   `json:"taskId"`
	Kind            string   `json:"kind"`
	Command         string   `json:"command"`
	Executable      string   `json:"executable"`
	Args            []string `json:"args"`
	TimeoutSeconds  int      `json:"timeoutSeconds"`
}

type storedCredentials struct {
	DeviceID     string `json:"deviceId"`
	DeviceSecret string `json:"deviceSecret"`
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
	if cfg.EnrollCode != "" {
		credentials, err := enroll(cfg)
		if err != nil {
			log.Fatalf("agent enrollment failed: %v", err)
		}
		if err := saveCredentials(cfg.CredentialsPath, credentials); err != nil {
			log.Fatalf("save agent credentials: %v", err)
		}
		cfg.AgentID, cfg.DeviceSecret = credentials.DeviceID, credentials.DeviceSecret
		log.Printf("agent enrolled as %s", cfg.AgentID)
		if cfg.EnrollOnly {
			return
		}
	}
	if cfg.DeviceSecret == "" && cfg.Token == "" {
		log.Fatal("agent has no device credential; run with --enroll-code first")
	}
	backoff := 1 * time.Second
	const maxBackoff = 60 * time.Second
	for {
		if err := run(cfg); err != nil {
			log.Printf("agent disconnected: %v", err)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		} else {
			backoff = 1 * time.Second
		}
	}
}

func parseFlags() config {
	hostname, _ := os.Hostname()
	cfg := config{}
	flag.StringVar(&cfg.ServerURL, "server", env("LABOPS_SERVER_URL", "http://localhost:8080"), "LabOps server URL")
	flag.StringVar(&cfg.Token, "token", env("LABOPS_AGENT_TOKEN", ""), "legacy shared agent token")
	flag.StringVar(&cfg.DeviceSecret, "device-secret", env("LABOPS_DEVICE_SECRET", ""), "per-device secret")
	flag.StringVar(&cfg.EnrollCode, "enroll-code", env("LABOPS_ENROLLMENT_CODE", ""), "one-time enrollment code")
	flag.StringVar(&cfg.CredentialsPath, "credentials", env("LABOPS_AGENT_CREDENTIALS", defaultCredentialsPath()), "credentials file path")
	flag.StringVar(&cfg.Name, "name", env("LABOPS_AGENT_NAME", hostname), "device name")
	flag.StringVar(&cfg.GroupName, "group", env("LABOPS_AGENT_GROUP", "default"), "device group")
	flag.StringVar(&cfg.MockProfile, "mock-profile", env("LABOPS_MOCK_PROFILE", "ubuntu"), "mock profile")
	flag.StringVar(&cfg.AgentID, "id", env("LABOPS_AGENT_ID", ""), "stable agent id")
	var realMetrics bool
	var enrollOnly bool
	flag.BoolVar(&realMetrics, "real", parseBoolEnv("LABOPS_AGENT_REAL"), "collect real system metrics instead of mock data")
	flag.BoolVar(&enrollOnly, "enroll-only", false, "enroll, save credentials, and exit")
	flag.Parse()
	if cfg.DeviceSecret == "" {
		if credentials, err := loadCredentials(cfg.CredentialsPath); err == nil {
			cfg.AgentID, cfg.DeviceSecret = credentials.DeviceID, credentials.DeviceSecret
			cfg.Token = ""
		}
	}
	if cfg.AgentID == "" {
		cfg.AgentID = "agent-" + sanitizeID(cfg.Name)
	}
	cfg.RealMetrics = realMetrics
	cfg.EnrollOnly = enrollOnly
	return cfg
}

func run(cfg config) error {
	wsURL, err := agentWSURL(cfg.ServerURL)
	if err != nil {
		return err
	}
	log.Printf("connecting to %s as %s", cfg.ServerURL, cfg.Name)
	header := http.Header{}
	if cfg.DeviceSecret != "" && cfg.AgentID != "" {
		header.Set("Authorization", "Agent "+cfg.AgentID+":"+cfg.DeviceSecret)
	} else if cfg.Token != "" {
		header.Set("X-Agent-Token", cfg.Token)
	}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
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
	if cfg.RealMetrics {
		primeCPUMetrics()
		go heartbeatLoop(ctx, send, cancel, collectMetrics)
	} else {
		go heartbeatLoop(ctx, send, cancel, func() heartbeatPayload { return mockHeartbeat(cfg.MockProfile) })
	}

	// Periodically reset the read deadline so a cancelled context (from a failed
	// heartbeat) causes ReadJSON to return an error rather than blocking forever.
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			}
		}
	}()

	for {
		var msg incomingEnvelope
		if err := conn.ReadJSON(&msg); err != nil {
			// If the read was interrupted by context cancellation, surface that instead
			// of the opaque websocket error.
			if ctx.Err() != nil {
				return ctx.Err()
			}
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
			if cmd.TaskID == "" || (cmd.Kind == "template" && cmd.Executable == "") || (cmd.Kind != "template" && cmd.Command == "") {
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

func heartbeatLoop(ctx context.Context, send func(any) error, cancel context.CancelFunc, collect func() heartbeatPayload) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := send(envelope{Type: "heartbeat", Payload: collect()}); err != nil {
				log.Printf("heartbeat send failed, triggering reconnect: %v", err)
				cancel()
				return
			}
		}
	}
}

func executeAndReport(send func(any) error, command commandPayload) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic recovered in executeAndReport: %v", r)
			// Send a failure result so the task doesn't stay "running" forever
			// waiting for the server-side 2-minute timeout.
			_ = send(envelope{Type: "task_result", Payload: taskResultPayload{
				TaskID:   command.TaskID,
				Status:   "failed",
				Stderr:   fmt.Sprintf("agent panic: %v", r),
				ExitCode: 1,
			}})
		}
	}()
	start := time.Now()
	stdout, stderr, exitCode := executePayload(command)
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

const (
	maxStdoutSize = 256 * 1024 // 256KB
	maxStderrSize = 256 * 1024 // 256KB
)

func executeCommand(command string) (string, string, int) {
	return executeProcess(commandPayload{Kind: "ad_hoc", Command: command, TimeoutSeconds: 30})
}

func executePayload(payload commandPayload) (string, string, int) {
	if payload.TimeoutSeconds < 1 {
		payload.TimeoutSeconds = 30
	}
	if payload.TimeoutSeconds > 300 {
		payload.TimeoutSeconds = 300
	}
	return executeProcess(payload)
}

func executeProcess(payload commandPayload) (string, string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(payload.TimeoutSeconds)*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if payload.Kind == "template" {
		if !filepath.IsAbs(payload.Executable) {
			return "", "template executable must be an absolute path", 126
		}
		cmd = exec.CommandContext(ctx, payload.Executable, payload.Args...)
	} else if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", payload.Command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", payload.Command)
	}
	cmd.Env = os.Environ()
	// Ensure LANG is set for consistent output; don't clobber the system PATH.
	if runtime.GOOS != "windows" {
		cmd.Env = append(cmd.Env, "LANG=C.UTF-8")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	stdoutStr := truncateOutput(&stdout, maxStdoutSize)
	stderrStr := truncateOutput(&stderr, maxStderrSize)

	if ctx.Err() == context.DeadlineExceeded {
		return stdoutStr, fmt.Sprintf("command timed out after %ds", payload.TimeoutSeconds), 124
	}
	if err == nil {
		return stdoutStr, stderrStr, 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return stdoutStr, stderrStr, exitErr.ExitCode()
	}
	return stdoutStr, err.Error(), 1
}

func defaultCredentialsPath() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = "."
		}
		return filepath.Join(base, "LabOps", "credentials.json")
	}
	return "/etc/labops-agent/credentials.json"
}

func loadCredentials(path string) (storedCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return storedCredentials{}, err
	}
	var credentials storedCredentials
	if err := json.Unmarshal(data, &credentials); err != nil {
		return storedCredentials{}, err
	}
	if credentials.DeviceID == "" || credentials.DeviceSecret == "" {
		return storedCredentials{}, fmt.Errorf("credentials file is incomplete")
	}
	return credentials, nil
}

func saveCredentials(path string, credentials storedCredentials) error {
	if path == "" {
		return fmt.Errorf("credentials path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func enroll(cfg config) (storedCredentials, error) {
	endpoint := strings.TrimRight(cfg.ServerURL, "/") + "/api/agent/enroll"
	payload := map[string]any{"code": cfg.EnrollCode, "device": buildRegister(cfg)}
	body, err := json.Marshal(payload)
	if err != nil {
		return storedCredentials{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return storedCredentials{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return storedCredentials{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return storedCredentials{}, err
	}
	if resp.StatusCode != http.StatusCreated {
		return storedCredentials{}, fmt.Errorf("server returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var result storedCredentials
	if err := json.Unmarshal(data, &result); err != nil {
		return storedCredentials{}, err
	}
	if result.DeviceID == "" || result.DeviceSecret == "" {
		return storedCredentials{}, fmt.Errorf("server returned incomplete credentials")
	}
	return result, nil
}

func truncateOutput(buf *bytes.Buffer, limit int64) string {
	limited := io.LimitReader(buf, limit)
	out, err := io.ReadAll(limited)
	if err != nil {
		return ""
	}
	result := string(out)
	if buf.Len() > 0 {
		result += "...[truncated]"
	}
	return result
}

func buildRegister(cfg config) registerPayload {
	if cfg.RealMetrics {
		return collectSystemInfo(cfg)
	}
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
	return math.Round(value*10) / 10
}

// collectSystemInfo gathers real system information for agent registration.
func collectSystemInfo(cfg config) registerPayload {
	hostname, _ := os.Hostname()

	info, err := host.Info()
	var osName string
	if err == nil {
		osName = fmt.Sprintf("%s %s", info.Platform, info.PlatformVersion)
	} else {
		osName = runtime.GOOS
	}

	cpuCount, _ := cpu.Counts(true)
	if cpuCount <= 0 {
		cpuCount = runtime.NumCPU()
	}

	memInfo, err := mem.VirtualMemory()
	var memoryMB int
	if err == nil {
		memoryMB = int(memInfo.Total / (1024 * 1024))
	}

	// Sum disk totals across all non-zero partitions.
	var diskTotalGB int
	partitions, err := disk.Partitions(false)
	if err == nil {
		for _, p := range partitions {
			usage, err := disk.Usage(p.Mountpoint)
			if err == nil && usage.Total > 0 {
				diskTotalGB += int(usage.Total / (1024 * 1024 * 1024))
			}
		}
	}

	// Pick first non-loopback IPv4 address.
	var ip string
	ifaces, err := psnet.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			for _, addr := range iface.Addrs {
				// addr.Addr is a string like "192.168.1.100/24"
				ipStr := strings.Split(addr.Addr, "/")[0]
				parsed := goNet.ParseIP(ipStr)
				if parsed != nil && !parsed.IsLoopback() && parsed.To4() != nil {
					ip = ipStr
					break
				}
			}
			if ip != "" {
				break
			}
		}
	}

	return registerPayload{
		AgentID:     cfg.AgentID,
		Name:        cfg.Name,
		GroupName:   cfg.GroupName,
		Version:     version,
		Profile:     "real",
		Hostname:    hostname,
		OS:          osName,
		IP:          ip,
		CPUCores:    cpuCount,
		MemoryMB:    memoryMB,
		DiskTotalGB: diskTotalGB,
	}
}

// primeCPUMetrics primes the gopsutil CPU percent collector so that
// subsequent zero-interval calls return instantaneous deltas.
func primeCPUMetrics() {
	cpu.Percent(time.Second, false)
}

// collectMetrics gathers real-time system metrics for heartbeat reporting.
func collectMetrics() heartbeatPayload {
	cpuPercents, _ := cpu.Percent(0, false)

	memInfo, _ := mem.VirtualMemory()
	var memPercent float64
	if memInfo != nil {
		memPercent = memInfo.UsedPercent
	}

	// Use the first available partition's used percent.
	var diskPercent float64
	partitions, err := disk.Partitions(false)
	if err == nil && len(partitions) > 0 {
		usage, err := disk.Usage(partitions[0].Mountpoint)
		if err == nil {
			diskPercent = usage.UsedPercent
		}
	}

	var cpuPercent float64
	if len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}

	return heartbeatPayload{
		CPUUsage:    math.Round(cpuPercent*10) / 10,
		MemoryUsage: math.Round(memPercent*10) / 10,
		DiskUsage:   math.Round(diskPercent*10) / 10,
	}
}

func agentWSURL(serverURL string) (string, error) {
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

func parseBoolEnv(key string) bool {
	v := strings.ToLower(os.Getenv(key))
	return v == "1" || v == "true" || v == "yes"
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
