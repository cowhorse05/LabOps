package core

import "time"

const (
	StatusOnline  = "online"
	StatusOffline = "offline"
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusTimeout = "timeout"
)

type User struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Roles       []string `json:"roles"`
}

type Device struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	GroupName   string  `json:"groupName"`
	Profile     string  `json:"profile"`
	Version     string  `json:"version"`
	Hostname    string  `json:"hostname"`
	OS          string  `json:"os"`
	IP          string  `json:"ip"`
	CPUCores    int     `json:"cpuCores"`
	MemoryMB    int     `json:"memoryMb"`
	DiskTotalGB int     `json:"diskTotalGb"`
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryUsage float64 `json:"memoryUsage"`
	DiskUsage   float64 `json:"diskUsage"`
	Status      string  `json:"status"`
	LastSeen    string  `json:"lastSeen"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

type DeviceGroup struct {
	Name        string `json:"name"`
	Total       int    `json:"total"`
	Online      int    `json:"online"`
	Description string `json:"description"`
}

type DeviceStats struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
}

type Task struct {
	ID          string      `json:"id"`
	DeviceID    string      `json:"deviceId"`
	DeviceName  string      `json:"deviceName"`
	GroupName   string      `json:"groupName"`
	Command     string      `json:"command"`
	Status      string      `json:"status"`
	RequestedBy string      `json:"requestedBy"`
	CreatedAt   string      `json:"createdAt"`
	StartedAt   string      `json:"startedAt,omitempty"`
	FinishedAt  string      `json:"finishedAt,omitempty"`
	Result      *TaskResult `json:"result,omitempty"`
}

type TaskResult struct {
	TaskID     string `json:"taskId"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exitCode"`
	DurationMS int64  `json:"durationMs"`
	CreatedAt  string `json:"createdAt"`
}

type AuditLog struct {
	ID        string `json:"id"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	DeviceID  string `json:"deviceId,omitempty"`
	Device    string `json:"device,omitempty"`
	TaskID    string `json:"taskId,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
}

type AgentEnvelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type RegisterPayload struct {
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

type HeartbeatPayload struct {
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryUsage float64 `json:"memoryUsage"`
	DiskUsage   float64 `json:"diskUsage"`
}

type TaskResultPayload struct {
	TaskID     string `json:"taskId"`
	Status     string `json:"status"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exitCode"`
	DurationMS int64  `json:"durationMs"`
}

type CommandPayload struct {
	TaskID  string `json:"taskId"`
	Command string `json:"command"`
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339)
}
