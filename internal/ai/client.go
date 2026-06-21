package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	config    Config
	httpCli   *http.Client
	lastUsage *Usage
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (c *Client) LastUsage() *Usage {
	return c.lastUsage
}

func (c *Client) ResetUsage() {
	c.lastUsage = nil
}

func NewClient(config Config) *Client {
	config.Endpoint = strings.TrimSpace(config.Endpoint)
	config.APIKey = strings.TrimSpace(config.APIKey)
	config.ModelName = strings.TrimSpace(config.ModelName)
	if config.Timeout <= 0 {
		config.Timeout = 120
	}
	return &Client{
		config: config,
		httpCli: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}
}

func (c *Client) Name() string {
	return string(c.config.Provider)
}

func (c *Client) Config() Config {
	return c.config
}

// SupportsStreaming returns false for providers that don't support SSE streaming
func (c *Client) SupportsStreaming() bool {
	return c.config.Provider != ProviderVolcano
}

func (c *Client) Chat(messages []ChatMessage) (string, error) {
	c.lastUsage = nil
	if c.config.Provider == ProviderVolcano {
		return c.chatVolcano(messages)
	}
	endpoint := c.getEndpoint()
	req := ChatRequest{
		Model:    c.config.ModelName,
		Messages: messages,
		Stream:   false,
	}
	body, _ := json.Marshal(req)
	respBody, err := c.doPost(endpoint, body)
	if err != nil {
		return "", err
	}

	// parse usage from response
	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err == nil && chatResp.Usage != nil {
		c.lastUsage = &Usage{
			PromptTokens:     chatResp.Usage.PromptTokens,
			CompletionTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:      chatResp.Usage.TotalTokens,
		}
	}

	return c.parseOpenAIResponse(respBody)
}

func (c *Client) ChatStream(messages []ChatMessage, onChunk func(string)) error {
	if c.config.Provider == ProviderVolcano {
		return fmt.Errorf("火山引擎暂不支持流式输出")
	}
	endpoint := c.getEndpoint()
	req := ChatRequest{
		Model:    c.config.ModelName,
		Messages: messages,
		Stream:   true,
	}
	body, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	resp, err := c.httpCli.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return nil
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *Usage `json:"usage,omitempty"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			c.lastUsage = &Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}
		if len(chunk.Choices) > 0 {
			onChunk(chunk.Choices[0].Delta.Content)
			if chunk.Choices[0].FinishReason != nil {
				return nil
			}
		}
	}
	return scanner.Err()
}

func (c *Client) ChatWithTools(messages []ChatMessage, tools []ToolDefinition) (*ChatResponse, error) {
	c.lastUsage = nil
	if c.config.Provider == ProviderVolcano {
		return nil, fmt.Errorf("火山引擎暂不支持工具调用")
	}
	endpoint := c.getEndpoint()
	req := ChatRequest{
		Model:    c.config.ModelName,
		Messages: messages,
		Stream:   false,
		Tools:    tools,
	}
	body, _ := json.Marshal(req)
	respBody, err := c.doPost(endpoint, body)
	if err != nil {
		return nil, err
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		// try Ollama format
		var ollamaResp struct {
			Message struct {
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
			Error string `json:"error"`
		}
		if err2 := json.Unmarshal(respBody, &ollamaResp); err2 == nil {
			if ollamaResp.Error != "" {
				return nil, fmt.Errorf("Ollama error: %s", ollamaResp.Error)
			}
			return &ChatResponse{
				Choices: []struct {
					Message struct {
						Role      string     `json:"role"`
						Content   string     `json:"content"`
						ToolCalls []ToolCall `json:"tool_calls,omitempty"`
					} `json:"message"`
				}{{Message: struct {
					Role      string     `json:"role"`
					Content   string     `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls,omitempty"`
				}{Role: "assistant", Content: ollamaResp.Message.Content, ToolCalls: ollamaResp.Message.ToolCalls}}},
			}, nil
		}
		return nil, fmt.Errorf("parse error: %w (body: %s)", err, string(respBody))
	}

	if chatResp.Usage != nil {
		c.lastUsage = &Usage{
			PromptTokens:     chatResp.Usage.PromptTokens,
			CompletionTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:      chatResp.Usage.TotalTokens,
		}
	}

	if chatResp.Error.Message != "" {
		return nil, fmt.Errorf("API error: %s", chatResp.Error.Message)
	}
	return &chatResp, nil
}

func (c *Client) parseOpenAIResponse(respBody []byte) (string, error) {
	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err == nil {
		if chatResp.Error.Message != "" {
			return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
		}
		if len(chatResp.Choices) > 0 {
			return chatResp.Choices[0].Message.Content, nil
		}
	}

	var ollamaResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &ollamaResp); err == nil {
		if ollamaResp.Error != "" {
			return "", fmt.Errorf("Ollama error: %s", ollamaResp.Error)
		}
		if ollamaResp.Message.Content != "" {
			return ollamaResp.Message.Content, nil
		}
	}

	return "", fmt.Errorf("no response choices (body: %s)", string(respBody))
}

