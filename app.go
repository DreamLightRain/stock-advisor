package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"stock-advisor/internal/ai"
	"stock-advisor/internal/analysis"
	"stock-advisor/internal/logger"
	"stock-advisor/internal/stock"
	"stock-advisor/internal/storage"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type moneyFlowCacheEntry struct {
	items []stock.MoneyFlowItem
	time  time.Time
}

type App struct {
	ctx      context.Context
	fetcher  *stock.FetcherManager
	store    *storage.Store
	analyzer *analysis.Analyzer
	aiClient *ai.Client
	log      *logger.Logger

	mu                sync.RWMutex
	marketSummary     *stock.MarketSummary
	marketSummaryTime time.Time
	sectorMoneyFlow   []stock.SectorMoneyFlow
	sectorMoneyFlowTime time.Time
	moneyFlowCache    map[string]moneyFlowCacheEntry
	realtimeCache     map[string]*stock.RealTimeData // cached real-time data for self-select stocks
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.log = logger.NewLogger("./logs")
	a.log.Info(logger.ModuleSystem, "Application starting up")

	a.fetcher = stock.NewFetcherManager()
	a.analyzer = analysis.NewAnalyzer()
	a.moneyFlowCache = make(map[string]moneyFlowCacheEntry)

	var err error
	a.store, err = storage.NewStore("./data")
	if err != nil {
		a.log.Error(logger.ModuleStorage, "Storage init error: %v", err)
	}

	cfg := ai.Config{}
	dataSource := "auto"
	var realTimePriority []string
	if a.store != nil {
		appData, loadErr := a.store.Load()
		if loadErr == nil && appData != nil {
			cfg = appData.Config
			dataSource = appData.DataSource
			realTimePriority = appData.RealTimePriority
		} else if loadErr != nil {
			a.log.Error(logger.ModuleStorage, "Load data error: %v", loadErr)
		}
	}

	a.fetcher.SetDataSource(dataSource)
	if len(realTimePriority) > 0 {
		a.fetcher.SetRealTimePriority(realTimePriority)
	}
	a.aiClient = ai.NewClient(cfg)
	a.log.Info(logger.ModuleSystem, "AI client initialized with provider: %s", cfg.Provider)
}

func (a *App) shutdown(ctx context.Context) {
	if a.fetcher != nil {
		a.fetcher.Close()
	}
	a.log.Info(logger.ModuleSystem, "Application shutting down")
}

// ==================== Stock Search ====================

