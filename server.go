package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"stock-advisor/internal/ai"
	"stock-advisor/internal/storage"
	"strings"
)

type WebServer struct {
	app    *App
	port   int
	user   string
	pass   string
	server *http.Server
}

func NewWebServer(app *App, port int, user, pass string) *WebServer {
	return &WebServer{
		app:  app,
		port: port,
		user: user,
		pass: pass,
	}
}

func (s *WebServer) Start() error {
	mux := http.NewServeMux()

	// API routes (protected by auth)
	mux.HandleFunc("/api/call", s.auth(s.handleCall))
	mux.HandleFunc("/api/chat/stream", s.auth(s.handleChatStream))

	// Static file server with SPA fallback
	mux.HandleFunc("/", s.auth(s.handleStatic))

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	log.Printf("[服务器] Web服务启动于 http://0.0.0.0:%d", s.port)
	return s.server.ListenAndServe()
}

func (s *WebServer) Stop() {
	if s.server != nil {
		s.server.Close()
	}
}

// auth wraps a handler with HTTP Basic Auth
func (s *WebServer) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.user == "" {
			next(w, r)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != s.user || pass != s.pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="Stock Advisor"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// writeJSON helper
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// writeError helper
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ==================== /api/call ====================

type callRequest struct {
	Method string          `json:"method"`
	Args   json.RawMessage `json:"args"` // JSON array of positional args
}

func (s *WebServer) handleCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "Method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, 400, "Bad request")
		return
	}

	var req callRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, 400, "Bad JSON")
		return
	}

	// Decode args as a flexible array
	var args []interface{}
	if req.Args != nil {
		json.Unmarshal(req.Args, &args)
	}

	result := s.dispatch(req.Method, args)
	writeJSON(w, result)
}

func (s *WebServer) dispatch(method string, args []interface{}) interface{} {
	a := s.app

	switch method {
	// Stock data
	case "SearchStock":
		return a.SearchStock(getArgString(args, 0))
	case "GetRealtimeData":
		codes := getArgStrings(args, 0)
		return a.GetRealtimeData(codes)
	case "GetKLineData":
		return a.GetKLineData(getArgString(args, 0), getArgInt(args, 1, 120))
	case "GetTimeSharingData":
		return a.GetTimeSharingData(getArgString(args, 0))
	case "GetMoneyFlow":
		return a.GetMoneyFlow(getArgString(args, 0), getArgInt(args, 1, 30))
	case "GetStockIndustry":
		return a.GetStockIndustry(getArgString(args, 0))
	case "GetTechnicalAnalysis":
		return a.GetTechnicalAnalysis(getArgString(args, 0))
	case "GetAIAnalysis":
		return a.GetAIAnalysis(getArgString(args, 0))
	case "GetMarketSummary":
		return a.GetMarketSummary()
	case "GetIndexMoneyFlow":
		return a.GetIndexMoneyFlow(getArgString(args, 0), getArgInt(args, 1, 10))
	case "GetSectorMoneyFlow":
		return a.GetSectorMoneyFlow()
	case "GetSectorMoneyFlowByDate":
		return a.GetSectorMoneyFlowByDate(getArgString(args, 0))
	case "GetSectorTree":
		return a.GetSectorTree()
	case "GetRealTimePriority":
		return a.GetRealTimePriority()
	case "SaveRealTimePriority":
		return a.SaveRealTimePriority(getArgStringSlice(args, 0))
	case "GetSourceStats":
		return a.GetSourceStats()

	// Self-select stocks
	case "GetSelfSelectStocks":
		return a.GetSelfSelectStocks()
	case "GetGroups":
		return a.GetGroups()
	case "AddSelfSelectStock":
		return a.AddSelfSelectStock(getArgString(args, 0), getArgString(args, 1), getArgString(args, 2))
	case "RemoveSelfSelectStock":
		return a.RemoveSelfSelectStock(getArgString(args, 0))
	case "MoveStockToGroup":
		return a.MoveStockToGroup(getArgString(args, 0), getArgString(args, 1))
	case "UpdateStockNotes":
		return a.UpdateStockNotes(getArgString(args, 0), getArgString(args, 1))
	case "AddGroup":
		return a.AddGroup(getArgString(args, 0))
	case "RemoveGroup":
		return a.RemoveGroup(getArgString(args, 0))
	case "RenameGroup":
		return a.RenameGroup(getArgString(args, 0), getArgString(args, 1))

	// Positions
	case "GetPosition":
		return a.GetPosition(getArgString(args, 0))
	case "GetAllPositions":
		return a.GetAllPositions()
	case "SavePosition":
		if len(args) < 1 {
			return "missing position"
		}
		b, _ := json.Marshal(args[0])
		var pos storage.Position
		json.Unmarshal(b, &pos)
		return a.SavePosition(pos)
	case "DeletePosition":
		return a.DeletePosition(getArgString(args, 0))

	// Settings
	case "GetSettings":
		return a.GetSettings()
	case "SaveSettings":
		if len(args) < 1 {
			return "missing settings"
		}
		b, _ := json.Marshal(args[0])
		var settings Settings
		json.Unmarshal(b, &settings)
		return a.SaveSettings(settings)
	case "SwitchModel":
		return a.SwitchModel(getArgString(args, 0), getArgString(args, 1), getArgString(args, 2), getArgString(args, 3))
	case "TestAIConnection":
		return a.TestAIConnection(getArgString(args, 0), getArgString(args, 1), getArgString(args, 2), getArgString(args, 3))
	case "ListModels":
		return a.ListModels(getArgString(args, 0), getArgString(args, 1), getArgString(args, 2))
	case "GetModelUsages":
		return a.GetModelUsages()
	case "DeleteModelUsage":
		return a.DeleteModelUsage(getArgString(args, 0), getArgString(args, 1))

	// Batch refresh
	case "RefreshAllSelfSelect":
		return a.RefreshAllSelfSelect()
	case "GetRefreshInterval":
		return a.GetRefreshInterval()

	// AI Chat (non-streaming)
	case "AIChat":
		return a.AIChat(getArgString(args, 0))
	case "AIChatWithHistory":
		return a.AIChatWithHistory(getArgString(args, 0), getArgString(args, 1))

	// Logs
	case "GetLogDates":
		return a.GetLogDates()
	case "GetLogModules":
		return a.GetLogModules(getArgString(args, 0))
	case "GetLogs":
		return a.GetLogs(getArgString(args, 0), getArgString(args, 1))
	case "GetLogAIInterpretation":
		return a.GetLogAIInterpretation(getArgString(args, 0), getArgString(args, 1))

	default:
		return map[string]string{"error": fmt.Sprintf("unknown method: %s", method)}
	}
}

