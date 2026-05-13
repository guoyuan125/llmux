package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/liuguoyuan/llmux/internal/transformer/types"
)

// ChatOutbound transforms internal requests to OpenAI upstream format.
type ChatOutbound struct{}

// TransformRequest converts an internal request to an OpenAI-compatible HTTP request.
func (o *ChatOutbound) TransformRequest(ctx context.Context, req *types.InternalRequest, baseURL, key string) (*types.OutboundHTTPRequest, error) {
	oReq := map[string]interface{}{
		"model": req.Model,
	}

	// Messages
	messages := make([]map[string]interface{}, 0, len(req.Messages))
	for _, msg := range req.Messages {
		m := map[string]interface{}{
			"role": msg.Role,
		}
		if msg.Content != nil {
			m["content"] = msg.Content
		}
		if msg.Name != "" {
			m["name"] = msg.Name
		}
		if msg.ToolCallID != "" {
			m["tool_call_id"] = msg.ToolCallID
		}
		if len(msg.ToolCalls) > 0 {
			tcs := make([]map[string]interface{}, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				tcs = append(tcs, map[string]interface{}{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			m["tool_calls"] = tcs
		}
		messages = append(messages, m)
	}
	oReq["messages"] = messages

	// Optional parameters
	if req.Stream != nil {
		oReq["stream"] = *req.Stream
		if *req.Stream {
			oReq["stream_options"] = map[string]interface{}{
				"include_usage": true,
			}
		}
	}
	if req.MaxTokens > 0 {
		oReq["max_completion_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		oReq["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		oReq["top_p"] = *req.TopP
	}
	if req.Stop != nil {
		oReq["stop"] = req.Stop
	}
	if req.ToolChoice != nil {
		oReq["tool_choice"] = req.ToolChoice
	}

	// Tools
	if len(req.Tools) > 0 {
		tools := make([]map[string]interface{}, 0, len(req.Tools))
		for _, t := range req.Tools {
			tool := map[string]interface{}{
				"type": t.Type,
				"function": map[string]interface{}{
					"name":        t.Function.Name,
					"description": t.Function.Description,
					"parameters":  t.Function.Parameters,
				},
			}
			tools = append(tools, tool)
		}
		oReq["tools"] = tools
	}

	body, err := json.Marshal(oReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal outbound request: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"

	return &types.OutboundHTTPRequest{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + key,
		},
		Body: body,
	}, nil
}

// TransformResponse converts an OpenAI upstream response to internal format.
func (o *ChatOutbound) TransformResponse(ctx context.Context, statusCode int, body []byte) (*types.InternalResponse, error) {
	var resp struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			Index        int    `json:"index"`
			Message      *json.RawMessage `json:"message,omitempty"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
			PromptTokensDetails *struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details,omitempty"`
		} `json:"usage,omitempty"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse upstream response: %w", err)
	}

	ir := &types.InternalResponse{
		ID:    resp.ID,
		Model: resp.Model,
	}

	for _, choice := range resp.Choices {
		ic := types.Choice{
			Index:        choice.Index,
			FinishReason: choice.FinishReason,
		}
		if choice.Message != nil {
			msg, err := parseOpenAIMessage(*choice.Message)
			if err == nil {
				ic.Message = msg
			}
		}
		ir.Choices = append(ir.Choices, ic)
	}

	if resp.Usage != nil {
		ir.Usage = &types.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		if resp.Usage.PromptTokensDetails != nil {
			ir.Usage.CacheReadTokens = resp.Usage.PromptTokensDetails.CachedTokens
		}
	}

	return ir, nil
}

// TransformStream parses a single SSE data line from upstream OpenAI into internal format.
func (o *ChatOutbound) TransformStream(ctx context.Context, eventData []byte) (*types.InternalResponse, error) {
	data := strings.TrimSpace(string(eventData))

	// Handle [DONE] signal
	if data == "[DONE]" {
		return &types.InternalResponse{IsDone: true}, nil
	}

	var chunk struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			Index        int              `json:"index"`
			Delta        *json.RawMessage `json:"delta,omitempty"`
			FinishReason *string          `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage,omitempty"`
	}

	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, fmt.Errorf("failed to parse stream chunk: %w", err)
	}

	ir := &types.InternalResponse{
		ID:       chunk.ID,
		Model:    chunk.Model,
		IsStream: true,
	}

	for _, choice := range chunk.Choices {
		ic := types.Choice{
			Index: choice.Index,
		}
		if choice.FinishReason != nil {
			ic.FinishReason = *choice.FinishReason
		}
		if choice.Delta != nil {
			msg, err := parseOpenAIMessage(*choice.Delta)
			if err == nil {
				ic.Delta = msg
			}
		}
		ir.Choices = append(ir.Choices, ic)
	}

	if chunk.Usage != nil {
		ir.Usage = &types.Usage{
			PromptTokens:     chunk.Usage.PromptTokens,
			CompletionTokens: chunk.Usage.CompletionTokens,
			TotalTokens:      chunk.Usage.TotalTokens,
		}
	}

	return ir, nil
}

// parseOpenAIMessage parses a raw JSON message into internal Message format.
func parseOpenAIMessage(raw json.RawMessage) (*types.Message, error) {
	var m struct {
		Role       string      `json:"role"`
		Content    interface{} `json:"content"`
		Name       string      `json:"name,omitempty"`
		ToolCallID string      `json:"tool_call_id,omitempty"`
		ToolCalls  []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
			Index *int `json:"index,omitempty"`
		} `json:"tool_calls,omitempty"`
	}

	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	msg := &types.Message{
		Role:       m.Role,
		Content:    m.Content,
		Name:       m.Name,
		ToolCallID: m.ToolCallID,
	}

	for _, tc := range m.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, types.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: types.ToolCallFunc{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return msg, nil
}
