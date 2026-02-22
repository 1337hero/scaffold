package sessionbus

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRegisterListAndSendPoll(t *testing.T) {
	bus := New(Config{SessionTTL: time.Hour, MaxQueuePerSess: 8})

	if _, err := bus.Register(context.Background(), RegisterRequest{SessionID: "codex-main", Provider: "codex", Name: "Codex"}); err != nil {
		t.Fatalf("register codex: %v", err)
	}
	if _, err := bus.Register(context.Background(), RegisterRequest{SessionID: "gemini-worker", Provider: "gemini", Name: "Gemini"}); err != nil {
		t.Fatalf("register gemini: %v", err)
	}

	sessions := bus.List(context.Background())
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	sent, err := bus.Send(context.Background(), SendRequest{
		FromSessionID: "codex-main",
		ToSessionID:   "gemini-worker",
		Mode:          "steer",
		Message:       "summarize docs",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if sent.ID == "" {
		t.Fatal("expected sent message id")
	}

	msgs, err := bus.Poll(context.Background(), "gemini-worker", 10, 0)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Message != "summarize docs" {
		t.Fatalf("unexpected message body: %q", msgs[0].Message)
	}
	if msgs[0].FromSessionID != "codex-main" {
		t.Fatalf("unexpected from id: %q", msgs[0].FromSessionID)
	}
}

func TestSendUnknownTarget(t *testing.T) {
	bus := New(Config{SessionTTL: time.Hour})
	if _, err := bus.Register(context.Background(), RegisterRequest{SessionID: "sender", Provider: "codex"}); err != nil {
		t.Fatalf("register sender: %v", err)
	}

	_, err := bus.Send(context.Background(), SendRequest{
		FromSessionID: "sender",
		ToSessionID:   "missing",
		Message:       "hello",
	})
	if !errors.Is(err, ErrUnknownSession) {
		t.Fatalf("expected ErrUnknownSession, got %v", err)
	}
}

func TestPollWaitsForMessage(t *testing.T) {
	bus := New(Config{SessionTTL: time.Hour})
	if _, err := bus.Register(context.Background(), RegisterRequest{SessionID: "sender", Provider: "codex"}); err != nil {
		t.Fatalf("register sender: %v", err)
	}
	if _, err := bus.Register(context.Background(), RegisterRequest{SessionID: "target", Provider: "gemini"}); err != nil {
		t.Fatalf("register target: %v", err)
	}

	go func() {
		time.Sleep(40 * time.Millisecond)
		_, _ = bus.Send(context.Background(), SendRequest{
			FromSessionID: "sender",
			ToSessionID:   "target",
			Message:       "ping",
		})
	}()

	start := time.Now()
	msgs, err := bus.Poll(context.Background(), "target", 1, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected one message, got %d", len(msgs))
	}
	if elapsed := time.Since(start); elapsed < 25*time.Millisecond {
		t.Fatalf("poll returned too fast: %v", elapsed)
	}
}

func TestQueueBoundedDropsOldest(t *testing.T) {
	bus := New(Config{SessionTTL: time.Hour, MaxQueuePerSess: 2})
	if _, err := bus.Register(context.Background(), RegisterRequest{SessionID: "s1", Provider: "codex"}); err != nil {
		t.Fatalf("register s1: %v", err)
	}
	if _, err := bus.Register(context.Background(), RegisterRequest{SessionID: "s2", Provider: "gemini"}); err != nil {
		t.Fatalf("register s2: %v", err)
	}

	for _, text := range []string{"m1", "m2", "m3"} {
		if _, err := bus.Send(context.Background(), SendRequest{FromSessionID: "s1", ToSessionID: "s2", Message: text}); err != nil {
			t.Fatalf("send %s: %v", text, err)
		}
	}

	msgs, err := bus.Poll(context.Background(), "s2", 10, 0)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Message != "m2" || msgs[1].Message != "m3" {
		t.Fatalf("expected [m2,m3], got [%s,%s]", msgs[0].Message, msgs[1].Message)
	}
}

func TestPruneRemovesStaleSessions(t *testing.T) {
	bus := New(Config{SessionTTL: 1 * time.Second})

	now := time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC)
	bus.nowFn = func() time.Time { return now }

	if _, err := bus.Register(context.Background(), RegisterRequest{SessionID: "old-session", Provider: "codex"}); err != nil {
		t.Fatalf("register old-session: %v", err)
	}
	if _, err := bus.Register(context.Background(), RegisterRequest{SessionID: "new-session", Provider: "gemini"}); err != nil {
		t.Fatalf("register new-session: %v", err)
	}

	now = now.Add(2 * time.Second)
	if _, err := bus.Register(context.Background(), RegisterRequest{SessionID: "fresh", Provider: "scaffold"}); err != nil {
		t.Fatalf("register fresh: %v", err)
	}

	sessions := bus.List(context.Background())
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session after prune, got %d", len(sessions))
	}
	if sessions[0].SessionID != "fresh" {
		t.Fatalf("expected fresh session, got %q", sessions[0].SessionID)
	}
}
