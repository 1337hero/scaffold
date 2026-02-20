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
	Parameters  map[string]interface{} `yaml:"parameters"`
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
		cfg.Agent.MaxResponseTokens = 300
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
}

func substituteVars(cfg *Config, userName string) {
	r := strings.NewReplacer(
		"{name}", cfg.Agent.Name,
		"{user_name}", userName,
	)
	cfg.Agent.Personality = r.Replace(cfg.Agent.Personality)
	cfg.Triage.Prompt = r.Replace(cfg.Triage.Prompt)
}
