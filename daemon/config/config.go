package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Agent  AgentConfig
	Tools  ToolsConfig
	Triage TriageConfig
	Cortex CortexConfig
}

type AgentConfig struct {
	Name              string   `yaml:"name"`
	Personality       string   `yaml:"personality"`
	Rules             []string `yaml:"rules"`
	MaxResponseTokens int      `yaml:"max_response_tokens"`
	Model             string   `yaml:"model"`
}

type ToolsConfig struct {
	Tools []ToolDef `yaml:"tools"`
}

type ToolDef struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	InputSchema map[string]interface{} `yaml:"input_schema"`
}

type TriageConfig struct {
	Prompt    string `yaml:"prompt"`
	Model     string `yaml:"model"`
	MaxTokens int    `yaml:"max_tokens"`
}

type CortexConfig struct {
	Bulletin BulletinConfig        `yaml:"bulletin"`
	Tasks    map[string]TaskConfig `yaml:"tasks"`
}

type BulletinConfig struct {
	IntervalMinutes    int    `yaml:"interval_minutes"`
	MaxWords           int    `yaml:"max_words"`
	MaxStaleMultiplier int    `yaml:"max_stale_multiplier"`
	Model              string `yaml:"model"`
}

type TaskConfig struct {
	IntervalHours   int      `yaml:"interval_hours"`
	TimeoutSeconds  int      `yaml:"timeout_seconds"`
	Factor          float64  `yaml:"factor,omitempty"`
	ExemptTypes     []string `yaml:"exempt_types,omitempty"`
	SuppressedDays  int      `yaml:"suppressed_days,omitempty"`
	ImportanceFloor float64  `yaml:"importance_floor,omitempty"`
}

func Load(configDir string, userName string) (*Config, error) {
	cfg := &Config{}

	if err := loadFile(filepath.Join(configDir, "agent.yaml"), &cfg.Agent); err != nil {
		return nil, fmt.Errorf("load agent.yaml: %w", err)
	}
	if err := loadFile(filepath.Join(configDir, "tools.yaml"), &cfg.Tools); err != nil {
		return nil, fmt.Errorf("load tools.yaml: %w", err)
	}
	if err := loadFile(filepath.Join(configDir, "triage.yaml"), &cfg.Triage); err != nil {
		return nil, fmt.Errorf("load triage.yaml: %w", err)
	}
	if err := loadFile(filepath.Join(configDir, "cortex.yaml"), &cfg.Cortex); err != nil {
		return nil, fmt.Errorf("load cortex.yaml: %w", err)
	}

	applyDefaults(cfg)
	substituteVars(cfg, userName)
	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadFile(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}

func applyDefaults(cfg *Config) {
	if cfg.Agent.Name == "" {
		cfg.Agent.Name = "Scaffold"
	}
	if cfg.Agent.MaxResponseTokens == 0 {
		cfg.Agent.MaxResponseTokens = 1024
	}
	if cfg.Agent.Model == "" {
		cfg.Agent.Model = "claude-haiku-4-5"
	}
	if cfg.Triage.Model == "" {
		cfg.Triage.Model = "claude-haiku-4-5"
	}
	if cfg.Triage.MaxTokens == 0 {
		cfg.Triage.MaxTokens = 300
	}
	if cfg.Cortex.Bulletin.IntervalMinutes == 0 {
		cfg.Cortex.Bulletin.IntervalMinutes = 60
	}
	if cfg.Cortex.Bulletin.MaxWords == 0 {
		cfg.Cortex.Bulletin.MaxWords = 500
	}
	if cfg.Cortex.Bulletin.MaxStaleMultiplier == 0 {
		cfg.Cortex.Bulletin.MaxStaleMultiplier = 3
	}
	if cfg.Cortex.Bulletin.Model == "" {
		cfg.Cortex.Bulletin.Model = "claude-haiku-4-5"
	}
	if cfg.Cortex.Tasks == nil {
		cfg.Cortex.Tasks = make(map[string]TaskConfig)
	}
	if _, ok := cfg.Cortex.Tasks["prioritize"]; !ok {
		cfg.Cortex.Tasks["prioritize"] = TaskConfig{
			IntervalHours:  24,
			TimeoutSeconds: 120,
		}
	}
	if _, ok := cfg.Cortex.Tasks["session_cleanup"]; !ok {
		cfg.Cortex.Tasks["session_cleanup"] = TaskConfig{
			IntervalHours:  24,
			TimeoutSeconds: 15,
		}
	}
}

func substituteVars(cfg *Config, userName string) {
	r := strings.NewReplacer(
		"{name}", cfg.Agent.Name,
		"{user_name}", userName,
	)
	cfg.Agent.Personality = r.Replace(cfg.Agent.Personality)
	cfg.Triage.Prompt = r.Replace(cfg.Triage.Prompt)
}

func validate(cfg *Config) error {
	if strings.TrimSpace(cfg.Agent.Name) == "" {
		return fmt.Errorf("agent.name must not be empty")
	}
	if cfg.Agent.MaxResponseTokens <= 0 {
		return fmt.Errorf("agent.max_response_tokens must be > 0")
	}
	if strings.TrimSpace(cfg.Agent.Model) == "" {
		return fmt.Errorf("agent.model must not be empty")
	}
	if strings.TrimSpace(cfg.Triage.Prompt) == "" {
		return fmt.Errorf("triage.prompt must not be empty")
	}
	if strings.TrimSpace(cfg.Triage.Model) == "" {
		return fmt.Errorf("triage.model must not be empty")
	}
	if cfg.Triage.MaxTokens <= 0 {
		return fmt.Errorf("triage.max_tokens must be > 0")
	}
	if cfg.Cortex.Bulletin.IntervalMinutes <= 0 {
		return fmt.Errorf("cortex.bulletin.interval_minutes must be > 0")
	}
	if cfg.Cortex.Bulletin.MaxWords <= 0 {
		return fmt.Errorf("cortex.bulletin.max_words must be > 0")
	}
	if cfg.Cortex.Bulletin.MaxStaleMultiplier <= 0 {
		return fmt.Errorf("cortex.bulletin.max_stale_multiplier must be > 0")
	}
	if strings.TrimSpace(cfg.Cortex.Bulletin.Model) == "" {
		return fmt.Errorf("cortex.bulletin.model must not be empty")
	}

	seenTools := make(map[string]struct{}, len(cfg.Tools.Tools))
	for i, tool := range cfg.Tools.Tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			return fmt.Errorf("tools[%d].name must not be empty", i)
		}
		if _, exists := seenTools[name]; exists {
			return fmt.Errorf("duplicate tool name %q", name)
		}
		seenTools[name] = struct{}{}
	}

	for name, task := range cfg.Cortex.Tasks {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("cortex task name must not be empty")
		}
		if task.IntervalHours <= 0 {
			return fmt.Errorf("cortex task %q interval_hours must be > 0", name)
		}
		if task.TimeoutSeconds <= 0 {
			return fmt.Errorf("cortex task %q timeout_seconds must be > 0", name)
		}
		if task.Factor != 0 && (task.Factor <= 0 || task.Factor >= 1) {
			return fmt.Errorf("cortex task %q factor must be between 0 and 1 (exclusive)", name)
		}
		if task.ImportanceFloor != 0 && (task.ImportanceFloor < 0 || task.ImportanceFloor > 1) {
			return fmt.Errorf("cortex task %q importance_floor must be between 0 and 1", name)
		}
		if task.SuppressedDays < 0 {
			return fmt.Errorf("cortex task %q suppressed_days must be >= 0", name)
		}
	}

	return nil
}
