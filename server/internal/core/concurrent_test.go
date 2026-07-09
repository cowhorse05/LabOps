package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// --- Concurrent login tests ---

func TestConcurrentFindUser(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	const goroutines = 50
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			user, ok, err := store.FindUser(ctx, "admin", "admin")
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: FindUser error: %v", idx, err)
				return
			}
			if !ok {
				errCh <- fmt.Errorf("goroutine %d: user not found", idx)
				return
			}
			if user.Username != "admin" {
				errCh <- fmt.Errorf("goroutine %d: unexpected username: %s", idx, user.Username)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	elapsed := time.Since(start)
	t.Logf("50 concurrent FindUser calls completed in %v", elapsed)

	for err := range errCh {
		t.Error(err)
	}
}

// --- Concurrent device upsert tests ---

func TestConcurrentUpsertDevices(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	const goroutines = 20
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			device := Device{
				ID:          fmt.Sprintf("conc_dev_%d", idx),
				Name:        fmt.Sprintf("concurrent-device-%d", idx),
				GroupName:   fmt.Sprintf("grp_%d", idx%3),
				Profile:     "linux",
				Version:     "1.0",
				Hostname:    fmt.Sprintf("host-%d", idx),
				OS:          "Linux",
				IP:          fmt.Sprintf("10.0.0.%d", idx),
				CPUCores:    4,
				MemoryMB:    4096,
				DiskTotalGB: 64,
				Status:      StatusOnline,
			}
			if err := store.UpsertDevice(ctx, device); err != nil {
				errCh <- fmt.Errorf("goroutine %d: UpsertDevice: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	elapsed := time.Since(start)
	t.Logf("20 concurrent UpsertDevice calls completed in %v", elapsed)

	for err := range errCh {
		t.Error(err)
	}

	devices, err := store.ListDevices(ctx)
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != goroutines {
		t.Fatalf("expected %d devices, got %d", goroutines, len(devices))
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Total != goroutines {
		t.Fatalf("expected %d total devices, got %d", goroutines, stats.Total)
	}
}

// --- Concurrent task creation tests ---

func TestConcurrentCreateTasks(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create 3 shared devices
	for i := 0; i < 3; i++ {
		device := Device{
			ID:          fmt.Sprintf("task_dev_%d", i),
			Name:        fmt.Sprintf("task-device-%d", i),
			GroupName:   "task-group",
			Profile:     "linux",
			Version:     "1.0",
			Hostname:    fmt.Sprintf("task-host-%d", i),
			OS:          "Linux",
			IP:          fmt.Sprintf("10.0.1.%d", i),
			CPUCores:    2,
			MemoryMB:    2048,
			DiskTotalGB: 32,
			Status:      StatusOnline,
		}
		if err := store.UpsertDevice(ctx, device); err != nil {
			t.Fatalf("UpsertDevice: %v", err)
		}
	}

	const goroutines = 30
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			deviceID := fmt.Sprintf("task_dev_%d", idx%3)
			task, err := store.CreateTask(ctx, deviceID, "task-group", fmt.Sprintf("echo task_%d", idx), "admin")
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: CreateTask: %v", idx, err)
				return
			}
			if err := store.MarkTaskRunning(ctx, task.ID); err != nil {
				errCh <- fmt.Errorf("goroutine %d: MarkTaskRunning: %v", idx, err)
				return
			}
			// Randomly complete or fail
			if idx%5 == 0 {
				if err := store.FailTask(ctx, task.ID, "simulated failure"); err != nil {
					errCh <- fmt.Errorf("goroutine %d: FailTask: %v", idx, err)
				}
			} else {
				if err := store.CompleteTask(ctx, TaskResultPayload{
					TaskID:   task.ID,
					ExitCode: 0,
					Stdout:   fmt.Sprintf("result_%d", idx),
					Status:   StatusSuccess,
				}); err != nil {
					errCh <- fmt.Errorf("goroutine %d: CompleteTask: %v", idx, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	elapsed := time.Since(start)
	t.Logf("30 concurrent task create+complete cycles completed in %v", elapsed)

	for err := range errCh {
		t.Error(err)
	}

	// Verify all tasks were persisted
	tasks, err := store.ListTasks(ctx)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) < goroutines {
		t.Fatalf("expected at least %d tasks, got %d", goroutines, len(tasks))
	}
}

// --- Concurrent read+write mix test ---

func TestConcurrentReadWriteMix(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Seed: create 10 devices
	for i := 0; i < 10; i++ {
		device := Device{
			ID:          fmt.Sprintf("mix_dev_%d", i),
			Name:        fmt.Sprintf("mix-device-%d", i),
			GroupName:   "mix-group",
			Profile:     "linux",
			Version:     "1.0",
			Hostname:    fmt.Sprintf("mix-host-%d", i),
			OS:          "Linux",
			IP:          fmt.Sprintf("10.0.2.%d", i),
			CPUCores:    2,
			MemoryMB:    2048,
			DiskTotalGB: 32,
			Status:      StatusOnline,
		}
		if err := store.UpsertDevice(ctx, device); err != nil {
			t.Fatalf("UpsertDevice: %v", err)
		}
	}

	const goroutines = 40
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup
	start := time.Now()

	// Half readers, half writers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				// Reader
				devices, err := store.ListDevices(ctx)
				if err != nil {
					errCh <- fmt.Errorf("reader %d: ListDevices: %v", idx, err)
					return
				}
				if len(devices) < 10 {
					errCh <- fmt.Errorf("reader %d: expected >=10 devices, got %d", idx, len(devices))
				}
			} else {
				// Writer: update heartbeat for a random device
				deviceID := fmt.Sprintf("mix_dev_%d", idx%10)
				hb := HeartbeatPayload{
					CPUUsage:    float64(idx%100),
					MemoryUsage: float64((idx*3)%100),
					DiskUsage:   float64((idx*7)%100),
				}
				if err := store.UpdateHeartbeat(ctx, deviceID, hb); err != nil {
					errCh <- fmt.Errorf("writer %d: UpdateHeartbeat: %v", idx, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	elapsed := time.Since(start)
	t.Logf("40 concurrent read/write mix calls completed in %v", elapsed)

	for err := range errCh {
		t.Error(err)
	}
}

// --- Connection pool stress test ---

func TestConnectionPoolStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping connection pool stress test in short mode")
	}
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	const goroutines = 100
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Mix of read and write operations
			switch idx % 5 {
			case 0:
				_, _, err := store.FindUser(ctx, "admin", "admin")
				if err != nil {
					errCh <- fmt.Errorf("g%d FindUser: %v", idx, err)
				}
			case 1:
				_, err := store.ListDevices(ctx)
				if err != nil {
					errCh <- fmt.Errorf("g%d ListDevices: %v", idx, err)
				}
			case 2:
				_, err := store.Stats(ctx)
				if err != nil {
					errCh <- fmt.Errorf("g%d Stats: %v", idx, err)
				}
			case 3:
				device := Device{
					ID:          fmt.Sprintf("stress_%d", idx%10),
					Name:        fmt.Sprintf("stress-%d", idx%10),
					GroupName:   "stress",
					Profile:     "edge",
					Version:     "1.0",
					Hostname:    fmt.Sprintf("stress-%d", idx%10),
					OS:          "Linux",
					IP:          fmt.Sprintf("10.1.0.%d", idx%10),
					CPUCores:    2,
					MemoryMB:    1024,
					DiskTotalGB: 16,
					Status:      StatusOnline,
				}
				if err := store.UpsertDevice(ctx, device); err != nil {
					errCh <- fmt.Errorf("g%d UpsertDevice: %v", idx, err)
				}
			case 4:
				_, err := store.ListAudit(ctx)
				if err != nil {
					errCh <- fmt.Errorf("g%d ListAudit: %v", idx, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	elapsed := time.Since(start)
	t.Logf("100 concurrent mixed operations completed in %v", elapsed)

	errCount := 0
	for err := range errCh {
		t.Error(err)
		errCount++
	}
	if errCount > 0 {
		t.Logf("%d/%d goroutines reported errors", errCount, goroutines)
	}
}
