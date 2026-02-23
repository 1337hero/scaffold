package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type openAIClient struct {
	baseURL                string
	apiKey                 string
	httpClient             *http.Client
	supportsToolUse        bool
	nativeJSONFormat       bool
	useMaxCompletionTokens bool
}

func newOpenAIClient(baseURL, apiKey string, timeout time.Duration, supportsToolUse, nativeJSONFormat, useMaxCompletionTokens bool) *openAIClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &openAIClient{
		baseURL:                baseURL,
		apiKey:                 strings.TrimSpace(apiKey),
		httpClient:             &http.Client{Timeout: timeout},
		supportsToolUse:        supportsToolUse,
		nativeJSONFormat:       nativeJSONFormat,
		useMaxCompletionTokens: useMaxCompletionTokens,
	}
}

func (c *openAIClient) applyMaxTokens(req *openAIChatCompletionRequest, maxTokens int) {
	if c.useMaxCompletionTokens {
		req.MaxCompletionTokens = maxTokens
	} else {
		req.MaxTokens = maxTokens
	}
}

func (c *openAIClient) CompletionJSON(ctx context.Context, model, systemPrompt, userPrompt string, maxTokens int64) (string, error) {
	req := openAIChatCompletionRequest{
		Model: strings.TrimSpace(model),
	}
	c.applyMaxTokens(&req, int(maxTokens))
	if system := strings.TrimSpace(systemPrompt); system != "" {
		req.Messages = append(req.Messages, openAIChatMessage{
			Role:    "system",
			Content: system,
		})
	}
	req.Messages = append(req.Messages, openAIChatMessage{
		Role:    "user",
		Content: strings.TrimSpace(userPrompt),
	})
	if c.nativeJSONFormat {
		req.ResponseFormat = map[string]string{"type": "json_object"}
	}

	resp, err := c.chatCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(extractOpenAIContent(resp.FirstMessage.Content)), nil
}

func (c *openAIClient) CompletionText(ctx context.Context, model, systemPrompt, userPrompt string, maxTokens int64) (string, error) {
	req := openAIChatCompletionRequest{
		Model: strings.TrimSpace(model),
	}
	c.applyMaxTokens(&req, int(maxTokens))
	if system := strings.TrimSpace(systemPrompt); system != "" {
		req.Messages = append(req.Messages, openAIChatMessage{
			Role:    "system",
			Content: system,
		})
	}
	req.Messages = append(req.Messages, openAIChatMessage{
		Role:    "user",
		Content: strings.TrimSpace(userPrompt),
	})

	resp, err := c.chatCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(extractOpenAIContent(resp.FirstMessage.Content)), nil
}

func (c *openAIClient) Respond(ctx context.Context, req ToolUseRequest) (*ToolUseResponse, error) {
	if !c.supportsToolUse {
		return nil, fmt.Errorf("provider does not support tool use")
	}

	chatReq := openAIChatCompletionRequest{
		Model: strings.TrimSpace(req.Model),
	}
	c.applyMaxTokens(&chatReq, req.MaxTokens)
	if system := strings.TrimSpace(req.SystemPrompt); system != "" {
		chatReq.Messages = append(chatReq.Messages, openAIChatMessage{
			Role:    "system",
			Content: system,
		})
	}

	for _, message := range req.Messages {
		role := normalizeOpenAIRole(message.Role)
		text := strings.TrimSpace(message.Text)

		if len(message.ToolCalls) > 0 {
			toolCalls := make([]openAIToolCall, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				raw := normalizeJSONBytes(call.Input)
				toolCalls = append(toolCalls, openAIToolCall{
					ID:   strings.TrimSpace(call.ID),
					Type: "function",
					Function: openAIToolFunctionCall{
						Name:      strings.TrimSpace(call.Name),
						Arguments: string(raw),
					},
				})
			}
			msg := openAIChatMessage{
				Role:      "assistant",
				ToolCalls: toolCalls,
			}
			if text != "" {
				msg.Content = text
			}
			chatReq.Messages = append(chatReq.Messages, msg)
		} else if text != "" {
			chatReq.Messages = append(chatReq.Messages, openAIChatMessage{
				Role:    role,
				Content: text,
			})
		}

		for _, result := range message.ToolResults {
			toolCallID := strings.TrimSpace(result.ToolUseID)
			if toolCallID == "" {
				continue
			}
			content := strings.TrimSpace(result.Content)
			if result.IsError && !strings.HasPrefix(strings.ToLower(content), "error") {
				content = "Error: " + content
			}
			chatReq.Messages = append(chatReq.Messages, openAIChatMessage{
				Role:       "tool",
				ToolCallID: toolCallID,
				Content:    content,
			})
		}
	}

	if len(req.Tools) > 0 {
		chatReq.ToolChoice = "auto"
		chatReq.Tools = make([]openAIChatTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			name := strings.TrimSpace(tool.Name)
			if name == "" {
				continue
			}
			params := map[string]any{
				"type": "object",
			}
			for key, value := range tool.InputSchema {
				params[key] = value
			}
			chatReq.Tools = append(chatReq.Tools, openAIChatTool{
				Type: "function",
				Function: openAIChatToolFunction{
					Name:        name,
					Description: strings.TrimSpace(tool.Description),
					Parameters:  params,
				},
			})
		}
	}

	resp, err := c.chatCompletion(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	out := &ToolUseResponse{
		Text: strings.TrimSpace(extractOpenAIContent(resp.FirstMessage.Content)),
	}
	for _, call := range resp.FirstMessage.ToolCalls {
		args := normalizeJSONString(call.Function.Arguments)
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:    strings.TrimSpace(call.ID),
			Name:  strings.TrimSpace(call.Function.Name),
			Input: append(json.RawMessage(nil), args...),
		})
	}
	return out, nil
}

