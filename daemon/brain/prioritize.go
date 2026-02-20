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

	model := b.respondModel
	if strings.TrimSpace(string(model)) == "" {
		model = anthropic.ModelClaudeHaiku4_5
	}

	resp, err := b.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
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
	tasks, err := parsePrioritizeTasks(text)
	if err != nil {
		return nil, fmt.Errorf("parse prioritize JSON: %w", err)
	}
	return tasks, nil
}

func parsePrioritizeTasks(raw string) ([]PrioritizedTask, error) {
	raw = strings.TrimSpace(raw)
	var tasks []PrioritizedTask

	if err := json.Unmarshal([]byte(raw), &tasks); err == nil {
		return tasks, nil
	}

	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
		if err := json.Unmarshal([]byte(trimmed), &tasks); err == nil {
			return tasks, nil
		}
	}

	start := strings.Index(trimmed, "[")
	end := strings.LastIndex(trimmed, "]")
	if start >= 0 && end > start {
		candidate := strings.TrimSpace(trimmed[start : end+1])
		if err := json.Unmarshal([]byte(candidate), &tasks); err == nil {
			return tasks, nil
		}
	}

	return nil, fmt.Errorf("invalid prioritize payload")
}
