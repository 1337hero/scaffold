package main

import (
	"testing"

	"scaffold/db"
)

func TestHistoryToThreadChronologicalAndRoleMapping(t *testing.T) {
	// Input is newest-first (as returned from DB query).
	history := []db.Capture{
		{Raw: "latest user", Source: "signal:user:+15550001111"},
		{Raw: "assistant reply", Source: "signal:assistant:+15550001111"},
		{Raw: "first user", Source: "signal:user:+15550001111"},
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
	history := []db.Capture{
		{Raw: "  ", Source: "signal:user:+15550001111"},
		{Raw: "ok", Source: "signal:user:+15550001111"},
	}

	thread := historyToThread(history)
	if len(thread) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(thread))
	}
	if thread[0].Content != "ok" {
		t.Fatalf("unexpected content: %+v", thread[0])
	}
}
