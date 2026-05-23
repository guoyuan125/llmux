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
			m["content"] = flattenContent(msg.Content)
		} else {
			m["content"] = nil
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
		if msg.ReasoningContent != nil {
			m["reasoning_content"] = msg.ReasoningContent
		}
		messages = append(messages, m)
	}
	sanitizeToolCalls(messages)
	// Filter out nil entries from sanitization
	cleaned := make([]map[string]interface{}, 0, len(messages))
	for _, m := range messages {
		if m != nil {
			cleaned = append(cleaned, m)
		}
	}
	oReq["messages"] = cleaned

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

// flattenContent normalizes msg.Content for OpenAI-compatible providers.
// If content is []types.ContentBlock containing only text blocks, it is
// concatenated into a single string. Providers like DeepSeek reject the
// array-of-objects format that Anthropic uses.
func flattenContent(content interface{}) interface{} {
	switch c := content.(type) {
	case string:
		return c
	case []types.ContentBlock:
		allText := true
		for _, b := range c {
			if b.Type != "text" && b.Type != "" {
				allText = false
				break
			}
		}
		if allText {
			var sb strings.Builder
			for i, b := range c {
				if i > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(b.Text)
			}
			return sb.String()
		}
		// Contains images or other blocks — pass as OpenAI content_parts format
		parts := make([]map[string]interface{}, 0, len(c))
		for _, b := range c {
			switch b.Type {
			case "text", "":
				parts = append(parts, map[string]interface{}{
					"type": "text",
					"text": b.Text,
				})
			case "image_url":
				part := map[string]interface{}{"type": "image_url"}
				if b.ImageURL != nil {
					part["image_url"] = map[string]interface{}{"url": b.ImageURL.URL}
				} else if b.Source != nil {
					dataURI := "data:" + b.Source.MediaType + ";base64," + b.Source.Data
					part["image_url"] = map[string]interface{}{"url": dataURI}
				}
				parts = append(parts, part)
			}
		}
		return parts
	default:
		return content
	}
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
		Role             string      `json:"role"`
		Content          interface{} `json:"content"`
		Name             string      `json:"name,omitempty"`
		ToolCallID       string      `json:"tool_call_id,omitempty"`
		ReasoningContent interface{} `json:"reasoning_content,omitempty"`
		ToolCalls        []struct {
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
		Role:             m.Role,
		Content:          m.Content,
		Name:             m.Name,
		ToolCallID:       m.ToolCallID,
		ReasoningContent: m.ReasoningContent,
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

// sanitizeToolCalls removes tool_calls from assistant messages that lack
// corresponding tool responses, which some providers (e.g. DeepSeek) reject.
// sanitizeToolCalls ensures tool_calls/tool message consistency for the OpenAI outbound path.
//
// This function is only relevant for cross-protocol relay (e.g. Anthropic→OpenAI) where
// message structure differences can produce invalid sequences. Specifically:
//
//   - Anthropic allows user messages between tool_use and tool_result (e.g. system-reminders).
//     After conversion to OpenAI format, this becomes assistant(tool_calls) → user → tool,
//     which violates the OpenAI/DeepSeek requirement that tool messages immediately follow
//     the assistant message with tool_calls.
//
//   - Anthropic conversations may include orphaned tool_calls (assistant declared tool_calls
//     but the conversation was truncated before tool responses arrived).
//
// This function reorders messages so tool responses are placed immediately after their
// corresponding assistant(tool_calls) message, and strips tool_calls that have no responses.
//
// NOTE: For same-protocol passthrough (Anthropic→Anthropic), this function is NOT called.
// The passthrough path forwards the raw request body without any transformation, which is
// the preferred approach when the upstream provider supports the client's native protocol.
func sanitizeToolCalls(messages []map[string]interface{}) {
	// Index tool response messages by tool_call_id
	toolResps := make(map[string]map[string]interface{})
	for _, m := range messages {
		if m["role"] == "tool" {
			if id, ok := m["tool_call_id"].(string); ok {
				toolResps[id] = m
			}
		}
	}

	// Rebuild message list: for each assistant with tool_calls, place tool
	// responses immediately after it; skip tool_calls without responses entirely.
	result := make([]map[string]interface{}, 0, len(messages))
	placed := make(map[string]bool) // tool messages already placed

	for _, m := range messages {
		if m["role"] == "tool" {
			// Skip here; will be placed after their assistant message
			continue
		}

		if m["role"] == "assistant" {
			tcs, ok := m["tool_calls"].([]map[string]interface{})
			if ok && len(tcs) > 0 {
				// Check if all tool_calls have responses
				allAnswered := true
				for _, tc := range tcs {
					id, _ := tc["id"].(string)
					if _, exists := toolResps[id]; !exists {
						allAnswered = false
						break
					}
				}
				if !allAnswered {
					// Strip tool_calls, keep message as plain assistant
					delete(m, "tool_calls")
					if m["content"] == nil {
						m["content"] = ""
					}
					result = append(result, m)
				} else {
					// Keep assistant with tool_calls, then place tool responses immediately after
					result = append(result, m)
					for _, tc := range tcs {
						id, _ := tc["id"].(string)
						result = append(result, toolResps[id])
						placed[id] = true
					}
				}
				continue
			}
		}

		result = append(result, m)
	}

	// Copy result back into original slice
	copy(messages, result)
	for i := len(result); i < len(messages); i++ {
		messages[i] = nil
	}
}
