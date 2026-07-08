package core

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// InsightType classifies the nature of an AI Ops finding.
type InsightType string

const (
	InsightWarning InsightType = "warning"
	InsightInfo    InsightType = "info"
	InsightSuccess InsightType = "success"
)

// DeviceInsight represents a single AI Ops finding about a device.
type DeviceInsight struct {
	Type      InsightType `json:"type"`
	DeviceID  string      `json:"deviceId"`
	Device    string      `json:"device"`
	GroupName string      `json:"groupName"`
	Title     string      `json:"title"`
	Detail    string      `json:"detail"`
	Score     int         `json:"score"` // 0-100 health score for this device
}

// GroupSummary aggregates insights per group.
type GroupSummary struct {
	GroupName  string `json:"groupName"`
	Total      int    `json:"total"`
	Online     int    `json:"online"`
	Offline    int    `json:"offline"`
	AvgScore   int    `json:"avgScore"`
	WarningCnt int    `json:"warningCount"`
}

// AiOpsReport is the complete analysis output.
type AiOpsReport struct {
	GeneratedAt string          `json:"generatedAt"`
	Summary     string          `json:"summary"`
	DeviceCount int             `json:"deviceCount"`
	OnlineCount int             `json:"onlineCount"`
	OfflineCnt  int             `json:"offlineCount"`
	AvgHealth   int             `json:"avgHealth"`
	Insights    []DeviceInsight `json:"insights"`
	Groups      []GroupSummary  `json:"groups"`
}

// Analyzer runs periodic AI Ops analysis over device and task data.
type Analyzer struct {
	store  *Store
	config Config

	mu     sync.RWMutex
	latest *AiOpsReport
	done   chan struct{}
}

// NewAnalyzer creates an Analyzer and starts its periodic analysis loop.
func NewAnalyzer(store *Store, config Config) *Analyzer {
	a := &Analyzer{store: store, config: config, done: make(chan struct{})}
	go a.loop()
	return a
}

// Stop gracefully shuts down the analyzer loop.
func (a *Analyzer) Stop() {
	close(a.done)
}

func (a *Analyzer) loop() {
	// Run once at startup, then every 30 minutes.
	a.run(context.Background())
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-ticker.C:
			a.run(context.Background())
		}
	}
}

// LatestReport returns the most recent analysis.
func (a *Analyzer) LatestReport() *AiOpsReport {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.latest
}

func (a *Analyzer) run(ctx context.Context) {
	devices, err := a.store.ListDevices(ctx)
	if err != nil {
		log.Printf("aiops: ListDevices error: %v", err)
		return
	}
	tasks, err := a.store.ListTasks(ctx)
	if err != nil {
		log.Printf("aiops: ListTasks error: %v", err)
	}
	groups, err := a.store.Groups(ctx)
	if err != nil {
		log.Printf("aiops: Groups error: %v", err)
	}

	report := a.analyze(devices, tasks, groups)

	a.mu.Lock()
	a.latest = report
	a.mu.Unlock()
}

func (a *Analyzer) analyze(devices []Device, tasks []Task, groups []DeviceGroup) *AiOpsReport {
	insights := make([]DeviceInsight, 0, len(devices))
	totalScore := 0
	onlineCnt := 0

	// Per-device analysis.
	for _, d := range devices {
		di := a.analyzeDevice(d, tasks)
		insights = append(insights, di)
		totalScore += di.Score
		if d.Status == StatusOnline {
			onlineCnt++
		}
	}

	// Group summaries.
	groupMap := map[string]*GroupSummary{}
	for _, g := range groups {
		groupMap[g.Name] = &GroupSummary{
			GroupName: g.Name,
			Total:     g.Total,
			Online:    g.Online,
			Offline:   g.Total - g.Online,
		}
	}
	for _, di := range insights {
		if gs, ok := groupMap[di.GroupName]; ok {
			gs.AvgScore += di.Score
			if di.Type == InsightWarning {
				gs.WarningCnt++
			}
		}
	}
	for _, gs := range groupMap {
		if gs.Total > 0 {
			gs.AvgScore = int(float64(gs.AvgScore)/float64(gs.Total) + 0.5)
		}
	}

	// Build sorted group list.
	groupList := make([]GroupSummary, 0, len(groupMap))
	for _, gs := range groupMap {
		groupList = append(groupList, *gs)
	}
	sort.Slice(groupList, func(i, j int) bool {
		return groupList[i].GroupName < groupList[j].GroupName
	})

	// Sort insights: warnings first, then by score ascending.
	sort.Slice(insights, func(i, j int) bool {
		if insights[i].Type != insights[j].Type {
			return insights[i].Type == InsightWarning
		}
		return insights[i].Score < insights[j].Score
	})

	// Overall health.
	avgHealth := 0
	if len(devices) > 0 {
		avgHealth = int(float64(totalScore)/float64(len(devices)) + 0.5)
	}

	summary := a.buildSummary(devices, onlineCnt, avgHealth, insights)

	return &AiOpsReport{
		GeneratedAt: nowString(),
		Summary:     summary,
		DeviceCount: len(devices),
		OnlineCount: onlineCnt,
		OfflineCnt:  len(devices) - onlineCnt,
		AvgHealth:   avgHealth,
		Insights:    insights,
		Groups:      groupList,
	}
}