func (a *App) SearchStock(keyword string) []stock.SearchResult {
	if keyword == "" {
		return nil
	}
	results, err := a.fetcher.SearchStocks(keyword)
	if err != nil {
		a.log.Error(logger.ModuleData, "SearchStock %q error: %v", keyword, err)
		return nil
	}
	a.log.Debug(logger.ModuleData, "SearchStock %q found %d results", keyword, len(results))

	if len(results) == 0 {
		return nil
	}

	var filtered []stock.SearchResult
	for _, r := range results {
		if r.Market == "SH" || r.Market == "SZ" {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	return results
}

// ==================== Real-time Data ====================

func (a *App) GetRealtimeData(codes []string) map[string]*stock.RealTimeData {
	if len(codes) == 0 {
		return nil
	}

	result, err := a.fetcher.GetRealTime(codes)
	if err != nil {
		a.log.Error(logger.ModuleData, "GetRealtimeData %v error: %v", codes, err)
		return nil
	}
	return result
}

// ==================== KLine ====================

func (a *App) GetKLineData(code string, days int) []stock.KLine {
	kline, err := a.fetcher.GetKLine(code, days)
	if err != nil {
		a.log.Error(logger.ModuleData, "GetKLineData %s error: %v", code, err)
		return nil
	}
	return kline
}

func (a *App) GetTimeSharingData(code string) []stock.KLine {
	data, err := a.fetcher.GetTimeSharing(code)
	if err != nil {
		a.log.Error(logger.ModuleData, "GetTimeSharingData %s error: %v", code, err)
		return nil
	}
	return data
}

const cacheTTL = 30 * time.Second // fresh cache TTL

func (a *App) getCachedMoneyFlow(code string) ([]stock.MoneyFlowItem, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	cached, ok := a.moneyFlowCache[code]
	if !ok {
		return nil, false
	}
	return cached.items, time.Since(cached.time) < cacheTTL
}

func (a *App) setCachedMoneyFlow(code string, data []stock.MoneyFlowItem) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.moneyFlowCache == nil {
		a.moneyFlowCache = make(map[string]moneyFlowCacheEntry)
	}
	a.moneyFlowCache[code] = moneyFlowCacheEntry{items: data, time: time.Now()}
}

func (a *App) GetMoneyFlow(code string, days int) []stock.MoneyFlowItem {
	// Check cache first
	if cached, fresh := a.getCachedMoneyFlow(code); fresh {
		return cached
	}

	data, err := a.fetcher.GetMoneyFlow(code, days)
	if err != nil {
		a.log.Error(logger.ModuleData, "GetMoneyFlow %s error: %v", code, err)
		if cached, _ := a.getCachedMoneyFlow(code); cached != nil {
			return cached
		}
		return nil
	}
	if len(data) > 0 {
		a.setCachedMoneyFlow(code, data)
	}
	return data
}

func (a *App) GetStockIndustry(code string) string {
	industry, err := a.fetcher.GetStockIndustry(code)
	if err != nil {
		a.log.Error(logger.ModuleData, "GetStockIndustry %s error: %v", code, err)
		return ""
	}
	return industry
}

func (a *App) GetMarketSummary() *stock.MarketSummary {
	a.mu.RLock()
	cached := a.marketSummary
	cachedTime := a.marketSummaryTime
	a.mu.RUnlock()
	if cached != nil && time.Since(cachedTime) < cacheTTL {
		return cached
	}

	data, err := a.fetcher.GetMarketSummary()
	if err != nil {
		a.log.Error(logger.ModuleData, "GetMarketSummary error: %v", err)
		return cached
	}
	if data != nil && (len(data.Indices) > 0 || len(data.MoneyFlow) > 0) {
		a.mu.Lock()
		a.marketSummary = data
		a.marketSummaryTime = time.Now()
		a.mu.Unlock()
		return data
	}
	// empty data → stale cache better than nothing
	if cached != nil {
		return cached
	}
	return data
}

func (a *App) GetIndexMoneyFlow(code string, days int) []stock.MarketMoneyFlow {
	mf := stock.NewMarketFetcher()
	data, err := mf.FetchIndexMoneyFlowRaw(code, days)
	if err != nil {
		a.log.Error(logger.ModuleData, "GetIndexMoneyFlow %s error: %v", code, err)
		return nil
	}
	return data
}

func (a *App) GetSectorMoneyFlow() []stock.SectorMoneyFlow {
	a.mu.RLock()
	cached := a.sectorMoneyFlow
	cachedTime := a.sectorMoneyFlowTime
	a.mu.RUnlock()
	if cached != nil && time.Since(cachedTime) < cacheTTL {
		return cached
	}

	data, err := a.fetcher.GetSectorMoneyFlow()
	if err != nil {
		a.log.Error(logger.ModuleData, "GetSectorMoneyFlow error: %v", err)
		return cached
	}
	if len(data) > 0 {
		a.mu.Lock()
		a.sectorMoneyFlow = data
		a.sectorMoneyFlowTime = time.Now()
		a.mu.Unlock()
		return data
	}
	if cached != nil {
		return cached
	}
	return data
}

func (a *App) GetSectorMoneyFlowByDate(date string) []stock.SectorMoneyFlow {
	a.log.Info(logger.ModuleData, "GetSectorMoneyFlowByDate date=%s", date)
	// For today, use cached if fresh
	if date == "" || date == time.Now().Format("2006-01-02") {
		a.mu.RLock()
		if a.sectorMoneyFlow != nil && time.Since(a.sectorMoneyFlowTime) < cacheTTL {
			cached := a.sectorMoneyFlow
			a.mu.RUnlock()
			return cached
		}
		a.mu.RUnlock()
	}
	data, err := a.fetcher.GetSectorMoneyFlowByDate(date)
	if err != nil {
		a.log.Error(logger.ModuleData, "GetSectorMoneyFlowByDate %s error: %v", date, err)
		a.mu.RLock()
		cached := a.sectorMoneyFlow
		a.mu.RUnlock()
		return cached
	}
	if len(data) > 0 {
		a.mu.Lock()
		a.sectorMoneyFlow = data
		a.sectorMoneyFlowTime = time.Now()
		a.mu.Unlock()
	}
	return data
}

func (a *App) GetSectorTree() []stock.SectorTreeNode {
	a.log.Info(logger.ModuleData, "GetSectorTree")
	tree, err := a.fetcher.GetSectorTree()
	if err != nil {
		a.log.Error(logger.ModuleData, "GetSectorTree error: %v", err)
		return nil
	}
	return tree
}

// saveAIUsage records token usage from the AI client to ModelUsage
func (a *App) saveAIUsage() {
	usage := a.aiClient.LastUsage()
	if usage == nil || a.store == nil {
		return
	}
	prov := string(a.aiClient.Name())
	mName := a.aiClient.Config().ModelName
	data, err := a.store.Load()
	if err != nil {
		return
	}
	for i, u := range data.ModelUsages {
		if u.Provider == prov && u.ModelName == mName {
			data.ModelUsages[i].InputTokens += int64(usage.PromptTokens)
			data.ModelUsages[i].OutputTokens += int64(usage.CompletionTokens)
			data.ModelUsages[i].TotalRequests++
			a.store.Save(data)
			return
		}
	}
	// No existing record — create one
	a.store.SaveModelUsage(storage.ModelUsage{
		Provider:      prov,
		ModelName:     mName,
		Status:        "available",
		LastTest:      time.Now().Format("2006-01-02 15:04:05"),
		InputTokens:   int64(usage.PromptTokens),
		OutputTokens:  int64(usage.CompletionTokens),
		TotalRequests: 1,
	})
}

// SaveAIUsage is the exported version for SSE server use
func (a *App) SaveAIUsage() {
	a.saveAIUsage()
}

// ==================== Technical Analysis ====================

func (a *App) GetTechnicalAnalysis(code string) *analysis.TechnicalReport {
	realtimeData := a.GetRealtimeData([]string{code})
	klineData := a.GetKLineData(code, 120)

	var rt *stock.RealTimeData
	if data, ok := realtimeData[code]; ok {
		rt = data
	}

	if len(klineData) == 0 && rt == nil {
		return &analysis.TechnicalReport{
			StockCode:  code,
			Suggestion: "无法获取数据",
		}
	}

	return a.analyzer.Analyze(klineData, rt)
}

// ==================== AI Analysis ====================

// isWebSearchRequest checks if the code string has the web search prefix
func isWebSearchRequest(code string) (cleanCode string, webSearch bool) {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "[联网搜索]") || strings.HasPrefix(code, "[websearch]") {
		return strings.TrimSpace(code[len("[联网搜索]"):]), true
	}
	return code, false
}

