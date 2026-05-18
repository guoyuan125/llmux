package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/liuguoyuan/llmux/internal/transformer/types"
)

// EmbedInbound handles OpenAI /v1/embeddings client requests.
// Response transformation is handled via passthrough in the gateway;
// this type only needs to parse the request for model extraction and routing.
type EmbedInbound struct {
	storedResp *types.InternalResponse
}

type openaiEmbedRequest struct {
	Model          string      `json:"model"`
	Input          interface{} `json:"input"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
	Dimensions     *int        `json:"dimensions,omitempty"`
	User           string      `json:"user,omitempty"`
}

// TransformRequest extracts model and input from an OpenAI embedding request body.
func (e *EmbedInbound) TransformRequest(_ context.Context, body []byte) (*types.InternalRequest, error) {
	var req openaiEmbedRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid embedding request: %w", err)
	}
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	return &types.InternalRequest{
		Model:          req.Model,
		Input:          req.Input,
		EncodingFormat: req.EncodingFormat,
		IsEmbedding:    true,
	}, nil
}

// TransformResponse is not used for embedding relay (passthrough path is used instead).
func (e *EmbedInbound) TransformResponse(_ context.Context, resp *types.InternalResponse) ([]byte, error) {
	e.storedResp = resp
	return json.Marshal(resp)
}

// TransformStream is not applicable for embeddings.
func (e *EmbedInbound) TransformStream(_ context.Context, _ *types.InternalResponse) ([]byte, error) {
	return nil, fmt.Errorf("embeddings do not support streaming")
}

// GetInternalResponse returns the stored response for token accounting.
func (e *EmbedInbound) GetInternalResponse(_ context.Context) (*types.InternalResponse, error) {
	return e.storedResp, nil
}
