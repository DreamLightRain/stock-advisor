package ai

type ProviderType string

const (
	ProviderOpenAI    ProviderType = "openai"
	ProviderOllama    ProviderType = "ollama"
	ProviderDeepSeek  ProviderType = "deepseek"
	ProviderVolcano   ProviderType = "volcano"
	ProviderOpenCode  ProviderType = "opencode"
)

type Config struct {
	Provider  ProviderType `json:"provider"`
	Endpoint  string       `json:"endpoint"`
	APIKey    string       `json:"apiKey"`
	ModelName string       `json:"modelName"`
	MaxTokens int          `json:"maxTokens"`
	Timeout   int          `json:"timeout"`
}

// ===== OpenAI-compatible types =====

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ChatRequest struct {
	Model    string           `json:"model"`
	Messages []ChatMessage    `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

type ToolResponse struct {
	Content    string `json:"content"`
	Name       string `json:"name"`
	Continue   bool   `json:"continue"`
}

// ===== Volcano Engine specific types =====

type VolcanoContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type VolcanoInputMessage struct {
	Role    string           `json:"role"`
	Content []VolcanoContent `json:"content"`
}

type VolcanoRequest struct {
	Model  string                `json:"model"`
	Input  []VolcanoInputMessage `json:"input"`
	Stream bool                  `json:"stream"`
}

type VolcanoResponse struct {
	ID     string              `json:"id"`
	Output []VolcanoOutputItem `json:"output"`
	Usage  *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type VolcanoOutputItem struct {
	Type    string           `json:"type"`
	Role    string           `json:"role,omitempty"`
	Content []VolcanoContent `json:"content,omitempty"`
}

type AIProvider interface {
	Name() string
	Chat(messages []ChatMessage) (string, error)
	ChatWithTools(messages []ChatMessage, tools []ToolDefinition) (*ChatResponse, error)
	AnalyzeStock(stockName, stockCode, stockInfo, klineData string) (string, error)
}
