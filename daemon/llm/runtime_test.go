package llm

import (
	"context"
	"fmt"
	"strings"
	"testing"

	appconfig "scaffold/config"
)

type stubTestResponder struct{}

func (s *stubTestResponder) Respond(_ context.Context, _ ToolUseRequest) (*ToolUseResponse, error) {
	return &ToolUseResponse{Text: "stub"}, nil
}

func stubAnthropicFactory(apiKey string) (ToolUseResponder, CompletionClient) {
	return &stubTestResponder{}, &UnconfiguredCompletionClient{}
}

type countingHealthChecker struct {
	calls int
	err   error
}

func (c *countingHealthChecker) Check(_ context.Context) error {
	c.calls++
	return c.err
}

type stubProvider struct {
	supportsToolUse        bool
	supportsCompletionJSON bool
	supportsCompletionText bool
	responderErr           error
	completionErr          error
	healthChecker          HealthChecker
}

func (p *stubProvider) SupportsToolUse() bool        { return p.supportsToolUse }
func (p *stubProvider) SupportsCompletionJSON() bool { return p.supportsCompletionJSON }
func (p *stubProvider) SupportsCompletionText() bool { return p.supportsCompletionText }

func (p *stubProvider) NewResponder() (ToolUseResponder, error) {
	if p.responderErr != nil {
		return nil, p.responderErr
	}
	return &stubTestResponder{}, nil
}

func (p *stubProvider) NewCompletion() (CompletionClient, error) {
	if p.completionErr != nil {
		return nil, p.completionErr
	}
	return &UnconfiguredCompletionClient{}, nil
}

func (p *stubProvider) HealthCheck() HealthChecker {
	if p.healthChecker != nil {
		return p.healthChecker
	}
	return &noopHealthChecker{}
}

func TestBindResponderUsesFallback(t *testing.T) {
	cfg := appconfig.LLMConfig{
		Version: 1,
		Providers: map[string]appconfig.LLMProviderConfig{
			"local": {
				Type:            "openai_compatible",
				BaseURL:         "http://127.0.0.1:11434/v1",
				SupportsToolUse: false,
			},
			"anth": {
				Type:      "anthropic",
				APIKeyEnv: "ANTHROPIC_API_KEY",
			},
		},
		Profiles: map[string]appconfig.LLMProfileConfig{
			"p1": {Provider: "local", Model: "local-model"},
			"p2": {Provider: "anth", Model: "claude-haiku-4-5"},
		},
		Routes: map[string]appconfig.LLMRouteConfig{
			appconfig.LLMRouteBrainRespond: {
				Profile:          "p1",
				FallbackProfiles: []string{"p2"},
				Required:         true,
			},
		},
		Startup: appconfig.LLMStartupConfig{VerifyRequiredRoutes: true},
	}

	factories := map[string]ProviderFactory{
		"anthropic": stubAnthropicFactory,
	}

	runtime, err := NewRuntimeWithEnv(cfg, factories, func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "test-key"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("NewRuntimeWithEnv: %v", err)
	}

	responder, model, err := runtime.BindResponder(appconfig.LLMRouteBrainRespond)
	if err != nil {
		t.Fatalf("BindResponder: %v", err)
	}
	if responder == nil {
		t.Fatal("expected responder")
	}
	if model != "claude-haiku-4-5" {
		t.Fatalf("expected fallback model, got %q", model)
	}
}

func TestBindResponderFailsWhenNoToolUseProvider(t *testing.T) {
	cfg := appconfig.LLMConfig{
		Version: 1,
		Providers: map[string]appconfig.LLMProviderConfig{
			"local": {
				Type:            "openai_compatible",
				BaseURL:         "http://127.0.0.1:11434/v1",
				SupportsToolUse: false,
			},
		},
		Profiles: map[string]appconfig.LLMProfileConfig{
			"p1": {Provider: "local", Model: "qwen2.5"},
		},
		Routes: map[string]appconfig.LLMRouteConfig{
			appconfig.LLMRouteBrainRespond: {Profile: "p1", Required: true},
		},
	}

	runtime, err := NewRuntimeWithEnv(cfg, nil, func(string) string { return "" })
	if err != nil {
		t.Fatalf("NewRuntimeWithEnv: %v", err)
	}
	if _, _, err := runtime.BindResponder(appconfig.LLMRouteBrainRespond); err == nil {
		t.Fatal("expected bind failure")
	} else if !strings.Contains(err.Error(), "does not support tool_use") {
		t.Fatalf("expected tool_use capability error, got %v", err)
	}
}