func (a *App) GetAIAnalysis(code string) string {
	a.log.Info(logger.ModuleAI, "GetAIAnalysis for code: %s", code)

	if a.aiClient == nil {
		a.log.Error(logger.ModuleAI, "GetAIAnalysis %s: AI client nil", code)
		return "AI客户端未初始化"
	}

	// Check for web search prefix
	cleanCode, webSearch := isWebSearchRequest(code)

	// Get stock name from realtime data
	realtimeData := a.GetRealtimeData([]string{cleanCode})
	stockName := cleanCode
	if data, ok := realtimeData[cleanCode]; ok {
		stockName = data.Name
	}

	// Build system prompt with multi-step analysis flow
	systemPrompt := `你是一名专业的A股股票分析师。请按以下步骤分析股票：

## 分析步骤
1. **初步判断** — 根据股票代码/名称，先给出初步的市场定位和板块判断
2. **获取数据** — 使用工具函数获取实时行情、K线数据、技术指标、资金流向
3. **综合分析** — 结合获取的数据进行技术面和基本面分析
4. **操作建议** — 给出具体的买卖建议和风险提示`

	if webSearch {
		systemPrompt += `
5. **联网信息** — 结合市场最新资讯和消息面进行分析（你有联网搜索能力的话请使用）`
	}

	systemPrompt += `

## 输出要求
- 使用markdown格式（###标题、**强调**等）让报告清晰易读
- 段落之间使用单个换行，不要多余空行
- 保持客观理性，不要保证收益
- 你提供的只是分析建议，不构成投资依据`

	// Append position info if available for this stock
	pos := a.store.GetPosition(cleanCode)
	if pos != nil && pos.CostPrice > 0 {
		posText := fmt.Sprintf("\n\n你的持仓：%.0f股，成本价%.2f", pos.Quantity, pos.CostPrice)
		if pos.TargetPrice > 0 {
			posText += fmt.Sprintf("，目标价%.2f", pos.TargetPrice)
		}
		if pos.StopLoss > 0 {
			posText += fmt.Sprintf("，止损价%.2f", pos.StopLoss)
		}
		systemPrompt += posText
	}

	userPrompt := fmt.Sprintf(`请分析股票 **%s**（%s）。使用工具获取数据后给出完整分析报告。`, stockName, cleanCode)

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// Try tool calling first (multi-step: AI decides what data to fetch)
	if a.aiClient != nil {
		result, err := a.aiChatWithTools(messages)
		if err == nil {
			a.saveAIUsage()
			return result
		}
		a.log.Error(logger.ModuleAI, "GetAIAnalysis tool calling failed: %v, falling back to direct chat", err)
	}

	// Fallback: pre-fetch data and send all at once
	realtimeData = a.GetRealtimeData([]string{cleanCode})
	klineData := a.GetKLineData(cleanCode, 60)
	moneyFlow := a.GetMoneyFlow(cleanCode, 10)
	techReport := a.GetTechnicalAnalysis(cleanCode)

	var stockInfo, klineSummary, techInfo, moneyFlowInfo string

	if data, ok := realtimeData[cleanCode]; ok {
		stockName = data.Name
		stockInfo = fmt.Sprintf("现价:%.2f 开:%.2f 高:%.2f 低:%.2f 昨收:%.2f 涨跌幅:%.2f%% 成交量:%.0f万手 成交额:%.2f亿",
			data.Price, data.Open, data.High, data.Low, data.PrevClose,
			data.ChangePercent, float64(data.Volume)/10000, data.Amount/100000000)
	} else {
		stockInfo = "无法获取实时数据"
	}

	if len(klineData) > 0 {
		klineSummary = a.analyzer.KLineSummary(klineData)
	}

	if techReport != nil && len(techReport.Indicators) > 0 {
		var parts []string
		for _, ind := range techReport.Indicators {
			parts = append(parts, fmt.Sprintf("%s: %.2f(%s)", ind.Name, ind.Value, ind.Signal))
		}
		techInfo = strings.Join(parts, ", ")
	}

	if len(moneyFlow) > 0 {
		last := moneyFlow[len(moneyFlow)-1]
		moneyFlowInfo = fmt.Sprintf("主力净流入:%.2f亿(占比%.1f%%)",
			last.MainNet/100000000, last.MainRatio)
	}

	fallbackPrompt := fmt.Sprintf(`分析股票 **%s**（%s）

## 当前行情
%s

## K线技术概要
%s

## 技术指标
%s

## 资金流向
%s

请给出详细的技术分析和操作建议。使用 ### 标题，段落之间单个换行。`,
		stockName, cleanCode, stockInfo, klineSummary, techInfo, moneyFlowInfo)

	msgFallback := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fallbackPrompt},
	}

	result, err := a.aiClient.Chat(msgFallback)
	if err != nil {
		return fmt.Sprintf("AI分析出错: %v。请检查API配置。", err)
	}
	a.saveAIUsage()
	return result
}

func (a *App) AIChat(message string) string {
	a.log.Info(logger.ModuleAI, "AIChat called, msg len: %d", len(message))
	if a.aiClient == nil {
		return "AI客户端未初始化"
	}

	// Check for /clear command (handled client-side, but support here too)
	if message == "/clear" {
		return "[上下文已清除]"
	}

	// Build system prompt with position context
	positions := a.store.GetAllPositions()
	positionText := ""
	if len(positions) > 0 {
		var lines []string
		for _, p := range positions {
			lines = append(lines, fmt.Sprintf("- %s(%s): 持仓%.0f股, 成本价%.2f", p.Name, p.Code, p.Quantity, p.CostPrice))
			if p.TargetPrice > 0 {
				lines = append(lines, fmt.Sprintf("  目标价%.2f", p.TargetPrice))
			}
			if p.StopLoss > 0 {
				lines = append(lines, fmt.Sprintf("  止损价%.2f", p.StopLoss))
			}
		}
		positionText = "\n\n你的持仓信息：\n" + strings.Join(lines, "\n")
	}

	systemPrompt := `你是一位专业的A股投资顾问，精通技术分析和基本面分析。` + positionText + `

重要提示：
- 以下是截至当前时间的实时市场数据，你可以基于这些数据进行分析
- 如果你需要查询某只股票的实时数据，请使用可用的工具函数
- 你可以调用的工具有：get_realtime_data（获取实时行情）、get_kline_data（获取K线数据）、get_technical_analysis（获取技术指标）
- 请基于客观数据和分析给出建议，并提醒投资风险
- 回答请用中文，简明扼要，专业客观
- 注意：你提供的只是分析建议，不构成投资依据`

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: message},
	}

	// Check if provider supports tools
	if a.aiClient != nil {
		result, err := a.aiChatWithTools(messages)
		if err == nil {
			a.saveAIUsage()
			return result
		}
		// Fallback to simple chat
	}

	result, err := a.aiClient.Chat(messages)
	if err != nil {
		return fmt.Sprintf("AI回复出错: %v。请检查API配置。", err)
	}
	a.saveAIUsage()
	return result
}

