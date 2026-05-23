package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/liuguoyuan/llmux/internal/transformer/types"
)

// ChatInbound handles OpenAI /v1/chat/completions client requests.
type ChatInbound struct {
	mu             sync.Mutex
	streamChunks   []*types.InternalResponse
	nonStreamResp  *types.InternalResponse
}

// openaiChatRequest represents the OpenAI Chat Completions request format.
type openaiChatRequest struct {
	Model            string             `json:"model"`
	Messages         []openaiMessage    `json:"messages"`
	Stream           *bool              `json:"stream,omitempty"`
	StreamOptions    *streamOptions     `json:"stream_options,omitempty"`
	MaxTokens        *int               `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	Temperature      *float64           `json:"temperature,omitempty"`
	TopP             *float64           `json:"top_p,omitempty"`
	N                *int               `json:"n,omitempty"`
	Stop             interface{}        `json:"stop,omitempty"`
	Tools            []openaiTool       `json:"tools,omitempty"`
	ToolChoice       interface{}        `json:"tool_choice,omitempty"`
	ResponseFormat   interface{}        `json:"response_format,omitempty"`
	Seed             *int               `json:"seed,omitempty"`
	FrequencyPenalty *float64           `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64           `json:"presence_penalty,omitempty"`
	LogProbs         *bool              `json:"logprobs,omitempty"`
	TopLogProbs      *int               `json:"top_logprobs,omitempty"`
	User             string             `json:"user,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type openaiMessage struct {
	Role             string      `json:"role"`
	Content          interface{} `json:"content"` // string or array
	Name             string      `json:"name,omitempty"`
	ToolCalls        []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string      `json:"tool_call_id,omitempty"`
	ReasoningContent interface{} `json:"reasoning_content,omitempty"`
}

type openaiTool struct {
	Type     string           `json:"type"`
	Function openaiToolFunc   `json:"function"`
}

type openaiToolFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Strict      *bool       `json:"strict,omitempty"`
}

type openaiToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openaiToolCallFunc `json:"function"`
}

type openaiToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openaiChatResponse represents OpenAI Chat Completions response.
type openaiChatResponse struct {
	ID                string           `json:"id"`
	Object            string           `json:"object"`
	Created           int64            `json:"created"`
	Model             string           `json:"model"`
	Choices           []openaiChoice   `json:"choices"`
	Usage             *openaiUsage     `json:"usage,omitempty"`
	SystemFingerprint string           `json:"system_fingerprint,omitempty"`
}

type openaiChoice struct {
	Index        int           `json:"index"`
	Message      *openaiMessage `json:"message,omitempty"`
	Delta        *openaiMessage `json:"delta,omitempty"`
	FinishReason *string       `json:"finish_reason"`
	Logprobs     interface{}   `json:"logprobs,omitempty"`
}

type openaiUsage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	PromptTokensDetails *promptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

type promptTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

// openaiStreamChunk represents a single SSE chunk in streaming response.
type openaiStreamChunk struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []openaiChoice `json:"choices"`
	Usage             *openaiUsage   `json:"usage,omitempty"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
}