func TestEnvOverrideValidProfile(t *testing.T) {
	cfg := appconfig.LLMConfig{
		Version: 1,
		Providers: map[string]appconfig.LLMProviderConfig{
			"anth": {
				Type:      "anthropic",
				APIKeyEnv: "ANTHROPIC_API_KEY",
			},
		},
		Profiles: map[string]appconfig.LLMProfileConfig{
			"default_respond": {Provider: "anth", Model: "claude-haiku-4-5"},
			"big_model":       {Provider: "anth", Model: "claude-sonnet-4-5"},
		},
		Routes: map[string]appconfig.LLMRouteConfig{
			appconfig.LLMRouteBrainRespond: {Profile: "default_respond", Required: true},
		},
		Startup: appconfig.LLMStartupConfig{VerifyRequiredRoutes: false},
	}

	factories := map[string]ProviderFactory{
		"anthropic": stubAnthropicFactory,
	}

	runtime, err := NewRuntimeWithEnv(cfg, factories, func(key string) string {
		switch key {
		case "ANTHROPIC_API_KEY":
			return "test-key"
		case "LLM_ROUTE_BRAIN_RESPOND":
			return "big_model"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("NewRuntimeWithEnv: %v", err)
	}

	_, model, err := runtime.BindResponder(appconfig.LLMRouteBrainRespond)
	if err != nil {
		t.Fatalf("BindResponder: %v", err)
	}
	if model != "claude-sonnet-4-5" {
		t.Fatalf("expected overridden model %q, got %q", "claude-sonnet-4-5", model)
	}
}

func TestEnvOverrideNonexistentProfileSkipped(t *testing.T) {
	cfg := appconfig.LLMConfig{
		Version: 1,
		Providers: map[string]appconfig.LLMProviderConfig{
			"anth": {
				Type:      "anthropic",
				APIKeyEnv: "ANTHROPIC_API_KEY",
			},
		},
		Profiles: map[string]appconfig.LLMProfileConfig{
			"default_respond": {Provider: "anth", Model: "claude-haiku-4-5"},
		},
		Routes: map[string]appconfig.LLMRouteConfig{
			appconfig.LLMRouteBrainRespond: {Profile: "default_respond", Required: true},
		},
		Startup: appconfig.LLMStartupConfig{VerifyRequiredRoutes: false},
	}

	factories := map[string]ProviderFactory{
		"anthropic": stubAnthropicFactory,
	}

	runtime, err := NewRuntimeWithEnv(cfg, factories, func(key string) string {
		switch key {
		case "ANTHROPIC_API_KEY":
			return "test-key"
		case "LLM_ROUTE_BRAIN_RESPOND":
			return "nonexistent_profile"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("NewRuntimeWithEnv: %v", err)
	}

	_, model, err := runtime.BindResponder(appconfig.LLMRouteBrainRespond)
	if err != nil {
		t.Fatalf("BindResponder: %v", err)
	}
	if model != "claude-haiku-4-5" {
		t.Fatalf("expected original model %q, got %q", "claude-haiku-4-5", model)
	}
}

func TestVerifyStartupChecksRequiredRoutes(t *testing.T) {
	cfg := appconfig.LLMConfig{
		Version: 1,
		Providers: map[string]appconfig.LLMProviderConfig{
			"anth": {
				Type:      "anthropic",
				APIKeyEnv: "ANTHROPIC_API_KEY",
			},
		},
		Profiles: map[string]appconfig.LLMProfileConfig{
			"p1": {Provider: "anth", Model: "claude-haiku-4-5"},
		},
		Routes: map[string]appconfig.LLMRouteConfig{
			appconfig.LLMRouteBrainRespond: {Profile: "p1", Required: true},
		},
		Startup: appconfig.LLMStartupConfig{VerifyRequiredRoutes: true},
	}

	factories := map[string]ProviderFactory{
		"anthropic": stubAnthropicFactory,
	}

	runtime, err := NewRuntimeWithEnv(cfg, factories, func(string) string { return "" })
	if err != nil {
		t.Fatalf("NewRuntimeWithEnv: %v", err)
	}
	if err := runtime.VerifyStartup(context.Background()); err == nil {
		t.Fatal("expected verification failure without anthropic key")
	}
}

func TestVerifyStartupWarmupUsesSelectedFallbackProvider(t *testing.T) {
	primaryChecker := &countingHealthChecker{}
	fallbackChecker := &countingHealthChecker{}

	runtime := &Runtime{
		cfg: appconfig.LLMConfig{
			Routes: map[string]appconfig.LLMRouteConfig{
				appconfig.LLMRouteBrainTriage: {
					Profile:          "primary_profile",
					FallbackProfiles: []string{"fallback_profile"},
					Required:         true,
				},
			},
			Profiles: map[string]appconfig.LLMProfileConfig{
				"primary_profile": {
					Provider: "primary_provider",
					Model:    "primary-model",
				},
				"fallback_profile": {
					Provider: "fallback_provider",
					Model:    "fallback-model",
				},
			},
			Startup: appconfig.LLMStartupConfig{
				VerifyRequiredRoutes: true,
				WarmupRoutes:         []string{appconfig.LLMRouteBrainTriage},
			},
		},
		providers: map[string]provider{
			"primary_provider": &stubProvider{
				supportsCompletionJSON: true,
				completionErr:          fmt.Errorf("primary unavailable"),
				healthChecker:          primaryChecker,
			},
			"fallback_provider": &stubProvider{
				supportsCompletionJSON: true,
				healthChecker:          fallbackChecker,
			},
		},
	}

	if err := runtime.VerifyStartup(context.Background()); err != nil {
		t.Fatalf("VerifyStartup: %v", err)
	}

	if primaryChecker.calls != 0 {
		t.Fatalf("expected primary health checker to be skipped, got %d calls", primaryChecker.calls)
	}
	if fallbackChecker.calls != 1 {
		t.Fatalf("expected fallback health checker to be called once, got %d", fallbackChecker.calls)
	}
}

func TestBindResponderFailsWhenFactoryReturnsNilResponder(t *testing.T) {
	cfg := appconfig.LLMConfig{
		Version: 1,
		Providers: map[string]appconfig.LLMProviderConfig{
			"custom_provider": {
				Type:      "custom_type",
				APIKeyEnv: "CUSTOM_API_KEY",
			},
		},
		Profiles: map[string]appconfig.LLMProfileConfig{
			"custom_profile": {Provider: "custom_provider", Model: "m1"},
		},
		Routes: map[string]appconfig.LLMRouteConfig{
			appconfig.LLMRouteBrainRespond: {Profile: "custom_profile", Required: true},
		},
	}

	factories := map[string]ProviderFactory{
		"custom_type": func(string) (ToolUseResponder, CompletionClient) {
			return nil, &UnconfiguredCompletionClient{}
		},
	}

	runtime, err := NewRuntimeWithEnv(cfg, factories, func(key string) string {
		if key == "CUSTOM_API_KEY" {
			return "test-key"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("NewRuntimeWithEnv: %v", err)
	}

	if _, _, err := runtime.BindResponder(appconfig.LLMRouteBrainRespond); err == nil {
		t.Fatal("expected bind failure")
	} else if !strings.Contains(err.Error(), "nil responder") {
		t.Fatalf("expected nil responder error, got %v", err)
	}
}

func TestBindCompletionFailsWhenFactoryReturnsNilCompletion(t *testing.T) {
	cfg := appconfig.LLMConfig{
		Version: 1,
		Providers: map[string]appconfig.LLMProviderConfig{
			"custom_provider": {
				Type:      "custom_type",
				APIKeyEnv: "CUSTOM_API_KEY",
			},
		},
		Profiles: map[string]appconfig.LLMProfileConfig{
			"custom_profile": {Provider: "custom_provider", Model: "m1"},
		},
		Routes: map[string]appconfig.LLMRouteConfig{
			appconfig.LLMRouteBrainTriage: {Profile: "custom_profile", Required: true},
		},
	}

	factories := map[string]ProviderFactory{
		"custom_type": func(string) (ToolUseResponder, CompletionClient) {
			return &stubTestResponder{}, nil
		},
	}

	runtime, err := NewRuntimeWithEnv(cfg, factories, func(key string) string {
		if key == "CUSTOM_API_KEY" {
			return "test-key"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("NewRuntimeWithEnv: %v", err)
	}

	if _, _, err := runtime.BindCompletion(appconfig.LLMRouteBrainTriage); err == nil {
		t.Fatal("expected bind failure")
	} else if !strings.Contains(err.Error(), "nil completion client") {
		t.Fatalf("expected nil completion client error, got %v", err)
	}
}
