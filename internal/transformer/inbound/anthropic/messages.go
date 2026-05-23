package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/liuguoyuan/llmux/internal/transformer/types"
)

// MessagesInbound handles Anthropic /v1/messages client requests.
type MessagesInbound struct {
	mu            sync.Mutex
	streamChunks  []*types.InternalResponse
	nonStreamResp *types.InternalResponse
}

// anthropicRequest represents an Anthropic Messages API request.
type anthropicRequest struct {
	Model       string              `json:"model"`
	Messages    []anthropicMessage  `json:"messages"`
	System      interface{}         `json:"system,omitempty"` // string or []systemBlock
	MaxTokens   int                 `json:"max_tokens"`
	Stream      *bool               `json:"stream,omitempty"`
	Temperature *float64            `json:"temperature,omitempty"`
	TopP        *float64            `json:"top_p,omitempty"`
	TopK        *int                `json:"top_k,omitempty"`
	StopSequences []string          `json:"stop_sequences,omitempty"`
	Tools       []anthropicTool     `json:"tools,omitempty"`
	ToolChoice  interface{}         `json:"tool_choice,omitempty"`
	Thinking    *anthropicThinking  `json:"thinking,omitempty"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []contentBlock
}

type anthropicContentBlock struct {
	Type      string       `json:"type"`
	Text      string       `json:"text,omitempty"`
	Source    *imageSource `json:"source,omitempty"`
	ID        string       `json:"id,omitempty"`
	Name      string       `json:"name,omitempty"`
	Input     interface{}  `json:"input,omitempty"`
	ToolUseID string       `json:"tool_use_id,omitempty"`
	Content   interface{}  `json:"content,omitempty"`
	Thinking  string       `json:"thinking,omitempty"`
}

type imageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema"`
}

type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// anthropicResponse represents an Anthropic Messages API response.
type anthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence"`
	Usage        anthropicUsage          `json:"usage"`
}

type anthropicUsage struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	CacheReadTokens   int `json:"cache_read_input_tokens,omitempty"`
	CacheCreatedTokens int `json:"cache_creation_input_tokens,omitempty"`
}

