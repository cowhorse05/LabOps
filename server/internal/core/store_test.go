package core

import (
	"context"
	"testing"
	"time"
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

func TestStoreEdgeCases(t *testing.T) {
	ctx := context.Background()
	freshStore := func(t *testing.T) *Store {
		t.Helper()
		store, err := OpenStore(":memory:")
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		t.Cleanup(func() { store.Close() })
		if err := store.Init(ctx); err != nil {
			t.Fatalf("init store: %v", err)
		}
		return store
	}

	t.Run("TestFindUser", func(t *testing.T) {
		store := freshStore(t)
		// success — default admin user seeded during Init
		user, ok, err := store.FindUser(ctx, "admin", "admin")
		if err != nil {
			t.Fatalf("FindUser err: %v", err)
		}
		if !ok {
			t.Fatal("expected user to be found")
		}
		if user.Username != "admin" {
			t.Fatalf("expected username admin, got %s", user.Username)
		}

		// wrong password
		_, ok, err = store.FindUser(ctx, "admin", "wrong")
		if err != nil {
			t.Fatalf("FindUser err: %v", err)
		}
		if ok {
			t.Fatal("expected user not found with wrong password")
		}

		// nonexistent user
		_, ok, err = store.FindUser(ctx, "nonexistent", "x")
		if err != nil {
			t.Fatalf("FindUser err: %v", err)
		}
		if ok {
			t.Fatal("expected user not found for nonexistent user")
		}
	})

	t.Run("TestUpsertDevice_Update", func(t *testing.T) {
		store := freshStore(t)
		d := Device{
			ID:          "update_test",
			Name:        "original-name",
			GroupName:   "lab",
			Profile:     "ubuntu",
			Version:     "1.0",
			Hostname:    "original-host",
			OS:          "Ubuntu",
			IP:          "10.0.0.1",
			CPUCores:    2,
			MemoryMB:    2048,
			DiskTotalGB: 32,
			Status:      StatusOnline,
		}
		if err := store.UpsertDevice(ctx, d); err != nil {
			t.Fatalf("insert device: %v", err)
		}

		// Update with new values
		d.Name = "updated-name"
		d.Version = "2.0"
		d.Hostname = "updated-host"
		d.IP = "10.0.0.2"
		d.CPUCores = 4
		d.Status = StatusOffline
		if err := store.UpsertDevice(ctx, d); err != nil {
			t.Fatalf("update device: %v", err)
		}

		got, ok, err := store.GetDevice(ctx, "update_test")
		if err != nil {
			t.Fatalf("GetDevice err: %v", err)
		}
		if !ok {
			t.Fatal("device not found after update")
		}
		if got.Name != "updated-name" {
			t.Fatalf("expected name updated-name, got %s", got.Name)
		}
		if got.Version != "2.0" {
			t.Fatalf("expected version 2.0, got %s", got.Version)
		}
		if got.IP != "10.0.0.2" {
			t.Fatalf("expected IP 10.0.0.2, got %s", got.IP)
		}
		if got.Status != StatusOffline {
			t.Fatalf("expected status offline, got %s", got.Status)
		}
	})

	t.Run("TestHeartbeatAndOffline", func(t *testing.T) {
		store := freshStore(t)
		d := Device{
			ID:          "hb_test",
			Name:        "hb-test",
			GroupName:   "lab",
			Profile:     "ubuntu",
			Version:     "1.0",
			Hostname:    "hb-test",
			OS:          "Ubuntu",
			IP:          "10.0.0.3",
			CPUCores:    2,
			MemoryMB:    2048,
			DiskTotalGB: 32,
			Status:      StatusOffline,
		}
		if err := store.UpsertDevice(ctx, d); err != nil {
			t.Fatalf("insert device: %v", err)
		}

		// Send heartbeat — should bring device online and update metrics
		hb := HeartbeatPayload{CPUUsage: 45.5, MemoryUsage: 60.0, DiskUsage: 30.0}
		if err := store.UpdateHeartbeat(ctx, "hb_test", hb); err != nil {
			t.Fatalf("UpdateHeartbeat: %v", err)
		}

		got, ok, err := store.GetDevice(ctx, "hb_test")
		if err != nil {
			t.Fatalf("GetDevice err: %v", err)
		}
		if !ok {
			t.Fatal("device not found")
		}
		if got.Status != StatusOnline {
			t.Fatalf("expected online after heartbeat, got %s", got.Status)
		}
		if got.CPUUsage != 45.5 {
			t.Fatalf("expected CPUUsage 45.5, got %f", got.CPUUsage)
		}
		if got.MemoryUsage != 60.0 {
			t.Fatalf("expected MemoryUsage 60.0, got %f", got.MemoryUsage)
		}

		// Mark offline explicitly
		if err := store.MarkDeviceOffline(ctx, "hb_test"); err != nil {
			t.Fatalf("MarkDeviceOffline: %v", err)
		}

		got, ok, err = store.GetDevice(ctx, "hb_test")
		if err != nil {
			t.Fatalf("GetDevice err: %v", err)
		}
		if !ok {
			t.Fatal("device not found")
		}
		if got.Status != StatusOffline {
			t.Fatalf("expected offline after MarkDeviceOffline, got %s", got.Status)
		}
	})

	t.Run("TestExpireDevices", func(t *testing.T) {
		store := freshStore(t)
		oldTime := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
		recentTime := time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339)

		d1 := Device{
			ID:          "expire_old",
			Name:        "expire-old",
			GroupName:   "lab",
			Profile:     "ubuntu",
			Version:     "1.0",
			Hostname:    "expire-old",
			OS:          "Ubuntu",
			IP:          "10.0.0.4",
			CPUCores:    2,
			MemoryMB:    2048,
			DiskTotalGB: 32,
			Status:      StatusOnline,
			LastSeen:    oldTime,
		}
		if err := store.UpsertDevice(ctx, d1); err != nil {
			t.Fatalf("insert device 1: %v", err)
		}

		d2 := Device{
			ID:          "expire_recent",
			Name:        "expire-recent",
			GroupName:   "lab",
			Profile:     "ubuntu",
			Version:     "1.0",
			Hostname:    "expire-recent",
			OS:          "Ubuntu",
			IP:          "10.0.0.5",
			CPUCores:    2,
			MemoryMB:    2048,
			DiskTotalGB: 32,
			Status:      StatusOnline,
			LastSeen:    recentTime,
		}
		if err := store.UpsertDevice(ctx, d2); err != nil {
			t.Fatalf("insert device 2: %v", err)
		}

		// Expire devices with last_seen older than 1 hour ago
		cutoff := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
		if err := store.ExpireDevices(ctx, cutoff); err != nil {
			t.Fatalf("ExpireDevices: %v", err)
		}

		got1, ok, err := store.GetDevice(ctx, "expire_old")
		if err != nil {
			t.Fatalf("GetDevice expire_old: %v", err)
		}
		if !ok {
			t.Fatal("expire_old not found")
		}
		if got1.Status != StatusOffline {
			t.Fatalf("expected expire_old to be offline, got %s", got1.Status)
		}

		got2, ok, err := store.GetDevice(ctx, "expire_recent")
		if err != nil {
			t.Fatalf("GetDevice expire_recent: %v", err)
		}
		if !ok {
			t.Fatal("expire_recent not found")
		}
		if got2.Status != StatusOnline {
			t.Fatalf("expected expire_recent to remain online, got %s", got2.Status)
		}
	})

	t.Run("TestTimeoutTasks", func(t *testing.T) {
		store := freshStore(t)
		task, err := store.CreateTask(ctx, "timeout_device", "lab", "sleep 1000", "admin")
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if err := store.MarkTaskRunning(ctx, task.ID); err != nil {
			t.Fatalf("MarkTaskRunning: %v", err)
		}

		// Manually set started_at to an old time so TimeoutTasks will catch it
		oldTime := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
		if _, err := store.db.ExecContext(ctx, `UPDATE tasks SET started_at = ? WHERE id = ?`, oldTime, task.ID); err != nil {
			t.Fatalf("update started_at: %v", err)
		}

		// Timeout tasks started more than 1 hour ago
		cutoff := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
		if err := store.TimeoutTasks(ctx, cutoff); err != nil {
			t.Fatalf("TimeoutTasks: %v", err)
		}

		gotTask, ok, err := store.GetTask(ctx, task.ID)
		if err != nil {
			t.Fatalf("GetTask: %v", err)
		}
		if !ok {
			t.Fatal("task not found")
		}
		if gotTask.Status != StatusTimeout {
			t.Fatalf("expected task status timeout, got %s", gotTask.Status)
		}
	})

	t.Run("TestListDevicesByGroup", func(t *testing.T) {
		store := freshStore(t)
		devices := []Device{
			{
				ID: "group_lab_1", Name: "lab-device-1", GroupName: "lab",
				Profile: "ubuntu", Version: "1.0", Hostname: "lab1", OS: "Ubuntu",
				IP: "10.0.1.1", CPUCores: 2, MemoryMB: 2048, DiskTotalGB: 32,
				Status: StatusOnline,
			},
			{
				ID: "group_lab_2", Name: "lab-device-2", GroupName: "lab",
				Profile: "ubuntu", Version: "1.0", Hostname: "lab2", OS: "Ubuntu",
				IP: "10.0.1.2", CPUCores: 4, MemoryMB: 4096, DiskTotalGB: 64,
				Status: StatusOffline,
			},
			{
				ID: "group_prod_1", Name: "prod-device-1", GroupName: "prod",
				Profile: "ubuntu", Version: "1.0", Hostname: "prod1", OS: "Ubuntu",
				IP: "10.0.2.1", CPUCores: 8, MemoryMB: 8192, DiskTotalGB: 128,
				Status: StatusOnline,
			},
		}
		for _, d := range devices {
			if err := store.UpsertDevice(ctx, d); err != nil {
				t.Fatalf("upsert device %s: %v", d.ID, err)
			}
		}

		labDevices, err := store.ListDevicesByGroup(ctx, "lab")
		if err != nil {
			t.Fatalf("ListDevicesByGroup lab: %v", err)
		}
		if len(labDevices) != 2 {
			t.Fatalf("expected 2 lab devices, got %d", len(labDevices))
		}

		prodDevices, err := store.ListDevicesByGroup(ctx, "prod")
		if err != nil {
			t.Fatalf("ListDevicesByGroup prod: %v", err)
		}
		if len(prodDevices) != 1 {
			t.Fatalf("expected 1 prod device, got %d", len(prodDevices))
		}

		emptyDevices, err := store.ListDevicesByGroup(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("ListDevicesByGroup nonexistent: %v", err)
		}
		if len(emptyDevices) != 0 {
			t.Fatalf("expected 0 devices for nonexistent group, got %d", len(emptyDevices))
		}
	})

	t.Run("TestGroups", func(t *testing.T) {
		store := freshStore(t)
		devices := []Device{
			{
				ID: "gt_lab_online", Name: "lab-online", GroupName: "grouptest_lab",
				Profile: "ubuntu", Version: "1.0", Hostname: "lab-online", OS: "Ubuntu",
				IP: "10.0.3.1", CPUCores: 2, MemoryMB: 2048, DiskTotalGB: 32,
				Status: StatusOnline,
			},
			{
				ID: "gt_lab_offline", Name: "lab-offline", GroupName: "grouptest_lab",
				Profile: "ubuntu", Version: "1.0", Hostname: "lab-offline", OS: "Ubuntu",
				IP: "10.0.3.2", CPUCores: 2, MemoryMB: 2048, DiskTotalGB: 32,
				Status: StatusOffline,
			},
			{
				ID: "gt_prod_online", Name: "prod-online", GroupName: "grouptest_prod",
				Profile: "ubuntu", Version: "1.0", Hostname: "prod-online", OS: "Ubuntu",
				IP: "10.0.4.1", CPUCores: 8, MemoryMB: 8192, DiskTotalGB: 128,
				Status: StatusOnline,
			},
		}
		for _, d := range devices {
			if err := store.UpsertDevice(ctx, d); err != nil {
				t.Fatalf("upsert device %s: %v", d.ID, err)
			}
		}

		groups, err := store.Groups(ctx)
		if err != nil {
			t.Fatalf("Groups: %v", err)
		}

		var labGroup, prodGroup *DeviceGroup
		for i := range groups {
			switch groups[i].Name {
			case "grouptest_lab":
				labGroup = &groups[i]
			case "grouptest_prod":
				prodGroup = &groups[i]
			}
		}

		if labGroup == nil {
			t.Fatal("grouptest_lab not found in groups")
		}
		if labGroup.Total != 2 {
			t.Fatalf("expected 2 total in grouptest_lab, got %d", labGroup.Total)
		}
		if labGroup.Online != 1 {
			t.Fatalf("expected 1 online in grouptest_lab, got %d", labGroup.Online)
		}

		if prodGroup == nil {
			t.Fatal("grouptest_prod not found in groups")
		}
		if prodGroup.Total != 1 {
			t.Fatalf("expected 1 total in grouptest_prod, got %d", prodGroup.Total)
		}
		if prodGroup.Online != 1 {
			t.Fatalf("expected 1 online in grouptest_prod, got %d", prodGroup.Online)
		}
	})

	t.Run("TestPendingTasksForDevice", func(t *testing.T) {
		store := freshStore(t)
		d1 := Device{
			ID: "pending_device", Name: "pending-device", GroupName: "lab",
			Profile: "ubuntu", Version: "1.0", Hostname: "pending-dev", OS: "Ubuntu",
			IP: "10.0.5.1", CPUCores: 2, MemoryMB: 2048, DiskTotalGB: 32,
			Status: StatusOnline,
		}
		if err := store.UpsertDevice(ctx, d1); err != nil {
			t.Fatalf("upsert device: %v", err)
		}

		d2 := Device{
			ID: "no_pending_device", Name: "no-pending-device", GroupName: "lab",
			Profile: "ubuntu", Version: "1.0", Hostname: "no-pending-dev", OS: "Ubuntu",
			IP: "10.0.5.2", CPUCores: 2, MemoryMB: 2048, DiskTotalGB: 32,
			Status: StatusOnline,
		}
		if err := store.UpsertDevice(ctx, d2); err != nil {
			t.Fatalf("upsert device: %v", err)
		}

		// Create a pending task for d1
		task, err := store.CreateTask(ctx, d1.ID, d1.GroupName, "echo hello", "admin")
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}

		// Device with pending task should find it
		pendingTasks, err := store.PendingTasksForDevice(ctx, d1.ID)
		if err != nil {
			t.Fatalf("PendingTasksForDevice: %v", err)
		}
		if len(pendingTasks) != 1 {
			t.Fatalf("expected 1 pending task, got %d", len(pendingTasks))
		}
		if pendingTasks[0].ID != task.ID {
			t.Fatalf("expected task ID %s, got %s", task.ID, pendingTasks[0].ID)
		}

		// Device without pending task should get empty slice
		noPendingTasks, err := store.PendingTasksForDevice(ctx, d2.ID)
		if err != nil {
			t.Fatalf("PendingTasksForDevice: %v", err)
		}
		if len(noPendingTasks) != 0 {
			t.Fatalf("expected 0 pending tasks, got %d", len(noPendingTasks))
		}

		// After marking task running, it should no longer be pending
		if err := store.MarkTaskRunning(ctx, task.ID); err != nil {
			t.Fatalf("MarkTaskRunning: %v", err)
		}
		pendingTasksAfterRunning, err := store.PendingTasksForDevice(ctx, d1.ID)
		if err != nil {
			t.Fatalf("PendingTasksForDevice: %v", err)
		}
		if len(pendingTasksAfterRunning) != 0 {
			t.Fatalf("expected 0 pending tasks after marking running, got %d", len(pendingTasksAfterRunning))
		}
	})

	t.Run("TestListTasks_Empty", func(t *testing.T) {
		emptyStore, err := OpenStore(":memory:")
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		defer emptyStore.Close()
		if err := emptyStore.Init(ctx); err != nil {
			t.Fatalf("init store: %v", err)
		}

		tasks, err := emptyStore.ListTasks(ctx)
		if err != nil {
			t.Fatalf("ListTasks: %v", err)
		}
		if tasks == nil {
			t.Fatal("expected non-nil empty slice, got nil")
		}
		if len(tasks) != 0 {
			t.Fatalf("expected 0 tasks, got %d", len(tasks))
		}
	})
}
