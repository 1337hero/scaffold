package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

type CompletionClient interface {
	CompletionJSON(ctx context.Context, model, systemPrompt, userPrompt string, maxTokens int64) (string, error)
	CompletionText(ctx context.Context, model, systemPrompt, userPrompt string, maxTokens int64) (string, error)
}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
}

type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

type ToolResult struct {
	ToolUseID string
	Content   string
	IsError   bool
}

type RespondMessage struct {
	Role        string
	Text        string
	ToolCalls   []ToolCall
	ToolResults []ToolResult
}

type ToolUseRequest struct {
	SystemPrompt string
	Model        string
	MaxTokens    int
	Tools        []ToolDefinition
	Messages     []RespondMessage
}

type ToolUseResponse struct {
	Text      string
	ToolCalls []ToolCall
}

type ToolUseResponder interface {
	Respond(ctx context.Context, req ToolUseRequest) (*ToolUseResponse, error)
}

type UnconfiguredCompletionClient struct{}

func (c *UnconfiguredCompletionClient) CompletionJSON(_ context.Context, _, _, _ string, _ int64) (string, error) {
	return "", fmt.Errorf("completion client is not configured")
}

func (c *UnconfiguredCompletionClient) CompletionText(_ context.Context, _, _, _ string, _ int64) (string, error) {
	return "", fmt.Errorf("completion client is not configured")
}
