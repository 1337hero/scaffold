package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"scaffold/db"
)

type PrioritizedTask struct {
	Title          string   `json:"title"`
	MicroSteps     []string `json:"micro_steps"`
	SourceMemoryID string   `json:"source_memory_id"`
	Why            string   `json:"why"`
}

func (b *Brain) Prioritize(ctx context.Context, todos []db.Memory, yesterdayDesk []db.DeskItem) ([]PrioritizedTask, error) {
	todosJSON, err := json.Marshal(todos)
	if err != nil {
		return nil, fmt.Errorf("marshal todos: %w", err)
	}
	yesterdayJSON, err := json.Marshal(yesterdayDesk)
	if err != nil {
		return nil, fmt.Errorf("marshal yesterday desk: %w", err)
	}

	system := `You are building the daily desk for Mike's ADHD executive function system.

Rules:
- Maximum 3 tasks. Position 1 is THE ONE — the single most important task today.
- Short, concrete micro-steps (3-5 per task, 15-30 min each).
- Time-sensitive items override importance scores.
- Carry forward incomplete items from yesterday if still relevant.
- Output ONLY valid JSON, no markdown, no explanation.`

	user := fmt.Sprintf(`Yesterday's desk:
%s

Active todos (by importance):
%s

Respond as JSON array: [{"title": "...", "micro_steps": ["step 1", "step 2"], "source_memory_id": "...", "why": "one sentence"}]
Maximum 3 items.`, string(yesterdayJSON), string(todosJSON))

	resp, err := b.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaude3_7SonnetLatest,
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: system},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API: %w", err)
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("empty response from claude")
	}

	text := strings.TrimSpace(resp.Content[0].Text)
	var tasks []PrioritizedTask
	if err := json.Unmarshal([]byte(text), &tasks); err != nil {
		return nil, fmt.Errorf("parse prioritize JSON: %w", err)
	}

	return tasks, nil
}
