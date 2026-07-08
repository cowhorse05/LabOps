package core

import (
	"context"
	"testing"
)

func TestStoreDeviceTaskAuditFlow(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Init(ctx); err != nil {
		t.Fatalf("init store: %v", err)
	}

	device := Device{
		ID:          "agent_test",
		Name:        "test-agent",
		GroupName:   "lab",
		Profile:     "ubuntu",
		Version:     "dev",
		Hostname:    "test-agent",
		OS:          "Ubuntu",
		IP:          "10.10.0.10",
		CPUCores:    4,
		MemoryMB:    4096,
		DiskTotalGB: 64,
		Status:      StatusOnline,
	}
	if err := store.UpsertDevice(ctx, device); err != nil {
		t.Fatalf("upsert device: %v", err)
	}
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Total != 1 || stats.Online != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	task, err := store.CreateTask(ctx, device.ID, device.GroupName, "echo hello", "admin")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := store.MarkTaskRunning(ctx, task.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	if err := store.CompleteTask(ctx, TaskResultPayload{TaskID: task.ID, ExitCode: 0, Stdout: "hello\n", Status: StatusSuccess}); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	task, ok, err := store.GetTask(ctx, task.ID)
	if err != nil || !ok {
		t.Fatalf("get task ok=%v err=%v", ok, err)
	}
	if task.Status != StatusSuccess || task.Result == nil || task.Result.Stdout != "hello\n" {
		t.Fatalf("unexpected task: %+v", task)
	}

	if err := store.CreateAudit(ctx, AuditLog{Actor: "admin", Action: "command.complete", DeviceID: device.ID, TaskID: task.ID, Status: StatusSuccess, Message: "ok"}); err != nil {
		t.Fatalf("create audit: %v", err)
	}
	logs, err := store.ListAudit(ctx)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(logs) != 1 || logs[0].Device != device.Name {
		t.Fatalf("unexpected logs: %+v", logs)
	}
}