// TransformRequest converts an Anthropic messages request to internal format.
func (a *MessagesInbound) TransformRequest(ctx context.Context, body []byte) (*types.InternalRequest, error) {
	var req anthropicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid Anthropic request: %w", err)
	}

	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	ir := &types.InternalRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	if req.StopSequences != nil {
		ir.Stop = req.StopSequences
	}

	// System prompt
	switch s := req.System.(type) {
	case string:
		ir.SystemPrompt = s
		// Prepend as system message for internal format
		ir.Messages = append(ir.Messages, types.Message{
			Role:    "system",
			Content: s,
		})
	case []interface{}:
		// Array of system blocks (with cache_control etc.)
		var text string
		for _, block := range s {
			if m, ok := block.(map[string]interface{}); ok {
				if t, ok := m["text"].(string); ok {
					text += t
				}
			}
		}
		if text != "" {
			ir.SystemPrompt = text
			ir.Messages = append(ir.Messages, types.Message{
				Role:    "system",
				Content: text,
			})
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		switch c := msg.Content.(type) {
		case string:
			ir.Messages = append(ir.Messages, types.Message{
				Role:    msg.Role,
				Content: c,
			})
		case []interface{}:
			// Parse content blocks. Anthropic packs tool_use, tool_result, text,
			// and image blocks into a single message. OpenAI requires:
			//   - tool_use blocks → single assistant message with tool_calls
			//   - tool_result blocks → one role:"tool" message per result
			//   - text/image blocks → one user/assistant message
			var textParts []string
			var contentBlocks []types.ContentBlock
			var toolCalls []types.ToolCall
			var toolResults []types.Message
			var thinkingText string
			hasNonText := false

			for _, item := range c {
				data, _ := json.Marshal(item)
				var block anthropicContentBlock
				json.Unmarshal(data, &block)

				switch block.Type {
				case "tool_use":
					args := ""
					if block.Input != nil {
						argsData, _ := json.Marshal(block.Input)
						args = string(argsData)
					}
					toolCalls = append(toolCalls, types.ToolCall{
						ID:   block.ID,
						Type: "function",
						Function: types.ToolCallFunc{
							Name:      block.Name,
							Arguments: args,
						},
					})
				case "tool_result":
					tm := types.Message{
						Role:       "tool",
						ToolCallID: block.ToolUseID,
					}
					if text, ok := block.Content.(string); ok {
						tm.Content = text
					} else if block.Content != nil {
						// content can be array of content blocks
						contentData, _ := json.Marshal(block.Content)
						tm.Content = string(contentData)
					} else {
						tm.Content = ""
					}
					toolResults = append(toolResults, tm)
				case "text":
					textParts = append(textParts, block.Text)
					contentBlocks = append(contentBlocks, types.ContentBlock{
						Type: "text",
						Text: block.Text,
					})
				case "thinking":
					// Store thinking content to pass as reasoning_content
					thinkingText = block.Thinking
				case "image":
					hasNonText = true
					if block.Source != nil {
						contentBlocks = append(contentBlocks, types.ContentBlock{
							Type: "image_url",
							Source: &types.ImageSource{
								Type:      block.Source.Type,
								MediaType: block.Source.MediaType,
								Data:      block.Source.Data,
							},
						})
					}
				}
			}

			// Emit assistant message with tool_calls (may also include text)
			if len(toolCalls) > 0 {
				am := types.Message{
					Role:      "assistant",
					ToolCalls: toolCalls,
				}
				if len(textParts) > 0 {
					am.Content = strings.Join(textParts, "\n")
				}
				if thinkingText != "" {
					am.ReasoningContent = thinkingText
				}
				ir.Messages = append(ir.Messages, am)
			} else if len(contentBlocks) > 0 {
				// Pure text/image message
				im := types.Message{Role: msg.Role}
				if hasNonText {
					im.Content = contentBlocks
				} else if len(textParts) > 0 {
					im.Content = strings.Join(textParts, "\n")
				}
				if thinkingText != "" {
					im.ReasoningContent = thinkingText
				}
				ir.Messages = append(ir.Messages, im)
			} else if len(toolResults) == 0 {
				// Empty message, still emit to preserve structure
				ir.Messages = append(ir.Messages, types.Message{Role: msg.Role})
			}

			// Emit each tool_result as its own message
			for _, tm := range toolResults {
				ir.Messages = append(ir.Messages, tm)
			}
		default:
			ir.Messages = append(ir.Messages, types.Message{Role: msg.Role})
		}
	}

	// Convert tools
	for _, t := range req.Tools {
		ir.Tools = append(ir.Tools, types.Tool{
			Type: "function",
			Function: types.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	// Thinking
	if req.Thinking != nil {
		ir.Thinking = &types.ThinkingConfig{
			Type:         req.Thinking.Type,
			BudgetTokens: req.Thinking.BudgetTokens,
		}
	}

	return ir, nil
}

// TransformResponse converts internal response to Anthropic format.
func (a *MessagesInbound) TransformResponse(ctx context.Context, resp *types.InternalResponse) ([]byte, error) {
	a.mu.Lock()
	a.nonStreamResp = resp
	a.mu.Unlock()

	aResp := anthropicResponse{
		ID:    resp.ID,
		Type:  "message",
		Role:  "assistant",
		Model: resp.Model,
	}

	for _, choice := range resp.Choices {
		if choice.Message != nil {
			// Convert reasoning_content to thinking block
			if choice.Message.ReasoningContent != nil {
				if text, ok := choice.Message.ReasoningContent.(string); ok && text != "" {
					aResp.Content = append(aResp.Content, anthropicContentBlock{
						Type:     "thinking",
						Thinking: text,
					})
				}
			}
			// Convert content
			if text, ok := choice.Message.Content.(string); ok && text != "" {
				aResp.Content = append(aResp.Content, anthropicContentBlock{
					Type: "text",
					Text: text,
				})
			}
			// Convert tool calls
			for _, tc := range choice.Message.ToolCalls {
				var input interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				aResp.Content = append(aResp.Content, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
		}
		if choice.FinishReason != "" {
			aResp.StopReason = mapFinishReason(choice.FinishReason)
		}
	}

	if resp.Usage != nil {
		aResp.Usage = anthropicUsage{
			InputTokens:      resp.Usage.PromptTokens,
			OutputTokens:     resp.Usage.CompletionTokens,
			CacheReadTokens:  resp.Usage.CacheReadTokens,
		}
	}

	return json.Marshal(aResp)
}

// TransformStream converts internal streaming response to Anthropic SSE format.
func (a *MessagesInbound) TransformStream(ctx context.Context, resp *types.InternalResponse) ([]byte, error) {
	a.mu.Lock()
	a.streamChunks = append(a.streamChunks, resp)
	a.mu.Unlock()

	if resp.IsDone {
		// Send message_stop event
		event := map[string]interface{}{
			"type": "message_stop",
		}
		data, _ := json.Marshal(event)
		return formatSSE("message_stop", data), nil
	}

	// For streaming, Anthropic uses different event types
	for _, choice := range resp.Choices {
		if choice.Delta != nil {
			// Handle reasoning_content as thinking block
			if choice.Delta.ReasoningContent != nil {
				if text, ok := choice.Delta.ReasoningContent.(string); ok && text != "" {
					event := map[string]interface{}{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]interface{}{
							"type":     "thinking_delta",
							"thinking": text,
						},
					}
					data, _ := json.Marshal(event)
					return formatSSE("content_block_delta", data), nil
				}
			}
			if text, ok := choice.Delta.Content.(string); ok && text != "" {
				event := map[string]interface{}{
					"type": "content_block_delta",
					"index": choice.Index,
					"delta": map[string]interface{}{
						"type": "text_delta",
						"text": text,
					},
				}
				data, _ := json.Marshal(event)
				return formatSSE("content_block_delta", data), nil
			}
		}
		if choice.FinishReason != "" {
			event := map[string]interface{}{
				"type":          "message_delta",
				"delta":         map[string]interface{}{"stop_reason": mapFinishReason(choice.FinishReason)},
			}
			if resp.Usage != nil {
				event["usage"] = map[string]interface{}{"output_tokens": resp.Usage.CompletionTokens}
			}
			data, _ := json.Marshal(event)
			return formatSSE("message_delta", data), nil
		}
	}

	return nil, nil
}

// GetInternalResponse returns aggregated response.
func (a *MessagesInbound) GetInternalResponse(ctx context.Context) (*types.InternalResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.nonStreamResp != nil {
		return a.nonStreamResp, nil
	}

	if len(a.streamChunks) == 0 {
		return nil, nil
	}

	aggregated := &types.InternalResponse{
		ID:    a.streamChunks[0].ID,
		Model: a.streamChunks[0].Model,
	}

	var content string
	var finishReason string
	for _, chunk := range a.streamChunks {
		for _, choice := range chunk.Choices {
			if choice.Delta != nil {
				if text, ok := choice.Delta.Content.(string); ok {
					content += text
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

	aggregated.Choices = []types.Choice{{
		Index:        0,
		FinishReason: finishReason,
		Message: &types.Message{
			Role:    "assistant",
			Content: content,
		},
	}}

	return aggregated, nil
}

func mapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return reason
	}
}

func formatSSE(eventType string, data []byte) []byte {
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(data)))
}

// Used only to suppress unused import in time package
var _ = time.Now
