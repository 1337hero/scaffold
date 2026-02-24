package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type WebhookConfig struct {
	RateLimit WebhookRateLimit          `yaml:"rate_limit"`
	Tokens    map[string]*WebhookToken  `yaml:"tokens"`
}

type WebhookRateLimit struct {
	Max           int `yaml:"max"`
	WindowMinutes int `yaml:"window_minutes"`
}

type WebhookToken struct {
	Token  string `yaml:"token"`
	Type   string `yaml:"type"`   // "github", "" for generic
	Secret string `yaml:"secret"` // HMAC secret for verification
}

func (t *WebhookToken) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		t.Token = value.Value
		return nil
	}
	type raw WebhookToken
	return value.Decode((*raw)(t))
}

func LoadWebhookConfig(path string) (*WebhookConfig, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read webhook config: %w", err)
	}

	var cfg WebhookConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, false, fmt.Errorf("parse webhook config: %w", err)
	}

	if cfg.RateLimit.Max == 0 {
		cfg.RateLimit.Max = 60
	}
	if cfg.RateLimit.WindowMinutes == 0 {
		cfg.RateLimit.WindowMinutes = 60
	}
	if cfg.Tokens == nil {
		cfg.Tokens = make(map[string]*WebhookToken)
	}

	return &cfg, true, nil
}
