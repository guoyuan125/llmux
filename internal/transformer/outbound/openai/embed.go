package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/liuguoyuan/llmux/internal/transformer/types"
)

// EmbedOutbound transforms internal embedding requests to OpenAI /v1/embeddings format.
type EmbedOutbound struct{}

// TransformRequest builds an HTTP request for POST /v1/embeddings.
func (o *EmbedOutbound) TransformRequest(_ context.Context, req *types.InternalRequest, baseURL, key string) (*types.OutboundHTTPRequest, error) {
	body := map[string]interface{}{
		"model": req.Model,
		"input": req.Input,
	}
	if req.EncodingFormat != "" {
		body["encoding_format"] = req.EncodingFormat
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	return &types.OutboundHTTPRequest{
		Method: "POST",
		URL:    strings.TrimRight(baseURL, "/") + "/v1/embeddings",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + key,
		},
		Body: bodyBytes,
	}, nil
}

// TransformResponse parses the upstream embedding response to extract usage for audit logging.
// The raw body is returned to the client unchanged via the passthrough handler.
func (o *EmbedOutbound) TransformResponse(_ context.Context, _ int, body []byte) (*types.InternalResponse, error) {
	var resp struct {
		Model string `json:"model"`
		Usage *struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage,omitempty"`
	}
	// Best-effort parse for usage; relay continues even if parsing fails.
	_ = json.Unmarshal(body, &resp)

	ir := &types.InternalResponse{Model: resp.Model}
	if resp.Usage != nil {
		ir.Usage = &types.Usage{
			PromptTokens: resp.Usage.PromptTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}
	return ir, nil
}

// TransformStream is not applicable for embeddings.
func (o *EmbedOutbound) TransformStream(_ context.Context, _ []byte) (*types.InternalResponse, error) {
	return nil, fmt.Errorf("embeddings do not support streaming")
}
