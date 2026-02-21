package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"scaffold/db"
	"scaffold/embedding"
	googlecal "scaffold/google"
)

const maxToolRounds = 5

type Config struct {
	AssistantName    string
	UserName         string
	SystemPrompt     string
	TriagePrompt     string
	RespondModel     string
	TriageModel      string
	RespondMaxTokens int
	TriageMaxTokens  int
	Tools            []ToolDefinition
}

type Brain struct {
	client           anthropic.Client
	responder        ToolUseResponder
	db               *db.DB
	embedder         embedding.Embedder
	calendarClient   *googlecal.CalendarClient
	tools            []ToolDefinition
	toolRegistry     map[string]ToolHandler
	bulletinProvider func() (string, bool)
	systemPrompt     string
	triagePrompt     string
	respondModel     anthropic.Model
	triageModel      anthropic.Model
	respondMaxTokens int
	triageMaxTokens  int
}

func New(apiKey string, database *db.DB, cfg Config) *Brain {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	systemPrompt := strings.TrimSpace(cfg.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = buildSystemPrompt(cfg)
	}
	triagePrompt := strings.TrimSpace(cfg.TriagePrompt)
	if triagePrompt == "" {
		triagePrompt = buildTriagePrompt(cfg)
	}

	respondModel := anthropic.ModelClaudeHaiku4_5
	if cfg.RespondModel != "" {
		respondModel = anthropic.Model(cfg.RespondModel)
	}
	triageModel := anthropic.ModelClaudeHaiku4_5
	if cfg.TriageModel != "" {
		triageModel = anthropic.Model(cfg.TriageModel)
	}

	respondMaxTokens := cfg.RespondMaxTokens
	if respondMaxTokens <= 0 {
		respondMaxTokens = 1024
	}
	triageMaxTokens := cfg.TriageMaxTokens
	if triageMaxTokens <= 0 {
		triageMaxTokens = 300
	}

	tools := make([]ToolDefinition, 0, len(cfg.Tools))
	for _, tool := range cfg.Tools {
		if strings.TrimSpace(tool.Name) == "" {
			continue
		}
		tools = append(tools, tool)
	}

	return &Brain{
		client:           client,
		responder:        newAnthropicResponder(client),
		db:               database,
		tools:            tools,
		toolRegistry:     defaultToolRegistry(),
		systemPrompt:     systemPrompt,
		triagePrompt:     triagePrompt,
		respondModel:     respondModel,
		triageModel:      triageModel,
		respondMaxTokens: respondMaxTokens,
		triageMaxTokens:  triageMaxTokens,
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

Memory types: Identity, Goal, Decision, Todo, Idea, Preference, Fact, Event, Observation
Importance defaults: Identity=1.0, Goal=0.9, Decision=0.8, Todo=0.8, Idea=0.7, Preference=0.7, Fact=0.6, Event=0.4, Observation=0.3

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
		Model:     b.triageModel,
		MaxTokens: int64(b.triageMaxTokens),
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
	messages := make([]RespondMessage, 0, len(history)+2)
	for _, turn := range history {
		text := strings.TrimSpace(turn.Content)
		if text == "" {
			continue
		}

		role := "user"
		if strings.EqualFold(strings.TrimSpace(turn.Role), "assistant") {
			role = "assistant"
		}
		messages = append(messages, RespondMessage{Role: role, Text: text})
	}

	if shouldAppendCurrentUserMessage(messages, message) {
		messages = append(messages, RespondMessage{Role: "user", Text: strings.TrimSpace(message)})
	}
	if len(messages) == 0 {
		return "", fmt.Errorf("no user message to respond to")
	}

	request := ToolUseRequest{
		SystemPrompt: b.renderSystemPrompt(),
		Model:        string(b.respondModel),
		MaxTokens:    b.respondMaxTokens,
		Tools:        b.tools,
	}

	lastText := ""
	for round := 0; round < maxToolRounds; round++ {
		request.Messages = messages
		resp, err := b.responder.Respond(ctx, request)
		if err != nil {
			return "", fmt.Errorf("respond model call: %w", err)
		}

		if text := strings.TrimSpace(resp.Text); text != "" {
			lastText = text
		}
		if len(resp.ToolCalls) == 0 {
			if lastText == "" {
				return "", fmt.Errorf("empty response from model")
			}
			return lastText, nil
		}

		messages = append(messages, RespondMessage{
			Role:      "assistant",
			Text:      strings.TrimSpace(resp.Text),
			ToolCalls: resp.ToolCalls,
		})

		toolResults := make([]ToolResult, 0, len(resp.ToolCalls))
		for _, toolCall := range resp.ToolCalls {
			resultText, err := b.executeTool(ctx, toolCall)
			if err != nil {
				toolResults = append(toolResults, ToolResult{
					ToolUseID: toolCall.ID,
					Content:   fmt.Sprintf("Tool %s failed: %v", toolCall.Name, err),
					IsError:   true,
				})
				continue
			}
			toolResults = append(toolResults, ToolResult{
				ToolUseID: toolCall.ID,
				Content:   resultText,
			})
		}

		messages = append(messages, RespondMessage{
			Role:        "user",
			ToolResults: toolResults,
		})
	}

	if lastText != "" {
		if len(lastText) > 160 {
			lastText = lastText[:160] + "..."
		}
		return "", fmt.Errorf("tool loop exceeded %d rounds (last text: %q)", maxToolRounds, lastText)
	}
	return "", fmt.Errorf("tool loop exceeded %d rounds", maxToolRounds)
}

func shouldAppendCurrentUserMessage(messages []RespondMessage, message string) bool {
	text := strings.TrimSpace(message)
	if text == "" {
		return false
	}
	if len(messages) == 0 {
		return true
	}

	last := messages[len(messages)-1]
	if !strings.EqualFold(last.Role, "user") {
		return true
	}
	if len(last.ToolResults) > 0 {
		return true
	}

	return strings.TrimSpace(last.Text) != text
}

func (b *Brain) executeTool(ctx context.Context, toolCall ToolCall) (string, error) {
	if strings.TrimSpace(toolCall.ID) == "" {
		return "", fmt.Errorf("tool %s missing id", toolCall.Name)
	}
	if strings.TrimSpace(toolCall.Name) == "" {
		return "", fmt.Errorf("tool call missing name")
	}

	return ExecuteTool(ctx, toolCall.Name, toolCall.Input, b.db, b, b.toolRegistry)
}

func (b *Brain) SetBulletinProvider(provider func() (string, bool)) {
	b.bulletinProvider = provider
}

func (b *Brain) SetEmbedder(e embedding.Embedder) {
	b.embedder = e
}

func (b *Brain) SetCalendarClient(c *googlecal.CalendarClient) {
	b.calendarClient = c
}

func (b *Brain) renderSystemPrompt() string {
	const bulletinToken = "{{cortex_bulletin}}"

	bulletinText := "No bulletin available yet."
	if b.bulletinProvider != nil {
		content, fresh := b.bulletinProvider()
		content = strings.TrimSpace(content)
		if content != "" {
			bulletinText = content
		}
		if !fresh {
			bulletinText = bulletinText + "\n\n[Context may be stale.]"
		}
	}

	prompt := strings.TrimSpace(b.systemPrompt)
	if strings.Contains(prompt, bulletinToken) {
		return strings.ReplaceAll(prompt, bulletinToken, bulletinText)
	}

	if prompt == "" {
		return "## Current Context\n" + bulletinText
	}

	var out strings.Builder
	out.WriteString(prompt)
	out.WriteString("\n\n## Current Context\n")
	out.WriteString(bulletinText)
	return out.String()
}