func (a *Analyzer) analyzeDevice(d Device, tasks []Task) DeviceInsight {
	di := DeviceInsight{
		DeviceID:  d.ID,
		Device:    d.Name,
		GroupName: d.GroupName,
		Score:     100,
	}

	var findings []string

	// Offline check.
	if d.Status != StatusOnline {
		di.Type = InsightWarning
		di.Score -= 40
		findings = append(findings, "设备离线")
	}

	// High CPU usage.
	if d.CPUUsage > 80 {
		di.Type = InsightWarning
		di.Score -= 20
		findings = append(findings, fmt.Sprintf("CPU 使用率偏高 (%.1f%%)", d.CPUUsage))
	} else if d.CPUUsage > 60 {
		if di.Type != InsightWarning {
			di.Type = InsightInfo
		}
		di.Score -= 10
		findings = append(findings, fmt.Sprintf("CPU 使用率较高 (%.1f%%)", d.CPUUsage))
	}

	// High memory usage.
	if d.MemoryUsage > 80 {
		di.Type = InsightWarning
		di.Score -= 20
		findings = append(findings, fmt.Sprintf("内存使用率偏高 (%.1f%%)", d.MemoryUsage))
	} else if d.MemoryUsage > 60 {
		if di.Type != InsightWarning {
			di.Type = InsightInfo
		}
		di.Score -= 10
		findings = append(findings, fmt.Sprintf("内存使用率较高 (%.1f%%)", d.MemoryUsage))
	}

	// Disk usage.
	if d.DiskUsage > 85 {
		di.Type = InsightWarning
		di.Score -= 15
		findings = append(findings, fmt.Sprintf("磁盘使用率偏高 (%.1f%%)", d.DiskUsage))
	}

	// Task failure analysis for this device.
	failedCnt := 0
	totalCnt := 0
	for _, t := range tasks {
		if t.DeviceID != d.ID {
			continue
		}
		totalCnt++
		if t.Status == StatusFailed || t.Status == StatusTimeout {
			failedCnt++
		}
	}
	if totalCnt > 0 {
		failRate := float64(failedCnt) / float64(totalCnt) * 100
		if failRate > 50 {
			di.Type = InsightWarning
			di.Score -= 20
			findings = append(findings, fmt.Sprintf("任务失败率 %.0f%% (%d/%d)", failRate, failedCnt, totalCnt))
		} else if failRate > 20 {
			if di.Type != InsightWarning {
				di.Type = InsightInfo
			}
			di.Score -= 10
			findings = append(findings, fmt.Sprintf("部分任务失败 (%.0f%%)", failRate))
		}
	}

	// Clamp score.
	if di.Score < 0 {
		di.Score = 0
	}

	if len(findings) == 0 {
		di.Type = InsightSuccess
		di.Title = "运行正常"
		di.Detail = fmt.Sprintf("%s (%s) 各项指标正常", d.Name, d.OS)
	} else {
		di.Title = strings.Join(findings, "；")
		di.Detail = fmt.Sprintf("健康评分: %d/100 | 系统: %s | IP: %s | Profile: %s",
			di.Score, d.OS, d.IP, d.Profile)
	}

	return di
}

func (a *Analyzer) buildSummary(devices []Device, onlineCnt, avgHealth int, insights []DeviceInsight) string {
	if len(devices) == 0 {
		return "暂无设备接入，等待 Agent 注册。"
	}

	warningCnt := 0
	for _, di := range insights {
		if di.Type == InsightWarning {
			warningCnt++
		}
	}

	parts := []string{
		fmt.Sprintf("共 %d 台设备，%d 在线", len(devices), onlineCnt),
	}

	if warningCnt > 0 {
		parts = append(parts, fmt.Sprintf("%d 台需要关注", warningCnt))
	}

	parts = append(parts, fmt.Sprintf("整体健康评分 %d/100", avgHealth))

	if avgHealth >= 90 {
		parts = append(parts, "系统运行良好")
	} else if avgHealth >= 70 {
		parts = append(parts, "建议关注部分设备状态")
	} else {
		parts = append(parts, "需要立即处理多个设备问题")
	}

	return strings.Join(parts, " · ")
}
