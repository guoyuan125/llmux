package types

import "context"

// InternalRequest is the unified internal representation of an LLM request.
// It is the superset of all supported protocols (OpenAI, Anthropic, Gemini).
type InternalRequest struct {
	Model       string         `json:"model"`
	Messages    []Message      `json:"messages,omitempty"`
	Stream      *bool          `json:"stream,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature *float64       `json:"temperature,omitempty"`
	TopP        *float64       `json:"top_p,omitempty"`
	Tools       []Tool         `json:"tools,omitempty"`
	ToolChoice  interface{}    `json:"tool_choice,omitempty"`
	Stop        interface{}    `json:"stop,omitempty"`
	Thinking    *ThinkingConfig `json:"thinking,omitempty"`

	// Embedding-specific
	Input         interface{} `json:"input,omitempty"`
	EncodingFormat string     `json:"encoding_format,omitempty"`

	// Protocol metadata
	SystemPrompt string `json:"system,omitempty"` // Anthropic-style top-level system

	// Internal routing metadata (not serialized to upstream)
	IsEmbedding bool   `json:"-"`
	RequestID   string `json:"-"`
}

// Message represents a single message in the conversation.
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // string or []ContentBlock
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ContentBlock represents a multimodal content block.
type ContentBlock struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *ImageURL    `json:"image_url,omitempty"`
	Source   *ImageSource `json:"source,omitempty"` // Anthropic base64 image
}

type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// Tool represents a function/tool definition.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// ToolCall represents a tool invocation in a response.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ThinkingConfig controls extended thinking (Claude).
type ThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// InternalResponse is the unified internal representation of an LLM response.
type InternalResponse struct {
	ID           string    `json:"id"`
	Model        string    `json:"model"`
	Object       string    `json:"object"`
	Choices      []Choice  `json:"choices,omitempty"`
	Usage        *Usage    `json:"usage,omitempty"`
	FinishReason string    `json:"finish_reason,omitempty"`

	// Streaming
	IsStream bool `json:"-"`
	IsDone   bool `json:"-"`
}

// Choice represents one completion choice.
type Choice struct {
	Index        int      `json:"index"`
	Message      *Message `json:"message,omitempty"`
	Delta        *Message `json:"delta,omitempty"` // streaming
	FinishReason string   `json:"finish_reason,omitempty"`
}

// Usage holds token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
}

// Inbound transforms client requests into internal format and internal responses back to client format.
type Inbound interface {
	TransformRequest(ctx context.Context, body []byte) (*InternalRequest, error)
	TransformResponse(ctx context.Context, resp *InternalResponse) ([]byte, error)
	TransformStream(ctx context.Context, resp *InternalResponse) ([]byte, error)
	GetInternalResponse(ctx context.Context) (*InternalResponse, error)
}

// Outbound transforms internal requests into upstream format and upstream responses back to internal format.
type Outbound interface {
	TransformRequest(ctx context.Context, req *InternalRequest, baseURL, key string) (*OutboundHTTPRequest, error)
	TransformResponse(ctx context.Context, statusCode int, body []byte) (*InternalResponse, error)
	TransformStream(ctx context.Context, eventData []byte) (*InternalResponse, error)
}

// OutboundHTTPRequest wraps the data needed to make an upstream HTTP call.
type OutboundHTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
}
