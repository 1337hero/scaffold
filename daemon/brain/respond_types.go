package brain

import (
	"context"
	"encoding/json"
)

// ToolDefinition is provider-agnostic tool metadata loaded from config.
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
