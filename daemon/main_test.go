package main

import (
	"testing"

	"scaffold/brain"
	"scaffold/db"
)

func TestHistoryToThreadChronologicalAndRoleMapping(t *testing.T) {
	// Input is chronological (as returned from conversation_log query).
	history := []db.ConversationEntry{
		{Role: "user", Content: "first user"},
		{Role: "assistant", Content: "assistant reply"},
		{Role: "user", Content: "latest user"},
	}

	thread := historyToThread(history)
	if len(thread) != 3 {
		t.Fatalf("expected 3 turns, got %d", len(thread))
	}

	if thread[0].Role != "user" || thread[0].Content != "first user" {
		t.Fatalf("unexpected first turn: %+v", thread[0])
	}
	if thread[1].Role != "assistant" || thread[1].Content != "assistant reply" {
		t.Fatalf("unexpected second turn: %+v", thread[1])
	}
	if thread[2].Role != "user" || thread[2].Content != "latest user" {
		t.Fatalf("unexpected third turn: %+v", thread[2])
	}
}

func TestHistoryToThreadSkipsEmptyMessages(t *testing.T) {
	history := []db.ConversationEntry{
		{Role: "assistant", Content: "  "},
		{Role: "not-a-role", Content: "ok"},
	}

	thread := historyToThread(history)
	if len(thread) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(thread))
	}
	if thread[0].Content != "ok" {
		t.Fatalf("unexpected content: %+v", thread[0])
	}
	if thread[0].Role != "user" {
		t.Fatalf("expected fallback user role, got %s", thread[0].Role)
	}
}

func TestEnsureCurrentUserMessage(t *testing.T) {
	thread := []brain.ConversationTurn{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}

	updated := ensureCurrentUserMessage(thread, "new message")
	if len(updated) != 3 {
		t.Fatalf("expected 3 turns, got %d", len(updated))
	}
	if updated[2].Role != "user" || updated[2].Content != "new message" {
		t.Fatalf("unexpected final turn: %+v", updated[2])
	}

	deduped := ensureCurrentUserMessage(updated, "new message")
	if len(deduped) != 3 {
		t.Fatalf("expected no duplicate append, got %d turns", len(deduped))
	}
}
