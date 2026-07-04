package config

import (
	"os"
	"path/filepath"
	"testing"
)

func configDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "config")
	if _, err := os.Stat(filepath.Join(dir, "google.yaml")); err != nil {
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

func TestGoogleConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "google.yaml"), []byte("client_id: test\nclient_secret: secret\n"), 0o644); err != nil {
		t.Fatalf("write google.yaml: %v", err)
	}
	cfg, err := Load(dir, "User")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Google.CalendarID != "primary" {
		t.Errorf("expected default calendar_id primary, got %q", cfg.Google.CalendarID)
	}
	if len(cfg.Google.Scopes) == 0 {
		t.Error("expected default google scopes")
	}
}

func TestGoogleConfigMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir, "User")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Google.CalendarID != "primary" {
		t.Errorf("expected default calendar_id primary, got %q", cfg.Google.CalendarID)
	}
}
