package brain

import (
	"errors"
	"testing"
)

func TestParseTriageContentValidJSON(t *testing.T) {
	raw := "renew example.com domain"
	content := `{"type":"Todo","importance":0.8,"action":"do","title":"Renew domain","micro_steps":["Open registrar","Pay invoice"],"tags":["ops"]}`

	result, err := parseTriageContent(raw, content)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Type != "Todo" {
		t.Fatalf("expected type Todo, got %s", result.Type)
	}
	if result.Action != "do" {
		t.Fatalf("expected action do, got %s", result.Action)
	}
	if result.Title != "Renew domain" {
		t.Fatalf("expected title Renew domain, got %s", result.Title)
	}
}

func TestParseTriageContentInvalidJSONFallsBackWithError(t *testing.T) {
	raw := "capture text"

	result, err := parseTriageContent(raw, "{not-json")
	if err == nil {
		t.Fatal("expected degraded error, got nil")
	}
	var degraded *TriageDegradedError
	if !errors.As(err, &degraded) {
		t.Fatalf("expected TriageDegradedError, got %T", err)
	}
	if result.Action != "reference" {
		t.Fatalf("expected fallback action reference, got %s", result.Action)
	}
	if result.Title != raw {
		t.Fatalf("expected fallback title %q, got %q", raw, result.Title)
	}
}

func TestParseTriageContentMissingFieldsFallsBackWithError(t *testing.T) {
	raw := "capture text"
	content := `{"type":"Todo","importance":0.8}`

	result, err := parseTriageContent(raw, content)
	if err == nil {
		t.Fatal("expected degraded error, got nil")
	}
	if result.Type != "Observation" {
		t.Fatalf("expected fallback type Observation, got %s", result.Type)
	}
}