type openAIChatCompletionRequest struct {
	Model                 string              `json:"model"`
	Messages              []openAIChatMessage `json:"messages"`
	MaxTokens             int                 `json:"max_tokens,omitempty"`
	MaxCompletionTokens   int                 `json:"max_completion_tokens,omitempty"`
	Tools                 []openAIChatTool    `json:"tools,omitempty"`
	ToolChoice            string              `json:"tool_choice,omitempty"`
	ResponseFormat        map[string]string   `json:"response_format,omitempty"`
}

type openAIChatMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openAIToolFunctionCall `json:"function"`
}

type openAIToolFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIChatTool struct {
	Type     string                 `json:"type"`
	Function openAIChatToolFunction `json:"function"`
}

type openAIChatToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type openAIChatCompletionResponse struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Choices []struct {
		Message openAIChatMessage `json:"message"`
	} `json:"choices"`
}

type openAICompletionResult struct {
	FirstMessage openAIChatMessage
}

func (c *openAIClient) chatCompletion(ctx context.Context, req openAIChatCompletionRequest) (openAICompletionResult, error) {
	if strings.TrimSpace(req.Model) == "" {
		return openAICompletionResult{}, fmt.Errorf("model is required")
	}
	if len(req.Messages) == 0 {
		return openAICompletionResult{}, fmt.Errorf("at least one message is required")
	}
	if strings.TrimSpace(c.baseURL) == "" {
		return openAICompletionResult{}, fmt.Errorf("base URL is empty")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return openAICompletionResult{}, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return openAICompletionResult{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return openAICompletionResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return openAICompletionResult{}, fmt.Errorf("read response: %w", err)
	}

	var parsed openAIChatCompletionResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return openAICompletionResult{}, fmt.Errorf("decode response: %w", err)
	}
	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return openAICompletionResult{}, errors.New(strings.TrimSpace(parsed.Error.Message))
	}
	if resp.StatusCode >= 400 {
		if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			return openAICompletionResult{}, fmt.Errorf("chat completion failed (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return openAICompletionResult{}, fmt.Errorf("chat completion failed with status %d", resp.StatusCode)
	}
	if len(parsed.Choices) == 0 {
		return openAICompletionResult{}, fmt.Errorf("empty response choices")
	}

	return openAICompletionResult{FirstMessage: parsed.Choices[0].Message}, nil
}

func normalizeOpenAIRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant":
		return "assistant"
	default:
		return "user"
	}
}

func normalizeJSONBytes(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	cleaned := bytes.TrimSpace(raw)
	if len(cleaned) == 0 {
		return []byte("{}")
	}
	if json.Valid(cleaned) {
		return cleaned
	}
	return []byte("{}")
}

func normalizeJSONString(raw string) []byte {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return []byte("{}")
	}
	if json.Valid([]byte(cleaned)) {
		return []byte(cleaned)
	}

	quoted, err := strconv.Unquote(cleaned)
	if err == nil {
		quoted = strings.TrimSpace(quoted)
		if json.Valid([]byte(quoted)) {
			return []byte(quoted)
		}
	}

	return []byte("{}")
}

func extractOpenAIContent(content any) string {
	switch value := content.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(value)
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, ok := itemMap["text"].(string)
			if !ok {
				continue
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			parts = append(parts, text)
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
