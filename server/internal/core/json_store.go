package core

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// JSONStore is a file-backed storage implementation that persists data as JSON
// files in a directory. It uses in-memory maps with a read-write mutex for
// concurrent access, and writes entire collections to disk on each mutation.
type JSONStore struct {
	mu            sync.RWMutex
	dataDir       string
	encryptionKey []byte

	users             map[string]*jsonUser
	devices           map[string]*jsonDevice
	tasks             map[string]*jsonTask
	taskResults       map[string]*jsonTaskResult
	auditLogs         []*jsonAuditLog
	sessions          map[string]*jsonSession
	enrollmentCodes   map[string]*jsonEnrollmentCode
	deviceCredentials map[string]*jsonDeviceCredential
	templates         map[string]*jsonTemplate
	llmConfig         *jsonLLMConfig
}

// --- JSON record types (mirror domain types for serialization) ---

type jsonUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	DisplayName   string `json:"displayName"`
	Password      string `json:"password"` // bcrypt hash
	Roles         string `json:"roles"`
	MustChangePwd int    `json:"mustChangePassword"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type jsonDevice struct {
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
	CredentialStatus string  `json:"credentialStatus"`
	RevokedAt        string  `json:"revokedAt,omitempty"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}

type jsonTask struct {
	ID             string   `json:"id"`
	DeviceID       string   `json:"deviceId"`
	GroupName      string   `json:"groupName"`
	Command        string   `json:"command"`
	Kind           string   `json:"kind"`
	TemplateID     string   `json:"templateId,omitempty"`
	Executable     string   `json:"executable,omitempty"`
	Args           []string `json:"args,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
	Status         string   `json:"status"`
	RequestedBy    string   `json:"requestedBy"`
	CreatedAt      string   `json:"createdAt"`
	StartedAt      string   `json:"startedAt,omitempty"`
	FinishedAt     string   `json:"finishedAt,omitempty"`
}

type jsonTaskResult struct {
	TaskID     string `json:"taskId"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exitCode"`
	DurationMS int64  `json:"durationMs"`
	CreatedAt  string `json:"createdAt"`
}

type jsonAuditLog struct {
	ID         string `json:"id"`
	Actor      string `json:"actor"`
	ActorID    string `json:"actorId,omitempty"`
	ActorRole  string `json:"actorRole,omitempty"`
	RemoteAddr string `json:"remoteAddr,omitempty"`
	RequestID  string `json:"requestId,omitempty"`
	Action     string `json:"action"`
	DeviceID   string `json:"deviceId,omitempty"`
	TaskID     string `json:"taskId,omitempty"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	CreatedAt  string `json:"createdAt"`
}

type jsonSession struct {
	ID                string `json:"id"`
	UserID            string `json:"userId"`
	TokenHash         string `json:"tokenHash"`
	CSRFHash          string `json:"csrfHash"`
	RemoteAddr        string `json:"remoteAddr,omitempty"`
	UserAgent         string `json:"userAgent,omitempty"`
	CreatedAt         string `json:"createdAt"`
	LastSeenAt        string `json:"lastSeenAt"`
	IdleExpiresAt     string `json:"idleExpiresAt"`
	AbsoluteExpiresAt string `json:"absoluteExpiresAt"`
}

type jsonEnrollmentCode struct {
	ID        string `json:"id"`
	CodeHash  string `json:"codeHash"`
	ExpiresAt string `json:"expiresAt"`
	MaxUses   int    `json:"maxUses"`
	UsedCount int    `json:"usedCount"`
	CreatedBy string `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
	RevokedAt string `json:"revokedAt,omitempty"`
}