// TransformRequest converts an OpenAI chat completions request body into internal format.
func (a *ChatInbound) TransformRequest(ctx context.Context, body []byte) (*types.InternalRequest, error) {
	var req openaiChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid OpenAI request: %w", err)
	}

	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	ir := &types.InternalRequest{
		Model:       req.Model,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.Stop,
		ToolChoice:  req.ToolChoice,
	}

	// Max tokens: prefer max_completion_tokens over max_tokens
	if req.MaxCompletionTokens != nil {
		ir.MaxTokens = *req.MaxCompletionTokens
	} else if req.MaxTokens != nil {
		ir.MaxTokens = *req.MaxTokens
	}

	// Convert messages
	for _, msg := range req.Messages {
		im := types.Message{
			Role:             msg.Role,
			Name:             msg.Name,
			ToolCallID:       msg.ToolCallID,
			ReasoningContent: msg.ReasoningContent,
		}

		// Content can be string or array of content blocks
		switch c := msg.Content.(type) {
		case string:
			im.Content = c
		case []interface{}:
			blocks := make([]types.ContentBlock, 0, len(c))
			for _, item := range c {
				block, err := parseContentBlock(item)
				if err != nil {
					continue
				}
				blocks = append(blocks, block)
			}
			im.Content = blocks
		default:
			im.Content = msg.Content
		}

		// Tool calls
		for _, tc := range msg.ToolCalls {
			im.ToolCalls = append(im.ToolCalls, types.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: types.ToolCallFunc{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}

		ir.Messages = append(ir.Messages, im)
	}

	// Convert tools
	for _, t := range req.Tools {
		ir.Tools = append(ir.Tools, types.Tool{
			Type: t.Type,
			Function: types.ToolFunction{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		})
	}

	return ir, nil
}

// TransformResponse converts an internal response back to OpenAI format for the client.
func (a *ChatInbound) TransformResponse(ctx context.Context, resp *types.InternalResponse) ([]byte, error) {
	a.mu.Lock()
	a.nonStreamResp = resp
	a.mu.Unlock()

	oResp := openaiChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
	}

	for _, choice := range resp.Choices {
		oc := openaiChoice{
			Index:        choice.Index,
			FinishReason: strPtr(choice.FinishReason),
		}
		if choice.Message != nil {
			oc.Message = internalMsgToOpenAI(choice.Message)
		}
		oResp.Choices = append(oResp.Choices, oc)
	}

	if resp.Usage != nil {
		oResp.Usage = &openaiUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		if resp.Usage.CacheReadTokens > 0 {
			oResp.Usage.PromptTokensDetails = &promptTokensDetails{
				CachedTokens: resp.Usage.CacheReadTokens,
			}
		}
	}

	return json.Marshal(oResp)
}

// TransformStream converts an internal streaming response chunk to OpenAI SSE format.
func (a *ChatInbound) TransformStream(ctx context.Context, resp *types.InternalResponse) ([]byte, error) {
	a.mu.Lock()
	a.streamChunks = append(a.streamChunks, resp)
	a.mu.Unlock()

	if resp.IsDone {
		return []byte("data: [DONE]\n\n"), nil
	}

	chunk := openaiStreamChunk{
		ID:      resp.ID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   resp.Model,
	}

	for _, choice := range resp.Choices {
		oc := openaiChoice{
			Index: choice.Index,
		}
		if choice.FinishReason != "" {
			oc.FinishReason = strPtr(choice.FinishReason)
		}
		if choice.Delta != nil {
			oc.Delta = internalMsgToOpenAI(choice.Delta)
		}
		chunk.Choices = append(chunk.Choices, oc)
	}

	if resp.Usage != nil {
		chunk.Usage = &openaiUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		return nil, err
	}

	return append([]byte("data: "), append(data, []byte("\n\n")...)...), nil
}

// GetInternalResponse aggregates streamed chunks or returns the stored non-stream response.
func (a *ChatInbound) GetInternalResponse(ctx context.Context) (*types.InternalResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.nonStreamResp != nil {
		return a.nonStreamResp, nil
	}

	// Aggregate stream chunks into a single response
	if len(a.streamChunks) == 0 {
		return nil, nil
	}

	aggregated := &types.InternalResponse{
		ID:    a.streamChunks[0].ID,
		Model: a.streamChunks[0].Model,
	}

	// Collect all content from delta messages
	contentByChoice := make(map[int]string)
	toolCallsByChoice := make(map[int][]types.ToolCall)
	var finishReason string

	for _, chunk := range a.streamChunks {
		for _, choice := range chunk.Choices {
			if choice.Delta != nil {
				if text, ok := choice.Delta.Content.(string); ok {
					contentByChoice[choice.Index] += text
				}
				for _, tc := range choice.Delta.ToolCalls {
					existing := toolCallsByChoice[choice.Index]
					// Merge tool call fragments
					merged := false
					for i := range existing {
						if existing[i].ID == tc.ID || (existing[i].ID == "" && tc.ID != "") {
							existing[i].Function.Arguments += tc.Function.Arguments
							if tc.ID != "" {
								existing[i].ID = tc.ID
							}
							if tc.Function.Name != "" {
								existing[i].Function.Name = tc.Function.Name
							}
							merged = true
							break
						}
					}
					if !merged {
						existing = append(existing, tc)
					}
					toolCallsByChoice[choice.Index] = existing
				}
			}
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
		}
		if chunk.Usage != nil {
			aggregated.Usage = chunk.Usage
		}
	}

	for idx, content := range contentByChoice {
		choice := types.Choice{
			Index:        idx,
			FinishReason: finishReason,
			Message: &types.Message{
				Role:    "assistant",
				Content: content,
			},
		}
		if tcs, ok := toolCallsByChoice[idx]; ok && len(tcs) > 0 {
			choice.Message.ToolCalls = tcs
		}
		aggregated.Choices = append(aggregated.Choices, choice)
	}

	return aggregated, nil
}

// parseContentBlock parses a single content block from the request.
func parseContentBlock(item interface{}) (types.ContentBlock, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return types.ContentBlock{}, err
	}
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	block := types.ContentBlock{}
	if t, ok := raw["type"].(string); ok {
		block.Type = t
	}

	switch block.Type {
	case "text":
		if text, ok := raw["text"].(string); ok {
			block.Text = text
		}
	case "image_url":
		if iu, ok := raw["image_url"].(map[string]interface{}); ok {
			block.ImageURL = &types.ImageURL{}
			if url, ok := iu["url"].(string); ok {
				block.ImageURL.URL = url
			}
			if detail, ok := iu["detail"].(string); ok {
				block.ImageURL.Detail = detail
			}
		}
	}

	return block, nil
}

func internalMsgToOpenAI(msg *types.Message) *openaiMessage {
	om := &openaiMessage{
		Role:             msg.Role,
		Content:          msg.Content,
		Name:             msg.Name,
		ToolCallID:       msg.ToolCallID,
		ReasoningContent: msg.ReasoningContent,
	}
	for _, tc := range msg.ToolCalls {
		om.ToolCalls = append(om.ToolCalls, openaiToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: openaiToolCallFunc{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return om
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
