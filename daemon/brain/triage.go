package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"scaffold/db"
	"scaffold/embedding"
	googlecal "scaffold/google"
	"scaffold/llm"
	"scaffold/sessionbus"
)

const maxToolRounds = 5

type Config struct {
	AssistantName    string
	UserName         string
	SystemPrompt     string
	TriagePrompt     string
	RespondModel     string
	TriageModel      string
	PrioritizeModel  string
	RespondMaxTokens int
	TriageMaxTokens  int
	Tools            []ToolDefinition
	CodeDispatchCWD  string
}

type Brain struct {
	responder        ToolUseResponder
	triageLLM        llm.CompletionClient
	prioritizeLLM    llm.CompletionClient
	db               *db.DB
	embedder         embedding.Embedder
	calendarClient   *googlecal.CalendarClient
	gmailClient      *googlecal.GmailClient
	sessionBus       *sessionbus.Bus
	tools            []ToolDefinition
	toolRegistry     map[string]ToolHandler
	bulletinProvider func() (string, bool)
	systemPrompt     string
	triagePrompt     string
	respondModel     string
	triageModel      string
	prioritizeModel  string
	respondMaxTokens int
	triageMaxTokens  int
	codeDispatchCWD  string
}

func New(apiKey string, database *db.DB, cfg Config) *Brain {
	responder := NewAnthropicResponder(apiKey)
	completion := NewAnthropicCompletionClient(apiKey)
	return NewWithDependencies(database, cfg, Dependencies{
		Responder:            responder,
		TriageCompletion:     completion,
		PrioritizeCompletion: completion,
	})
}

