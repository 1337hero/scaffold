package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWebhookConfigMissingFile(t *testing.T) {
	cfg, found, err := LoadWebhookConfig("/nonexistent/webhooks.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if found {
		t.Fatal("expected found=false for missing file")
	}
	if cfg != nil {
		t.Fatal("expected nil config for missing file")
	}
}

func TestLoadWebhookConfigValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "webhooks.yaml")
	content := `
rate_limit:
  max: 30
  window_minutes: 10
tokens:
  fitness: tok-abc123
  homelab: tok-def456
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cfg, found, err := LoadWebhookConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if cfg.RateLimit.Max != 30 {
		t.Errorf("expected max=30, got %d", cfg.RateLimit.Max)
	}
	if cfg.RateLimit.WindowMinutes != 10 {
		t.Errorf("expected window_minutes=10, got %d", cfg.RateLimit.WindowMinutes)
	}
	if cfg.Tokens["fitness"] == nil || cfg.Tokens["fitness"].Token != "tok-abc123" {
		t.Errorf("expected fitness token tok-abc123, got %v", cfg.Tokens["fitness"])
	}
	if cfg.Tokens["homelab"] == nil || cfg.Tokens["homelab"].Token != "tok-def456" {
		t.Errorf("expected homelab token tok-def456, got %v", cfg.Tokens["homelab"])
	}
}

func TestLoadWebhookConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "webhooks.yaml")
	if err := os.WriteFile(path, []byte("tokens:\n  test: tok-xyz\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, _, err := LoadWebhookConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RateLimit.Max != 60 {
		t.Errorf("expected default max=60, got %d", cfg.RateLimit.Max)
	}
	if cfg.RateLimit.WindowMinutes != 60 {
		t.Errorf("expected default window=60, got %d", cfg.RateLimit.WindowMinutes)
	}
}

func TestLoadWebhookConfigStructTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "webhooks.yaml")
	content := `
tokens:
  test: tok-simple
  github:
    token: tok-gh-abc
    type: github
    secret: whsec_1234
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, found, err := LoadWebhookConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if cfg.Tokens["test"] == nil || cfg.Tokens["test"].Token != "tok-simple" {
		t.Errorf("expected simple token tok-simple, got %v", cfg.Tokens["test"])
	}
	if cfg.Tokens["test"].Type != "" {
		t.Errorf("expected empty type for simple token, got %q", cfg.Tokens["test"].Type)
	}
	gh := cfg.Tokens["github"]
	if gh == nil {
		t.Fatal("expected github token")
	}
	if gh.Token != "tok-gh-abc" {
		t.Errorf("expected token tok-gh-abc, got %q", gh.Token)
	}
	if gh.Type != "github" {
		t.Errorf("expected type github, got %q", gh.Type)
	}
	if gh.Secret != "whsec_1234" {
		t.Errorf("expected secret whsec_1234, got %q", gh.Secret)
	}
}

func TestLoadWebhookConfigInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "webhooks.yaml")
	if err := os.WriteFile(path, []byte("tokens:\n\t- bad\n  key: val"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, err := LoadWebhookConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
