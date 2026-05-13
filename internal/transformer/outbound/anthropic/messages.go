package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/liuguoyuan/llmux/internal/transformer/types"
)

// MessagesOutbound transforms internal requests to Anthropic upstream format.
type MessagesOutbound struct{}

// TransformRequest converts internal request to Anthropic Messages API format.
func (o *MessagesOutbound) TransformRequest(ctx context.Context, req *types.InternalRequest, baseURL, key string) (*types.OutboundHTTPRequest, error) {
	aReq := map[string]interface{}{
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
	}

	if req.MaxTokens == 0 {
		aReq["max_tokens"] = 4096
	}

	// System prompt
	if req.SystemPrompt != "" {
		aReq["system"] = req.SystemPrompt
	}

	// Messages (skip system messages, they go to "system" field)
	messages := make([]map[string]interface{}, 0)
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Already handled via SystemPrompt or extract here
			if req.SystemPrompt == "" {
				if text, ok := msg.Content.(string); ok {
					aReq["system"] = text
				}
			}
			continue
		}

		m := map[string]interface{}{
			"role": msg.Role,
		}

		// Handle tool messages -> convert to user message with tool_result
		if msg.Role == "tool" {
			m["role"] = "user"
			content := []map[string]interface{}{{
				"type":        "tool_result",
				"tool_use_id": msg.ToolCallID,
				"content":     msg.Content,
			}}
			m["content"] = content
			messages = append(messages, m)
			continue
		}

		// Handle assistant messages with tool_calls -> convert to tool_use blocks
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			content := make([]map[string]interface{}, 0)
			if text, ok := msg.Content.(string); ok && text != "" {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": text,
				})
			}
			for _, tc := range msg.ToolCalls {
				var input interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": input,
				})
			}
			m["content"] = content
			messages = append(messages, m)
			continue
		}

		// Regular message
		m["content"] = msg.Content
		messages = append(messages, m)
	}
	aReq["messages"] = messages

	// Optional parameters
	if req.Stream != nil {
		aReq["stream"] = *req.Stream
	}
	if req.Temperature != nil {
		aReq["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		aReq["top_p"] = *req.TopP
	}
	if req.Stop != nil {
		aReq["stop_sequences"] = req.Stop
	}

	// Tools
	if len(req.Tools) > 0 {
		tools := make([]map[string]interface{}, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]interface{}{
				"name":         t.Function.Name,
				"description":  t.Function.Description,
				"input_schema": t.Function.Parameters,
			})
		}
		aReq["tools"] = tools
	}

	// Thinking
	if req.Thinking != nil {
		aReq["thinking"] = map[string]interface{}{
			"type":          req.Thinking.Type,
			"budget_tokens": req.Thinking.BudgetTokens,
		}
	}

	body, err := json.Marshal(aReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic request: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/messages"

	return &types.OutboundHTTPRequest{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"Content-Type":      "application/json",
			"x-api-key":         key,
			"anthropic-version": "2023-06-01",
		},
		Body: body,
	}, nil
}

// TransformResponse converts Anthropic upstream response to internal format.
func (o *MessagesOutbound) TransformResponse(ctx context.Context, statusCode int, body []byte) (*types.InternalResponse, error) {
	var resp struct {
		ID           string `json:"id"`
		Type         string `json:"type"`
		Role         string `json:"role"`
		Model        string `json:"model"`
		Content      []json.RawMessage `json:"content"`
		StopReason   string `json:"stop_reason"`
		Usage        struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			CacheReadInputTokens int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse anthropic response: %w", err)
	}

	ir := &types.InternalResponse{
		ID:    resp.ID,
		Model: resp.Model,
	}

	msg := &types.Message{Role: "assistant"}
	var textContent string

	for _, rawBlock := range resp.Content {
		var block struct {
			Type  string          `json:"type"`
			Text  string          `json:"text,omitempty"`
			ID    string          `json:"id,omitempty"`
			Name  string          `json:"name,omitempty"`
			Input json.RawMessage `json:"input,omitempty"`
		}
		json.Unmarshal(rawBlock, &block)

		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			msg.ToolCalls = append(msg.ToolCalls, types.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: types.ToolCallFunc{
					Name:      block.Name,
					Arguments: string(block.Input),
				},
			})
		}
	}
	msg.Content = textContent

	finishReason := mapStopReason(resp.StopReason)
	ir.Choices = []types.Choice{{
		Index:        0,
		Message:      msg,
		FinishReason: finishReason,
	}}

	ir.Usage = &types.Usage{
		PromptTokens:     resp.Usage.InputTokens,
		CompletionTokens: resp.Usage.OutputTokens,
		TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		CacheReadTokens:  resp.Usage.CacheReadInputTokens,
	}

	return ir, nil
}

// TransformStream parses Anthropic SSE events into internal format.
func (o *MessagesOutbound) TransformStream(ctx context.Context, eventData []byte) (*types.InternalResponse, error) {
	data := strings.TrimSpace(string(eventData))

	var event struct {
		Type  string          `json:"type"`
		Index int             `json:"index,omitempty"`
		Delta json.RawMessage `json:"delta,omitempty"`
		Usage *struct {
			InputTokens  int `json:"input_tokens,omitempty"`
			OutputTokens int `json:"output_tokens,omitempty"`
		} `json:"usage,omitempty"`
		Message *struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		} `json:"message,omitempty"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return nil, fmt.Errorf("failed to parse anthropic stream event: %w", err)
	}

	ir := &types.InternalResponse{IsStream: true}

	switch event.Type {
	case "message_start":
		if event.Message != nil {
			ir.ID = event.Message.ID
			ir.Model = event.Message.Model
			ir.Usage = &types.Usage{
				PromptTokens: event.Message.Usage.InputTokens,
			}
		}
		return ir, nil

	case "content_block_delta":
		var delta struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		}
		json.Unmarshal(event.Delta, &delta)

		if delta.Type == "text_delta" {
			ir.Choices = []types.Choice{{
				Index: 0,
				Delta: &types.Message{
					Role:    "assistant",
					Content: delta.Text,
				},
			}}
		}
		return ir, nil

	case "message_delta":
		var delta struct {
			StopReason string `json:"stop_reason"`
		}
		json.Unmarshal(event.Delta, &delta)

		ir.Choices = []types.Choice{{
			Index:        0,
			FinishReason: mapStopReason(delta.StopReason),
		}}
		if event.Usage != nil {
			ir.Usage = &types.Usage{
				CompletionTokens: event.Usage.OutputTokens,
			}
		}
		return ir, nil

	case "message_stop":
		ir.IsDone = true
		return ir, nil

	case "content_block_start", "content_block_stop", "ping":
		// Skip these events
		return ir, nil
	}

	return ir, nil
}

func mapStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return reason
	}
}
