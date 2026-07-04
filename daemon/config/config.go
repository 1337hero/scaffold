package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Google GoogleConfig
}

type GoogleConfig struct {
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	CalendarID   string   `yaml:"calendar_id"`
	Scopes       []string `yaml:"scopes"`
}

func Load(configDir string, _ string) (*Config, error) {
	cfg := &Config{}

	if err := loadFileOptional(filepath.Join(configDir, "google.yaml"), &cfg.Google); err != nil {
		return nil, fmt.Errorf("load google.yaml: %w", err)
	}

	applyDefaults(cfg)

	return cfg, nil
}

func loadFile(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}

func loadFileOptional(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return yaml.Unmarshal(data, target)
}

func applyDefaults(cfg *Config) {
	if cfg.Google.CalendarID == "" {
		cfg.Google.CalendarID = "primary"
	}
	if len(cfg.Google.Scopes) == 0 {
		cfg.Google.Scopes = []string{
			"https://www.googleapis.com/auth/calendar.events",
		}
	}
}