// AIChatWithHistoryRaw accepts pre-parsed ChatMessage slice (used by SSE server)
func (a *App) AIChatWithHistoryRaw(messages []ai.ChatMessage) string {
	a.log.Info(logger.ModuleAI, "AIChatWithHistoryRaw called, msgs: %d", len(messages))
	if a.aiClient == nil {
		return "AI客户端未初始化"
	}

	// Prepend system prompt with position context
	positions := a.store.GetAllPositions()
	positionText := ""
	if len(positions) > 0 {
		var lines []string
		for _, p := range positions {
			lines = append(lines, fmt.Sprintf("- %s(%s): 持仓%.0f股, 成本价%.2f", p.Name, p.Code, p.Quantity, p.CostPrice))
			if p.TargetPrice > 0 {
				lines = append(lines, fmt.Sprintf("  目标价%.2f", p.TargetPrice))
			}
			if p.StopLoss > 0 {
				lines = append(lines, fmt.Sprintf("  止损价%.2f", p.StopLoss))
			}
		}
		positionText = "\n\n当前持仓信息：\n" + strings.Join(lines, "\n")
	}

	// Check if first message is system prompt, append position text
	if len(messages) > 0 && messages[0].Role == "system" {
		messages[0].Content += positionText
	}

	// Try tool calling
	if a.aiClient != nil {
		result, err := a.aiChatWithTools(messages)
		if err == nil {
			a.saveAIUsage()
			return result
		}
	}

	result, err := a.aiClient.Chat(messages)
	if err != nil {
		return fmt.Sprintf("AI回复出错: %v。请检查API配置。", err)
	}
	a.saveAIUsage()
	return result
}

// AIChatWithHistory accepts full message history as JSON and optional custom system prompt
func (a *App) AIChatWithHistory(messagesJSON, systemPrompt string) string {
	a.log.Info(logger.ModuleAI, "AIChatWithHistory called, sysPrompt len: %d", len(systemPrompt))
	if a.aiClient == nil {
		return "AI客户端未初始化"
	}

	var messages []ai.ChatMessage
	if err := json.Unmarshal([]byte(messagesJSON), &messages); err != nil {
		return fmt.Sprintf("参数解析失败: %v", err)
	}

	// Append position context
	positions := a.store.GetAllPositions()
	positionText := ""
	if len(positions) > 0 {
		var lines []string
		for _, p := range positions {
			lines = append(lines, fmt.Sprintf("- %s(%s): 持仓%.0f股, 成本价%.2f", p.Name, p.Code, p.Quantity, p.CostPrice))
			if p.TargetPrice > 0 {
				lines = append(lines, fmt.Sprintf("  目标价%.2f", p.TargetPrice))
			}
			if p.StopLoss > 0 {
				lines = append(lines, fmt.Sprintf("  止损价%.2f", p.StopLoss))
			}
		}
		positionText = "\n\n你的持仓信息：\n" + strings.Join(lines, "\n")
	}

	// Prepend system prompt with position context
	if systemPrompt != "" {
		systemPrompt += positionText
		messages = append([]ai.ChatMessage{{Role: "system", Content: systemPrompt}}, messages...)
	}

	// Try tool calling
	if a.aiClient != nil {
		result, err := a.aiChatWithTools(messages)
		if err == nil {
			a.saveAIUsage()
			return result
		}
	}

	result, err := a.aiClient.Chat(messages)
	if err != nil {
		return fmt.Sprintf("AI回复出错: %v。请检查API配置。", err)
	}
	a.saveAIUsage()
	return result
}

// AIChatStream streams AI response via Wails events
func (a *App) AIChatStream(messagesJSON, systemPrompt string) {
	if a.aiClient == nil {
		runtime.EventsEmit(a.ctx, "ai:stream:error", "AI客户端未配置，请在设置中配置API")
		return
	}

	var messages []ai.ChatMessage
	if err := json.Unmarshal([]byte(messagesJSON), &messages); err != nil {
		runtime.EventsEmit(a.ctx, "ai:stream:error", fmt.Sprintf("参数解析失败: %v", err))
		return
	}

	if systemPrompt != "" {
		// Append position context
		positions := a.store.GetAllPositions()
		if len(positions) > 0 {
			var lines []string
			for _, p := range positions {
				lines = append(lines, fmt.Sprintf("- %s(%s): 持仓%.0f股, 成本价%.2f", p.Name, p.Code, p.Quantity, p.CostPrice))
				if p.TargetPrice > 0 {
					lines = append(lines, fmt.Sprintf("  目标价%.2f", p.TargetPrice))
				}
				if p.StopLoss > 0 {
					lines = append(lines, fmt.Sprintf("  止损价%.2f", p.StopLoss))
				}
			}
			systemPrompt += "\n\n你的持仓信息：\n" + strings.Join(lines, "\n")
		}
		messages = append([]ai.ChatMessage{{Role: "system", Content: systemPrompt}}, messages...)
	}

	go func() {
		var fullContent strings.Builder

		// Check if provider supports streaming before attempting
		if !a.aiClient.SupportsStreaming() {
			a.log.Info(logger.ModuleAI, "Provider does not support streaming, falling back to non-streaming")
			result := a.AIChatWithHistoryRaw(messages)
			runtime.EventsEmit(a.ctx, "ai:stream:chunk", result)
			a.saveAIUsage()
			runtime.EventsEmit(a.ctx, "ai:stream:done", result)
			return
		}

		err := a.aiClient.ChatStream(messages, func(chunk string) {
			fullContent.WriteString(chunk)
			runtime.EventsEmit(a.ctx, "ai:stream:chunk", chunk)
		})
		if err != nil {
			// Streaming unavailable → fall back to non-streaming
			a.log.Info(logger.ModuleAI, "ChatStream failed: %v, falling back to non-streaming", err)
			result := a.AIChatWithHistoryRaw(messages)
			runtime.EventsEmit(a.ctx, "ai:stream:chunk", result)
			a.saveAIUsage()
			runtime.EventsEmit(a.ctx, "ai:stream:done", result)
		} else {
			a.saveAIUsage()
			runtime.EventsEmit(a.ctx, "ai:stream:done", fullContent.String())
		}
	}()
}

