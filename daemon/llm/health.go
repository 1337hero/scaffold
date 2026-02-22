package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type HealthChecker interface {
	Check(ctx context.Context) error
}

type noopHealthChecker struct{}

func (c *noopHealthChecker) Check(_ context.Context) error { return nil }

type anthropicHealthChecker struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func (c *anthropicHealthChecker) endpoint() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return "https://api.anthropic.com/v1"
}

func (c *anthropicHealthChecker) Check(ctx context.Context) error {
	body := strings.NewReader(`{"model":"claude-haiku-4-5","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint()+"/messages", body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("anthropic health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("anthropic health check: status %d", resp.StatusCode)
	}
	return nil
}

type openAIHealthChecker struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func (c *openAIHealthChecker) Check(ctx context.Context) error {
	url := strings.TrimRight(c.baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("openai health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("openai health check: status %d", resp.StatusCode)
	}
	return nil
}
