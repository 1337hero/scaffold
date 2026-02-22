package brain

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"scaffold/sessionbus"
)

func TestHandleListSessions(t *testing.T) {
	bus := sessionbus.New(sessionbus.Config{SessionTTL: time.Hour})
	if _, err := bus.Register(context.Background(), sessionbus.RegisterRequest{SessionID: "codex-main", Provider: "codex", Name: "Codex"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	b := &Brain{sessionBus: bus}
	result, err := handleListSessions(context.Background(), nil, b, nil)
	if err != nil {
		t.Fatalf("handleListSessions: %v", err)
	}
	if !strings.Contains(result, "codex-main") {
		t.Fatalf("expected codex-main in output, got %q", result)
	}
}

func TestHandleSendToSession(t *testing.T) {
	bus := sessionbus.New(sessionbus.Config{SessionTTL: time.Hour})
	if _, err := bus.Register(context.Background(), sessionbus.RegisterRequest{SessionID: "gemini-worker", Provider: "gemini", Name: "Gemini"}); err != nil {
		t.Fatalf("register target: %v", err)
	}

	b := &Brain{sessionBus: bus}
	params, _ := json.Marshal(map[string]any{
		"to_session_id": "gemini-worker",
		"message":       "hello from scaffold",
		"mode":          "steer",
	})

	result, err := handleSendToSession(context.Background(), nil, b, params)
	if err != nil {
		t.Fatalf("handleSendToSession: %v", err)
	}
	if !strings.Contains(result, "Message sent to gemini-worker") {
		t.Fatalf("unexpected result: %q", result)
	}

	messages, err := bus.Poll(context.Background(), "gemini-worker", 10, 0)
	if err != nil {
		t.Fatalf("poll target: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 delivered message, got %d", len(messages))
	}
	if messages[0].Message != "hello from scaffold" {
		t.Fatalf("unexpected delivered message %q", messages[0].Message)
	}
}

func TestHandleSendToSessionWaitForReply(t *testing.T) {
	bus := sessionbus.New(sessionbus.Config{SessionTTL: time.Hour})
	if _, err := bus.Register(context.Background(), sessionbus.RegisterRequest{SessionID: "gemini-worker", Provider: "gemini"}); err != nil {
		t.Fatalf("register target: %v", err)
	}

	b := &Brain{sessionBus: bus}

	go func() {
		time.Sleep(40 * time.Millisecond)
		_, _ = bus.Send(context.Background(), sessionbus.SendRequest{
			FromSessionID: "gemini-worker",
			ToSessionID:   "scaffold-agent",
			Mode:          "follow_up",
			Message:       "done",
		})
	}()

	params, _ := json.Marshal(map[string]any{
		"to_session_id": "gemini-worker",
		"message":       "run task",
		"wait_seconds":  1,
	})

	result, err := handleSendToSession(context.Background(), nil, b, params)
	if err != nil {
		t.Fatalf("handleSendToSession wait: %v", err)
	}
	if !strings.Contains(result, "Reply from gemini-worker:") {
		t.Fatalf("expected reply text, got %q", result)
	}
	if !strings.Contains(result, "done") {
		t.Fatalf("expected reply content in result, got %q", result)
	}
}