// aiChatWithTools runs the AI chat with tool calling loop (max 5 rounds)
func (a *App) aiChatWithTools(messages []ai.ChatMessage) (string, error) {
	tools := []ai.ToolDefinition{
		{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        "get_realtime_data",
				Description: "获取A股股票实时行情数据，包括当前价格、涨跌幅、成交量、成交额等",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code": map[string]any{
							"type":        "string",
							"description": "股票代码，如 600519 或 sh600519",
						},
					},
					"required": []string{"code"},
				},
			},
		},
		{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        "get_kline_data",
				Description: "获取A股股票K线数据，用于技术分析，包含日期、开盘价、收盘价、最高价、最低价、成交量",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code": map[string]any{"type": "string", "description": "股票代码"},
						"days": map[string]any{"type": "integer", "description": "获取最近多少天的数据，默认60"},
					},
					"required": []string{"code"},
				},
			},
		},
		{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        "get_technical_analysis",
				Description: "获取A股股票技术分析报告，包含MA均线、RSI、MACD、KDJ等指标信号和操作建议",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code": map[string]any{"type": "string", "description": "股票代码"},
					},
					"required": []string{"code"},
				},
			},
		},
		{
			Type: "function",
			Function: ai.ToolFunction{
				Name:        "get_money_flow",
				Description: "获取A股股票资金流向数据，包含主力资金净流入、散户资金流入流出等",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code": map[string]any{"type": "string", "description": "股票代码"},
						"days": map[string]any{"type": "integer", "description": "获取最近多少天的数据，默认10"},
					},
					"required": []string{"code"},
				},
			},
		},
	}

	for round := 0; round < 5; round++ {
		var toolSourceRefs []string
		resp, err := a.aiClient.ChatWithTools(messages, tools)
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("AI无有效回复")
		}

		choice := resp.Choices[0].Message
		if choice.ToolCalls == nil || len(choice.ToolCalls) == 0 {
			// Add source attribution for previous tool results
			content := choice.Content
			if len(toolSourceRefs) > 0 {
				content += "\n\n---\n数据来源:\n" + strings.Join(toolSourceRefs, "\n")
			}
			return content, nil
		}

		// Add assistant message with tool calls
		messages = append(messages, ai.ChatMessage{
			Role:      "assistant",
			Content:   choice.Content,
			ToolCalls: choice.ToolCalls,
		})

		// Execute each tool call
		for _, tc := range choice.ToolCalls {
			result, source := a.executeToolWithSource(tc.Function.Name, tc.Function.Arguments)
			toolSourceRefs = append(toolSourceRefs, source)
			messages = append(messages, ai.ChatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	return "AI分析已达到最大轮次，请简化您的问题或尝试更具体的提问。", nil
}

// extractCode strips exchange prefix and returns bare A-share code
func extractCode(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.ToLower(raw)
	raw = strings.TrimPrefix(raw, "sh")
	raw = strings.TrimPrefix(raw, "sz")
	raw = strings.TrimPrefix(raw, "bj")
	return raw
}

func (a *App) executeToolWithSource(name, argsJSON string) (resultJSON, source string) {
	// Flexible parsing: accept both "code" and "codes" param names
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return `{"error":"参数解析失败"}`, ""
	}

	getCode := func() string {
		if c, ok := args["code"].(string); ok && c != "" {
			return extractCode(c)
		}
		if codes, ok := args["codes"].([]any); ok && len(codes) > 0 {
			if c, ok := codes[0].(string); ok {
				return extractCode(c)
			}
		}
		return ""
	}

	getDays := func(def int) int {
		switch v := args["days"].(type) {
		case float64:
			return int(v)
		case int:
			return v
		}
		if count, ok := args["count"].(float64); ok {
			return int(count)
		}
		return def
	}

	code := getCode()

	switch name {
	case "get_realtime_data":
		if code == "" {
			return `{"error":"缺少股票代码"}`, ""
		}
		data := a.GetRealtimeData([]string{code})
		b, _ := json.Marshal(data)
		source = fmt.Sprintf("• %s 实时行情: 东方财富", code)
		return string(b), source

	case "get_kline_data":
		if code == "" {
			return `{"error":"缺少股票代码"}`, ""
		}
		days := getDays(60)
		data := a.GetKLineData(code, days)
		if len(data) == 0 {
			return `{"error":"未获取到K线数据"}`, ""
		}
		b, _ := json.Marshal(data)
		source = fmt.Sprintf("• %s K线数据(近%d日): 东方财富", code, days)
		return string(b), source

	case "get_technical_analysis":
		if code == "" {
			return `{"error":"缺少股票代码"}`, ""
		}
		data := a.GetTechnicalAnalysis(code)
		b, _ := json.Marshal(data)
		source = fmt.Sprintf("• %s 技术分析: MA/RSI/MACD/KD", code)
		return string(b), source

	case "get_money_flow":
		if code == "" {
			return `{"error":"缺少股票代码"}`, ""
		}
		days := getDays(10)
		data := a.GetMoneyFlow(code, days)
		b, _ := json.Marshal(data)
		source = fmt.Sprintf("• %s 资金流向(近%d日): 东方财富", code, days)
		return string(b), source

	default:
		return fmt.Sprintf(`{"error":"未知工具: %s"}`, name), ""
	}
}

