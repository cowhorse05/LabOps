package core

import (
	"context"
	"testing"
)

func mustOpenStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatalf("OpenStore(:memory:) failed: %v", err)
	}
	return store
}

func mustInit(t *testing.T, store *Store) {
	t.Helper()
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("store.Init() failed: %v", err)
	}
}

func defaultDevice(overrides ...func(*Device)) Device {
	d := Device{
		ID: "dev-1", Name: "test-device", GroupName: "default",
		Profile: "default", Version: "1.0", Hostname: "test-host",
		OS: "linux", IP: "10.0.0.1",
		CPUCores: 4, MemoryMB: 8192, DiskTotalGB: 100,
		CPUUsage: 30, MemoryUsage: 40, DiskUsage: 50,
		Status: StatusOnline,
	}
	for _, fn := range overrides {
		fn(&d)
	}
	return d
}

// ---------------------------------------------------------------------------
// Test 1: Empty database -- no devices -> empty report
// ---------------------------------------------------------------------------

func TestAnalyzer_EmptyDB(t *testing.T) {
	ctx := context.Background()
	store := mustOpenStore(t)
	defer store.Close()
	mustInit(t, store)

	a := &Analyzer{store: store}
	a.run(ctx)

	r := a.LatestReport()
	if r == nil {
		t.Fatal("LatestReport() returned nil")
	}
	if r.DeviceCount != 0 {
		t.Errorf("DeviceCount = %d, want 0", r.DeviceCount)
	}
	if r.OnlineCount != 0 {
		t.Errorf("OnlineCount = %d, want 0", r.OnlineCount)
	}
	if r.OfflineCnt != 0 {
		t.Errorf("OfflineCnt = %d, want 0", r.OfflineCnt)
	}
	if r.AvgHealth != 0 {
		t.Errorf("AvgHealth = %d, want 0", r.AvgHealth)
	}
	if len(r.Insights) != 0 {
		t.Errorf("len(Insights) = %d, want 0", len(r.Insights))
	}
	if len(r.Groups) != 0 {
		t.Errorf("len(Groups) = %d, want 0", len(r.Groups))
	}
	if want := "暂无设备接入，等待 Agent 注册。"; r.Summary != want {
		t.Errorf("Summary = %q, want %q", r.Summary, want)
	}
}

// ---------------------------------------------------------------------------
// Test 2: Single online device with normal metrics -> score=100, type=success
// ---------------------------------------------------------------------------

func TestAnalyzer_OnlineDevice(t *testing.T) {
	ctx := context.Background()
	store := mustOpenStore(t)
	defer store.Close()
	mustInit(t, store)

	dev := defaultDevice()
	dev.ID = "dev-online"
	dev.Name = "healthy-box"
	if err := store.UpsertDevice(ctx, dev); err != nil {
		t.Fatal(err)
	}

	a := &Analyzer{store: store}
	a.run(ctx)

	r := a.LatestReport()
	if r.DeviceCount != 1 {
		t.Fatalf("DeviceCount = %d, want 1", r.DeviceCount)
	}
	if r.OnlineCount != 1 {
		t.Errorf("OnlineCount = %d, want 1", r.OnlineCount)
	}
	if r.AvgHealth != 100 {
		t.Errorf("AvgHealth = %d, want 100", r.AvgHealth)
	}

	if len(r.Insights) != 1 {
		t.Fatalf("len(Insights) = %d, want 1", len(r.Insights))
	}
	di := r.Insights[0]
	if di.Score != 100 {
		t.Errorf("insight Score = %d, want 100", di.Score)
	}
	if di.Type != InsightSuccess {
		t.Errorf("insight Type = %q, want %q", di.Type, InsightSuccess)
	}
	if di.Title != "运行正常" {
		t.Errorf("insight Title = %q, want %q", di.Title, "运行正常")
	}
	if di.DeviceID != "dev-online" {
		t.Errorf("insight DeviceID = %q, want %q", di.DeviceID, "dev-online")
	}
}

// ---------------------------------------------------------------------------
// Test 3: Offline device -> score=100-40=60, type=warning
// ---------------------------------------------------------------------------

func TestAnalyzer_OfflineDevice(t *testing.T) {
	ctx := context.Background()
	store := mustOpenStore(t)
	defer store.Close()
	mustInit(t, store)

	dev := defaultDevice(func(d *Device) {
		d.ID = "dev-offline"
		d.Name = "offline-box"
		d.Status = StatusOffline
	})
	if err := store.UpsertDevice(ctx, dev); err != nil {
		t.Fatal(err)
	}

	a := &Analyzer{store: store}
	a.run(ctx)

	r := a.LatestReport()
	if r.OfflineCnt != 1 {
		t.Errorf("OfflineCnt = %d, want 1", r.OfflineCnt)
	}
	if r.OnlineCount != 0 {
		t.Errorf("OnlineCount = %d, want 0", r.OnlineCount)
	}
	// score: 100 - 40 (offline) = 60
	di := r.Insights[0]
	if di.Score != 60 {
		t.Errorf("insight Score = %d, want 60 (100-40 for offline)", di.Score)
	}
	if di.Type != InsightWarning {
		t.Errorf("insight Type = %q, want %q", di.Type, InsightWarning)
	}
}

