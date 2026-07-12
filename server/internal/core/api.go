package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token              string `json:"token,omitempty"`
	User               User   `json:"user"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type createTaskRequest struct {
	DeviceID     string         `json:"deviceId"`
	GroupName    string         `json:"groupName"`
	Kind         string         `json:"kind"`
	Command      string         `json:"command"`
	TemplateID   string         `json:"templateId"`
	Arguments    map[string]any `json:"arguments"`
	Confirmation string         `json:"confirmation"`
}

type createDeviceRequest struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	GroupName   string `json:"groupName"`
	Hostname    string `json:"hostname,omitempty"`
	OS          string `json:"os,omitempty"`
	IP          string `json:"ip,omitempty"`
	CPUCores    int    `json:"cpuCores,omitempty"`
	MemoryMB    int    `json:"memoryMb,omitempty"`
	DiskTotalGB int    `json:"diskTotalGb,omitempty"`
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !readJSON(w, r, &req) {
		return
	}
	user, ok, err := a.store.FindUser(r.Context(), req.Username, req.Password)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "LOGIN_FAILED", "unable to complete login")
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	sessionToken, csrfToken, err := a.store.CreateWebSession(r.Context(), user.ID, clientIP(r), r.UserAgent())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "SESSION_CREATE_FAILED", "unable to create session")
		return
	}
	setAuthCookies(w, sessionToken, csrfToken, a.config.SecureCookies)

	// Check if must change password (from DB)
	mustChange := a.store.MustChangePassword(r.Context(), user.Username)

	writeJSON(w, http.StatusOK, loginResponse{
		User:               user,
		MustChangePassword: mustChange,
	})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r.Context()))
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if !readJSON(w, r, &req) {
		return
	}
	if req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "new password is required")
		return
	}
	if len(req.NewPassword) < 12 {
		writeAPIError(w, http.StatusBadRequest, "PASSWORD_TOO_SHORT", "password must be at least 12 characters")
		return
	}
	user := currentUser(r.Context())
	username := user.Username

	// Prevent password reuse
	if req.NewPassword == req.OldPassword {
		writeError(w, http.StatusBadRequest, "new password must differ from current password")
		return
	}

	// Verify old password
	_, ok, err := a.store.FindUser(r.Context(), username, req.OldPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "old password is incorrect")
		return
	}

	// Update password
	if err := a.store.UpdatePassword(r.Context(), username, req.NewPassword); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := a.store.DeleteUserSessions(r.Context(), user.ID); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "SESSION_REVOKE_FAILED", "password changed but sessions could not be revoked")
		return
	}
	sessionToken, csrfToken, err := a.store.CreateWebSession(r.Context(), user.ID, clientIP(r), r.UserAgent())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "SESSION_CREATE_FAILED", "password changed; please log in again")
		return
	}
	setAuthCookies(w, sessionToken, csrfToken, a.config.SecureCookies)
	writeJSON(w, http.StatusOK, map[string]string{"status": "password_changed"})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	auth := currentAuth(r.Context())
	if auth.SessionID != "" {
		_ = a.store.DeleteWebSession(r.Context(), auth.SessionID)
	}
	clearAuthCookies(w, a.config.SecureCookies)
	w.WriteHeader(http.StatusNoContent)
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)

func (a *App) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.store.ListUsers(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "USERS_LIST_FAILED", "unable to list users")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username, DisplayName, Password, Role string
	}
	if !readJSON(w, r, &req) {
		return
	}
	req.Username = normalizeUsername(req.Username)
	if !usernamePattern.MatchString(req.Username) || strings.TrimSpace(req.DisplayName) == "" {
		writeAPIError(w, http.StatusBadRequest, "USER_INVALID", "invalid username or display name")
		return
	}
	user, err := a.store.CreateUser(r.Context(), req.Username, strings.TrimSpace(req.DisplayName), req.Password, req.Role)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "USER_CREATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (a *App) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	var req struct{ Role, Status string }
	if !readJSON(w, r, &req) {
		return
	}
	current := currentUser(r.Context())
	if r.PathValue("id") == current.ID && (req.Status == "disabled" || req.Role != current.Role) {
		writeAPIError(w, http.StatusBadRequest, "SELF_ACCESS_CHANGE_FORBIDDEN", "cannot disable or change the role of the current user")
		return
	}
	if err := a.store.UpdateUserAccess(r.Context(), r.PathValue("id"), req.Role, req.Status); err != nil {
		writeAPIError(w, http.StatusBadRequest, "USER_UPDATE_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleStats(w http.ResponseWriter, r *http.Request) {
	a.refreshState(r.Context())
	stats, err := a.store.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (a *App) handleListDevices(w http.ResponseWriter, r *http.Request) {
	a.refreshState(r.Context())
	devices, err := a.store.ListDevices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

func (a *App) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	a.refreshState(r.Context())
	device, ok, err := a.store.GetDevice(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, device)
}

func (a *App) handleListDeviceTasks(w http.ResponseWriter, r *http.Request) {
	a.refreshState(r.Context())
	deviceID := r.PathValue("id")
	tasks, err := a.store.ListTasksByDevice(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (a *App) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	var req createDeviceRequest
	if !readJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.GroupName = strings.TrimSpace(req.GroupName)

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.GroupName == "" {
		writeError(w, http.StatusBadRequest, "groupName is required")
		return
	}

	id := req.ID
	if id == "" {
		id = "dev_" + strings.TrimPrefix(newID(""), "_")
	} else if _, ok, _ := a.store.GetDevice(r.Context(), id); ok {
		writeError(w, http.StatusConflict, "device with this id already exists")
		return
	}

	hostname := req.Hostname
	if hostname == "" {
		hostname = req.Name
	}

	device := Device{
		ID:          id,
		Name:        req.Name,
		GroupName:   req.GroupName,
		Profile:     "manual",
		Version:     "0.0.0",
		Hostname:    hostname,
		OS:          strings.TrimSpace(req.OS),
		IP:          strings.TrimSpace(req.IP),
		CPUCores:    req.CPUCores,
		MemoryMB:    req.MemoryMB,
		DiskTotalGB: req.DiskTotalGB,
		Status:      StatusOffline,
	}

	if err := a.store.CreateDevice(r.Context(), device); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = a.store.CreateAudit(r.Context(), AuditLog{
		Actor:    "admin",
		Action:   "device.create",
		DeviceID: id,
		Status:   StatusSuccess,
		Message:  fmt.Sprintf("manually created device '%s' in group '%s'", req.Name, req.GroupName),
	})

	writeJSON(w, http.StatusCreated, device)
}

func (a *App) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	device, ok, err := a.store.GetDevice(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}

	if err := a.store.DeleteDevice(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = a.store.CreateAudit(r.Context(), AuditLog{
		Actor:    "admin",
		Action:   "device.delete",
		DeviceID: id,
		Status:   StatusSuccess,
		Message:  fmt.Sprintf("deleted device '%s'", device.Name),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *App) handleGroups(w http.ResponseWriter, r *http.Request) {
	a.refreshState(r.Context())
	groups, err := a.store.Groups(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

func (a *App) handleListTasks(w http.ResponseWriter, r *http.Request) {
	a.refreshState(r.Context())
	tasks, err := a.store.ListTasks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (a *App) handleGetTask(w http.ResponseWriter, r *http.Request) {
	a.refreshState(r.Context())
	task, ok, err := a.store.GetTask(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (a *App) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if !readJSON(w, r, &req) {
		return
	}
	req.Command = strings.TrimSpace(req.Command)
	req.Kind = strings.TrimSpace(req.Kind)
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.GroupName = strings.TrimSpace(req.GroupName)
	auth := currentAuth(r.Context())
	user := auth.User
	if req.Kind == "" {
		req.Kind = TaskKindAdHoc
	}
	var template CommandTemplate
	var renderedArgs []string
	if req.Kind == TaskKindTemplate {
		if !HasPermission(user, PermissionTemplateRun) {
			writeAPIError(w, http.StatusForbidden, "PERMISSION_DENIED", "template execution permission is required")
			return
		}
		var ok bool
		var err error
		template, ok, err = a.store.GetCommandTemplate(r.Context(), strings.TrimSpace(req.TemplateID))
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "TEMPLATE_LOAD_FAILED", "unable to load template")
			return
		}
		if !ok {
			writeAPIError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", "template not found")
			return
		}
		renderedArgs, err = RenderTemplate(template, req.Arguments)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "TEMPLATE_ARGUMENTS_INVALID", err.Error())
			return
		}
		req.Command = template.Name
	} else if req.Kind == TaskKindAdHoc {
		if !HasPermission(user, PermissionAdHocRun) {
			writeAPIError(w, http.StatusForbidden, "PERMISSION_DENIED", "ad-hoc command permission is required")
			return
		}
		if req.Command == "" {
			writeAPIError(w, http.StatusBadRequest, "COMMAND_REQUIRED", "command is required")
			return
		}
		if !auth.Legacy && req.Confirmation != "EXECUTE" {
			writeAPIError(w, http.StatusBadRequest, "COMMAND_CONFIRMATION_REQUIRED", "confirmation must be EXECUTE")
			return
		}
	} else {
		writeAPIError(w, http.StatusBadRequest, "TASK_KIND_INVALID", "kind must be template or ad_hoc")
		return
	}
	if req.DeviceID == "" && req.GroupName == "" {
		writeError(w, http.StatusBadRequest, "deviceId or groupName is required")
		return
	}

	var devices []Device
	var err error
	if req.DeviceID != "" {
		device, ok, err := a.store.GetDevice(r.Context(), req.DeviceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "device not found")
			return
		}
		devices = []Device{device}
	} else {
		devices, err = a.store.ListDevicesByGroup(r.Context(), req.GroupName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if len(devices) == 0 {
			writeError(w, http.StatusNotFound, "group has no devices")
			return
		}
	}

	tasks := make([]Task, 0, len(devices))
	var errs []string
	for _, device := range devices {
		taskSpec := Task{DeviceID: device.ID, GroupName: device.GroupName, Command: req.Command, Kind: req.Kind, RequestedBy: user.Username}
		if req.Kind == TaskKindTemplate {
			taskSpec.TemplateID = template.ID
			taskSpec.Executable = template.Executable
			taskSpec.Args = renderedArgs
			taskSpec.TimeoutSeconds = template.TimeoutSeconds
		} else {
			taskSpec.TimeoutSeconds = 300
		}
		task, err := a.store.CreateTaskSpec(r.Context(), taskSpec)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: create failed: %v", device.Name, err))
			continue
		}
		task.DeviceName = device.Name
		_ = a.store.CreateAudit(r.Context(), AuditLog{Actor: user.Username, ActorID: user.ID, ActorRole: user.Role, RemoteAddr: clientIP(r), RequestID: requestID(r.Context()), Action: "command.create", DeviceID: device.ID, TaskID: task.ID, Status: StatusPending, Message: auditMessage(req.Command)})
		if err := a.dispatchTask(r.Context(), task); err != nil {
			errs = append(errs, fmt.Sprintf("%s: dispatch failed: %v", device.Name, err))
			continue
		}
		fresh, ok, err := a.store.GetTask(r.Context(), task.ID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: verify failed: %v", device.Name, err))
			continue
		}
		if ok {
			tasks = append(tasks, fresh)
		} else {
			tasks = append(tasks, task)
		}
	}
	// Unified response shape: always returns { tasks, errors? } so the frontend
	// sees a consistent structure regardless of success/failure count.
	status := http.StatusCreated
	if len(tasks) == 0 && len(errs) > 0 {
		status = http.StatusInternalServerError
	} else if len(errs) > 0 {
		status = http.StatusOK
	}
	resp := map[string]any{"tasks": tasks}
	if len(errs) > 0 {
		resp["errors"] = errs
	}
	writeJSON(w, status, resp)
}

func (a *App) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	a.refreshState(r.Context())
	logs, err := a.store.ListAudit(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (a *App) handleAiOpsReport(w http.ResponseWriter, r *http.Request) {
	report := a.analyzer.LatestReport()
	if report == nil {
		writeJSON(w, http.StatusOK, map[string]string{"message": "analysis in progress, check back soon"})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// handleGetLLMConfig returns the current LLM configuration (API key redacted).
func (a *App) handleGetLLMConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := a.store.GetLLMConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Redact API key: show only first 4 + last 4 characters
	if len(cfg.APIKey) > 8 {
		cfg.APIKey = cfg.APIKey[:4] + "****" + cfg.APIKey[len(cfg.APIKey)-4:]
	} else if len(cfg.APIKey) > 0 {
		cfg.APIKey = "****"
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handleSaveLLMConfig updates the LLM configuration.
func (a *App) handleSaveLLMConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderURL         string `json:"providerUrl"`
		APIKey              string `json:"apiKey"`
		Model               string `json:"model"`
		ProviderType        string `json:"providerType"`
		Enabled             bool   `json:"enabled"`
		AutoExecuteReadOnly bool   `json:"autoExecuteReadOnly"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Enabled {
		if req.ProviderURL == "" {
			writeError(w, http.StatusBadRequest, "providerUrl is required when LLM is enabled")
			return
		}
	}
	// If API key is empty, keep the existing one from the database
	if req.APIKey == "" || strings.Contains(req.APIKey, "****") {
		existing, err := a.store.GetLLMConfig(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		req.APIKey = existing.APIKey
	}
	if req.ProviderType == "" {
		req.ProviderType = "openai"
	}
	cfg := LLMConfig{
		ProviderURL:         req.ProviderURL,
		APIKey:              req.APIKey,
		Model:               req.Model,
		ProviderType:        req.ProviderType,
		Enabled:             req.Enabled,
		AutoExecuteReadOnly: req.AutoExecuteReadOnly,
	}
	if err := a.store.SaveLLMConfig(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Trigger immediate re-analysis with the new config
	go a.analyzer.TriggerRun()
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// handleTestLLM makes a minimal test call to the configured LLM and returns raw request/response.
func (a *App) handleTestLLM(w http.ResponseWriter, r *http.Request) {
	// Get current config (DB first, then env)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	dbCfg, _ := a.store.GetLLMConfig(ctx)
	url := a.config.LLMURL
	key := a.config.LLMAPIKey
	model := "gpt-3.5-turbo"
	providerType := "openai"
	if dbCfg.Enabled && dbCfg.ProviderURL != "" {
		url = dbCfg.ProviderURL
		key = dbCfg.APIKey
		if dbCfg.Model != "" {
			model = dbCfg.Model
		}
		if dbCfg.ProviderType != "" {
			providerType = dbCfg.ProviderType
		}
	}
	if url == "" || key == "" {
		writeJSON(w, http.StatusOK, LLMTestResult{
			OK:     false,
			Status: "not_configured",
			Error:  "LLM not configured. Please set Provider URL and API Key first.",
		})
		return
	}

	client := NewLLMClient(url, key, model, providerType)
	result := client.Test(ctx)
	writeJSON(w, http.StatusOK, result)
}

// handleExecuteRecommendation creates and dispatches tasks from LLM recommendations.
func (a *App) handleExecuteRecommendation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RecommendationID  string   `json:"recommendationId"`
		RecommendationIDs []string `json:"recommendationIds"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	report := a.analyzer.LatestReport()
	if report == nil {
		writeError(w, http.StatusServiceUnavailable, "no analysis report available")
		return
	}

	ids := req.RecommendationIDs
	if req.RecommendationID != "" {
		ids = append(ids, req.RecommendationID)
	}
	if len(ids) == 0 {
		writeError(w, http.StatusBadRequest, "recommendationId or recommendationIds required")
		return
	}

	var tasks []Task
	var errs []string
	for _, id := range ids {
		var found *LLMRecommendation
		for i := range report.Recommendations {
			if report.Recommendations[i].ID == id {
				found = &report.Recommendations[i]
				break
			}
		}
		if found == nil {
			errs = append(errs, fmt.Sprintf("recommendation %s not found", id))
			continue
		}
		if found.Status != "pending" {
			errs = append(errs, fmt.Sprintf("recommendation %s already %s", id, found.Status))
			continue
		}
		task, err := a.store.CreateTask(r.Context(), found.DeviceID, found.GroupName, found.Command, "user-llm")
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: create task failed: %v", found.DeviceName, err))
			continue
		}
		task.DeviceName = found.DeviceName
		if err := a.dispatchTask(r.Context(), task); err != nil {
			errs = append(errs, fmt.Sprintf("%s: dispatch failed: %v", found.DeviceName, err))
			found.Status = "error"
		} else {
			found.Status = "executed"
		}
		found.TaskID = task.ID
		tasks = append(tasks, task)
	}
	resp := map[string]any{"tasks": tasks}
	if len(errs) > 0 {
		resp["errors"] = errs
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleGetAutoMode returns the current auto-execute setting.
func (a *App) handleGetAutoMode(w http.ResponseWriter, r *http.Request) {
	cfg, err := a.store.GetLLMConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"autoExecuteReadOnly": cfg.AutoExecuteReadOnly})
}

// handleSaveAutoMode toggles the auto-execute setting.
func (a *App) handleSaveAutoMode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AutoExecuteReadOnly bool `json:"autoExecuteReadOnly"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	cfg, err := a.store.GetLLMConfig(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cfg.AutoExecuteReadOnly = req.AutoExecuteReadOnly
	if err := a.store.SaveLLMConfig(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Re-init to pick up the new setting
	a.analyzer.TriggerRun()
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// handleSetupStatus returns whether the system requires first-time setup.
func (a *App) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	count, err := a.store.CountUsers(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "SETUP_CHECK_FAILED", "unable to check setup status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setupRequired": count == 0})
}

// handleSetupAdmin creates the first admin user when the system is uninitialized.
func (a *App) handleSetupAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username        string `json:"username"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirmPassword"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(strings.ToLower(req.Username))
	if !usernamePattern.MatchString(req.Username) {
		writeAPIError(w, http.StatusBadRequest, "USERNAME_INVALID", "username must be 3-64 lowercase alphanumeric characters")
		return
	}
	if len(req.Password) < 12 {
		writeAPIError(w, http.StatusBadRequest, "PASSWORD_TOO_SHORT", "password must be at least 12 characters")
		return
	}
	if req.Password != req.ConfirmPassword {
		writeAPIError(w, http.StatusBadRequest, "PASSWORD_MISMATCH", "passwords do not match")
		return
	}

	user, err := a.store.TryCreateInitialAdmin(r.Context(), req.Username, req.Username, req.Password)
	if errors.Is(err, ErrAlreadyInitialized) {
		writeAPIError(w, http.StatusConflict, "ALREADY_INITIALIZED", "system is already initialized")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "SETUP_FAILED", err.Error())
		return
	}

	// Auto-login: create a session and set cookies so the user is logged in
	sessionToken, csrfToken, err := a.store.CreateWebSession(r.Context(), user.ID, clientIP(r), r.UserAgent())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "SESSION_FAILED", "admin created but auto-login failed")
		return
	}
	setAuthCookies(w, sessionToken, csrfToken, a.config.SecureCookies)

	writeJSON(w, http.StatusCreated, map[string]any{
		"user":               user,
		"mustChangePassword": true,
	})
}

const maxBodySize = 1 << 20 // 1 MiB

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		msg := "invalid json body"
		if strings.Contains(err.Error(), "request body too large") {
			msg = "request body too large"
		}
		writeError(w, http.StatusBadRequest, msg)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeAPIError(w, status, "REQUEST_FAILED", message)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	requestID := w.Header().Get("X-Request-ID")
	writeJSON(w, status, map[string]string{
		"error": message, "code": code, "message": message, "requestId": requestID,
	})
}

func clientIP(r *http.Request) string {
	value := r.Header.Get("X-Forwarded-For")
	if idx := strings.Index(value, ","); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	value = r.RemoteAddr
	if idx := strings.LastIndex(value, ":"); idx > 0 {
		value = value[:idx]
	}
	return value
}

func auditMessage(command string) string {
	if len(command) <= 120 {
		return command
	}
	return fmt.Sprintf("%s...", command[:117])
}