func NewWithDependencies(database *db.DB, cfg Config, deps Dependencies) *Brain {
	systemPrompt := strings.TrimSpace(cfg.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = buildSystemPrompt(cfg)
	}
	triagePrompt := strings.TrimSpace(cfg.TriagePrompt)
	if triagePrompt == "" {
		triagePrompt = buildTriagePrompt(cfg)
	}

	respondModel := "claude-haiku-4-5"
	if cfg.RespondModel != "" {
		respondModel = strings.TrimSpace(cfg.RespondModel)
	}
	triageModel := "claude-haiku-4-5"
	if cfg.TriageModel != "" {
		triageModel = strings.TrimSpace(cfg.TriageModel)
	}
	prioritizeModel := respondModel
	if strings.TrimSpace(cfg.PrioritizeModel) != "" {
		prioritizeModel = strings.TrimSpace(cfg.PrioritizeModel)
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

	if deps.Responder == nil {
		deps.Responder = &unconfiguredResponder{}
	}
	if deps.TriageCompletion == nil {
		deps.TriageCompletion = &llm.UnconfiguredCompletionClient{}
	}
	if deps.PrioritizeCompletion == nil {
		deps.PrioritizeCompletion = deps.TriageCompletion
	}

	return &Brain{
		responder:        deps.Responder,
		triageLLM:        deps.TriageCompletion,
		prioritizeLLM:    deps.PrioritizeCompletion,
		db:               database,
		tools:            tools,
		toolRegistry:     defaultToolRegistry(),
		systemPrompt:     systemPrompt,
		triagePrompt:     triagePrompt,
		respondModel:     respondModel,
		triageModel:      triageModel,
		prioritizeModel:  prioritizeModel,
		respondMaxTokens: respondMaxTokens,
		triageMaxTokens:  triageMaxTokens,
		codeDispatchCWD:  cfg.CodeDispatchCWD,
	}
}

func buildSystemPrompt(cfg Config) string {
	return fmt.Sprintf(`You are %s — an AI assistant connected to %s via Signal.

%s texts you thoughts, tasks, questions, ideas — whatever's on their mind.
Your job right now is to respond conversationally and helpfully.

Keep responses concise — this is Signal, not a doc. 2-4 sentences max unless asked for more.
Be direct, technical, no fluff. Match their energy.
You cannot view images, open file attachments, or transcribe audio from Signal. Never pretend you did; ask for a text description/transcript.

If they send a task or todo, acknowledge it and tell them it'll go in their inbox.
If they ask a question, answer it.
If they're just thinking out loud, reflect it back and engage.`, cfg.AssistantName, cfg.UserName, cfg.UserName)
}

type TriageResult struct {
	Type       string   `json:"type"`
	Importance float64  `json:"importance"`
	Action     string   `json:"action"`
	Title      string   `json:"title"`
	Domain     string   `json:"domain,omitempty"`
	GoalType   string   `json:"goal_type,omitempty"`
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

type unconfiguredResponder struct{}

func (r *unconfiguredResponder) Respond(_ context.Context, _ ToolUseRequest) (*ToolUseResponse, error) {
	return nil, fmt.Errorf("tool responder is not configured")
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
- "explore" = worth thinking about, explore further
- "reference" = file it, find it later
- "waiting" = tracking something external

Domains (always assign exactly one best-fit domain):
Work/Business, Personal Projects, Homelife, Relationships, Personal Development, Finances, Hobbies

Heuristics:
- Personal profile, working style, communication norms, and stable behavior patterns => Identity or Preference.
- Explicit targets, outcomes, and deadlines => Goal (type must be "Goal").
- Aspirational outcomes, weight loss, fitness targets, launch deadlines, ongoing commitments => Goal.
- Concrete one-off actions (call someone, buy something, set up X) => Todo with action "do".

If type is "Goal", also classify goal_type:
- "binary" = done or not done (e.g., "launch website", "get passport renewed")
- "measurable" = has a numeric target (e.g., "lose 50lbs", "save $10k", "run a 5k in under 25min")
- "habit" = recurring behavior tracked over time (e.g., "workout 4x/week", "no soda", "read daily")

If it's a task (action=do), suggest micro_steps (3-5 concrete actions, each short and specific).

Respond ONLY with valid JSON, no other text:
{"type": "Goal", "importance": 0.9, "action": "do", "title": "Lose 50lbs", "domain": "Personal Development", "goal_type": "measurable", "tags": ["health"]}
{"type": "Todo", "importance": 0.8, "action": "do", "title": "Short title", "domain": "Work/Business", "micro_steps": ["step 1", "step 2"], "tags": ["tag1"]}`, cfg.UserName)
}

func (b *Brain) Triage(ctx context.Context, raw string) (*TriageResult, error) {
	prompt := b.triagePrompt + "\n\nCapture: " + raw
	resp, err := b.triageLLM.CompletionJSON(ctx, b.triageModel, "", prompt, int64(b.triageMaxTokens))
	if err != nil {
		return fallbackTriage(raw), &TriageDegradedError{
			Reason: "triage model request failed",
			Cause:  err,
		}
	}
	return parseTriageContent(raw, resp)
}

func parseTriageContent(raw, content string) (*TriageResult, error) {
	text := sanitizeTriageJSON(content)
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
		Title:      fallbackTitle(raw),
	}
}

func sanitizeTriageJSON(content string) string {
	text := strings.TrimSpace(content)
	if text == "" {
		return ""
	}

	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 {
			start := 1
			end := len(lines) - 1
			for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
				start++
			}
			for end >= 0 && strings.TrimSpace(lines[end]) != "```" {
				end--
			}
			if end > start {
				text = strings.Join(lines[start:end], "\n")
			}
		}
	}

	if obj, ok := extractFirstJSONObject(text); ok {
		text = obj
	} else {
		first := strings.Index(text, "{")
		last := strings.LastIndex(text, "}")
		if first >= 0 && last > first {
			text = text[first : last+1]
		}
	}

	return strings.TrimSpace(text)
}

func extractFirstJSONObject(text string) (string, bool) {
	start := strings.Index(text, "{")
	if start < 0 {
		return "", false
	}

	depth := 0
	inString := false
	escaping := false
	for i := start; i < len(text); i++ {
		ch := text[i]

		if inString {
			if escaping {
				escaping = false
				continue
			}
			if ch == '\\' {
				escaping = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(text[start : i+1]), true
			}
		}
	}

	return "", false
}

func fallbackTitle(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if len(trimmed) > 96 {
			return trimmed[:96]
		}
		return trimmed
	}
	return "Captured Note"
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
		Model:        b.respondModel,
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

func (b *Brain) SetGmailClient(c *googlecal.GmailClient) {
	b.gmailClient = c
}

func (b *Brain) SetSessionBus(bus *sessionbus.Bus) {
	b.sessionBus = bus
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