type jsonDeviceCredential struct {
	DeviceID   string `json:"deviceId"`
	SecretHash string `json:"secretHash"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
	RevokedAt  string `json:"revokedAt,omitempty"`
}

type jsonTemplate struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Description       string              `json:"description,omitempty"`
	OS                string              `json:"os"`
	Executable        string              `json:"executable"`
	Args              []string            `json:"args,omitempty"`
	Parameters        []TemplateParameter `json:"parameters,omitempty"`
	RequiresPrivilege bool                `json:"requiresPrivilege"`
	Enabled           bool                `json:"enabled"`
	TimeoutSeconds    int                 `json:"timeoutSeconds"`
	CreatedAt         string              `json:"createdAt"`
	UpdatedAt         string              `json:"updatedAt"`
}

type jsonLLMConfig struct {
	ProviderURL         string `json:"providerUrl,omitempty"`
	APIKey              string `json:"apiKey,omitempty"`
	Model               string `json:"model,omitempty"`
	ProviderType        string `json:"providerType"`
	Enabled             bool   `json:"enabled"`
	AutoExecuteReadOnly bool   `json:"autoExecuteReadOnly"`
	UpdatedAt           string `json:"updatedAt,omitempty"`
}

// OpenJSONStore creates or opens a JSONStore backed by the given data directory.
func OpenJSONStore(dataDir string) (*JSONStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("json store: create data dir: %w", err)
	}
	js := &JSONStore{
		dataDir:           dataDir,
		users:             make(map[string]*jsonUser),
		devices:           make(map[string]*jsonDevice),
		tasks:             make(map[string]*jsonTask),
		taskResults:       make(map[string]*jsonTaskResult),
		auditLogs:         make([]*jsonAuditLog, 0),
		sessions:          make(map[string]*jsonSession),
		enrollmentCodes:   make(map[string]*jsonEnrollmentCode),
		deviceCredentials: make(map[string]*jsonDeviceCredential),
		templates:         make(map[string]*jsonTemplate),
		llmConfig:         &jsonLLMConfig{ProviderType: "openai"},
	}
	if err := js.loadAll(); err != nil {
		return nil, err
	}
	return js, nil
}

// --- File I/O helpers ---

func (js *JSONStore) loadFile(name string, dst any) error {
	path := filepath.Join(js.dataDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read %s: %w", name, err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	return nil
}

func (js *JSONStore) saveFile(name string, src any) error {
	path := filepath.Join(js.dataDir, name)
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (js *JSONStore) loadAll() error {
	for _, item := range []struct {
		name string
		dst  any
	}{
		{"users.json", &js.users},
		{"devices.json", &js.devices},
		{"tasks.json", &js.tasks},
		{"task_results.json", &js.taskResults},
		{"audit_logs.json", &js.auditLogs},
		{"sessions.json", &js.sessions},
		{"enrollment_codes.json", &js.enrollmentCodes},
		{"device_credentials.json", &js.deviceCredentials},
		{"templates.json", &js.templates},
		{"llm_config.json", &js.llmConfig},
	} {
		if err := js.loadFile(item.name, item.dst); err != nil {
			return err
		}
	}
	// Ensure maps are non-nil after loading
	if js.users == nil {
		js.users = make(map[string]*jsonUser)
	}
	if js.devices == nil {
		js.devices = make(map[string]*jsonDevice)
	}
	if js.tasks == nil {
		js.tasks = make(map[string]*jsonTask)
	}
	if js.taskResults == nil {
		js.taskResults = make(map[string]*jsonTaskResult)
	}
	if js.auditLogs == nil {
		js.auditLogs = make([]*jsonAuditLog, 0)
	}
	if js.sessions == nil {
		js.sessions = make(map[string]*jsonSession)
	}
	if js.enrollmentCodes == nil {
		js.enrollmentCodes = make(map[string]*jsonEnrollmentCode)
	}
	if js.deviceCredentials == nil {
		js.deviceCredentials = make(map[string]*jsonDeviceCredential)
	}
	if js.templates == nil {
		js.templates = make(map[string]*jsonTemplate)
	}
	if js.llmConfig == nil {
		js.llmConfig = &jsonLLMConfig{ProviderType: "openai"}
	}
	return nil
}

// randomHex generates a hex-encoded random string of n bytes.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// --- Lifecycle ---

func (js *JSONStore) Close() error {
	return nil
}

func (js *JSONStore) ConfigureEncryptionKey(raw string) error {
	if strings.TrimSpace(raw) == "" {
		js.encryptionKey = nil
		return nil
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(key) != 32 {
		return fmt.Errorf("LABOPS_ENCRYPTION_KEY must be standard base64 encoding of exactly 32 random bytes")
	}
	js.encryptionKey = key
	return nil
}

func (js *JSONStore) ProtectStoredLLMSecret(ctx context.Context) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	if len(js.encryptionKey) == 0 || js.llmConfig == nil || js.llmConfig.APIKey == "" || strings.HasPrefix(js.llmConfig.APIKey, encryptedSecretPrefix) {
		return nil
	}
	protected, err := js.encryptSecret(js.llmConfig.APIKey)
	if err != nil {
		return err
	}
	js.llmConfig.APIKey = protected
	js.llmConfig.UpdatedAt = nowString()
	return js.saveFile("llm_config.json", js.llmConfig)
}

func (js *JSONStore) Init(ctx context.Context) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	if err := js.seedCommandTemplatesLocked(); err != nil {
		return err
	}
	return js.bootstrapAdminLocked("admin")
}

func (js *JSONStore) InitSecure(ctx context.Context, bootstrapPassword string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	if err := js.seedCommandTemplatesLocked(); err != nil {
		return err
	}
	if !js.hasActiveAdminLocked() {
		if len(bootstrapPassword) >= 12 {
			return js.bootstrapAdminLocked(bootstrapPassword)
		}
	}
	return nil
}

func (js *JSONStore) bootstrapAdminLocked(password string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	now := nowString()
	js.users["admin"] = &jsonUser{
		ID:            "user_admin",
		Username:      "admin",
		DisplayName:   "LabOps Admin",
		Password:      string(hashed),
		Roles:         RoleAdmin,
		MustChangePwd: 1,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return js.saveFile("users.json", js.users)
}

// --- Users ---

func (js *JSONStore) CountUsers(ctx context.Context) (int, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	return len(js.users), nil
}

func (js *JSONStore) SetupStatus(ctx context.Context) (SetupStatus, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	status := SetupStatus{TotalUsers: len(js.users)}
	for _, rec := range js.users {
		if rec == nil {
			continue
		}
		if rec.Roles == RoleAdmin {
			status.AdminExists = true
			if rec.Status == "" || rec.Status == "active" {
				status.ActiveAdminExists = true
			}
		}
	}
	status.Initialized = status.ActiveAdminExists
	status.RegistrationAllowed = !status.ActiveAdminExists
	status.RecoveryRequired = status.TotalUsers > 0 && !status.ActiveAdminExists
	return status, nil
}

func (js *JSONStore) TryCreateInitialAdmin(ctx context.Context, username, displayName, password string) (User, error) {
	return js.BootstrapFirstAdmin(ctx, BootstrapAdminInput{Username: username, DisplayName: displayName, Password: password, ConfirmPassword: password})
}

func (js *JSONStore) BootstrapFirstAdmin(ctx context.Context, input BootstrapAdminInput) (User, error) {
	js.mu.Lock()
	defer js.mu.Unlock()
	if js.hasActiveAdminLocked() {
		return User{}, ErrAlreadyInitialized
	}
	if _, exists := js.users[input.Username]; exists {
		return User{}, ErrUsernameExists
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return User{}, err
	}
	now := nowString()
	id := newID("user")
	js.users[input.Username] = &jsonUser{
		ID:            id,
		Username:      input.Username,
		DisplayName:   input.DisplayName,
		Password:      string(hashed),
		Roles:         RoleAdmin,
		MustChangePwd: 0,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := js.saveFile("users.json", js.users); err != nil {
		return User{}, err
	}
	user := User{ID: id, Username: input.Username, DisplayName: input.DisplayName, Status: "active"}
	applyUserAuthorization(&user, RoleAdmin)
	return user, nil
}

func (js *JSONStore) hasActiveAdminLocked() bool {
	for _, rec := range js.users {
		if rec == nil {
			continue
		}
		if rec.Roles == RoleAdmin && (rec.Status == "" || rec.Status == "active") {
			return true
		}
	}
	return false
}

func (js *JSONStore) FindUser(ctx context.Context, username, password string) (User, bool, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	rec, ok := js.users[username]
	if !ok {
		return User{}, false, nil
	}
	if rec.Status != "active" {
		return User{}, false, nil
	}
	if bcrypt.CompareHashAndPassword([]byte(rec.Password), []byte(password)) != nil {
		return User{}, false, nil
	}
	user := User{
		ID:          rec.ID,
		Username:    rec.Username,
		DisplayName: rec.DisplayName,
		Status:      rec.Status,
	}
	applyUserAuthorization(&user, rec.Roles)
	return user, true, nil
}

func (js *JSONStore) FindUserByUsername(ctx context.Context, username string) (User, bool, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	rec, ok := js.users[username]
	if !ok {
		return User{}, false, nil
	}
	user := User{
		ID:          rec.ID,
		Username:    rec.Username,
		DisplayName: rec.DisplayName,
		Status:      rec.Status,
	}
	applyUserAuthorization(&user, rec.Roles)
	return user, true, nil
}

func (js *JSONStore) AdminUser() User {
	js.mu.RLock()
	defer js.mu.RUnlock()
	for _, rec := range js.users {
		if rec.Roles == RoleAdmin || containsRole(rec.Roles, RoleAdmin) {
			user := User{ID: rec.ID, Username: rec.Username, DisplayName: rec.DisplayName, Status: rec.Status}
			applyUserAuthorization(&user, rec.Roles)
			return user
		}
	}
	return User{ID: "user_admin", Username: "admin", DisplayName: "LabOps Admin", Status: "active", Roles: []string{RoleAdmin}, Role: RoleAdmin, Permissions: permissionsByRole[RoleAdmin]}
}

func (js *JSONStore) CreateUser(ctx context.Context, username, displayName, password, role string) (User, error) {
	if !validRole(role) {
		return User{}, fmt.Errorf("invalid role")
	}
	if len(password) < 12 {
		return User{}, fmt.Errorf("password must be at least 12 characters")
	}
	js.mu.Lock()
	defer js.mu.Unlock()
	if _, exists := js.users[username]; exists {
		return User{}, fmt.Errorf("username already exists")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return User{}, err
	}
	now := nowString()
	id := "user_" + newID("")
	js.users[username] = &jsonUser{
		ID:            id,
		Username:      username,
		DisplayName:   displayName,
		Password:      string(hashed),
		Roles:         role,
		MustChangePwd: 1,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_ = js.saveFile("users.json", js.users)
	user := User{ID: id, Username: username, DisplayName: displayName, Status: "active"}
	applyUserAuthorization(&user, role)
	return user, nil
}

func (js *JSONStore) UpdatePassword(ctx context.Context, username, newPassword string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	return js.setPasswordLocked(username, newPassword)
}

func (js *JSONStore) MustChangePassword(ctx context.Context, username string) bool {
	js.mu.RLock()
	defer js.mu.RUnlock()
	rec, ok := js.users[username]
	if !ok {
		return false
	}
	return rec.MustChangePwd == 1
}

func (js *JSONStore) UpdateUserAccess(ctx context.Context, id, role, status string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	for _, rec := range js.users {
		if rec.ID == id {
			rec.Roles = role
			rec.Status = status
			rec.UpdatedAt = nowString()
			_ = js.saveFile("users.json", js.users)
			return nil
		}
	}
	return fmt.Errorf("user not found")
}

func (js *JSONStore) ListUsers(ctx context.Context) ([]User, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	users := make([]User, 0, len(js.users))
	for _, rec := range js.users {
		user := User{ID: rec.ID, Username: rec.Username, DisplayName: rec.DisplayName, Status: rec.Status}
		applyUserAuthorization(&user, rec.Roles)
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })
	return users, nil
}

func (js *JSONStore) setPasswordLocked(username, newPassword string) error {
	rec, ok := js.users[username]
	if !ok {
		return fmt.Errorf("user not found")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}
	rec.Password = string(hashed)
	rec.MustChangePwd = 0
	rec.UpdatedAt = nowString()
	return js.saveFile("users.json", js.users)
}

func containsRole(roles, target string) bool {
	for _, r := range strings_split(roles, ",") {
		if strings_trim(r) == target {
			return true
		}
	}
	return false
}

func strings_split(s, sep string) []string {
	if s == "" {
		return nil
	}
	result := make([]string, 0)
	for len(s) > 0 {
		idx := len(s)
		for i := 0; i < len(s); i++ {
			if string(s[i]) == sep {
				idx = i
				break
			}
		}
		result = append(result, s[:idx])
		if idx >= len(s) {
			break
		}
		s = s[idx+1:]
	}
	return result
}

func strings_trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// --- Agent Sessions ---

func (js *JSONStore) CreateSession(ctx context.Context, deviceID, remoteAddr string) (int64, error) {
	return time.Now().UnixNano(), nil
}

func (js *JSONStore) CloseSession(ctx context.Context, id int64) error {
	return nil
}

// --- Web Sessions ---

func (js *JSONStore) CreateWebSession(ctx context.Context, userID, remoteAddr, userAgent string) (string, string, error) {
	js.mu.Lock()
	defer js.mu.Unlock()
	sessionToken := randomHex(32)
	csrfToken := randomHex(32)
	now := nowString()
	idleExpires := time.Now().UTC().Add(8 * time.Hour).Format(time.RFC3339)
	absExpires := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	id := "sess_" + newID("")
	js.sessions[id] = &jsonSession{
		ID:                id,
		UserID:            userID,
		TokenHash:         tokenHash(sessionToken),
		CSRFHash:          tokenHash(csrfToken),
		RemoteAddr:        remoteAddr,
		UserAgent:         userAgent,
		CreatedAt:         now,
		LastSeenAt:        now,
		IdleExpiresAt:     idleExpires,
		AbsoluteExpiresAt: absExpires,
	}
	_ = js.saveFile("sessions.json", js.sessions)
	return sessionToken, csrfToken, nil
}

func (js *JSONStore) AuthenticateWebSession(ctx context.Context, rawToken string) (User, string, string, bool, error) {
	js.mu.Lock()
	defer js.mu.Unlock()
	th := tokenHash(rawToken)
	for id, sess := range js.sessions {
		if sess.TokenHash != th {
			continue
		}
		absExp, _ := time.Parse(time.RFC3339, sess.AbsoluteExpiresAt)
		if time.Now().UTC().After(absExp) {
			delete(js.sessions, id)
			return User{}, "", "", false, nil
		}
		idleExp, _ := time.Parse(time.RFC3339, sess.IdleExpiresAt)
		if time.Now().UTC().After(idleExp) {
			delete(js.sessions, id)
			return User{}, "", "", false, nil
		}
		rec, ok := js.users_byID(sess.UserID)
		if !ok || rec.Status != "active" {
			return User{}, "", "", false, nil
		}
		user := User{ID: rec.ID, Username: rec.Username, DisplayName: rec.DisplayName, Status: rec.Status}
		applyUserAuthorization(&user, rec.Roles)
		sess.LastSeenAt = nowString()
		sess.IdleExpiresAt = time.Now().UTC().Add(8 * time.Hour).Format(time.RFC3339)
		return user, id, sess.CSRFHash, true, nil
	}
	return User{}, "", "", false, nil
}

func (js *JSONStore) users_byID(id string) (*jsonUser, bool) {
	for _, u := range js.users {
		if u.ID == id {
			return u, true
		}
	}
	return nil, false
}

func (js *JSONStore) DeleteWebSession(ctx context.Context, sessionID string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	delete(js.sessions, sessionID)
	return js.saveFile("sessions.json", js.sessions)
}

func (js *JSONStore) DeleteUserSessions(ctx context.Context, userID string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	for id, sess := range js.sessions {
		if sess.UserID == userID {
			delete(js.sessions, id)
		}
	}
	return js.saveFile("sessions.json", js.sessions)
}

func (js *JSONStore) PruneExpiredWebSessions(ctx context.Context) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	now := time.Now().UTC()
	changed := false
	for id, sess := range js.sessions {
		absExp, _ := time.Parse(time.RFC3339, sess.AbsoluteExpiresAt)
		if now.After(absExp) {
			delete(js.sessions, id)
			changed = true
			continue
		}
		idleExp, _ := time.Parse(time.RFC3339, sess.IdleExpiresAt)
		if now.After(idleExp) {
			delete(js.sessions, id)
			changed = true
		}
	}
	if changed {
		return js.saveFile("sessions.json", js.sessions)
	}
	return nil
}

// --- Devices ---

func (js *JSONStore) UpsertDevice(ctx context.Context, d Device) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	now := nowString()
	if existing, ok := js.devices[d.ID]; ok {
		existing.Name = d.Name
		existing.GroupName = d.GroupName
		existing.Profile = d.Profile
		existing.Version = d.Version
		existing.Hostname = d.Hostname
		existing.OS = d.OS
		existing.IP = d.IP
		existing.CPUCores = d.CPUCores
		existing.MemoryMB = d.MemoryMB
		existing.DiskTotalGB = d.DiskTotalGB
		existing.UpdatedAt = now
	} else {
		js.devices[d.ID] = &jsonDevice{
			ID: d.ID, Name: d.Name, GroupName: d.GroupName, Profile: d.Profile,
			Version: d.Version, Hostname: d.Hostname, OS: d.OS, IP: d.IP,
			CPUCores: d.CPUCores, MemoryMB: d.MemoryMB, DiskTotalGB: d.DiskTotalGB,
			Status: StatusOnline, LastSeen: now, CreatedAt: now, UpdatedAt: now,
		}
	}
	return js.saveFile("devices.json", js.devices)
}

func (js *JSONStore) CreateDevice(ctx context.Context, d Device) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	if _, exists := js.devices[d.ID]; exists {
		return fmt.Errorf("device already exists")
	}
	js.devices[d.ID] = &jsonDevice{
		ID: d.ID, Name: d.Name, GroupName: d.GroupName, Profile: d.Profile,
		Version: d.Version, Hostname: d.Hostname, OS: d.OS, IP: d.IP,
		CPUCores: d.CPUCores, MemoryMB: d.MemoryMB, DiskTotalGB: d.DiskTotalGB,
		Status: StatusOffline, CreatedAt: nowString(), UpdatedAt: nowString(),
	}
	return js.saveFile("devices.json", js.devices)
}

func (js *JSONStore) DeleteDevice(ctx context.Context, id string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	delete(js.devices, id)
	return js.saveFile("devices.json", js.devices)
}

func (js *JSONStore) UpdateHeartbeat(ctx context.Context, deviceID string, hb HeartbeatPayload) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	d, ok := js.devices[deviceID]
	if !ok {
		return nil
	}
	d.CPUUsage = hb.CPUUsage
	d.MemoryUsage = hb.MemoryUsage
	d.DiskUsage = hb.DiskUsage
	d.Status = StatusOnline
	d.LastSeen = nowString()
	return js.saveFile("devices.json", js.devices)
}

func (js *JSONStore) MarkDeviceOffline(ctx context.Context, deviceID string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	d, ok := js.devices[deviceID]
	if !ok {
		return nil
	}
	d.Status = StatusOffline
	return js.saveFile("devices.json", js.devices)
}

func (js *JSONStore) ExpireDevices(ctx context.Context, cutoff string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	for _, d := range js.devices {
		if d.Status == StatusOnline && d.LastSeen < cutoff {
			d.Status = StatusOffline
		}
	}
	return js.saveFile("devices.json", js.devices)
}

func (js *JSONStore) ListDevices(ctx context.Context) ([]Device, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	return js.listDevicesLocked(), nil
}

func (js *JSONStore) listDevicesLocked() []Device {
	result := make([]Device, 0, len(js.devices))
	for _, d := range js.devices {
		result = append(result, js.deviceFromJSON(d))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (js *JSONStore) deviceFromJSON(d *jsonDevice) Device {
	return Device{
		ID: d.ID, Name: d.Name, GroupName: d.GroupName, Profile: d.Profile,
		Version: d.Version, Hostname: d.Hostname, OS: d.OS, IP: d.IP,
		CPUCores: d.CPUCores, MemoryMB: d.MemoryMB, DiskTotalGB: d.DiskTotalGB,
		CPUUsage: d.CPUUsage, MemoryUsage: d.MemoryUsage, DiskUsage: d.DiskUsage,
		Status: d.Status, LastSeen: d.LastSeen, CredentialStatus: d.CredentialStatus,
		RevokedAt: d.RevokedAt, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func (js *JSONStore) ListDevicesByGroup(ctx context.Context, groupName string) ([]Device, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	result := make([]Device, 0)
	for _, d := range js.devices {
		if d.GroupName == groupName {
			result = append(result, js.deviceFromJSON(d))
		}
	}
	return result, nil
}

func (js *JSONStore) GetDevice(ctx context.Context, id string) (Device, bool, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	d, ok := js.devices[id]
	if !ok {
		return Device{}, false, nil
	}
	return js.deviceFromJSON(d), true, nil
}

func (js *JSONStore) Stats(ctx context.Context) (DeviceStats, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	stats := DeviceStats{}
	for _, d := range js.devices {
		stats.Total++
		if d.Status == StatusOnline {
			stats.Online++
		} else {
			stats.Offline++
		}
	}
	return stats, nil
}

func (js *JSONStore) Groups(ctx context.Context) ([]DeviceGroup, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	groupMap := make(map[string]*DeviceGroup)
	for _, d := range js.devices {
		g, ok := groupMap[d.GroupName]
		if !ok {
			g = &DeviceGroup{Name: d.GroupName}
			groupMap[d.GroupName] = g
		}
		g.Total++
		if d.Status == StatusOnline {
			g.Online++
		}
	}
	result := make([]DeviceGroup, 0, len(groupMap))
	for _, g := range groupMap {
		result = append(result, *g)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// --- Tasks ---

func (js *JSONStore) CreateTask(ctx context.Context, deviceID, groupName, command, requestedBy string) (Task, error) {
	return js.CreateTaskSpec(ctx, Task{
		DeviceID: deviceID, GroupName: groupName, Command: command,
		Kind: TaskKindAdHoc, RequestedBy: requestedBy, TimeoutSeconds: 300,
	})
}

func (js *JSONStore) CreateTaskSpec(ctx context.Context, task Task) (Task, error) {
	js.mu.Lock()
	defer js.mu.Unlock()
	if task.ID == "" {
		task.ID = "task_" + newID("")
	}
	task.Status = StatusPending
	task.CreatedAt = nowString()
	js.tasks[task.ID] = &jsonTask{
		ID: task.ID, DeviceID: task.DeviceID, GroupName: task.GroupName,
		Command: task.Command, Kind: task.Kind, TemplateID: task.TemplateID,
		Executable: task.Executable, Args: task.Args,
		TimeoutSeconds: task.TimeoutSeconds, Status: task.Status,
		RequestedBy: task.RequestedBy, CreatedAt: task.CreatedAt,
	}
	_ = js.saveFile("tasks.json", js.tasks)
	return task, nil
}

func (js *JSONStore) GetTask(ctx context.Context, id string) (Task, bool, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	t, ok := js.tasks[id]
	if !ok {
		return Task{}, false, nil
	}
	task := js.taskFromJSON(t)
	if tr, ok := js.taskResults[id]; ok {
		task.Result = &TaskResult{
			TaskID: tr.TaskID, Stdout: tr.Stdout, Stderr: tr.Stderr,
			ExitCode: tr.ExitCode, DurationMS: tr.DurationMS, CreatedAt: tr.CreatedAt,
		}
	}
	return task, true, nil
}

func (js *JSONStore) taskFromJSON(t *jsonTask) Task {
	return Task{
		ID: t.ID, DeviceID: t.DeviceID, GroupName: t.GroupName,
		Command: t.Command, Kind: t.Kind, TemplateID: t.TemplateID,
		Executable: t.Executable, Args: t.Args,
		TimeoutSeconds: t.TimeoutSeconds, Status: t.Status,
		RequestedBy: t.RequestedBy, CreatedAt: t.CreatedAt,
		StartedAt: t.StartedAt, FinishedAt: t.FinishedAt,
	}
}

func (js *JSONStore) ListTasks(ctx context.Context) ([]Task, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	result := make([]Task, 0, len(js.tasks))
	for _, t := range js.tasks {
		task := js.taskFromJSON(t)
		if tr, ok := js.taskResults[t.ID]; ok {
			task.Result = &TaskResult{
				TaskID: tr.TaskID, Stdout: tr.Stdout, Stderr: tr.Stderr,
				ExitCode: tr.ExitCode, DurationMS: tr.DurationMS, CreatedAt: tr.CreatedAt,
			}
		}
		result = append(result, task)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt > result[j].CreatedAt })
	if len(result) > 200 {
		result = result[:200]
	}
	return result, nil
}

func (js *JSONStore) ListTasksByDevice(ctx context.Context, deviceID string) ([]Task, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	result := make([]Task, 0)
	for _, t := range js.tasks {
		if t.DeviceID == deviceID {
			task := js.taskFromJSON(t)
			if tr, ok := js.taskResults[t.ID]; ok {
				task.Result = &TaskResult{
					TaskID: tr.TaskID, Stdout: tr.Stdout, Stderr: tr.Stderr,
					ExitCode: tr.ExitCode, DurationMS: tr.DurationMS, CreatedAt: tr.CreatedAt,
				}
			}
			result = append(result, task)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt > result[j].CreatedAt })
	return result, nil
}

func (js *JSONStore) MarkTaskRunning(ctx context.Context, taskID string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	t, ok := js.tasks[taskID]
	if !ok {
		return nil
	}
	t.Status = StatusRunning
	t.StartedAt = nowString()
	return js.saveFile("tasks.json", js.tasks)
}

func (js *JSONStore) CompleteTask(ctx context.Context, result TaskResultPayload) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	t, ok := js.tasks[result.TaskID]
	if ok {
		if result.Status == "success" {
			t.Status = StatusSuccess
		} else {
			t.Status = StatusFailed
		}
		t.FinishedAt = nowString()
	}
	now := nowString()
	js.taskResults[result.TaskID] = &jsonTaskResult{
		TaskID: result.TaskID, Stdout: result.Stdout, Stderr: result.Stderr,
		ExitCode: result.ExitCode, DurationMS: result.DurationMS, CreatedAt: now,
	}
	_ = js.saveFile("tasks.json", js.tasks)
	return js.saveFile("task_results.json", js.taskResults)
}

func (js *JSONStore) FailTask(ctx context.Context, taskID, stderr string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	t, ok := js.tasks[taskID]
	if ok {
		t.Status = StatusFailed
		t.FinishedAt = nowString()
	}
	now := nowString()
	js.taskResults[taskID] = &jsonTaskResult{
		TaskID: taskID, Stderr: stderr, ExitCode: 1, CreatedAt: now,
	}
	_ = js.saveFile("tasks.json", js.tasks)
	return js.saveFile("task_results.json", js.taskResults)
}

func (js *JSONStore) TimeoutTasks(ctx context.Context, cutoff string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	for _, t := range js.tasks {
		if (t.Status == StatusPending || t.Status == StatusRunning) && t.CreatedAt < cutoff {
			t.Status = StatusTimeout
		}
	}
	return js.saveFile("tasks.json", js.tasks)
}

func (js *JSONStore) PendingTasksForDevice(ctx context.Context, deviceID string) ([]Task, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	result := make([]Task, 0)
	for _, t := range js.tasks {
		if t.DeviceID == deviceID && (t.Status == StatusPending || t.Status == StatusRunning) {
			result = append(result, js.taskFromJSON(t))
		}
	}
	return result, nil
}

// --- Audit ---

func (js *JSONStore) CreateAudit(ctx context.Context, audit AuditLog) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	if audit.ID == "" {
		audit.ID = "audit_" + newID("")
	}
	if audit.CreatedAt == "" {
		audit.CreatedAt = nowString()
	}
	js.auditLogs = append(js.auditLogs, &jsonAuditLog{
		ID: audit.ID, Actor: audit.Actor, ActorID: audit.ActorID, ActorRole: audit.ActorRole,
		RemoteAddr: audit.RemoteAddr, RequestID: audit.RequestID,
		Action: audit.Action, DeviceID: audit.DeviceID, TaskID: audit.TaskID,
		Status: audit.Status, Message: audit.Message, CreatedAt: audit.CreatedAt,
	})
	return js.saveFile("audit_logs.json", js.auditLogs)
}

func (js *JSONStore) ListAudit(ctx context.Context) ([]AuditLog, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	result := make([]AuditLog, 0, len(js.auditLogs))
	for _, a := range js.auditLogs {
		result = append(result, AuditLog{
			ID: a.ID, Actor: a.Actor, ActorID: a.ActorID, ActorRole: a.ActorRole,
			RemoteAddr: a.RemoteAddr, RequestID: a.RequestID,
			Action: a.Action, DeviceID: a.DeviceID, TaskID: a.TaskID,
			Status: a.Status, Message: a.Message, CreatedAt: a.CreatedAt,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt > result[j].CreatedAt })
	if len(result) > 200 {
		result = result[:200]
	}
	return result, nil
}

// --- Enrollment ---

func (js *JSONStore) CreateEnrollmentCode(ctx context.Context, createdBy string, ttl time.Duration, maxUses int) (EnrollmentCode, error) {
	if ttl <= 0 || ttl > time.Hour || maxUses < 1 || maxUses > 20 {
		return EnrollmentCode{}, fmt.Errorf("ttl must be at most 1 hour and maxUses between 1 and 20")
	}
	js.mu.Lock()
	defer js.mu.Unlock()
	code, err := randomToken(24)
	if err != nil {
		return EnrollmentCode{}, err
	}
	id := "enroll_" + newID("")
	now := nowString()
	js.enrollmentCodes[id] = &jsonEnrollmentCode{
		ID: id, CodeHash: tokenHash(code),
		ExpiresAt: time.Now().UTC().Add(ttl).Format(time.RFC3339),
		MaxUses:   maxUses, CreatedBy: createdBy, CreatedAt: now,
	}
	_ = js.saveFile("enrollment_codes.json", js.enrollmentCodes)
	return EnrollmentCode{ID: id, Code: code, ExpiresAt: js.enrollmentCodes[id].ExpiresAt, MaxUses: maxUses, UsedCount: 0, CreatedBy: createdBy, CreatedAt: now}, nil
}

func (js *JSONStore) ListEnrollmentCodes(ctx context.Context) ([]EnrollmentCode, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	result := make([]EnrollmentCode, 0, len(js.enrollmentCodes))
	for _, ec := range js.enrollmentCodes {
		result = append(result, EnrollmentCode{
			ID: ec.ID, ExpiresAt: ec.ExpiresAt, MaxUses: ec.MaxUses,
			UsedCount: ec.UsedCount, CreatedBy: ec.CreatedBy, CreatedAt: ec.CreatedAt, RevokedAt: ec.RevokedAt,
		})
	}
	return result, nil
}

func (js *JSONStore) RevokeEnrollmentCode(ctx context.Context, id string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	ec, ok := js.enrollmentCodes[id]
	if !ok {
		return fmt.Errorf("enrollment code not found")
	}
	ec.RevokedAt = nowString()
	return js.saveFile("enrollment_codes.json", js.enrollmentCodes)
}

func (js *JSONStore) EnrollDevice(ctx context.Context, rawCode string, reg RegisterPayload) (Device, string, error) {
	js.mu.Lock()
	defer js.mu.Unlock()
	codeHash := tokenHash(rawCode)
	var found *jsonEnrollmentCode
	for _, ec := range js.enrollmentCodes {
		if ec.CodeHash == codeHash && ec.RevokedAt == "" {
			exp, _ := time.Parse(time.RFC3339, ec.ExpiresAt)
			if time.Now().UTC().Before(exp) && ec.UsedCount < ec.MaxUses {
				found = ec
				break
			}
		}
	}
	if found == nil {
		return Device{}, "", fmt.Errorf("invalid or expired enrollment code")
	}
	found.UsedCount++
	_ = js.saveFile("enrollment_codes.json", js.enrollmentCodes)

	deviceID := reg.AgentID
	if deviceID == "" {
		deviceID = "agent_" + newID("")
	}
	secret := randomHex(32)
	now := nowString()
	js.deviceCredentials[deviceID] = &jsonDeviceCredential{
		DeviceID: deviceID, SecretHash: tokenHash(secret),
		Status: "active", CreatedAt: now,
	}
	_ = js.saveFile("device_credentials.json", js.deviceCredentials)

	device := Device{
		ID: deviceID, Name: reg.Name, GroupName: reg.GroupName,
		Profile: reg.Profile, Version: reg.Version,
		Hostname: reg.Hostname, OS: reg.OS, IP: reg.IP,
		CPUCores: reg.CPUCores, MemoryMB: reg.MemoryMB, DiskTotalGB: reg.DiskTotalGB,
		Status: StatusOnline, LastSeen: now, CredentialStatus: "active",
		CreatedAt: now, UpdatedAt: now,
	}
	js.devices[deviceID] = &jsonDevice{
		ID: device.ID, Name: device.Name, GroupName: device.GroupName,
		Profile: device.Profile, Version: device.Version,
		Hostname: device.Hostname, OS: device.OS, IP: device.IP,
		CPUCores: device.CPUCores, MemoryMB: device.MemoryMB, DiskTotalGB: device.DiskTotalGB,
		Status: StatusOnline, LastSeen: now, CredentialStatus: "active",
		CreatedAt: now, UpdatedAt: now,
	}
	_ = js.saveFile("devices.json", js.devices)
	return device, secret, nil
}

func (js *JSONStore) ValidateDeviceCredential(ctx context.Context, deviceID, secret string) (bool, error) {
	js.mu.Lock()
	defer js.mu.Unlock()
	dc, ok := js.deviceCredentials[deviceID]
	if !ok || dc.Status != "active" {
		return false, nil
	}
	valid := subtle.ConstantTimeCompare([]byte(tokenHash(secret)), []byte(dc.SecretHash)) == 1
	if valid {
		dc.LastUsedAt = nowString()
		_ = js.saveFile("device_credentials.json", js.deviceCredentials)
	}
	return valid, nil
}

func (js *JSONStore) RevokeDeviceCredential(ctx context.Context, deviceID string) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	dc, ok := js.deviceCredentials[deviceID]
	if !ok {
		return fmt.Errorf("device credential not found")
	}
	dc.Status = "revoked"
	dc.RevokedAt = nowString()
	_ = js.saveFile("device_credentials.json", js.deviceCredentials)
	if d, ok := js.devices[deviceID]; ok {
		d.CredentialStatus = "revoked"
		d.RevokedAt = nowString()
		_ = js.saveFile("devices.json", js.devices)
	}
	return nil
}

// --- Templates ---

func (js *JSONStore) ListCommandTemplates(ctx context.Context) ([]CommandTemplate, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	result := make([]CommandTemplate, 0, len(js.templates))
	for _, t := range js.templates {
		result = append(result, CommandTemplate{
			ID: t.ID, Name: t.Name, Description: t.Description, OS: t.OS,
			Executable: t.Executable, Args: t.Args, Parameters: t.Parameters,
			RequiresPrivilege: t.RequiresPrivilege, Enabled: t.Enabled,
			TimeoutSeconds: t.TimeoutSeconds, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		})
	}
	return result, nil
}

func (js *JSONStore) GetCommandTemplate(ctx context.Context, id string) (CommandTemplate, bool, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	t, ok := js.templates[id]
	if !ok {
		return CommandTemplate{}, false, nil
	}
	return CommandTemplate{
		ID: t.ID, Name: t.Name, Description: t.Description, OS: t.OS,
		Executable: t.Executable, Args: t.Args, Parameters: t.Parameters,
		RequiresPrivilege: t.RequiresPrivilege, Enabled: t.Enabled,
		TimeoutSeconds: t.TimeoutSeconds, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}, true, nil
}

func (js *JSONStore) SaveCommandTemplate(ctx context.Context, item CommandTemplate) (CommandTemplate, error) {
	js.mu.Lock()
	defer js.mu.Unlock()
	if err := validateTemplate(item); err != nil {
		return CommandTemplate{}, err
	}
	now := nowString()
	if item.ID == "" {
		item.ID = "tmpl_" + newID("")
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	js.templates[item.ID] = &jsonTemplate{
		ID: item.ID, Name: item.Name, Description: item.Description, OS: item.OS,
		Executable: item.Executable, Args: item.Args, Parameters: item.Parameters,
		RequiresPrivilege: item.RequiresPrivilege, Enabled: item.Enabled,
		TimeoutSeconds: item.TimeoutSeconds, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	_ = js.saveFile("templates.json", js.templates)
	return item, nil
}

func (js *JSONStore) seedCommandTemplatesLocked() error {
	if len(js.templates) > 0 {
		return nil
	}
	now := nowString()
	for _, item := range defaultCommandTemplates() {
		if err := validateTemplate(item); err != nil {
			return err
		}
		item.CreatedAt = now
		item.UpdatedAt = now
		js.templates[item.ID] = &jsonTemplate{
			ID: item.ID, Name: item.Name, Description: item.Description, OS: item.OS,
			Executable: item.Executable, Args: item.Args, Parameters: item.Parameters,
			RequiresPrivilege: item.RequiresPrivilege, Enabled: item.Enabled,
			TimeoutSeconds: item.TimeoutSeconds, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		}
	}
	return js.saveFile("templates.json", js.templates)
}

// --- LLM Config ---

func (js *JSONStore) GetLLMConfig(ctx context.Context) (LLMConfig, error) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	apiKey, err := js.decryptSecret(js.llmConfig.APIKey)
	if err != nil {
		return LLMConfig{}, err
	}
	return LLMConfig{
		ProviderURL: js.llmConfig.ProviderURL, APIKey: apiKey,
		Model: js.llmConfig.Model, ProviderType: js.llmConfig.ProviderType,
		Enabled: js.llmConfig.Enabled, AutoExecuteReadOnly: js.llmConfig.AutoExecuteReadOnly,
		UpdatedAt: js.llmConfig.UpdatedAt,
	}, nil
}

func (js *JSONStore) SaveLLMConfig(ctx context.Context, cfg LLMConfig) error {
	js.mu.Lock()
	defer js.mu.Unlock()
	apiKey, err := js.encryptSecret(cfg.APIKey)
	if err != nil {
		return err
	}
	js.llmConfig.ProviderURL = cfg.ProviderURL
	js.llmConfig.APIKey = apiKey
	js.llmConfig.Model = cfg.Model
	js.llmConfig.ProviderType = cfg.ProviderType
	js.llmConfig.Enabled = cfg.Enabled
	js.llmConfig.AutoExecuteReadOnly = cfg.AutoExecuteReadOnly
	js.llmConfig.UpdatedAt = nowString()
	return js.saveFile("llm_config.json", js.llmConfig)
}

func (js *JSONStore) encryptSecret(value string) (string, error) {
	if value == "" || strings.HasPrefix(value, encryptedSecretPrefix) || len(js.encryptionKey) == 0 {
		return value, nil
	}
	block, err := aes.NewCipher(js.encryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)
	payload := append(nonce, ciphertext...)
	return encryptedSecretPrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

func (js *JSONStore) decryptSecret(value string) (string, error) {
	if value == "" || !strings.HasPrefix(value, encryptedSecretPrefix) {
		return value, nil
	}
	if len(js.encryptionKey) == 0 {
		return "", fmt.Errorf("encrypted value exists but LABOPS_ENCRYPTION_KEY is not configured")
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, encryptedSecretPrefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(js.encryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted payload is truncated")
	}
	plain, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// userByIDLocked finds a user by ID (caller must hold at least RLock)
// Used internally for AdminUser without requiring context
