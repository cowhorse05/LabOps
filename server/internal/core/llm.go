package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// llmMessage is a single message in the chat completion request.
type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// --- OpenAI-compatible request/response types ---

// openAIRequest is the body sent to the OpenAI chat completions endpoint.
type openAIRequest struct {
	Model       string       `json:"model"`
	Messages    []llmMessage `json:"messages"`
	Temperature float64      `json:"temperature,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
}

// openAIResponse is the response from the OpenAI chat completions endpoint.
type openAIResponse struct {
	Choices []struct {
		Message llmMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// --- Anthropic Messages API request/response types ---

type anthropicContent struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

type anthropicRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	Messages  []llmMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// LLMClient calls an OpenAI-compatible or Anthropic-compatible API.
type LLMClient struct {
	url          string
	apiKey       string
	model        string
	providerType string // "openai" or "anthropic"
	http         *http.Client
}

// useBearerAuth returns true if the provider uses "Authorization: Bearer" (OpenAI, etc.).
// Anthropic-compatible endpoints (including DeepSeek's /anthropic) use "x-api-key".
func (c *LLMClient) useBearerAuth() bool {
	return c.providerType != "anthropic"
}

// NewLLMClient creates an LLM client for the given provider.
// providerType should be "openai" (default) or "anthropic".
func NewLLMClient(url, apiKey, model, providerType string) *LLMClient {
	url = strings.TrimRight(url, "/")
	if model == "" {
		if providerType == "anthropic" {
			model = "claude-sonnet-4-6"
		} else {
			model = "gpt-3.5-turbo"
		}
	}
	if providerType == "" {
		providerType = "openai"
	}
	return &LLMClient{
		url:          url,
		apiKey:       apiKey,
		model:        model,
		providerType: providerType,
		http:         &http.Client{Timeout: 90 * time.Second},
	}
}

// AnalyzeDevices sends device metrics to the LLM and returns Chinese analysis text.
func (c *LLMClient) AnalyzeDevices(ctx context.Context, devices []Device, tasks []Task) (string, error) {
	switch c.providerType {
	case "anthropic":
		return c.analyzeAnthropic(ctx, devices, tasks)
	default:
		return c.analyzeOpenAI(ctx, devices, tasks)
	}
}

func (c *LLMClient) analyzeOpenAI(ctx context.Context, devices []Device, tasks []Task) (string, error) {
	prompt := c.buildPrompt(devices, tasks)

	body := openAIRequest{
		Model:       c.model,
		Messages:    []llmMessage{{Role: "user", Content: prompt}},
		Temperature: 0.3,
		MaxTokens:   2000,
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("llm marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.url+"/v1/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("llm read body: %w", err)
	}

	var llmResp openAIResponse
	if err := json.Unmarshal(respBytes, &llmResp); err != nil {
		return "", fmt.Errorf("llm decode (status %d): %w", resp.StatusCode, err)
	}

	if llmResp.Error != nil {
		return "", fmt.Errorf("llm api error: %s (%s)", llmResp.Error.Message, llmResp.Error.Type)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}

	return strings.TrimSpace(llmResp.Choices[0].Message.Content), nil
}

func (c *LLMClient) analyzeAnthropic(ctx context.Context, devices []Device, tasks []Task) (string, error) {
	prompt := c.buildPrompt(devices, tasks)

	body := anthropicRequest{
		Model:     c.model,
		MaxTokens: 2000,
		Messages:  []llmMessage{{Role: "user", Content: prompt}},
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("llm marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.url+"/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.useBearerAuth() {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	} else {
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("llm read body: %w", err)
	}

	var llmResp anthropicResponse
	if err := json.Unmarshal(respBytes, &llmResp); err != nil {
		return "", fmt.Errorf("llm decode (status %d): %w", resp.StatusCode, err)
	}

	if llmResp.Error != nil {
		return "", fmt.Errorf("llm api error: %s (%s)", llmResp.Error.Message, llmResp.Error.Type)
	}

	if len(llmResp.Content) == 0 {
		return "", fmt.Errorf("llm returned no content")
	}

	var texts []string
	for _, c := range llmResp.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}
	if len(texts) == 0 {
		return "", fmt.Errorf("llm returned no text content")
	}

	return strings.TrimSpace(strings.Join(texts, "\n")), nil
}

func (c *LLMClient) buildPrompt(devices []Device, tasks []Task) string {
	var sb strings.Builder
	sb.WriteString("你是一個實驗室設備運維專家。以下是當前所有設備的監控數據，請進行綜合分析並給出中文建議。\n\n")
	sb.WriteString("## 設備列表\n")
	for _, d := range devices {
		sb.WriteString(fmt.Sprintf("- %s (分組:%s): CPU %.1f%%, 内存 %.1f%%, 磁盘 %.1f%%, 状态=%s, 系统=%s, IP=%s\n",
			d.Name, d.GroupName, d.CPUUsage, d.MemoryUsage, d.DiskUsage, d.Status, d.OS, d.IP))
	}

	total, failed := 0, 0
	for _, t := range tasks {
		total++
		if t.Status == StatusFailed || t.Status == StatusTimeout {
			failed++
		}
	}
	sb.WriteString(fmt.Sprintf("\n## 任務統計\n总任务数: %d, 失败: %d", total, failed))
	if total > 0 {
		sb.WriteString(fmt.Sprintf(", 失敗率: %.0f%%", float64(failed)/float64(total)*100))
	}
	sb.WriteString("\n")

	sb.WriteString("\n請按以下格式輸出分析報告：\n")
	sb.WriteString("1. 整體健康評估（一段簡短總結）\n")
	sb.WriteString("2. 需要關注的設備（列出設備名稱和具體問題）\n")
	sb.WriteString("3. 優化建議（具體可操作的建议）\n")

	return sb.String()
}

// --- Structured LLM analysis (JSON output with actionable recommendations) ---

// structuredLLMResponse is the expected JSON shape from the LLM.
type structuredLLMResponse struct {
	TextAnalysis    string `json:"textAnalysis"`
	Recommendations []struct {
		DeviceID   string `json:"deviceId"`
		DeviceName string `json:"deviceName"`
		Command    string `json:"command"`
		Reason     string `json:"reason"`
		Priority   string `json:"priority"`
		IsMutation bool   `json:"isMutation"`
	} `json:"recommendations"`
}

// AnalyzeDevicesStructured sends device metrics to the LLM and returns both
// Chinese analysis text and a list of structured, actionable recommendations.
func (c *LLMClient) AnalyzeDevicesStructured(ctx context.Context, devices []Device, tasks []Task) (string, []LLMRecommendation, error) {
	switch c.providerType {
	case "anthropic":
		return c.analyzeStructuredAnthropic(ctx, devices, tasks)
	default:
		return c.analyzeStructuredOpenAI(ctx, devices, tasks)
	}
}

func (c *LLMClient) analyzeStructuredOpenAI(ctx context.Context, devices []Device, tasks []Task) (string, []LLMRecommendation, error) {
	prompt := c.buildStructuredPrompt(devices, tasks)

	body := openAIRequest{
		Model:       c.model,
		Messages:    []llmMessage{{Role: "user", Content: prompt}},
		Temperature: 0.3,
		MaxTokens:   4000,
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return "", nil, fmt.Errorf("llm marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.url+"/v1/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return "", nil, fmt.Errorf("llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("llm http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("llm read body: %w", err)
	}

	var llmResp openAIResponse
	if err := json.Unmarshal(respBytes, &llmResp); err != nil {
		return "", nil, fmt.Errorf("llm decode (status %d): %w", resp.StatusCode, err)
	}

	if llmResp.Error != nil {
		return "", nil, fmt.Errorf("llm api error: %s (%s)", llmResp.Error.Message, llmResp.Error.Type)
	}

	if len(llmResp.Choices) == 0 {
		return "", nil, fmt.Errorf("llm returned no choices")
	}

	raw := strings.TrimSpace(llmResp.Choices[0].Message.Content)
	return c.parseStructuredResponse(raw, devices)
}

func (c *LLMClient) analyzeStructuredAnthropic(ctx context.Context, devices []Device, tasks []Task) (string, []LLMRecommendation, error) {
	prompt := c.buildStructuredPrompt(devices, tasks)

	body := anthropicRequest{
		Model:     c.model,
		MaxTokens: 4000,
		Messages:  []llmMessage{{Role: "user", Content: prompt}},
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return "", nil, fmt.Errorf("llm marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.url+"/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return "", nil, fmt.Errorf("llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.useBearerAuth() {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	} else {
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("llm http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("llm read body: %w", err)
	}

	var llmResp anthropicResponse
	if err := json.Unmarshal(respBytes, &llmResp); err != nil {
		return "", nil, fmt.Errorf("llm decode (status %d): %w", resp.StatusCode, err)
	}

	if llmResp.Error != nil {
		return "", nil, fmt.Errorf("llm api error: %s (%s)", llmResp.Error.Message, llmResp.Error.Type)
	}

	if len(llmResp.Content) == 0 {
		return "", nil, fmt.Errorf("llm returned no content")
	}

	var texts []string
	for _, ct := range llmResp.Content {
		if ct.Type == "text" {
			texts = append(texts, ct.Text)
		}
	}
	if len(texts) == 0 {
		return "", nil, fmt.Errorf("llm returned no text content")
	}

	raw := strings.TrimSpace(strings.Join(texts, "\n"))
	return c.parseStructuredResponse(raw, devices)
}

// buildStructuredPrompt constructs a prompt that instructs the LLM to output
// JSON with both a Chinese text analysis and a list of structured recommendations.
func (c *LLMClient) buildStructuredPrompt(devices []Device, tasks []Task) string {
	var sb strings.Builder
	sb.WriteString("你是一個實驗室設備運維專家。以下是當前所有設備的監控數據，請進行綜合分析。\n\n")

	// Device list
	sb.WriteString("## 設備列表\n")
	for _, d := range devices {
		sb.WriteString(fmt.Sprintf("- ID=%s | %s (分組:%s): CPU %.1f%%, 内存 %.1f%%, 磁盘 %.1f%%, 状态=%s, 系统=%s, IP=%s, Profile=%s\n",
			d.ID, d.Name, d.GroupName, d.CPUUsage, d.MemoryUsage, d.DiskUsage, d.Status, d.OS, d.IP, d.Profile))
	}

	// Task statistics
	total, failed := 0, 0
	for _, t := range tasks {
		total++
		if t.Status == StatusFailed || t.Status == StatusTimeout {
			failed++
		}
	}
	sb.WriteString(fmt.Sprintf("\n## 任務統計\n总任务数: %d, 失败: %d", total, failed))
	if total > 0 {
		sb.WriteString(fmt.Sprintf(", 失敗率: %.0f%%", float64(failed)/float64(total)*100))
	}
	sb.WriteString("\n")

	// Available diagnostic commands reference
	sb.WriteString("\n## 可用診斷命令參考\n")
	sb.WriteString("只讀診斷（isMutation=false）：\n")
	sb.WriteString("- df -h          # 檢查磁盤空間\n")
	sb.WriteString("- free -m        # 檢查記憶體\n")
	sb.WriteString("- top -bn1 | head -10   # CPU 和進程\n")
	sb.WriteString("- ps aux --sort=-%cpu | head -10  # CPU 消耗最高的進程\n")
	sb.WriteString("- ps aux --sort=-%mem | head -10  # 記憶體消耗最高的進程\n")
	sb.WriteString("- uptime         # 系統運行時間和負載\n")
	sb.WriteString("- dmesg | tail -20  # 最近的系統訊息\n")
	sb.WriteString("- systemctl status <service>  # 檢查服務狀態\n")
	sb.WriteString("- netstat -tlnp  # 檢查監聽端口\n")
	sb.WriteString("- uname -a       # 系統信息\n")
	sb.WriteString("變更操作（isMutation=true，需用戶確認）：\n")
	sb.WriteString("- systemctl restart <service>  # 重啟服務\n")
	sb.WriteString("- systemctl start <service>    # 啟動服務\n")
	sb.WriteString("- apt-get update / yum update  # 更新軟件包\n")

	// Output format
	sb.WriteString("\n## 輸出格式\n")
	sb.WriteString("請嚴格按照以下 JSON 格式輸出（不要包含 markdown 代碼塊標記，只輸出純 JSON）：\n\n")
	sb.WriteString(`{
  "textAnalysis": "一段中文的整體分析報告，包含健康評估、需關注的設備和具體建議。使用實際換行符，不要在 JSON 中使用 \\n 字面量。",
  "recommendations": [
    {
      "deviceId": "設備ID（必須與上面設備列表中的 ID 完全一致）",
      "deviceName": "設備名稱",
      "command": "要執行的 shell 命令",
      "reason": "推薦原因（中文）",
      "priority": "high|medium|low",
      "isMutation": false
    }
  ]
}`)
	sb.WriteString("\n\n規則：\n")
	sb.WriteString("- deviceId 必須與上面設備列表中的 ID 完全一致，不要編造\n")
	sb.WriteString("- 僅在確實有問題需要處理時才建議命令，健康設備不需要推薦\n")
	sb.WriteString("- priority: 离線或资源 >90% 為 high，60-90% 為 medium，其他為 low\n")
	sb.WriteString("- isMutation: 只讀診斷命令為 false，會修改系統狀態的命令為 true\n")
	sb.WriteString("- 不需要的設備不要包含在 recommendations 中\n")
	sb.WriteString("- 如果所有設備都健康，recommendations 為空數組 []\n")

	return sb.String()
}

// parseStructuredResponse parses the LLM's JSON response into text + recommendations.
func (c *LLMClient) parseStructuredResponse(raw string, devices []Device) (string, []LLMRecommendation, error) {
	// Strip markdown code fences if present
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var parsed structuredLLMResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		// Fallback: return raw text as analysis, no recommendations
		return raw, nil, fmt.Errorf("llm json parse: %w (raw: %.200s)", err, raw)
	}

	// Normalize newlines: some LLMs output literal "\n" instead of real newlines.
	parsed.TextAnalysis = normalizeNewlines(parsed.TextAnalysis)

	// Build device lookup map
	deviceMap := make(map[string]Device)
	for _, d := range devices {
		deviceMap[d.ID] = d
	}

	recs := make([]LLMRecommendation, 0, len(parsed.Recommendations))
	for _, r := range parsed.Recommendations {
		cmd := strings.TrimSpace(r.Command)
		if cmd == "" {
			continue
		}
		// Cross-reference and fill in device info
		dev, ok := deviceMap[r.DeviceID]
		if !ok {
			continue // skip recommendations for unknown devices
		}
		groupName := dev.GroupName
		deviceName := r.DeviceName
		if deviceName == "" {
			deviceName = dev.Name
		}
		priority := r.Priority
		if priority == "" {
			priority = "medium"
		}
		// Sanitize: force-mark dangerous commands as mutations
		isMutation := r.IsMutation || isDangerousCommand(cmd)

		recs = append(recs, LLMRecommendation{
			ID:         newID("rec"),
			DeviceID:   r.DeviceID,
			DeviceName: deviceName,
			GroupName:  groupName,
			Command:    cmd,
			Reason:     r.Reason,
			Priority:   priority,
			IsMutation: isMutation,
			Status:     "pending",
			CreatedAt:  nowString(),
		})
	}

	return parsed.TextAnalysis, recs, nil
}

// normalizeNewlines replaces literal "\n" with actual newlines in LLM output.
// Some LLMs emit literal backslash-n even when told to use real newlines.
func normalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\\n", "\n")
}

// LLMTestResult contains the raw request/response details from a test call.
type LLMTestResult struct {
	OK          bool   `json:"ok"`
	Status      string `json:"status"`
	RequestURL  string `json:"requestUrl"`
	RequestBody string `json:"requestBody"`
	ReqHeaders  string `json:"reqHeaders"`
	RespStatus  int    `json:"respStatus"`
	RespBody    string `json:"respBody"`
	Error       string `json:"error,omitempty"`
	ModelUsed   string `json:"modelUsed"`
}

// Test sends a simple "Hello" message to the LLM and returns raw request/response info.
func (c *LLMClient) Test(ctx context.Context) LLMTestResult {
	result := LLMTestResult{
		ModelUsed: c.model,
	}

	// Build a minimal test message
	testMsg := llmMessage{Role: "user", Content: "Hi"}

	switch c.providerType {
	case "anthropic":
		return c.testAnthropic(ctx, testMsg, &result)
	default:
		return c.testOpenAI(ctx, testMsg, &result)
	}
}

func (c *LLMClient) testOpenAI(ctx context.Context, msg llmMessage, result *LLMTestResult) LLMTestResult {
	body := openAIRequest{
		Model:       c.model,
		Messages:    []llmMessage{msg},
		Temperature: 0,
		MaxTokens:   500,
	}

	buf, _ := json.MarshalIndent(body, "", "  ")
	result.RequestBody = string(buf)
	result.RequestURL = c.url + "/v1/chat/completions"
	result.ReqHeaders = "Authorization: Bearer " + maskKey(c.apiKey) + "\nContent-Type: application/json"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, result.RequestURL, bytes.NewReader(buf))
	if err != nil {
		result.Error = err.Error()
		result.Status = "error"
		return *result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		result.Error = err.Error()
		result.Status = "error"
		return *result
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	result.RespStatus = resp.StatusCode
	result.RespBody = string(respBytes)

	if resp.StatusCode >= 400 {
		result.OK = false
		result.Status = "failed"
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return *result
	}

	var llmResp openAIResponse
	if err := json.Unmarshal(respBytes, &llmResp); err != nil {
		result.OK = false
		result.Status = "failed"
		result.Error = fmt.Sprintf("JSON decode error: %v", err)
		return *result
	}
	if llmResp.Error != nil {
		result.OK = false
		result.Status = "failed"
		result.Error = fmt.Sprintf("API error: %s (%s)", llmResp.Error.Message, llmResp.Error.Type)
		return *result
	}

	result.OK = true
	result.Status = "ok"
	return *result
}

func (c *LLMClient) testAnthropic(ctx context.Context, msg llmMessage, result *LLMTestResult) LLMTestResult {
	body := anthropicRequest{
		Model:     c.model,
		MaxTokens: 500,
		Messages:  []llmMessage{msg},
	}

	buf, _ := json.MarshalIndent(body, "", "  ")
	result.RequestBody = string(buf)
	result.RequestURL = c.url + "/v1/messages"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, result.RequestURL, bytes.NewReader(buf))
	if err != nil {
		result.Error = err.Error()
		result.Status = "error"
		return *result
	}
	req.Header.Set("Content-Type", "application/json")
	if c.useBearerAuth() {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		result.ReqHeaders = "Authorization: Bearer " + maskKey(c.apiKey) + "\nContent-Type: application/json"
	} else {
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		result.ReqHeaders = "x-api-key: " + maskKey(c.apiKey) + "\nanthropic-version: 2023-06-01\nContent-Type: application/json"
	}

	resp, err := c.http.Do(req)
	if err != nil {
		result.Error = err.Error()
		result.Status = "error"
		return *result
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	result.RespStatus = resp.StatusCode
	result.RespBody = string(respBytes)

	if resp.StatusCode >= 400 {
		result.OK = false
		result.Status = "failed"
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return *result
	}

	var llmResp anthropicResponse
	if err := json.Unmarshal(respBytes, &llmResp); err != nil {
		result.OK = false
		result.Status = "failed"
		result.Error = fmt.Sprintf("JSON decode error: %v", err)
		return *result
	}
	if llmResp.Error != nil {
		result.OK = false
		result.Status = "failed"
		result.Error = fmt.Sprintf("API error: %s (%s)", llmResp.Error.Message, llmResp.Error.Type)
		return *result
	}

	result.OK = true
	result.Status = "ok"
	return *result
}

// maskKey returns a masked API key showing only first 7 and last 4 chars.
func maskKey(key string) string {
	if len(key) <= 11 {
		return "****"
	}
	return key[:7] + "****" + key[len(key)-4:]
}

// isDangerousCommand returns true if the command looks destructive.
func isDangerousCommand(cmd string) bool {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	dangerous := []string{
		"rm -rf /", "rm -rf ~", "rm -rf .",
		"dd if=", "mkfs", "mkswap",
		"> /dev/sda", "> /dev/hda",
		"chmod 777 /",
		"wget ", "curl ", // piping to shell
		"| sh", "| bash", "| /bin/sh", "| /bin/bash",
		"fork bomb", ":(){",
		"shutdown", "reboot", "halt", "poweroff",
	}
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return true
		}
	}
	return false
}
