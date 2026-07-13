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

	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"

	TaskKindAdHoc    = "ad_hoc"
	TaskKindTemplate = "template"
)

type User struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Roles       []string `json:"roles"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	Status      string   `json:"status"`
}

type SetupStatus struct {
	Initialized         bool `json:"initialized"`
	AdminExists         bool `json:"adminExists"`
	ActiveAdminExists   bool `json:"activeAdminExists"`
	RegistrationAllowed bool `json:"registrationAllowed"`
	RecoveryRequired    bool `json:"recoveryRequired"`
	TotalUsers          int  `json:"totalUsers"`
}

type BootstrapAdminInput struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
	DisplayName     string `json:"displayName"`
}

type Device struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	GroupName        string  `json:"groupName"`
	Profile          string  `json:"profile"`
	Version          string  `json:"version"`
	Hostname         string  `json:"hostname"`
	OS               string  `json:"os"`
	IP               string  `json:"ip"`
	CPUCores         int     `json:"cpuCores"`
	MemoryMB         int     `json:"memoryMb"`
	DiskTotalGB      int     `json:"diskTotalGb"`
	CPUUsage         float64 `json:"cpuUsage"`
	MemoryUsage      float64 `json:"memoryUsage"`
	DiskUsage        float64 `json:"diskUsage"`
	Status           string  `json:"status"`
	LastSeen         string  `json:"lastSeen"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	CredentialStatus string  `json:"credentialStatus"`
	RevokedAt        string  `json:"revokedAt,omitempty"`
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
	ID             string      `json:"id"`
	DeviceID       string      `json:"deviceId"`
	DeviceName     string      `json:"deviceName"`
	GroupName      string      `json:"groupName"`
	Command        string      `json:"command"`
	Kind           string      `json:"kind"`
	TemplateID     string      `json:"templateId,omitempty"`
	Executable     string      `json:"executable,omitempty"`
	Args           []string    `json:"args,omitempty"`
	TimeoutSeconds int         `json:"timeoutSeconds"`
	Status         string      `json:"status"`
	RequestedBy    string      `json:"requestedBy"`
	CreatedAt      string      `json:"createdAt"`
	StartedAt      string      `json:"startedAt,omitempty"`
	FinishedAt     string      `json:"finishedAt,omitempty"`
	Result         *TaskResult `json:"result,omitempty"`
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
	ID         string `json:"id"`
	Actor      string `json:"actor"`
	ActorID    string `json:"actorId,omitempty"`
	ActorRole  string `json:"actorRole,omitempty"`
	RemoteAddr string `json:"remoteAddr,omitempty"`
	RequestID  string `json:"requestId,omitempty"`
	Action     string `json:"action"`
	DeviceID   string `json:"deviceId,omitempty"`
	Device     string `json:"device,omitempty"`
	TaskID     string `json:"taskId,omitempty"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	CreatedAt  string `json:"createdAt"`
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
	ProtocolVersion int      `json:"protocolVersion"`
	TaskID          string   `json:"taskId"`
	Kind            string   `json:"kind"`
	Command         string   `json:"command,omitempty"`
	Executable      string   `json:"executable,omitempty"`
	Args            []string `json:"args,omitempty"`
	TimeoutSeconds  int      `json:"timeoutSeconds"`
}

type WebSession struct {
	ID                string
	UserID            string
	TokenHash         string
	CSRFHash          string
	RemoteAddr        string
	UserAgent         string
	CreatedAt         string
	LastSeenAt        string
	IdleExpiresAt     string
	AbsoluteExpiresAt string
}

type EnrollmentCode struct {
	ID        string `json:"id"`
	Code      string `json:"code,omitempty"`
	ExpiresAt string `json:"expiresAt"`
	MaxUses   int    `json:"maxUses"`
	UsedCount int    `json:"usedCount"`
	CreatedBy string `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
	RevokedAt string `json:"revokedAt,omitempty"`
}

type DeviceCredential struct {
	DeviceID   string
	SecretHash string
	Status     string
	CreatedAt  string
	LastUsedAt string
	RevokedAt  string
}

type CommandTemplate struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Description       string              `json:"description"`
	OS                string              `json:"os"`
	Executable        string              `json:"executable"`
	Args              []string            `json:"args"`
	Parameters        []TemplateParameter `json:"parameters"`
	RequiresPrivilege bool                `json:"requiresPrivilege"`
	Enabled           bool                `json:"enabled"`
	TimeoutSeconds    int                 `json:"timeoutSeconds"`
	CreatedAt         string              `json:"createdAt"`
	UpdatedAt         string              `json:"updatedAt"`
}

type TemplateParameter struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Pattern string   `json:"pattern,omitempty"`
	Enum    []string `json:"enum,omitempty"`
	Min     *int     `json:"min,omitempty"`
	Max     *int     `json:"max,omitempty"`
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// LLMConfig holds the configuration for the LLM-powered AI Ops analysis.
type LLMConfig struct {
	ProviderURL         string `json:"providerUrl"`
	APIKey              string `json:"apiKey"`
	Model               string `json:"model"`
	ProviderType        string `json:"providerType"` // "openai" or "anthropic"
	Enabled             bool   `json:"enabled"`
	AutoExecuteReadOnly bool   `json:"autoExecuteReadOnly"`
	UpdatedAt           string `json:"updatedAt"`
}

// LLMRecommendation is a structured, executable recommendation from the LLM.
type LLMRecommendation struct {
	ID         string `json:"id"` // rec_xxx
	DeviceID   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
	GroupName  string `json:"groupName"`
	Command    string `json:"command"`    // shell command to execute
	Reason     string `json:"reason"`     // human-readable explanation
	Priority   string `json:"priority"`   // "high", "medium", or "low"
	IsMutation bool   `json:"isMutation"` // true if command modifies system state
	Status     string `json:"status"`     // "pending", "executed", "error"
	TaskID     string `json:"taskId,omitempty"`
	CreatedAt  string `json:"createdAt"`
}
