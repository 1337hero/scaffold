package config

import (
	"strings"
	"testing"
)

func TestApplyLLMDefaults_ZeroValue(t *testing.T) {
	cfg := &Config{}
	applyLLMDefaults(cfg)

	if cfg.LLM.Version != 1 {
		t.Errorf("version: got %d, want 1", cfg.LLM.Version)
	}
	if _, ok := cfg.LLM.Providers["anthropic_main"]; !ok {
		t.Fatal("expected default provider anthropic_main")
	}
	p := cfg.LLM.Providers["anthropic_main"]
	if p.Type != llmProviderAnthropic {
		t.Errorf("provider type: got %q, want %q", p.Type, llmProviderAnthropic)
	}
	if p.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Errorf("api_key_env: got %q, want ANTHROPIC_API_KEY", p.APIKeyEnv)
	}

	for _, name := range []string{"respond_default", "triage_default", "cortex_default"} {
		if _, ok := cfg.LLM.Profiles[name]; !ok {
			t.Errorf("missing default profile %q", name)
		}
	}

	allRoutes := []string{
		LLMRouteBrainRespond,
		LLMRouteBrainTriage,
		LLMRouteBrainPrioritize,
		LLMRouteCortexBulletin,
		LLMRouteCortexSemantic,
		LLMRouteCortexObservations,
	}
	for _, r := range allRoutes {
		if _, ok := cfg.LLM.Routes[r]; !ok {
			t.Errorf("missing default route %q", r)
		}
	}
}

func TestApplyLLMDefaults_PartialRoutes(t *testing.T) {
	cfg := &Config{}
	cfg.LLM.Providers = map[string]LLMProviderConfig{
		"my_prov": {Type: llmProviderAnthropic},
	}
	cfg.LLM.Profiles = map[string]LLMProfileConfig{
		"custom": {Provider: "my_prov", Model: "m1"},
	}
	cfg.LLM.Routes = map[string]LLMRouteConfig{
		LLMRouteBrainRespond: {Profile: "custom", Required: false},
	}

	applyLLMDefaults(cfg)

	if cfg.LLM.Routes[LLMRouteBrainRespond].Profile != "custom" {
		t.Error("existing route was overwritten")
	}
	if cfg.LLM.Routes[LLMRouteBrainRespond].Required != false {
		t.Error("existing route Required flag was overwritten")
	}

	for _, r := range []string{
		LLMRouteBrainTriage,
		LLMRouteBrainPrioritize,
		LLMRouteCortexBulletin,
		LLMRouteCortexSemantic,
		LLMRouteCortexObservations,
	} {
		if _, ok := cfg.LLM.Routes[r]; !ok {
			t.Errorf("missing filled-in route %q", r)
		}
	}
}

func TestValidateLLM_ProfileReferencesUnknownProvider(t *testing.T) {
	cfg := validLLMConfig()
	cfg.LLM.Profiles["bad"] = LLMProfileConfig{Provider: "ghost", Model: "x"}

	err := validateLLM(cfg)
	if err == nil {
		t.Fatal("expected error for unknown provider reference")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error should mention provider name, got: %v", err)
	}
}

func TestValidateLLM_RouteReferencesUnknownProfile(t *testing.T) {
	cfg := validLLMConfig()
	cfg.LLM.Routes[LLMRouteBrainRespond] = LLMRouteConfig{Profile: "nope"}

	err := validateLLM(cfg)
	if err == nil {
		t.Fatal("expected error for unknown profile reference")
	}
	if !strings.Contains(err.Error(), LLMRouteBrainRespond) {
		t.Errorf("error should mention route name, got: %v", err)
	}
}

func TestValidateLLM_LockProviderMismatch(t *testing.T) {
	cfg := validLLMConfig()
	cfg.LLM.Providers["other"] = LLMProviderConfig{Type: llmProviderAnthropic}
	cfg.LLM.Profiles["fb"] = LLMProfileConfig{Provider: "other", Model: "x"}
	cfg.LLM.Routes[LLMRouteBrainRespond] = LLMRouteConfig{
		Profile:          "p",
		FallbackProfiles: []string{"fb"},
		LockProvider:     true,
	}

	err := validateLLM(cfg)
	if err == nil {
		t.Fatal("expected error for lock_provider mismatch")
	}
	if !strings.Contains(err.Error(), "lock_provider") {
		t.Errorf("error should mention lock_provider, got: %v", err)
	}
}

func TestValidateLLM_WarmupUnknownRoute(t *testing.T) {
	cfg := validLLMConfig()
	cfg.LLM.Startup.WarmupRoutes = []string{"nonexistent.route"}

	err := validateLLM(cfg)
	if err == nil {
		t.Fatal("expected error for unknown warmup route")
	}
	if !strings.Contains(err.Error(), "nonexistent.route") {
		t.Errorf("error should mention route name, got: %v", err)
	}
}

func TestValidateLLM_ValidConfig(t *testing.T) {
	cfg := validLLMConfig()
	if err := validateLLM(cfg); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestApplyRouteEnvOverrides_ValidOverride(t *testing.T) {
	cfg := validLLMConfig()
	cfg.LLM.Profiles["fast"] = LLMProfileConfig{Provider: "prov", Model: "fast-model"}

	ApplyRouteEnvOverrides(&cfg.LLM, func(key string) string {
		if key == "LLM_ROUTE_BRAIN_RESPOND" {
			return "fast"
		}
		return ""
	})

	if cfg.LLM.Routes[LLMRouteBrainRespond].Profile != "fast" {
		t.Errorf("expected profile override to fast, got %q", cfg.LLM.Routes[LLMRouteBrainRespond].Profile)
	}
}

func TestApplyRouteEnvOverrides_UnknownProfileSkipped(t *testing.T) {
	cfg := validLLMConfig()
	original := cfg.LLM.Routes[LLMRouteBrainRespond].Profile

	ApplyRouteEnvOverrides(&cfg.LLM, func(key string) string {
		if key == "LLM_ROUTE_BRAIN_RESPOND" {
			return "does_not_exist"
		}
		return ""
	})

	if cfg.LLM.Routes[LLMRouteBrainRespond].Profile != original {
		t.Errorf("profile should be unchanged, got %q", cfg.LLM.Routes[LLMRouteBrainRespond].Profile)
	}
}

func validLLMConfig() *Config {
	cfg := &Config{}
	cfg.LLM = LLMConfig{
		Version: 1,
		Providers: map[string]LLMProviderConfig{
			"prov": {Type: llmProviderAnthropic},
		},
		Profiles: map[string]LLMProfileConfig{
			"p": {Provider: "prov", Model: "m"},
		},
		Routes: map[string]LLMRouteConfig{
			LLMRouteBrainRespond:       {Profile: "p"},
			LLMRouteBrainTriage:        {Profile: "p"},
			LLMRouteBrainPrioritize:    {Profile: "p"},
			LLMRouteCortexBulletin:     {Profile: "p"},
			LLMRouteCortexSemantic:     {Profile: "p"},
			LLMRouteCortexObservations: {Profile: "p"},
		},
	}
	return cfg
}
