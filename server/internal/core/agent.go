package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type AgentClient struct {
	deviceID  string
	sessionID int64
	conn      *websocket.Conn
	writeMu   sync.Mutex
}

func (c *AgentClient) Send(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteJSON(v)
}

func (a *App) handleAgentWS(w http.ResponseWriter, r *http.Request) {
	if a.config.AgentToken != "" && r.URL.Query().Get("token") != a.config.AgentToken {
		writeError(w, http.StatusUnauthorized, "invalid agent token")
		return
	}

	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	var first struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := conn.ReadJSON(&first); err != nil {
		return
	}
	if first.Type != "register" {
		_ = conn.WriteJSON(map[string]string{"type": "error", "message": "first message must be register"})
		return
	}
	var reg RegisterPayload
	if err := json.Unmarshal(first.Payload, &reg); err != nil {
		return
	}
	device := deviceFromRegister(reg)
	if device.ID == "" || device.Name == "" {
		_ = conn.WriteJSON(map[string]string{"type": "error", "message": "agentId and name are required"})
		return
	}

	ctx := r.Context()
	if err := a.store.UpsertDevice(ctx, device); err != nil {
		return
	}
	sessionID, err := a.store.CreateSession(ctx, device.ID, r.RemoteAddr)
	if err != nil {
		return
	}
	client := &AgentClient{deviceID: device.ID, sessionID: sessionID, conn: conn}
	a.registerClient(client)
	defer a.unregisterClient(client)

	_ = a.store.CreateAudit(context.Background(), AuditLog{
		Actor:    "agent",
		Action:   "agent.register",
		DeviceID: device.ID,
		Status:   StatusSuccess,
		Message:  fmt.Sprintf("%s connected from %s", device.Name, r.RemoteAddr),
	})

	_ = client.Send(AgentEnvelope{Type: "registered", Payload: map[string]string{"deviceId": device.ID}})
	_ = a.dispatchPendingTasks(context.Background(), device.ID)

	_ = conn.SetReadDeadline(time.Now().Add(2 * a.config.HeartbeatTimeout))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(2 * a.config.HeartbeatTimeout))
		return nil
	})

	for {
		var msg struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * a.config.HeartbeatTimeout))
		switch msg.Type {
		case "heartbeat":
			var hb HeartbeatPayload
			if err := json.Unmarshal(msg.Payload, &hb); err == nil {
				_ = a.store.UpdateHeartbeat(context.Background(), device.ID, hb)
			}
		case "task_result":
			var result TaskResultPayload
			if err := json.Unmarshal(msg.Payload, &result); err == nil {
				_ = a.store.CompleteTask(context.Background(), result)
				status := result.Status
				if status == "" {
					status = StatusSuccess
					if result.ExitCode != 0 {
						status = StatusFailed
					}
				}
				_ = a.store.CreateAudit(context.Background(), AuditLog{
					Actor:    "agent",
					Action:   "command.complete",
					DeviceID: device.ID,
					TaskID:   result.TaskID,
					Status:   status,
					Message:  fmt.Sprintf("exit_code=%d duration_ms=%d", result.ExitCode, result.DurationMS),
				})
			}
		}
	}
}

func (a *App) registerClient(client *AgentClient) {
	a.mu.Lock()
	old := a.clients[client.deviceID]
	a.clients[client.deviceID] = client
	a.mu.Unlock()
	if old != nil {
		_ = old.conn.Close()
	}
}

func (a *App) unregisterClient(client *AgentClient) {
	a.mu.Lock()
	current := a.clients[client.deviceID]
	if current == client {
		delete(a.clients, client.deviceID)
	}
	a.mu.Unlock()
	_ = a.store.CloseSession(context.Background(), client.sessionID)
	_ = a.store.MarkDeviceOffline(context.Background(), client.deviceID)
	_ = a.store.CreateAudit(context.Background(), AuditLog{
		Actor:    "agent",
		Action:   "agent.disconnect",
		DeviceID: client.deviceID,
		Status:   StatusOffline,
		Message:  "agent connection closed",
	})
}

func (a *App) dispatchTask(ctx context.Context, task Task) error {
	a.mu.RLock()
	client := a.clients[task.DeviceID]
	a.mu.RUnlock()
	if client == nil {
		return a.store.CreateAudit(ctx, AuditLog{
			Actor:    task.RequestedBy,
			Action:   "command.queue",
			DeviceID: task.DeviceID,
			TaskID:   task.ID,
			Status:   StatusPending,
			Message:  auditMessage(task.Command),
		})
	}
	if err := a.store.MarkTaskRunning(ctx, task.ID); err != nil {
		return err
	}
	msg := AgentEnvelope{
		Type: "command",
		Payload: CommandPayload{
			TaskID:  task.ID,
			Command: task.Command,
		},
	}
	if err := client.Send(msg); err != nil {
		_ = a.store.FailTask(ctx, task.ID, err.Error())
		return a.store.CreateAudit(ctx, AuditLog{
			Actor:    task.RequestedBy,
			Action:   "command.dispatch",
			DeviceID: task.DeviceID,
			TaskID:   task.ID,
			Status:   StatusFailed,
			Message:  err.Error(),
		})
	}
	return a.store.CreateAudit(ctx, AuditLog{
		Actor:    task.RequestedBy,
		Action:   "command.dispatch",
		DeviceID: task.DeviceID,
		TaskID:   task.ID,
		Status:   StatusRunning,
		Message:  auditMessage(task.Command),
	})
}

func (a *App) dispatchPendingTasks(ctx context.Context, deviceID string) error {
	tasks, err := a.store.PendingTasksForDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := a.dispatchTask(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

func deviceFromRegister(reg RegisterPayload) Device {
	groupName := strings.TrimSpace(reg.GroupName)
	if groupName == "" {
		groupName = "default"
	}
	profile := strings.TrimSpace(reg.Profile)
	if profile == "" {
		profile = "generic"
	}
	version := strings.TrimSpace(reg.Version)
	if version == "" {
		version = "dev"
	}
	now := nowString()
	return Device{
		ID:          strings.TrimSpace(reg.AgentID),
		Name:        strings.TrimSpace(reg.Name),
		GroupName:   groupName,
		Profile:     profile,
		Version:     version,
		Hostname:    strings.TrimSpace(reg.Hostname),
		OS:          strings.TrimSpace(reg.OS),
		IP:          strings.TrimSpace(reg.IP),
		CPUCores:    reg.CPUCores,
		MemoryMB:    reg.MemoryMB,
		DiskTotalGB: reg.DiskTotalGB,
		Status:      StatusOnline,
		LastSeen:    now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
