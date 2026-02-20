package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Available(ctx context.Context) bool
	ModelName() string
}

type OllamaClient struct {
	baseURL    string
	model      string
	dimensions int
	http       *http.Client
}

func NewOllamaClient(baseURL, model string, dimensions int) *OllamaClient {
	return &OllamaClient{
		baseURL:    baseURL,
		model:      model,
		dimensions: dimensions,
		http:       &http.Client{Timeout: 60 * time.Second},
	}
}

type embedRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func (c *OllamaClient) Embed(ctx context.Context, text string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	vecs, err := c.doEmbed(ctx, text)
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("ollama returned no embeddings")
	}
	if len(vecs[0]) != c.dimensions {
		return nil, fmt.Errorf("dimension mismatch: expected %d, got %d", c.dimensions, len(vecs[0]))
	}
	return vecs[0], nil
}

func (c *OllamaClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	vecs, err := c.doEmbed(ctx, texts)
	if err != nil {
		return nil, err
	}
	if len(vecs) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(vecs))
	}
	for i, v := range vecs {
		if len(v) != c.dimensions {
			return nil, fmt.Errorf("embedding[%d] dimension mismatch: expected %d, got %d", i, c.dimensions, len(v))
		}
	}
	return vecs, nil
}

func (c *OllamaClient) Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *OllamaClient) ModelName() string {
	return c.model
}

func (c *OllamaClient) doEmbed(ctx context.Context, input interface{}) ([][]float32, error) {
	body, err := json.Marshal(embedRequest{Model: c.model, Input: input})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	return result.Embeddings, nil
}
