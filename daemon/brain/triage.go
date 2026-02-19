package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type Config struct {
	AssistantName string
	UserName      string
}

type Brain struct {
	client        anthropic.Client
	systemPrompt  string
	triagePrompt  string
}

func New(apiKey string, cfg Config) *Brain {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Brain{
		client:       client,
		systemPrompt: buildSystemPrompt(cfg),
		triagePrompt: buildTriagePrompt(cfg),
	}
}

func buildSystemPrompt(cfg Config) string {
	return fmt.Sprintf(`You are %s — an AI assistant connected to %s via Signal.

%s texts you thoughts, tasks, questions, ideas — whatever's on their mind.
Your job right now is to respond conversationally and helpfully.

Keep responses concise — this is Signal, not a doc. 2-4 sentences max unless asked for more.
Be direct, technical, no fluff. Match their energy.

If they send a task or todo, acknowledge it and tell them it'll go in their inbox (even though that's not wired yet).
If they ask a question, answer it.
If they're just thinking out loud, reflect it back and engage.`, cfg.AssistantName, cfg.UserName, cfg.UserName)
}

type TriageResult struct {
	Type       string   `json:"type"`
	Importance float64  `json:"importance"`
	Action     string   `json:"action"`
	Title      string   `json:"title"`
	MicroSteps []string `json:"micro_steps,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

type ConversationTurn struct {
	Role    string
	Content string
}

// TriageDegradedError means triage fell back to a safe default result.
// Callers should treat the returned TriageResult as usable but log this error.
type TriageDegradedError struct {
	Reason string
	Cause  error
}

func (e *TriageDegradedError) Error() string {
	if e.Cause != nil {
		return "triage degraded: " + e.Reason + ": " + e.Cause.Error()
	}
	return "triage degraded: " + e.Reason
}

func (e *TriageDegradedError) Unwrap() error {
	return e.Cause
}

func buildTriagePrompt(cfg Config) string {
	return fmt.Sprintf(`You are the triage brain for %s's executive function system.
Your job: classify this capture and decide what to do with it.

Memory types: Identity, Goal, Decision, Todo, Preference, Fact, Event, Observation
Importance defaults: Identity=1.0, Goal=0.9, Decision=0.8, Todo=0.8, Preference=0.7, Fact=0.6, Event=0.4, Observation=0.3

Actions:
- "do" = this needs action, becomes a task
- "explore" = worth thinking about, open a notebook thread
- "reference" = file it, find it later
- "waiting" = tracking something external

If it's a task (action=do), suggest micro_steps (3-5 concrete actions, each short and specific).

Respond ONLY with valid JSON, no other text:
{"type": "Todo", "importance": 0.8, "action": "do", "title": "Short title", "micro_steps": ["step 1", "step 2"], "tags": ["tag1"]}`, cfg.UserName)
}

func (b *Brain) Triage(ctx context.Context, raw string) (*TriageResult, error) {
	prompt := b.triagePrompt + "\n\nCapture: " + raw
	resp, err := b.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 300,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return fallbackTriage(raw), &TriageDegradedError{
			Reason: "claude request failed",
			Cause:  err,
		}
	}

	if len(resp.Content) == 0 {
		return fallbackTriage(raw), &TriageDegradedError{
			Reason: "claude returned no content",
		}
	}

	return parseTriageContent(raw, resp.Content[0].Text)
}

func parseTriageContent(raw, content string) (*TriageResult, error) {
	text := strings.TrimSpace(content)
	if text == "" {
		return fallbackTriage(raw), &TriageDegradedError{
			Reason: "empty triage response",
		}
	}

	var result TriageResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return fallbackTriage(raw), &TriageDegradedError{
			Reason: "invalid triage JSON",
			Cause:  err,
		}
	}
	if strings.TrimSpace(result.Type) == "" || strings.TrimSpace(result.Action) == "" || strings.TrimSpace(result.Title) == "" {
		return fallbackTriage(raw), &TriageDegradedError{
			Reason: "triage JSON missing required fields",
		}
	}

	return &result, nil
}

func fallbackTriage(raw string) *TriageResult {
	return &TriageResult{
		Type:       "Observation",
		Importance: 0.3,
		Action:     "reference",
		Title:      raw,
	}
}

func (b *Brain) Respond(ctx context.Context, message string, history []ConversationTurn) (string, error) {
	messages := make([]anthropic.MessageParam, 0, len(history))
	for _, turn := range history {
		text := strings.TrimSpace(turn.Content)
		if text == "" {
			continue
		}
		if turn.Role == "assistant" {
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(text)))
			continue
		}
		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(text)))
	}
	if len(messages) == 0 {
		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(message)))
	}

	resp, err := b.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 300,
		System: []anthropic.TextBlockParam{
			{Text: b.systemPrompt},
		},
		Messages: messages,
	})
	if err != nil {
		return "", fmt.Errorf("claude API: %w", err)
	}

	if len(resp.Content) == 0 {
		return "", fmt.Errorf("empty response from claude")
	}

	return resp.Content[0].Text, nil
}
