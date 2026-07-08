package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type createTaskRequest struct {
	DeviceID  string `json:"deviceId"`
	GroupName string `json:"groupName"`
	Command   string `json:"command"`
}

type createTaskResponse struct {
	Tasks []Task `json:"tasks"`
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
	writeJSON(w, http.StatusOK, loginResponse{Token: a.config.WebToken, User: user})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.AdminUser())
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
	for _, device := range devices {
		task, err := a.store.CreateTask(r.Context(), device.ID, device.GroupName, req.Command, "admin")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		task.DeviceName = device.Name
		if err := a.dispatchTask(r.Context(), task); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		fresh, ok, err := a.store.GetTask(r.Context(), task.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if ok {
			tasks = append(tasks, fresh)
		} else {
			tasks = append(tasks, task)
		}
	}
	writeJSON(w, http.StatusCreated, createTaskResponse{Tasks: tasks})
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

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
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
