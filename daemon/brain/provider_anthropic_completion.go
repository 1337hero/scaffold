package brain

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"scaffold/llm"
)

type anthropicCompletionClient struct {
	client anthropic.Client
}

func NewAnthropicCompletionClient(apiKey string) llm.CompletionClient {
	client := anthropic.NewClient(option.WithAPIKey(strings.TrimSpace(apiKey)))
	return &anthropicCompletionClient{client: client}
}

func (c *anthropicCompletionClient) complete(ctx context.Context, model, systemPrompt, userPrompt string, maxTokens int64) (string, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt))},
	}
	if system := strings.TrimSpace(systemPrompt); system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}

	resp, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("anthropic completion: %w", err)
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return strings.TrimSpace(resp.Content[0].Text), nil
}

func (c *anthropicCompletionClient) CompletionJSON(ctx context.Context, model, systemPrompt, userPrompt string, maxTokens int64) (string, error) {
	return c.complete(ctx, model, systemPrompt, userPrompt, maxTokens)
}

func (c *anthropicCompletionClient) CompletionText(ctx context.Context, model, systemPrompt, userPrompt string, maxTokens int64) (string, error) {
	return c.complete(ctx, model, systemPrompt, userPrompt, maxTokens)
}
