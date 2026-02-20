package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

type anthropicResponder struct {
	client anthropic.Client
}

func newAnthropicResponder(client anthropic.Client) ToolUseResponder {
	return &anthropicResponder{client: client}
}

func (r *anthropicResponder) Respond(ctx context.Context, req ToolUseRequest) (*ToolUseResponse, error) {
	messages, err := toAnthropicMessages(req.Messages)
	if err != nil {
		return nil, err
	}
	tools, err := toAnthropicTools(req.Tools)
	if err != nil {
		return nil, err
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		MaxTokens: int64(req.MaxTokens),
		Messages:  messages,
		Tools:     tools,
	}
	if system := strings.TrimSpace(req.SystemPrompt); system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}

	resp, err := r.client.Messages.New(ctx, params)
	if err != nil {
		return nil, err
	}

	out := &ToolUseResponse{}
	textParts := make([]string, 0)
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if text := strings.TrimSpace(block.Text); text != "" {
				textParts = append(textParts, text)
			}
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: append(json.RawMessage(nil), block.Input...),
			})
		}
	}
	out.Text = strings.TrimSpace(strings.Join(textParts, "\n"))

	return out, nil
}

func toAnthropicMessages(messages []RespondMessage) ([]anthropic.MessageParam, error) {
	out := make([]anthropic.MessageParam, 0, len(messages))
	for _, message := range messages {
		blocks := make([]anthropic.ContentBlockParamUnion, 0, 1+len(message.ToolCalls)+len(message.ToolResults))

		if text := strings.TrimSpace(message.Text); text != "" {
			blocks = append(blocks, anthropic.NewTextBlock(text))
		}

		for _, toolCall := range message.ToolCalls {
			if strings.TrimSpace(toolCall.ID) == "" || strings.TrimSpace(toolCall.Name) == "" {
				return nil, fmt.Errorf("tool call requires id and name")
			}

			var input any = map[string]any{}
			if len(toolCall.Input) > 0 {
				if err := json.Unmarshal(toolCall.Input, &input); err != nil {
					return nil, fmt.Errorf("invalid tool call input for %s: %w", toolCall.Name, err)
				}
			}

			blocks = append(blocks, anthropic.NewToolUseBlock(toolCall.ID, input, toolCall.Name))
		}

		for _, toolResult := range message.ToolResults {
			if strings.TrimSpace(toolResult.ToolUseID) == "" {
				return nil, fmt.Errorf("tool result requires tool_use_id")
			}
			blocks = append(blocks, anthropic.NewToolResultBlock(toolResult.ToolUseID, toolResult.Content, toolResult.IsError))
		}

		if len(blocks) == 0 {
			continue
		}

		role := strings.ToLower(strings.TrimSpace(message.Role))
		switch role {
		case "assistant":
			out = append(out, anthropic.NewAssistantMessage(blocks...))
		case "user", "":
			out = append(out, anthropic.NewUserMessage(blocks...))
		default:
			return nil, fmt.Errorf("unsupported message role %q", message.Role)
		}
	}

	return out, nil
}

func toAnthropicTools(toolDefs []ToolDefinition) ([]anthropic.ToolUnionParam, error) {
	out := make([]anthropic.ToolUnionParam, 0, len(toolDefs))
	for _, toolDef := range toolDefs {
		name := strings.TrimSpace(toolDef.Name)
		if name == "" {
			return nil, fmt.Errorf("tool name is required")
		}

		schema := anthropic.ToolInputSchemaParam{}
		if props, ok := toolDef.InputSchema["properties"]; ok {
			schema.Properties = props
		}
		if req, ok := toolDef.InputSchema["required"]; ok {
			required, err := toStringSlice(req)
			if err != nil {
				return nil, fmt.Errorf("tool %s required list: %w", name, err)
			}
			schema.Required = required
		}
		for key, value := range toolDef.InputSchema {
			switch key {
			case "type", "properties", "required":
				continue
			default:
				if schema.ExtraFields == nil {
					schema.ExtraFields = make(map[string]any)
				}
				schema.ExtraFields[key] = value
			}
		}

		tool := anthropic.ToolParam{
			Name:        name,
			InputSchema: schema,
		}
		if description := strings.TrimSpace(toolDef.Description); description != "" {
			tool.Description = anthropic.String(description)
		}

		out = append(out, anthropic.ToolUnionParam{OfTool: &tool})
	}
	return out, nil
}

func toStringSlice(v any) ([]string, error) {
	switch typed := v.(type) {
	case nil:
		return nil, nil
	case []string:
		return typed, nil
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("non-string required item %T", item)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected array, got %T", v)
	}
}