// ==================== Self-Select Stocks ====================

func (a *App) GetSelfSelectStocks() []stock.SelfSelectStock {
	if a.store == nil {
		return []stock.SelfSelectStock{}
	}
	data, err := a.store.Load()
	if err != nil {
		return []stock.SelfSelectStock{}
	}
	if data.Stocks == nil {
		return []stock.SelfSelectStock{}
	}
	return data.Stocks
}

func (a *App) GetGroups() []stock.StockGroup {
	if a.store == nil {
		return []stock.StockGroup{{Name: "自选", Order: 0}}
	}
	data, err := a.store.Load()
	if err != nil {
		return []stock.StockGroup{{Name: "自选", Order: 0}}
	}
	if data.Groups == nil {
		return []stock.StockGroup{{Name: "自选", Order: 0}}
	}
	return data.Groups
}

func (a *App) AddSelfSelectStock(code, name, group string) string {
	if group == "" {
		data, _ := a.store.Load()
		if len(data.Groups) > 0 {
			group = data.Groups[0].Name
		} else {
			group = "自选"
		}
	}

	stock := stock.SelfSelectStock{
		Code:    strings.ToLower(code),
		Name:    name,
		Group:   group,
		AddedAt: time.Now().UnixMilli(),
	}

	if err := a.store.AddStock(stock); err != nil {
		return err.Error()
	}
	return "ok"
}

func (a *App) RemoveSelfSelectStock(code string) string {
	if err := a.store.RemoveStock(strings.ToLower(code)); err != nil {
		return err.Error()
	}
	return "ok"
}

func (a *App) MoveStockToGroup(code, group string) string {
	if err := a.store.MoveStock(strings.ToLower(code), group); err != nil {
		return err.Error()
	}
	return "ok"
}

func (a *App) UpdateStockNotes(code, notes string) string {
	if err := a.store.UpdateStockNotes(strings.ToLower(code), notes); err != nil {
		return err.Error()
	}
	return "ok"
}

// ==================== Group Management ====================

func (a *App) AddGroup(name string) string {
	if err := a.store.AddGroup(name); err != nil {
		return err.Error()
	}
	return "ok"
}

func (a *App) RemoveGroup(name string) string {
	if err := a.store.RemoveGroup(name); err != nil {
		return err.Error()
	}
	return "ok"
}

func (a *App) RenameGroup(oldName, newName string) string {
	if err := a.store.RenameGroup(oldName, newName); err != nil {
		return err.Error()
	}
	return "ok"
}

// ==================== Position Management ====================

func (a *App) GetPosition(code string) *storage.Position {
	return a.store.GetPosition(code)
}

func (a *App) GetAllPositions() []storage.Position {
	return a.store.GetAllPositions()
}

func (a *App) SavePosition(pos storage.Position) string {
	if err := a.store.SavePosition(pos); err != nil {
		return err.Error()
	}
	return "ok"
}

func (a *App) DeletePosition(code string) string {
	if err := a.store.DeletePosition(code); err != nil {
		return err.Error()
	}
	return "ok"
}

// ==================== Model Usage ====================

func (a *App) GetModelUsages() []storage.ModelUsage {
	return a.store.GetModelUsages()
}

func (a *App) DeleteModelUsage(provider, modelName string) string {
	if err := a.store.DeleteModelUsage(provider, modelName); err != nil {
		return err.Error()
	}
	return "ok"
}

// ==================== Settings ====================

type Settings struct {
	Config          ai.Config `json:"config"`
	RefreshInterval int       `json:"refreshInterval"`
	DataSource      string    `json:"dataSource"`
}

func (a *App) GetSettings() *Settings {
	data, err := a.store.Load()
	if err != nil {
		return &Settings{
			RefreshInterval: 5,
			DataSource:      "auto",
		}
	}
	ds := data.DataSource
	if ds == "" {
		ds = "auto"
	}
	return &Settings{
		Config:          data.Config,
		RefreshInterval: data.RefreshInterval,
		DataSource:      ds,
	}
}

func (a *App) SaveSettings(settings Settings) string {
	if err := a.store.SaveConfig(settings.Config); err != nil {
		return err.Error()
	}
	if err := a.store.SaveRefreshInterval(settings.RefreshInterval); err != nil {
		return err.Error()
	}
	if err := a.store.SaveDataSource(settings.DataSource); err != nil {
		return err.Error()
	}

	// Also save/update model usage record
	usage := storage.ModelUsage{
		Provider:  string(settings.Config.Provider),
		ModelName: settings.Config.ModelName,
		Endpoint:  settings.Config.Endpoint,
		APIKey:    settings.Config.APIKey,
		Status:    "available",
		LastTest:  time.Now().Format("2006-01-02 15:04:05"),
	}
	a.store.SaveModelUsage(usage)

	a.fetcher.SetDataSource(settings.DataSource)
	a.aiClient = ai.NewClient(settings.Config)
	return "ok"
}

func (a *App) GetRealTimePriority() []string {
	return a.fetcher.GetRealTimePriority()
}

func (a *App) SaveRealTimePriority(priority []string) string {
	a.fetcher.SetRealTimePriority(priority)
	if err := a.store.SaveRealTimePriority(priority); err != nil {
		return err.Error()
	}
	return "ok"
}

func (a *App) GetSourceStats() []*stock.SourceStats {
	return a.fetcher.GetSourceStats()
}

// TTS methods
type TTSConfig struct {
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
}

func (a *App) GetTTSConfig() *TTSConfig {
	p, k := a.store.GetTTS()
	return &TTSConfig{Provider: p, APIKey: k}
}

func (a *App) SaveTTSConfig(cfg TTSConfig) string {
	if cfg.Provider == "" {
		cfg.Provider = "browser"
	}
	if err := a.store.SaveTTS(cfg.Provider, cfg.APIKey); err != nil {
		return err.Error()
	}
	return "ok"
}

