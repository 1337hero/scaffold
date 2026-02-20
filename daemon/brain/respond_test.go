package brain

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

type stubResponder struct {
	responses []ToolUseResponse
	requests  []ToolUseRequest
}

func (s *stubResponder) Respond(_ context.Context, req ToolUseRequest) (*ToolUseResponse, error) {
	s.requests = append(s.requests, cloneRequest(req))
	if len(s.responses) == 0 {
		return &ToolUseResponse{}, nil
	}

	resp := s.responses[0]
	s.responses = s.responses[1:]
	copied := cloneResponse(resp)
	return &copied, nil
}

func cloneRequest(req ToolUseRequest) ToolUseRequest {
	copyReq := req

	copyReq.Tools = make([]ToolDefinition, 0, len(req.Tools))
	for _, tool := range req.Tools {
		toolCopy := ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: make(map[string]interface{}, len(tool.InputSchema)),
		}
		for k, v := range tool.InputSchema {
			toolCopy.InputSchema[k] = v
		}
		copyReq.Tools = append(copyReq.Tools, toolCopy)
	}

	copyReq.Messages = make([]RespondMessage, 0, len(req.Messages))
	for _, message := range req.Messages {
		messageCopy := RespondMessage{
			Role: message.Role,
			Text: message.Text,
		}
		if len(message.ToolCalls) > 0 {
			messageCopy.ToolCalls = make([]ToolCall, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				messageCopy.ToolCalls = append(messageCopy.ToolCalls, ToolCall{
					ID:    call.ID,
					Name:  call.Name,
					Input: append(json.RawMessage(nil), call.Input...),
				})
			}
		}
		if len(message.ToolResults) > 0 {
			messageCopy.ToolResults = append([]ToolResult(nil), message.ToolResults...)
		}
		copyReq.Messages = append(copyReq.Messages, messageCopy)
	}

	return copyReq
}

func cloneResponse(resp ToolUseResponse) ToolUseResponse {
	copyResp := ToolUseResponse{
		Text: resp.Text,
	}
	if len(resp.ToolCalls) == 0 {
		return copyResp
	}

	copyResp.ToolCalls = make([]ToolCall, 0, len(resp.ToolCalls))
	for _, call := range resp.ToolCalls {
		copyResp.ToolCalls = append(copyResp.ToolCalls, ToolCall{
			ID:    call.ID,
			Name:  call.Name,
			Input: append(json.RawMessage(nil), call.Input...),
		})
	}
	return copyResp
}

func newRespondTestBrain(t *testing.T, responder ToolUseResponder) *Brain {
	t.Helper()
	return &Brain{
		responder: responder,
		db:        openTestDB(t),
		tools: []ToolDefinition{
			{
				Name:        "add_to_notebook",
				Description: "stub notebook writer",
				InputSchema: map[string]interface{}{},
			},
		},
		toolRegistry:     defaultToolRegistry(),
		systemPrompt:     "test system prompt",
		respondModel:     anthropic.Model("claude-haiku-4-5"),
		respondMaxTokens: 1024,
	}
}