// ---------------------------------------------------------------------------
// Test 4: High CPU (>80) -> warning, score=100-20=80
// ---------------------------------------------------------------------------

func TestAnalyzer_HighCPU(t *testing.T) {
	ctx := context.Background()
	store := mustOpenStore(t)
	defer store.Close()
	mustInit(t, store)

	dev := defaultDevice(func(d *Device) {
		d.ID = "dev-highcpu"
		d.Name = "hot-cpu"
		d.CPUUsage = 85 // > 80 -> warning
	})
	if err := store.UpsertDevice(ctx, dev); err != nil {
		t.Fatal(err)
	}

	a := &Analyzer{store: store}
	a.run(ctx)

	di := a.LatestReport().Insights[0]
	if di.Score != 80 {
		t.Errorf("insight Score = %d, want 80 (100-20 for CPU>80)", di.Score)
	}
	if di.Type != InsightWarning {
		t.Errorf("insight Type = %q, want %q", di.Type, InsightWarning)
	}
}

// ---------------------------------------------------------------------------
// Test 5: High memory (>80) -> warning, score=100-20=80
// ---------------------------------------------------------------------------

func TestAnalyzer_HighMemory(t *testing.T) {
	ctx := context.Background()
	store := mustOpenStore(t)
	defer store.Close()
	mustInit(t, store)

	dev := defaultDevice(func(d *Device) {
		d.ID = "dev-highmem"
		d.Name = "full-mem"
		d.MemoryUsage = 85 // > 80 -> warning
	})
	if err := store.UpsertDevice(ctx, dev); err != nil {
		t.Fatal(err)
	}

	a := &Analyzer{store: store}
	a.run(ctx)

	di := a.LatestReport().Insights[0]
	if di.Score != 80 {
		t.Errorf("insight Score = %d, want 80 (100-20 for Memory>80)", di.Score)
	}
	if di.Type != InsightWarning {
		t.Errorf("insight Type = %q, want %q", di.Type, InsightWarning)
	}
}

// ---------------------------------------------------------------------------
// Test 6: Mix of online/offline -> correct counts and average
// ---------------------------------------------------------------------------

func TestAnalyzer_MultipleDevices(t *testing.T) {
	ctx := context.Background()
	store := mustOpenStore(t)
	defer store.Close()
	mustInit(t, store)

	devices := []Device{
		defaultDevice(func(d *Device) {
			d.ID = "dev-a"; d.Name = "alpha"; d.CPUUsage = 10; d.MemoryUsage = 20; d.DiskUsage = 30
		}),
		defaultDevice(func(d *Device) {
			d.ID = "dev-b"; d.Name = "beta"; d.CPUUsage = 50; d.MemoryUsage = 60; d.DiskUsage = 70
		}),
		defaultDevice(func(d *Device) {
			d.ID = "dev-c"; d.Name = "gamma"; d.Status = StatusOffline
		}),
	}
	for _, d := range devices {
		if err := store.UpsertDevice(ctx, d); err != nil {
			t.Fatal(err)
		}
	}

	a := &Analyzer{store: store}
	a.run(ctx)

	r := a.LatestReport()
	if r.DeviceCount != 3 {
		t.Errorf("DeviceCount = %d, want 3", r.DeviceCount)
	}
	if r.OnlineCount != 2 {
		t.Errorf("OnlineCount = %d, want 2", r.OnlineCount)
	}
	if r.OfflineCnt != 1 {
		t.Errorf("OfflineCnt = %d, want 1", r.OfflineCnt)
	}

	// Scores: dev-a=100, dev-b=100, dev-c=60 => total=260, avg=260/3=86
	if r.AvgHealth != 86 {
		t.Errorf("AvgHealth = %d, want 86 (260/3)", r.AvgHealth)
	}

	if len(r.Insights) != 3 {
		t.Fatalf("len(Insights) = %d, want 3", len(r.Insights))
	}

	// Sorting: warnings first (dev-c), then by score ascending
	if r.Insights[0].DeviceID != "dev-c" {
		t.Errorf("first insight should be the offline device, got %q", r.Insights[0].DeviceID)
	}
}

// ---------------------------------------------------------------------------
// Test 7: High task failure rate -> warning, score=100-20=80
// ---------------------------------------------------------------------------