func (a *App) TextToSpeech(text, provider string) string {
	// Browser TTS is handled on frontend via Web Speech API
	// Fish Audio / Qwen3 API requires external HTTP calls
	// For now, return empty (frontend will use browser TTS as fallback)
	if provider == "" || provider == "browser" {
		return ""
	}
	return ""
}

// SwitchModel switches the AI model at runtime and persists to storage
func (a *App) SwitchModel(provider, model, endpoint, apiKey string) string {
	if a.aiClient == nil {
		return "AI客户端未初始化"
	}
	// Preserve existing apiKey/endpoint if new ones are empty (protect against accidental clearing)
	if apiKey == "" && a.aiClient.Config().APIKey != "" {
		apiKey = a.aiClient.Config().APIKey
	}
	if endpoint == "" && a.aiClient.Config().Endpoint != "" {
		endpoint = a.aiClient.Config().Endpoint
	}
	cfg := ai.Config{
		Provider:  ai.ProviderType(provider),
		ModelName: model,
		Endpoint:  endpoint,
		APIKey:    apiKey,
		Timeout:   30,
	}
	a.aiClient.UpdateConfig(cfg)
	// Persist to storage
	if a.store != nil {
		a.store.SaveConfig(cfg)
	}
	return "ok"
}

// ==================== AI test ====================

func (a *App) TestAIConnection(provider, endpoint, apiKey, model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		switch ai.ProviderType(provider) {
		case ai.ProviderOpenAI:
			model = "gpt-3.5-turbo"
		case ai.ProviderDeepSeek:
			model = "deepseek-chat"
		case ai.ProviderVolcano:
			model = "" // 火山引擎需要用户自行填写已部署的模型名
		case ai.ProviderOllama:
			// will try whatever model is installed
		case ai.ProviderOpenCode:
			model = "deepseek-v4-flash-free"
		}
	}

	cfg := ai.Config{
		Provider:  ai.ProviderType(strings.TrimSpace(provider)),
		Endpoint:  strings.TrimSpace(endpoint),
		APIKey:    strings.TrimSpace(apiKey),
		ModelName: model,
		Timeout:   15,
	}
	client := ai.NewClient(cfg)
	msg := []ai.ChatMessage{
		{Role: "user", Content: "你好，请回复「ok」测试连接"},
	}
	result, err := client.Chat(msg)
	if err != nil {
		errText := err.Error()
		msg := fmt.Sprintf("✗ 连接失败: %s", errText)
		if strings.Contains(errText, "model") || strings.Contains(errText, "Model") {
			msg += "\n\n提示: 模型名称可能不正确，可点击「获取模型列表」查看可用模型"
		}
		if strings.Contains(errText, "API key") || strings.Contains(errText, "apikey") || strings.Contains(errText, "unauthorized") || strings.Contains(errText, "401") {
			msg += "\n\n提示: API密钥无效或未配置"
		}
		if strings.Contains(errText, "429") {
			msg += "\n\n提示: 请求过于频繁(429)，请稍后再试"
		}
		return msg
	}
	if result == "" {
		return "✗ 连接成功但返回内容为空，请检查模型名称是否正确"
	}

	// Save model usage status
	now := time.Now().Format("2006-01-02 15:04:05")
	a.store.SaveModelUsage(storage.ModelUsage{
		Provider:  provider,
		ModelName: model,
		Status:    "available",
		LastTest:  now,
	})

	return "ok"
}

// ==================== AI Model List ====================

type ModelListResult struct {
	Models []string `json:"models"`
	Error  string   `json:"error"`
}

func (a *App) ListModels(provider, endpoint, apiKey string) ModelListResult {
	cli := &http.Client{Timeout: 10 * time.Second}
	endpoint = strings.TrimSpace(endpoint)
	apiKey = strings.TrimSpace(apiKey)
	ep := strings.TrimRight(endpoint, "/")

	switch ai.ProviderType(provider) {
	case ai.ProviderOllama:
		base := ep
		if base == "" {
			base = "http://localhost:11434"
		}
		base = strings.TrimSuffix(base, "/v1")
		return listOllamaModels(cli, base+"/api/tags")

	case ai.ProviderVolcano:
		base := ep
		if base == "" {
			base = "https://ark.cn-beijing.volces.com"
		}
		return listVolcanoModels(cli, base+"/api/v3/models", apiKey)

	default:
		base := ep
		if base == "" {
			base = "https://api.openai.com"
		}
		return listOpenAIModels(cli, base+"/v1/models", apiKey)
	}
}

