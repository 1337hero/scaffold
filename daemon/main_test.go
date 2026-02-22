package main

import (
	"path/filepath"
	"strings"
	"testing"

	"scaffold/brain"
	appconfig "scaffold/config"
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

func TestBuildAgentSystemPromptIncludesRulesAndBulletinToken(t *testing.T) {
	cfg := &appconfig.Config{
		Agent: appconfig.AgentConfig{
			Personality: "Base prompt",
			Rules:       []string{"Rule one", "Rule two"},
		},
	}

	prompt := buildAgentSystemPrompt(cfg)

	if !strings.Contains(prompt, "Base prompt") {
		t.Fatalf("expected base personality in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "Rules:") || !strings.Contains(prompt, "Rule one") {
		t.Fatalf("expected rules section in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "## Current Context") {
		t.Fatalf("expected context heading in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "{{cortex_bulletin}}") {
		t.Fatalf("expected bulletin token in prompt, got %q", prompt)
	}
}

func TestAnnotateUserMessageWithSignalMetadata(t *testing.T) {
	got := annotateUserMessageWithSignalMetadata("check this", "image and audio")
	if !strings.Contains(got, "check this") {
		t.Fatalf("expected original message, got %q", got)
	}
	if !strings.Contains(got, "Signal metadata: user also sent image and audio") {
		t.Fatalf("expected signal metadata note, got %q", got)
	}
}

func TestAnnotateUserMessageWithSignalMetadataNoSummary(t *testing.T) {
	got := annotateUserMessageWithSignalMetadata("plain text", "")
	if got != "plain text" {
		t.Fatalf("expected unchanged text, got %q", got)
	}
}

func TestDefaultIngestDirFromConfigDir(t *testing.T) {
	got := defaultIngestDir(filepath.Join("..", "config"))
	if !strings.HasSuffix(filepath.ToSlash(got), "/ingest") {
		t.Fatalf("expected ingest dir suffix /ingest, got %q", got)
	}
}
