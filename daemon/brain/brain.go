package brain

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"scaffold/agentprompt"
	"scaffold/db"
	googlecal "scaffold/google"
)

const maxToolRounds = 5

type Config struct {
	AssistantName    string
	UserName         string
	SystemPrompt     string
	Identity         agentprompt.Identity
	PromptFactLimit  int
	RespondModel     string
	RespondMaxTokens int
	Tools            []ToolDefinition
}

type Brain struct {
	responder        ToolUseResponder
	db               *db.DB
	calendarClient   *googlecal.CalendarClient
	tools            []ToolDefinition
	toolRegistry     map[string]ToolHandler
	bulletinProvider func() (string, bool)
	systemPrompt     string
	identity         agentprompt.Identity
	identityPrompt   bool
	promptFactLimit  int
	userName         string
	respondModel     string
	respondMaxTokens int
}

func New(apiKey string, database *db.DB, cfg Config) *Brain {
	responder := NewAnthropicResponder(apiKey)
	return NewWithDependencies(database, cfg, responder)
}

func NewWithDependencies(database *db.DB, cfg Config, responder ToolUseResponder) *Brain {
	useIdentityPrompt := identityConfigured(cfg.Identity)
	systemPrompt := strings.TrimSpace(cfg.SystemPrompt)
	if systemPrompt == "" && !useIdentityPrompt {
		systemPrompt = buildSystemPrompt(cfg)
	}

	identity := cfg.Identity
	if useIdentityPrompt {
		if strings.TrimSpace(identity.Name) == "" {
			identity.Name = cfg.AssistantName
		}
		if strings.TrimSpace(identity.UserName) == "" {
			identity.UserName = cfg.UserName
		}
	}

	promptFactLimit := cfg.PromptFactLimit
	if promptFactLimit <= 0 {
		promptFactLimit = 10
	}

	respondModel := "claude-sonnet-4-6"
	if cfg.RespondModel != "" {
		respondModel = strings.TrimSpace(cfg.RespondModel)
	}

	respondMaxTokens := cfg.RespondMaxTokens
	if respondMaxTokens <= 0 {
		respondMaxTokens = 1024
	}

	tools := make([]ToolDefinition, 0, len(cfg.Tools))
	for _, tool := range cfg.Tools {
		if strings.TrimSpace(tool.Name) == "" {
			continue
		}
		tools = append(tools, tool)
	}

	if responder == nil {
		responder = &unconfiguredResponder{}
	}

	return &Brain{
		responder:        responder,
		db:               database,
		tools:            tools,
		toolRegistry:     defaultToolRegistry(),
		systemPrompt:     systemPrompt,
		identity:         identity,
		identityPrompt:   useIdentityPrompt,
		promptFactLimit:  promptFactLimit,
		userName:         strings.TrimSpace(cfg.UserName),
		respondModel:     respondModel,
		respondMaxTokens: respondMaxTokens,
	}
}

func buildSystemPrompt(cfg Config) string {
	return fmt.Sprintf(`You are %s, an AI assistant connected to %s via Signal.

%s texts you thoughts, tasks, questions, ideas, whatever is on their mind.
Your job right now is to respond conversationally and helpfully.

Keep responses concise. This is Signal, not a doc. 2-4 sentences max unless asked for more.
Be direct, technical, no fluff. Match their energy.
You cannot view images, open file attachments, or transcribe audio from Signal. Never pretend you did; ask for a text description/transcript.

Create tasks only when they ask you to create, add, save, or remind them about a task.
If they ask a question, answer it.
If they're just thinking out loud, reflect it back and engage.`, cfg.AssistantName, cfg.UserName, cfg.UserName)
}

type ConversationTurn struct {
	Role    string
	Content string
}

type unconfiguredResponder struct{}

func (r *unconfiguredResponder) Respond(_ context.Context, _ ToolUseRequest) (*ToolUseResponse, error) {
	return nil, fmt.Errorf("tool responder is not configured")
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
		SystemPrompt: b.renderSystemPrompt(ctx, latestUserText(messages)),
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

func (b *Brain) SetCalendarClient(c *googlecal.CalendarClient) {
	b.calendarClient = c
}

func (b *Brain) renderSystemPrompt(ctx context.Context, message string) string {
	if b.identityPrompt {
		surface := agentprompt.DetectSurface(message, time.Now(), b.promptCalendarEvents(ctx), message)
		return agentprompt.AssembleSystemPrompt(
			b.identity,
			b.currentBulletin(),
			surface,
			b.promptFacts(message),
		)
	}

	const bulletinToken = "{{cortex_bulletin}}"

	bulletinText := b.currentBulletin()

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

func (b *Brain) currentBulletin() string {
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
	return bulletinText
}

func (b *Brain) promptFacts(message string) []agentprompt.Fact {
	if b == nil || b.db == nil {
		return nil
	}
	entities := []string{b.userName}
	if !strings.EqualFold(strings.TrimSpace(b.userName), "Mike") {
		entities = append(entities, "Mike")
	}
	facts, err := b.db.PromptFacts(entities, message, b.promptFactLimit)
	if err != nil {
		log.Printf("brain: prompt fact injection failed: %v", err)
		return nil
	}
	out := make([]agentprompt.Fact, 0, len(facts))
	for _, fact := range facts {
		out = append(out, agentprompt.Fact{
			Entity:   fact.Entity,
			Content:  fact.Content,
			Category: fact.Category.String,
			Trust:    fact.Trust,
		})
	}
	return out
}

func (b *Brain) promptCalendarEvents(ctx context.Context) []agentprompt.CalendarEvent {
	if b == nil || b.calendarClient == nil {
		return nil
	}
	calCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	events, err := b.calendarClient.UpcomingEvents(calCtx, b.calendarClient.CalendarID, 4)
	if err != nil {
		log.Printf("brain: prompt calendar context unavailable: %v", err)
		return nil
	}
	out := make([]agentprompt.CalendarEvent, 0, len(events))
	for _, event := range events {
		out = append(out, agentprompt.CalendarEvent{
			Title:       event.Title,
			Description: event.Description,
		})
	}
	return out
}

func latestUserText(messages []RespondMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(messages[i].Role, "user") && len(messages[i].ToolResults) == 0 {
			if text := strings.TrimSpace(messages[i].Text); text != "" {
				return text
			}
		}
	}
	return ""
}

func identityConfigured(identity agentprompt.Identity) bool {
	return strings.TrimSpace(identity.Name) != "" ||
		strings.TrimSpace(identity.UserName) != "" ||
		len(identity.Voice) > 0 ||
		len(identity.Values) > 0 ||
		len(identity.Posture) > 0 ||
		len(identity.CannotDo) > 0 ||
		len(identity.Rules) > 0
}