func getArgString(args []interface{}, idx int) string {
	if idx >= len(args) {
		return ""
	}
	s, _ := args[idx].(string)
	return s
}

func getArgStringSlice(args []interface{}, idx int) []string {
	if idx >= len(args) {
		return nil
	}
	raw, ok := args[idx].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, len(raw))
	for i, v := range raw {
		out[i], _ = v.(string)
	}
	return out
}

func getArgInt(args []interface{}, idx int, def int) int {
	if idx >= len(args) {
		return def
	}
	switch v := args[idx].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		fmt.Sscanf(v, "%d", &def)
		return def
	}
	return def
}

func getArgStrings(args []interface{}, idx int) []string {
	if idx >= len(args) {
		return nil
	}
	switch v := args[idx].(type) {
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			result[i], _ = item.(string)
		}
		return result
	case string:
		return []string{v}
	}
	return nil
}

// ==================== /api/chat/stream (SSE) ====================

type streamRequest struct {
	Messages      string `json:"messages"`
	SystemPrompt  string `json:"systemPrompt"`
}

func (s *WebServer) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "Method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, 400, "Bad request")
		return
	}

	var req streamRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, 400, "Bad JSON")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "Streaming not supported")
		return
	}

	// Check AI client
	if s.app.aiClient == nil {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"AI客户端未配置\"}\n\n")
		flusher.Flush()
		return
	}

	// Parse messages
	var parsedMsgs []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(req.Messages), &parsedMsgs); err != nil {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"参数解析失败: %v\"}\n\n", err)
		flusher.Flush()
		return
	}

	// Build ai.ChatMessage slice
	messages := make([]ai.ChatMessage, 0, len(parsedMsgs)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, ai.ChatMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range parsedMsgs {
		messages = append(messages, ai.ChatMessage{Role: m.Role, Content: m.Content})
	}

	// Try real streaming first (skip if provider doesn't support it)
	streamed := false
	if s.app.aiClient.SupportsStreaming() {
		err = s.app.aiClient.ChatStream(messages, func(chunk string) {
			streamed = true
			fmt.Fprintf(w, "event: chunk\ndata: {\"text\":%s}\n\n", mustJSON(chunk))
			flusher.Flush()
		})
	}

	if streamed {
		// At least some chunks were sent — signal done regardless of error
		fmt.Fprintf(w, "event: done\ndata: {}\n\n")
		flusher.Flush()
		if err == nil {
			s.app.SaveAIUsage()
		}
		return
	}

	// No chunks streamed at all — fall back to non-streaming
	if err != nil {
		log.Printf("[SSE] ChatStream failed: %v, falling back to non-streaming", err)
	}

	// Rebuild messages (ChatStream may have modified internal state)
	messages = make([]ai.ChatMessage, 0, len(parsedMsgs)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, ai.ChatMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range parsedMsgs {
		messages = append(messages, ai.ChatMessage{Role: m.Role, Content: m.Content})
	}

	result := s.app.AIChatWithHistoryRaw(messages)

	if result == "" || strings.HasPrefix(result, "AI回复出错") || strings.HasPrefix(result, "AI客户端未初始化") {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":%q}\n\n", result)
		flusher.Flush()
		return
	}

	fmt.Fprintf(w, "event: chunk\ndata: {\"text\":%s}\n\n", mustJSON(result))
	flusher.Flush()
	fmt.Fprintf(w, "event: done\ndata: {\"text\":%s}\n\n", mustJSON(result))
	flusher.Flush()
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ==================== Static File Server ====================

func (s *WebServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Serve embedded frontend dist
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		data, err := assets.ReadFile("frontend/dist/index.html")
		if err != nil {
			http.Error(w, "Not found", 404)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
		return
	}

	// Try to serve the requested file
	path := "frontend/dist" + r.URL.Path
	data, err := assets.ReadFile(path)
	if err != nil {
		// SPA fallback: serve index.html for non-file routes
		data, err = assets.ReadFile("frontend/dist/index.html")
		if err != nil {
			http.Error(w, "Not found", 404)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
		return
	}

	// Set content type based on extension
	ct := detectContentType(path)
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Write(data)
}

func detectContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".js"):
		return "application/javascript"
	case strings.HasSuffix(path, ".css"):
		return "text/css"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".gif"):
		return "image/gif"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(path, ".json"):
		return "application/json"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(path, ".woff"):
		return "font/woff"
	case strings.HasSuffix(path, ".ttf"):
		return "font/ttf"
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	}
	return ""
}

// EncodeCredentials returns the Base64-encoded Basic auth token
func EncodeCredentials(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}
