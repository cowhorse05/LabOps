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
	GeneratedAt     string              `json:"generatedAt"`
	Summary         string              `json:"summary"`
	LLMAnalysis     string              `json:"llmAnalysis,omitempty"`
	DeviceCount     int                 `json:"deviceCount"`
	OnlineCount     int                 `json:"onlineCount"`
	OfflineCnt      int                 `json:"offlineCount"`
	AvgHealth       int                 `json:"avgHealth"`
	Insights        []DeviceInsight     `json:"insights"`
	Groups          []GroupSummary      `json:"groups"`
	Recommendations []LLMRecommendation `json:"recommendations,omitempty"`
}

// Analyzer runs periodic AI Ops analysis over device and task data.
type Analyzer struct {
	store     *Store
	config    Config
	llmClient *LLMClient

	// OnDispatch is called to dispatch a task to an agent. Set by App after construction.
	OnDispatch func(ctx context.Context, task Task) error

	autoExecuteReadOnly bool

	mu     sync.RWMutex
	latest *AiOpsReport
	done   chan struct{}
}

// NewAnalyzer creates an Analyzer. Call Start() to begin the analysis loop.
func NewAnalyzer(store *Store, config Config) *Analyzer {
	a := &Analyzer{store: store, config: config, done: make(chan struct{})}
	a.initLLM()
	return a
}

// Start begins the periodic analysis loop.
func (a *Analyzer) Start() {
	go a.loop()
}

// Stop gracefully shuts down the analyzer loop.
func (a *Analyzer) Stop() {
	close(a.done)
}

// initLLM reads LLM config (DB first, then env vars) and creates the client.
func (a *Analyzer) initLLM() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dbCfg, err := a.store.GetLLMConfig(ctx)
	if err != nil {
		log.Printf("aiops: GetLLMConfig error: %v", err)
	}
	url := a.config.LLMURL
	key := a.config.LLMAPIKey
	model := "gpt-3.5-turbo"
	providerType := "openai"
	a.autoExecuteReadOnly = false
	if dbCfg.Enabled && dbCfg.ProviderURL != "" {
		url = dbCfg.ProviderURL
		key = dbCfg.APIKey
		if dbCfg.Model != "" {
			model = dbCfg.Model
		}
		if dbCfg.ProviderType != "" {
			providerType = dbCfg.ProviderType
		}
		a.autoExecuteReadOnly = dbCfg.AutoExecuteReadOnly
	}
	if url != "" && key != "" {
		a.llmClient = NewLLMClient(url, key, model, providerType)
		log.Printf("aiops: LLM client initialized (%s, model=%s, type=%s, autoExec=%v)", url, model, providerType, a.autoExecuteReadOnly)
	} else {
		a.llmClient = nil
		if url == "" && key == "" {
			log.Printf("aiops: LLM not configured — set LABOPS_LLM_URL and LABOPS_LLM_API_KEY env vars, or configure via Settings UI. Using rule-based analysis only.")
		} else if url == "" {
			log.Printf("aiops: LLM URL not configured — set LABOPS_LLM_URL env var or configure via Settings UI.")
		} else {
			log.Printf("aiops: LLM API key not configured — set LABOPS_LLM_API_KEY env var or configure via Settings UI.")
		}
	}
}

// TriggerRun forces an immediate analysis (e.g., after LLM config changes).
func (a *Analyzer) TriggerRun() {
	a.initLLM()
	go a.run(context.Background())
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

	// LLM-enhanced analysis (now with structured recommendations)
	if a.llmClient != nil && len(devices) > 0 {
		llmCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		textAnalysis, recs, err := a.llmClient.AnalyzeDevicesStructured(llmCtx, devices, tasks)
		if err != nil {
			log.Printf("aiops: LLM structured analysis error: %v", err)
			report.LLMAnalysis = "LLM 分析暂时不可用，显示规则引擎分析结果。"
		} else {
			if textAnalysis != "" {
				report.LLMAnalysis = textAnalysis
			}
			report.Recommendations = a.validateRecommendations(recs, devices)
		}
	}

	// Auto-execute read-only recommendations if enabled
	if a.autoExecuteReadOnly && len(report.Recommendations) > 0 {
		a.autoExecuteRecommendations(ctx, report.Recommendations)
	}

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

// validateRecommendations filters and sanitizes LLM-generated recommendations.
func (a *Analyzer) validateRecommendations(recs []LLMRecommendation, devices []Device) []LLMRecommendation {
	deviceMap := make(map[string]Device)
	for _, d := range devices {
		deviceMap[d.ID] = d
	}

	validated := make([]LLMRecommendation, 0, len(recs))
	for _, rec := range recs {
		dev, ok := deviceMap[rec.DeviceID]
		if !ok {
			log.Printf("aiops: recommendation %s references unknown device %s, skipping", rec.ID, rec.DeviceID)
			continue
		}
		if rec.DeviceName == "" {
			rec.DeviceName = dev.Name
		}
		rec.GroupName = dev.GroupName
		if rec.Command == "" {
			continue
		}
		if rec.Priority == "" {
			rec.Priority = "medium"
		}
		validated = append(validated, rec)
	}
	return validated
}

// autoExecuteRecommendations creates and dispatches tasks for read-only recommendations.
func (a *Analyzer) autoExecuteRecommendations(ctx context.Context, recs []LLMRecommendation) {
	if a.OnDispatch == nil {
		log.Printf("aiops: dispatch callback not ready, skipping auto-execute")
		return
	}
	for i := range recs {
		rec := &recs[i]
		if rec.IsMutation {
			continue
		}
		if rec.Status != "pending" {
			continue
		}
		task, err := a.store.CreateTask(ctx, rec.DeviceID, rec.GroupName, rec.Command, "llm-auto")
		if err != nil {
			log.Printf("aiops: auto-execute create task error for rec %s: %v", rec.ID, err)
			rec.Status = "error"
			continue
		}
		rec.TaskID = task.ID
		if err := a.OnDispatch(ctx, task); err != nil {
			log.Printf("aiops: auto-execute dispatch error for rec %s: %v", rec.ID, err)
			rec.Status = "error"
		} else {
			rec.Status = "executed"
			log.Printf("aiops: auto-dispatched rec %s (%s → %s)", rec.ID, rec.DeviceName, rec.Command)
		}
	}
}
