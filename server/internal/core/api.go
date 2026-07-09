package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token              string `json:"token"`
	User               User   `json:"user"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type createTaskRequest struct {
	DeviceID  string `json:"deviceId"`
	GroupName string `json:"groupName"`
	Command   string `json:"command"`
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	// Create JWT with 24h expiry
	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(a.config.JWTSecret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Check if must change password (from DB)
	mustChange := a.store.MustChangePassword(r.Context(), user.Username)

	writeJSON(w, http.StatusOK, loginResponse{
		Token:              tokenString,
		User:               user,
		MustChangePassword: mustChange,
	})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.AdminUser())
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
	if len(req.NewPassword) < 4 {
		writeError(w, http.StatusBadRequest, "password must be at least 4 characters")
		return
	}

	// Get current user from JWT
	authHeader := r.Header.Get("Authorization")
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Allow static token for backward compat (use admin user)
	var username string
	if tokenString == a.config.WebToken {
		username = "admin"
	} else {
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
			return []byte(a.config.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}
		username, _ = claims["username"].(string)
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

	// Issue new JWT
	claims := jwt.MapClaims{
		"sub":      username,
		"username": username,
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	newTokenString, err := newToken.SignedString([]byte(a.config.JWTSecret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": newTokenString})
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
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.GroupName = strings.TrimSpace(req.GroupName)
	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
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
		task, err := a.store.CreateTask(r.Context(), device.ID, device.GroupName, req.Command, "admin")
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: create failed: %v", device.Name, err))
			continue
		}
		task.DeviceName = device.Name
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
	writeJSON(w, status, map[string]string{"error": message})
}

func auditMessage(command string) string {
	if len(command) <= 120 {
		return command
	}
	return fmt.Sprintf("%s...", command[:117])
}
