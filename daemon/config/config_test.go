package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func configDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "config")
	if _, err := os.Stat(filepath.Join(dir, "agent.yaml")); err != nil {
		t.Fatalf("config dir not found at %s: %v", dir, err)
	}
	return dir
}

func TestLoadSuccess(t *testing.T) {
	cfg, err := Load(configDir(t), "Mike")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
}

func TestAgentConfig(t *testing.T) {
	cfg, err := Load(configDir(t), "Mike")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Agent.Name != "Scaffold" {
		t.Errorf("expected agent name Scaffold, got %q", cfg.Agent.Name)
	}
	if cfg.Agent.MaxResponseTokens != 1024 {
		t.Errorf("expected max_response_tokens 1024, got %d", cfg.Agent.MaxResponseTokens)
	}
	if cfg.Agent.Model != "claude-haiku-4-5" {
		t.Errorf("expected model claude-haiku-4-5, got %q", cfg.Agent.Model)
	}
	if len(cfg.Agent.Rules) != 4 {
		t.Errorf("expected 4 rules, got %d", len(cfg.Agent.Rules))
	}
}

func TestToolsConfig(t *testing.T) {
	cfg, err := Load(configDir(t), "Mike")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Tools.Tools) != 6 {
		t.Errorf("expected 6 tools, got %d", len(cfg.Tools.Tools))
	}

	names := make(map[string]bool)
	for _, tool := range cfg.Tools.Tools {
		names[tool.Name] = true
	}
	expected := []string{"save_to_inbox", "get_desk", "search_memories", "update_desk_item", "get_inbox", "add_to_notebook"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestCortexConfig(t *testing.T) {
	cfg, err := Load(configDir(t), "Mike")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Cortex.Bulletin.IntervalMinutes != 60 {
		t.Errorf("expected bulletin interval 60, got %d", cfg.Cortex.Bulletin.IntervalMinutes)
	}
	if cfg.Cortex.Bulletin.MaxWords != 500 {
		t.Errorf("expected bulletin max_words 500, got %d", cfg.Cortex.Bulletin.MaxWords)
	}
	if cfg.Cortex.Bulletin.MaxStaleMultiplier != 3 {
		t.Errorf("expected max_stale_multiplier 3, got %d", cfg.Cortex.Bulletin.MaxStaleMultiplier)
	}

	if _, ok := cfg.Cortex.Tasks["consolidation"]; !ok {
		t.Error("missing cortex task: consolidation")
	}
	if _, ok := cfg.Cortex.Tasks["decay"]; !ok {
		t.Error("missing cortex task: decay")
	}
	if _, ok := cfg.Cortex.Tasks["prune"]; !ok {
		t.Error("missing cortex task: prune")
	}
	if _, ok := cfg.Cortex.Tasks["reindex"]; !ok {
		t.Error("missing cortex task: reindex")
	}

	decay := cfg.Cortex.Tasks["decay"]
	if decay.Factor != 0.95 {
		t.Errorf("expected decay factor 0.95, got %f", decay.Factor)
	}
}

func TestTemplateSubstitution(t *testing.T) {
	cfg, err := Load(configDir(t), "Mike")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if strings.Contains(cfg.Agent.Personality, "{name}") {
		t.Error("agent personality still contains {name} placeholder")
	}
	if strings.Contains(cfg.Agent.Personality, "{user_name}") {
		t.Error("agent personality still contains {user_name} placeholder")
	}
	if !strings.Contains(cfg.Agent.Personality, "Scaffold") {
		t.Error("agent personality should contain substituted name 'Scaffold'")
	}
	if !strings.Contains(cfg.Agent.Personality, "Mike") {
		t.Error("agent personality should contain substituted user_name 'Mike'")
	}

	if strings.Contains(cfg.Triage.Prompt, "{user_name}") {
		t.Error("triage prompt still contains {user_name} placeholder")
	}
	if !strings.Contains(cfg.Triage.Prompt, "Mike") {
		t.Error("triage prompt should contain substituted user_name 'Mike'")
	}
}

func TestDefaults(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("name: Test\n"), 0644)
	os.WriteFile(filepath.Join(dir, "tools.yaml"), []byte("tools: []\n"), 0644)
	os.WriteFile(filepath.Join(dir, "triage.yaml"), []byte("prompt: test\n"), 0644)
	os.WriteFile(filepath.Join(dir, "cortex.yaml"), []byte("bulletin:\n  interval_minutes: 0\n"), 0644)

	cfg, err := Load(dir, "User")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Agent.MaxResponseTokens != 1024 {
		t.Errorf("expected default max_response_tokens 1024, got %d", cfg.Agent.MaxResponseTokens)
	}
	if cfg.Agent.Model != "claude-haiku-4-5" {
		t.Errorf("expected default model claude-haiku-4-5, got %q", cfg.Agent.Model)
	}
	if cfg.Cortex.Bulletin.IntervalMinutes != 60 {
		t.Errorf("expected default bulletin interval 60, got %d", cfg.Cortex.Bulletin.IntervalMinutes)
	}
	if cfg.Cortex.Bulletin.MaxWords != 500 {
		t.Errorf("expected default bulletin max_words 500, got %d", cfg.Cortex.Bulletin.MaxWords)
	}
}