func (c *Client) chatVolcano(messages []ChatMessage) (string, error) {
	endpoint := c.getEndpoint()
	volcInput := make([]VolcanoInputMessage, len(messages))
	for i, msg := range messages {
		if msg.Role == "tool" {
			volcInput[i] = VolcanoInputMessage{
				Role: "user",
				Content: []VolcanoContent{
					{Type: "input_text", Text: fmt.Sprintf("工具 [%s] 返回结果: %s", msg.ToolCallID, msg.Content)},
				},
			}
			continue
		}
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			var parts []string
			for _, tc := range msg.ToolCalls {
				parts = append(parts, fmt.Sprintf("调用工具: %s(%s)", tc.Function.Name, tc.Function.Arguments))
			}
			volcInput[i] = VolcanoInputMessage{
				Role: "assistant",
				Content: []VolcanoContent{
					{Type: "input_text", Text: strings.Join(parts, "\n") + "\n\n等待工具返回结果..."},
				},
			}
			continue
		}
		volcInput[i] = VolcanoInputMessage{
			Role: msg.Role,
			Content: []VolcanoContent{
				{Type: "input_text", Text: msg.Content},
			},
		}
	}

	req := VolcanoRequest{
		Model:  c.config.ModelName,
		Input:  volcInput,
		Stream: false,
	}
	body, _ := json.Marshal(req)
	respBody, err := c.doPost(endpoint, body)
	if err != nil {
		return "", err
	}

	var volcResp VolcanoResponse
	if err := json.Unmarshal(respBody, &volcResp); err != nil {
		return "", fmt.Errorf("parse error: %w (body: %s)", err, string(respBody))
	}
	if volcResp.Error != nil {
		return "", fmt.Errorf("API error: %s - %s", volcResp.Error.Code, volcResp.Error.Message)
	}
	for _, item := range volcResp.Output {
		if item.Type == "message" && len(item.Content) > 0 {
			return item.Content[0].Text, nil
		}
	}
	if len(volcResp.Output) > 0 && len(volcResp.Output[0].Content) > 0 {
		return volcResp.Output[0].Content[0].Text, nil
	}
	var altResp struct {
		Output struct{ Text string } `json:"output"`
	}
	if err := json.Unmarshal(respBody, &altResp); err == nil && altResp.Output.Text != "" {
		return altResp.Output.Text, nil
	}
	return "", fmt.Errorf("no response output (body: %s)", string(respBody))
}

func (c *Client) doPost(url string, body []byte) ([]byte, error) {
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}
	resp, err := c.httpCli.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func (c *Client) AnalyzeStock(stockName, stockCode, stockInfo, klineSummary string) (string, error) {
	systemPrompt := `你是一名专业的股票分析师，精通A股市场技术分析和基本面分析。
请基于提供的股票数据，给出专业的买卖建议。分析应包括：
1. 短期趋势判断（1-5个交易日）
2. 中期趋势判断（1-3个月）
3. 关键支撑位和压力位
4. 具体的买卖建议（买入/持有/卖出/观望）
5. 风险提示

请用中文回答，保持客观理性，不要保证收益。`

	userPrompt := fmt.Sprintf(`请分析以下股票：

股票名称：%s
股票代码：%s

当前行情（实时数据）：
%s

K线技术数据（最近交易日）：
%s

请给出详细的技术分析和操作建议。`, stockName, stockCode, stockInfo, klineSummary)

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	return c.Chat(messages)
}

func (c *Client) getEndpoint() string {
	ep := strings.TrimSpace(c.config.Endpoint)
	ep = strings.TrimRight(ep, "/")
	switch c.config.Provider {
	case ProviderOpenAI:
		if ep != "" {
			return ep + "/v1/chat/completions"
		}
		return "https://api.openai.com/v1/chat/completions"
	case ProviderOllama:
		if ep == "" {
			ep = "http://localhost:11434"
		}
		if strings.Contains(ep, "/v1/chat") || strings.Contains(ep, "/api/chat") {
			return ep
		}
		if strings.HasSuffix(ep, "/v1") {
			return ep + "/chat/completions"
		}
		return ep + "/v1/chat/completions"
	case ProviderDeepSeek:
		if ep != "" {
			return ep + "/v1/chat/completions"
		}
		return "https://api.deepseek.com/v1/chat/completions"
	case ProviderVolcano:
		if ep != "" {
			return ep + "/api/v3/responses"
		}
		return "https://ark.cn-beijing.volces.com/api/v3/responses"
	case ProviderOpenCode:
		if ep != "" {
			return ep + "/v1/chat/completions"
		}
		return "https://opencode.ai/zen/v1/chat/completions"
	default:
		if ep != "" {
			return ep + "/v1/chat/completions"
		}
		return "https://api.openai.com/v1/chat/completions"
	}
}

func (c *Client) UpdateConfig(config Config) {
	config.Endpoint = strings.TrimSpace(config.Endpoint)
	config.APIKey = strings.TrimSpace(config.APIKey)
	c.config = config
}