func listOllamaModels(cli *http.Client, url string) ModelListResult {
	resp, err := cli.Get(url)
	if err != nil {
		return ModelListResult{Error: fmt.Sprintf("请求失败: %v", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return ModelListResult{Error: fmt.Sprintf("API返回 %d: %s", resp.StatusCode, string(body))}
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return ModelListResult{Error: fmt.Sprintf("解析失败: %v", err)}
	}
	var models []string
	if list, ok := result["models"].([]interface{}); ok {
		for _, m := range list {
			if mm, ok := m.(map[string]interface{}); ok {
				if name, ok := mm["name"].(string); ok {
					models = append(models, name)
				}
			}
		}
	}
	if len(models) == 0 {
		return ModelListResult{Error: "未获取到模型，请检查Ollama服务是否运行"}
	}
	return ModelListResult{Models: models}
}

func listOpenAIModels(cli *http.Client, url, apiKey string) ModelListResult {
	req, _ := http.NewRequest("GET", url, nil)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")
	resp, err := cli.Do(req)
	if err != nil {
		return ModelListResult{Error: fmt.Sprintf("请求失败: %v", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		errMsg := fmt.Sprintf("API返回 %d: %s", resp.StatusCode, string(body))
		if resp.StatusCode == 401 {
			errMsg = "认证失败(401)，请检查API密钥是否正确"
		} else if resp.StatusCode == 404 {
			errMsg = fmt.Sprintf("端点不存在(404)，请检查API端点地址是否正确。\n请求URL: %s", url)
		}
		return ModelListResult{Error: errMsg}
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return ModelListResult{Error: fmt.Sprintf("解析失败: %v", err)}
	}
	var models []string
	if list, ok := result["data"].([]interface{}); ok {
		for _, m := range list {
			if mm, ok := m.(map[string]interface{}); ok {
				if id, ok := mm["id"].(string); ok {
					models = append(models, id)
				}
			}
		}
	}
	if len(models) == 0 {
		return ModelListResult{Error: "该API端点未返回可用模型列表"}
	}
	return ModelListResult{Models: models}
}

func listVolcanoModels(cli *http.Client, url, apiKey string) ModelListResult {
	req, _ := http.NewRequest("GET", url, nil)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")
	resp, err := cli.Do(req)
	if err != nil {
		return ModelListResult{Error: fmt.Sprintf("请求失败: %v", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		errMsg := fmt.Sprintf("API返回 %d: %s", resp.StatusCode, string(body))
		if resp.StatusCode == 401 {
			errMsg = "认证失败(401)，请检查API密钥是否正确"
		} else if resp.StatusCode == 404 {
			errMsg = fmt.Sprintf("端点不存在(404)，请检查API端点地址是否正确。\n请求URL: %s", url)
		}
		return ModelListResult{Error: errMsg}
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return ModelListResult{Error: fmt.Sprintf("解析失败: %v", err)}
	}
	var models []string
	if list, ok := result["data"].([]interface{}); ok {
		for _, m := range list {
			if mm, ok := m.(map[string]interface{}); ok {
				id, _ := mm["id"].(string)
				if id == "" {
					continue
				}
				// 过滤掉已下线(Shutdown/Retiring)的模型
				if status, ok := mm["status"].(string); ok && (status == "Shutdown" || status == "Retiring") {
					continue
				}
				models = append(models, id)
			}
		}
	}
	if len(models) == 0 {
		return ModelListResult{Error: "未找到可用模型。请确保已在火山方舟控制台部署模型。\n列表URL: " + url}
	}
	return ModelListResult{Models: models}
}

// ==================== Batch Refresh ====================

type StockWithRealtime struct {
	Info   stock.SelfSelectStock `json:"info"`
	Market *stock.RealTimeData   `json:"market,omitempty"`
}

func (a *App) RefreshAllSelfSelect() []StockWithRealtime {
	stocks := a.GetSelfSelectStocks()
	if len(stocks) == 0 {
		return nil
	}

	codes := make([]string, len(stocks))
	codeMap := make(map[string]int)
	for i, s := range stocks {
		code := s.Code
		codes[i] = code
		codeMap[code] = i
	}

	realtimeMap := a.GetRealtimeData(codes)

	// Cache successful results
	a.mu.Lock()
	if a.realtimeCache == nil {
		a.realtimeCache = make(map[string]*stock.RealTimeData)
	}
	if realtimeMap != nil {
		for code, rt := range realtimeMap {
			if rt != nil {
				a.realtimeCache[code] = rt
			}
		}
	}
	cache := make(map[string]*stock.RealTimeData, len(a.realtimeCache))
	for k, v := range a.realtimeCache {
		cache[k] = v
	}
	a.mu.Unlock()

	result := make([]StockWithRealtime, len(stocks))
	for i, s := range stocks {
		result[i] = StockWithRealtime{Info: s}
		if rt, ok := realtimeMap[s.Code]; ok && rt != nil {
			result[i].Market = rt
		} else if cached, ok := cache[s.Code]; ok {
			// Fall back to cached data
			result[i].Market = cached
		}
	}

	return result
}

func (a *App) GetRefreshInterval() int {
	data, err := a.store.Load()
	if err != nil {
		return 5
	}
	return data.RefreshInterval
}

// ==================== Logging ====================

type LogQueryResult struct {
	Modules []string          `json:"modules"`
	Entries []logger.LogEntry `json:"entries"`
}

func (a *App) GetLogDates() []string {
	dirs, err := a.log.GetDateDirs()
	if err != nil {
		return nil
	}
	return dirs
}

func (a *App) GetLogModules(date string) []string {
	modules, err := a.log.GetModules(date)
	if err != nil {
		return nil
	}
	return modules
}

func (a *App) GetLogs(date, module string) []logger.LogEntry {
	entries, err := a.log.GetLogs(date, module)
	if err != nil {
		return nil
	}
	return entries
}

func (a *App) GetLogAIInterpretation(date, module string) string {
	entries, err := a.log.GetLogs(date, module)
	if err != nil {
		return "获取日志失败"
	}
	if len(entries) == 0 {
		return "暂无日志可分析"
	}

	// Build a summary for AI interpretation
	errorCount := 0
	warnCount := 0
	infoCount := 0
	var recentErrors []string
	for _, e := range entries {
		switch e.Level {
		case "ERROR":
			errorCount++
			if len(recentErrors) < 5 {
				recentErrors = append(recentErrors, e.Message)
			}
		case "WARN":
			warnCount++
		case "INFO":
			infoCount++
		}
	}

	summary := fmt.Sprintf(`请分析以下日志内容，指出潜在问题和改进建议：

模块: %s
日期: %s

日志统计:
- ERROR: %d 条
- WARN: %d 条
- INFO: %d 条

最近错误:
%s

请用中文回答，指出可能的问题原因和解决方案。`,
		module, date, errorCount, warnCount, infoCount,
		strings.Join(recentErrors, "\n"))

	if a.aiClient == nil {
		return "AI客户端未初始化"
	}

	messages := []ai.ChatMessage{
		{Role: "system", Content: "你是一位系统运维专家，擅长分析日志定位问题。"},
		{Role: "user", Content: summary},
	}
	reply, err := a.aiClient.Chat(messages)
	if err != nil {
		return fmt.Sprintf("AI分析失败: %v", err)
	}
	a.saveAIUsage()
	return reply
}
