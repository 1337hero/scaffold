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
	if len(cfg.Agent.Rules) == 0 {
		t.Fatal("expected at least one agent rule")
	}

	requiredRules := []string{
		"Never store raw message text as a memory. Distill and synthesize first.",
		`Never let the desk grow past 3. If he tries, gently hold the line — "let's finish one first."`,
	}
	seen := make(map[string]struct{}, len(cfg.Agent.Rules))
	for _, rule := range cfg.Agent.Rules {
		seen[rule] = struct{}{}
	}
	for _, rule := range requiredRules {
		if _, ok := seen[rule]; !ok {
			t.Errorf("missing required agent rule: %q", rule)
		}
	}
}

func TestToolsConfig(t *testing.T) {
	cfg, err := Load(configDir(t), "Mike")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Tools.Tools) != 18 {
		t.Errorf("expected 18 tools, got %d", len(cfg.Tools.Tools))
	}

	names := make(map[string]bool)
	for _, tool := range cfg.Tools.Tools {
		names[tool.Name] = true
	}
	expected := []string{
		"save_to_inbox",
		"search_memories",
		"get_inbox",
		"get_calendar_events",
		"create_calendar_event",
		"update_calendar_event",
		"list_sessions",
		"send_to_session",
		"search_email",
		"get_email",
	}
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
	if _, ok := cfg.Cortex.Tasks["prioritize"]; !ok {
		t.Error("missing cortex task: prioritize")
	}
	if _, ok := cfg.Cortex.Tasks["session_cleanup"]; !ok {
		t.Error("missing cortex task: session_cleanup")
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

	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("name: Test\n"), 0o644); err != nil {
		t.Fatalf("write agent.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tools.yaml"), []byte("tools: []\n"), 0o644); err != nil {
		t.Fatalf("write tools.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "triage.yaml"), []byte("prompt: test\n"), 0o644); err != nil {
		t.Fatalf("write triage.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cortex.yaml"), []byte("bulletin:\n  interval_minutes: 0\n"), 0o644); err != nil {
		t.Fatalf("write cortex.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "embedding.yaml"), []byte("provider: ollama\n"), 0o644); err != nil {
		t.Fatalf("write embedding.yaml: %v", err)
	}

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
	if _, ok := cfg.Cortex.Tasks["prioritize"]; !ok {
		t.Error("expected default prioritize task")
	}
	if _, ok := cfg.Cortex.Tasks["session_cleanup"]; !ok {
		t.Error("expected default session_cleanup task")
	}
	if cfg.LLM.Version != 1 {
		t.Errorf("expected default llm version 1, got %d", cfg.LLM.Version)
	}
	if _, ok := cfg.LLM.Routes[LLMRouteBrainRespond]; !ok {
		t.Errorf("expected default llm route %q", LLMRouteBrainRespond)
	}
	if _, ok := cfg.LLM.Profiles["respond_default"]; !ok {
		t.Error("expected default llm profile respond_default")
	}
}

func TestCustomLLMConfig(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("name: Test\nmodel: a\n"), 0o644); err != nil {
		t.Fatalf("write agent.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tools.yaml"), []byte("tools: []\n"), 0o644); err != nil {
		t.Fatalf("write tools.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "triage.yaml"), []byte("prompt: test\nmodel: b\n"), 0o644); err != nil {
		t.Fatalf("write triage.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cortex.yaml"), []byte("bulletin:\n  interval_minutes: 60\n  model: c\n"), 0o644); err != nil {
		t.Fatalf("write cortex.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "embedding.yaml"), []byte("provider: ollama\n"), 0o644); err != nil {
		t.Fatalf("write embedding.yaml: %v", err)
	}

	llmYAML := `
version: 1
providers:
  local:
    type: openai_compatible
    base_url: http://127.0.0.1:11434/v1
profiles:
  p1:
    provider: local
    model: qwen2.5:14b
routes:
  brain.respond:
    profile: p1
    required: true
  brain.triage:
    profile: p1
  brain.prioritize:
    profile: p1
  cortex.bulletin:
    profile: p1
  cortex.semantic:
    profile: p1
  cortex.observations:
    profile: p1
`
	if err := os.WriteFile(filepath.Join(dir, "llm.yaml"), []byte(llmYAML), 0o644); err != nil {
		t.Fatalf("write llm.yaml: %v", err)
	}

	cfg, err := Load(dir, "User")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.LLM.Providers["local"].Type != "openai_compatible" {
		t.Fatalf("expected local provider type openai_compatible, got %q", cfg.LLM.Providers["local"].Type)
	}
	if cfg.LLM.Routes[LLMRouteBrainRespond].Profile != "p1" {
		t.Fatalf("expected brain.respond profile p1, got %q", cfg.LLM.Routes[LLMRouteBrainRespond].Profile)
	}
}

func TestLLMLockProviderValidation(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("name: Test\n"), 0o644); err != nil {
		t.Fatalf("write agent.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tools.yaml"), []byte("tools: []\n"), 0o644); err != nil {
		t.Fatalf("write tools.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "triage.yaml"), []byte("prompt: test\n"), 0o644); err != nil {
		t.Fatalf("write triage.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cortex.yaml"), []byte("bulletin:\n  interval_minutes: 60\n"), 0o644); err != nil {
		t.Fatalf("write cortex.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "embedding.yaml"), []byte("provider: ollama\n"), 0o644); err != nil {
		t.Fatalf("write embedding.yaml: %v", err)
	}

	llmYAML := `
version: 1
providers:
  p1:
    type: anthropic
  p2:
    type: openai_compatible
    base_url: http://127.0.0.1:11434/v1
profiles:
  a:
    provider: p1
    model: m1
  b:
    provider: p2
    model: m2
routes:
  brain.respond:
    profile: a
    fallback_profiles: [b]
    lock_provider: true
  brain.triage:
    profile: a
  brain.prioritize:
    profile: a
  cortex.bulletin:
    profile: a
  cortex.semantic:
    profile: a
  cortex.observations:
    profile: a
`
	if err := os.WriteFile(filepath.Join(dir, "llm.yaml"), []byte(llmYAML), 0o644); err != nil {
		t.Fatalf("write llm.yaml: %v", err)
	}

	if _, err := Load(dir, "User"); err == nil {
		t.Fatal("expected lock_provider validation to fail")
	}
}

func TestLoadFailsOnInvalidTaskInterval(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("name: Test\n"), 0o644); err != nil {
		t.Fatalf("write agent.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tools.yaml"), []byte("tools: []\n"), 0o644); err != nil {
		t.Fatalf("write tools.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "triage.yaml"), []byte("prompt: test\n"), 0o644); err != nil {
		t.Fatalf("write triage.yaml: %v", err)
	}

	cortexYAML := `
bulletin:
  interval_minutes: 60
tasks:
  prioritize:
    interval_hours: 0
    timeout_seconds: 30
`
	if err := os.WriteFile(filepath.Join(dir, "cortex.yaml"), []byte(cortexYAML), 0o644); err != nil {
		t.Fatalf("write cortex.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "embedding.yaml"), []byte("provider: ollama\n"), 0o644); err != nil {
		t.Fatalf("write embedding.yaml: %v", err)
	}

	if _, err := Load(dir, "User"); err == nil {
		t.Fatal("expected invalid cortex task interval to fail")
	}
}

func TestLoadFailsOnDuplicateToolName(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("name: Test\n"), 0o644); err != nil {
		t.Fatalf("write agent.yaml: %v", err)
	}
	toolsYAML := `
tools:
  - name: search
    description: one
    input_schema: {}
  - name: search
    description: two
    input_schema: {}
`
	if err := os.WriteFile(filepath.Join(dir, "tools.yaml"), []byte(toolsYAML), 0o644); err != nil {
		t.Fatalf("write tools.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "triage.yaml"), []byte("prompt: test\n"), 0o644); err != nil {
		t.Fatalf("write triage.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cortex.yaml"), []byte("bulletin:\n  interval_minutes: 60\n"), 0o644); err != nil {
		t.Fatalf("write cortex.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "embedding.yaml"), []byte("provider: ollama\n"), 0o644); err != nil {
		t.Fatalf("write embedding.yaml: %v", err)
	}

	if _, err := Load(dir, "User"); err == nil {
		t.Fatal("expected duplicate tool names to fail")
	}
}

func TestLoadFailsOnUnsupportedEmbeddingProvider(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "agent.yaml"), []byte("name: Test\n"), 0o644); err != nil {
		t.Fatalf("write agent.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tools.yaml"), []byte("tools: []\n"), 0o644); err != nil {
		t.Fatalf("write tools.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "triage.yaml"), []byte("prompt: test\n"), 0o644); err != nil {
		t.Fatalf("write triage.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cortex.yaml"), []byte("bulletin:\n  interval_minutes: 60\n"), 0o644); err != nil {
		t.Fatalf("write cortex.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "embedding.yaml"), []byte("provider: fake-provider\n"), 0o644); err != nil {
		t.Fatalf("write embedding.yaml: %v", err)
	}

	if _, err := Load(dir, "User"); err == nil {
		t.Fatal("expected unsupported embedding provider to fail")
	}
}