func TestRespondReturnsDirectTextWithoutTools(t *testing.T) {
	stub := &stubResponder{
		responses: []ToolUseResponse{{Text: "All good."}},
	}
	b := newRespondTestBrain(t, stub)

	response, err := b.Respond(context.Background(), "hello", []ConversationTurn{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "All good." {
		t.Fatalf("expected direct text response, got %q", response)
	}
	if len(stub.requests) != 1 {
		t.Fatalf("expected 1 model request, got %d", len(stub.requests))
	}
	if len(stub.requests[0].Messages) != 1 {
		t.Fatalf("expected 1 message in request, got %d", len(stub.requests[0].Messages))
	}
}

func TestRenderSystemPromptReplacesBulletinToken(t *testing.T) {
	b := newRespondTestBrain(t, &stubResponder{})
	b.systemPrompt = "Base prompt\n\n## Current Context\n{{cortex_bulletin}}"
	b.SetBulletinProvider(func() (string, bool) {
		return "Key context bulletin", true
	})

	prompt := b.renderSystemPrompt()
	if strings.Contains(prompt, "{{cortex_bulletin}}") {
		t.Fatalf("expected bulletin token replacement, got %q", prompt)
	}
	if !strings.Contains(prompt, "Key context bulletin") {
		t.Fatalf("expected bulletin content in prompt, got %q", prompt)
	}
}

func TestRenderSystemPromptMarksStaleBulletin(t *testing.T) {
	b := newRespondTestBrain(t, &stubResponder{})
	b.systemPrompt = "Base prompt\n\n## Current Context\n{{cortex_bulletin}}"
	b.SetBulletinProvider(func() (string, bool) {
		return "Older context", false
	})

	prompt := b.renderSystemPrompt()
	if !strings.Contains(prompt, "[Context may be stale.]") {
		t.Fatalf("expected stale marker in prompt, got %q", prompt)
	}
}

func TestRespondExecutesToolLoopAndReturnsFinalText(t *testing.T) {
	stub := &stubResponder{
		responses: []ToolUseResponse{
			{
				ToolCalls: []ToolCall{{
					ID:    "tool-1",
					Name:  "add_to_notebook",
					Input: json.RawMessage(`{"notebook":"ideas","content":"kernel thought"}`),
				}},
			},
			{Text: "Saved it for later."},
		},
	}
	b := newRespondTestBrain(t, stub)

	response, err := b.Respond(context.Background(), "save this", []ConversationTurn{{Role: "user", Content: "save this"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "Saved it for later." {
		t.Fatalf("unexpected response: %q", response)
	}
	if len(stub.requests) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(stub.requests))
	}

	followup := stub.requests[1]
	if len(followup.Messages) < 2 {
		t.Fatalf("expected tool followup messages, got %d", len(followup.Messages))
	}
	last := followup.Messages[len(followup.Messages)-1]
	if last.Role != "user" {
		t.Fatalf("expected final followup role user, got %q", last.Role)
	}
	if len(last.ToolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(last.ToolResults))
	}
	if last.ToolResults[0].IsError {
		t.Fatalf("expected non-error tool result, got %#v", last.ToolResults[0])
	}
	if !strings.Contains(last.ToolResults[0].Content, "Notebooks are not yet available") {
		t.Fatalf("expected notebook stub result, got %q", last.ToolResults[0].Content)
	}
}

func TestRespondContinuesAfterToolError(t *testing.T) {
	stub := &stubResponder{
		responses: []ToolUseResponse{
			{
				ToolCalls: []ToolCall{{
					ID:    "tool-err",
					Name:  "does_not_exist",
					Input: json.RawMessage(`{}`),
				}},
			},
			{Text: "Handled the failure."},
		},
	}
	b := newRespondTestBrain(t, stub)

	response, err := b.Respond(context.Background(), "test", []ConversationTurn{{Role: "user", Content: "test"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "Handled the failure." {
		t.Fatalf("unexpected response: %q", response)
	}

	followup := stub.requests[1]
	last := followup.Messages[len(followup.Messages)-1]
	if len(last.ToolResults) != 1 || !last.ToolResults[0].IsError {
		t.Fatalf("expected error tool result in followup, got %#v", last.ToolResults)
	}
	if !strings.Contains(last.ToolResults[0].Content, "unknown tool") {
		t.Fatalf("expected unknown tool error text, got %q", last.ToolResults[0].Content)
	}
}

func TestRespondStopsAfterMaxToolRounds(t *testing.T) {
	responses := make([]ToolUseResponse, 0, maxToolRounds)
	for i := 0; i < maxToolRounds; i++ {
		responses = append(responses, ToolUseResponse{
			ToolCalls: []ToolCall{{
				ID:    "loop",
				Name:  "add_to_notebook",
				Input: json.RawMessage(`{"notebook":"loop","content":"again"}`),
			}},
		})
	}

	stub := &stubResponder{responses: responses}
	b := newRespondTestBrain(t, stub)

	_, err := b.Respond(context.Background(), "loop forever", []ConversationTurn{{Role: "user", Content: "loop forever"}})
	if err == nil {
		t.Fatal("expected max rounds error")
	}
	if !strings.Contains(err.Error(), "tool loop exceeded") {
		t.Fatalf("expected tool loop exceeded error, got %v", err)
	}
}