func TestAnalyzer_FailedTasks(t *testing.T) {
	ctx := context.Background()
	store := mustOpenStore(t)
	defer store.Close()
	mustInit(t, store)

	dev := defaultDevice(func(d *Device) {
		d.ID = "dev-fail"
		d.Name = "failing-box"
	})
	if err := store.UpsertDevice(ctx, dev); err != nil {
		t.Fatal(err)
	}

	// Create 4 tasks: 3 failed (rate=75%), 1 success
	for i := 0; i < 4; i++ {
		task, err := store.CreateTask(ctx, dev.ID, dev.GroupName, "echo hello", "admin")
		if err != nil {
			t.Fatal(err)
		}
		if i < 3 {
			// Mark as failed
			if err := store.FailTask(ctx, task.ID, "exit code 1"); err != nil {
				t.Fatal(err)
			}
		} else {
			// Mark as success
			if err := store.CompleteTask(ctx, TaskResultPayload{
				TaskID: task.ID, ExitCode: 0, Stdout: "ok", Stderr: "", DurationMS: 100,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}

	a := &Analyzer{store: store}
	a.run(ctx)

	di := a.LatestReport().Insights[0]
	// score: 100 - 20 (failure rate >50%) = 80
	if di.Score != 80 {
		t.Errorf("insight Score = %d, want 80 (100-20 for fail-rate>50%%)", di.Score)
	}
	if di.Type != InsightWarning {
		t.Errorf("insight Type = %q, want %q", di.Type, InsightWarning)
	}
}

// ---------------------------------------------------------------------------
// Test 8: Two groups -> correct aggregation per group
// ---------------------------------------------------------------------------

func TestAnalyzer_GroupSummary(t *testing.T) {
	ctx := context.Background()
	store := mustOpenStore(t)
	defer store.Close()
	mustInit(t, store)

	devices := []Device{
		// group-a: 2 online devices, one with high CPU
		defaultDevice(func(d *Device) {
			d.ID = "ga-1"; d.Name = "ga-normal"; d.GroupName = "group-a"
			d.CPUUsage = 30
		}),
		defaultDevice(func(d *Device) {
			d.ID = "ga-2"; d.Name = "ga-hot"; d.GroupName = "group-a"
			d.CPUUsage = 85 // warning
		}),
		// group-b: 1 online, 1 offline
		defaultDevice(func(d *Device) {
			d.ID = "gb-1"; d.Name = "gb-online"; d.GroupName = "group-b"
		}),
		defaultDevice(func(d *Device) {
			d.ID = "gb-2"; d.Name = "gb-offline"; d.GroupName = "group-b"
			d.Status = StatusOffline
		}),
	}
	for _, d := range devices {
		if err := store.UpsertDevice(ctx, d); err != nil {
			t.Fatal(err)
		}
	}

	a := &Analyzer{store: store}
	a.run(ctx)

	r := a.LatestReport()
	if len(r.Groups) != 2 {
		t.Fatalf("len(Groups) = %d, want 2", len(r.Groups))
	}

	// Groups are sorted alphabetically: group-a, group-b
	g1, g2 := r.Groups[0], r.Groups[1]

	// ---- group-a ----
	if g1.GroupName != "group-a" {
		t.Errorf("Groups[0].GroupName = %q, want %q", g1.GroupName, "group-a")
	}
	if g1.Total != 2 {
		t.Errorf("group-a Total = %d, want 2", g1.Total)
	}
	if g1.Online != 2 {
		t.Errorf("group-a Online = %d, want 2", g1.Online)
	}
	if g1.Offline != 0 {
		t.Errorf("group-a Offline = %d, want 0", g1.Offline)
	}
	// Scores: ga-1=100, ga-2=80 => avg = 180/2 = 90
	if g1.AvgScore != 90 {
		t.Errorf("group-a AvgScore = %d, want 90", g1.AvgScore)
	}
	if g1.WarningCnt != 1 {
		t.Errorf("group-a WarningCnt = %d, want 1 (ga-2 high CPU)", g1.WarningCnt)
	}

	// ---- group-b ----
	if g2.GroupName != "group-b" {
		t.Errorf("Groups[1].GroupName = %q, want %q", g2.GroupName, "group-b")
	}
	if g2.Total != 2 {
		t.Errorf("group-b Total = %d, want 2", g2.Total)
	}
	if g2.Online != 1 {
		t.Errorf("group-b Online = %d, want 1", g2.Online)
	}
	if g2.Offline != 1 {
		t.Errorf("group-b Offline = %d, want 1", g2.Offline)
	}
	// Scores: gb-1=100, gb-2=60 => avg = 160/2 = 80
	if g2.AvgScore != 80 {
		t.Errorf("group-b AvgScore = %d, want 80", g2.AvgScore)
	}
	if g2.WarningCnt != 1 {
		t.Errorf("group-b WarningCnt = %d, want 1 (gb-2 offline)", g2.WarningCnt)
	}
}
