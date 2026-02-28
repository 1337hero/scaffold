package config

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

const (
	LLMRouteBrainRespond       = "brain.respond"
	LLMRouteBrainTriage        = "brain.triage"
	LLMRouteBrainPrioritize    = "brain.prioritize"
	LLMRouteCortexBulletin     = "cortex.bulletin"
	LLMRouteCortexSemantic     = "cortex.semantic"
	LLMRouteCortexObservations = "cortex.observations"
)

const (
	llmProviderAnthropic        = "anthropic"
	llmProviderOpenAI           = "openai"
	llmProviderOpenAICompatible = "openai_compatible"
	llmProviderLocal            = "local" // passthrough — Pi resolves via models.json
)

type LLMConfig struct {
	Version   int                          `yaml:"version"`
	Providers map[string]LLMProviderConfig `yaml:"providers"`
	Profiles  map[string]LLMProfileConfig  `yaml:"profiles"`
	Routes    map[string]LLMRouteConfig    `yaml:"routes"`
	Startup   LLMStartupConfig             `yaml:"startup"`
}

type LLMProviderConfig struct {
	Type            string `yaml:"type"`
	APIKeyEnv       string `yaml:"api_key_env"`
	APIKey          string `yaml:"api_key"`
	BaseURL         string `yaml:"base_url"`
	TimeoutSeconds  int    `yaml:"timeout_seconds"`
	SupportsToolUse bool   `yaml:"supports_tool_use"`
}

type LLMProfileConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type LLMRouteConfig struct {
	Profile          string   `yaml:"profile"`
	FallbackProfiles []string `yaml:"fallback_profiles"`
	Required         bool     `yaml:"required"`
	LockProvider     bool     `yaml:"lock_provider"`
}

type LLMStartupConfig struct {
	VerifyRequiredRoutes bool     `yaml:"verify_required_routes"`
	WarmupRoutes         []string `yaml:"warmup_routes"`
}

func applyLLMDefaults(cfg *Config) {
	if cfg == nil {
		return
	}

	if cfg.LLM.Version == 0 {
		cfg.LLM.Version = 1
	}
	if cfg.LLM.Providers == nil {
		cfg.LLM.Providers = map[string]LLMProviderConfig{}
	}
	if cfg.LLM.Profiles == nil {
		cfg.LLM.Profiles = map[string]LLMProfileConfig{}
	}
	if cfg.LLM.Routes == nil {
		cfg.LLM.Routes = map[string]LLMRouteConfig{}
	}

	if len(cfg.LLM.Providers) == 0 {
		cfg.LLM.Providers["anthropic_main"] = LLMProviderConfig{
			Type:           llmProviderAnthropic,
			APIKeyEnv:      "ANTHROPIC_API_KEY",
			TimeoutSeconds: 30,
		}
	}

	defaultProvider := firstLLMProviderName(cfg.LLM.Providers)

	if len(cfg.LLM.Profiles) == 0 {
		cfg.LLM.Profiles["respond_default"] = LLMProfileConfig{
			Provider: defaultProvider,
			Model:    cfg.Agent.Model,
		}
		cfg.LLM.Profiles["triage_default"] = LLMProfileConfig{
			Provider: defaultProvider,
			Model:    cfg.Triage.Model,
		}
		cfg.LLM.Profiles["cortex_default"] = LLMProfileConfig{
			Provider: defaultProvider,
			Model:    cfg.Cortex.Bulletin.Model,
		}
	}

	if _, ok := cfg.LLM.Routes[LLMRouteBrainRespond]; !ok {
		cfg.LLM.Routes[LLMRouteBrainRespond] = LLMRouteConfig{
			Profile:  "respond_default",
			Required: true,
		}
	}
	if _, ok := cfg.LLM.Routes[LLMRouteBrainTriage]; !ok {
		cfg.LLM.Routes[LLMRouteBrainTriage] = LLMRouteConfig{
			Profile:  "triage_default",
			Required: true,
		}
	}
	if _, ok := cfg.LLM.Routes[LLMRouteBrainPrioritize]; !ok {
		cfg.LLM.Routes[LLMRouteBrainPrioritize] = LLMRouteConfig{
			Profile:  "triage_default",
			Required: true,
		}
	}
	if _, ok := cfg.LLM.Routes[LLMRouteCortexBulletin]; !ok {
		cfg.LLM.Routes[LLMRouteCortexBulletin] = LLMRouteConfig{
			Profile:  "cortex_default",
			Required: true,
		}
	}
	if _, ok := cfg.LLM.Routes[LLMRouteCortexSemantic]; !ok {
		cfg.LLM.Routes[LLMRouteCortexSemantic] = LLMRouteConfig{
			Profile:  "cortex_default",
			Required: true,
		}
	}
	if _, ok := cfg.LLM.Routes[LLMRouteCortexObservations]; !ok {
		cfg.LLM.Routes[LLMRouteCortexObservations] = LLMRouteConfig{
			Profile:  "cortex_default",
			Required: true,
		}
	}

	if !cfg.LLM.Startup.VerifyRequiredRoutes {
		cfg.LLM.Startup.VerifyRequiredRoutes = true
	}
}

func validateLLM(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if cfg.LLM.Version != 1 {
		return fmt.Errorf("llm.version must be 1")
	}
	if len(cfg.LLM.Providers) == 0 {
		return fmt.Errorf("llm.providers must not be empty")
	}
	if len(cfg.LLM.Profiles) == 0 {
		return fmt.Errorf("llm.profiles must not be empty")
	}
	if len(cfg.LLM.Routes) == 0 {
		return fmt.Errorf("llm.routes must not be empty")
	}

	for name, provider := range cfg.LLM.Providers {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("llm provider name must not be empty")
		}
		provider.Type = strings.ToLower(strings.TrimSpace(provider.Type))
		if provider.TimeoutSeconds <= 0 {
			provider.TimeoutSeconds = 30
		}
		switch provider.Type {
		case llmProviderAnthropic:
			// API key can come from env lookup at runtime; keep validation permissive.
		case llmProviderOpenAI:
			if strings.TrimSpace(provider.BaseURL) == "" {
				provider.BaseURL = "https://api.openai.com/v1"
			}
		case llmProviderOpenAICompatible:
			if strings.TrimSpace(provider.BaseURL) == "" {
				return fmt.Errorf("llm provider %q base_url is required for type %q", name, provider.Type)
			}
		case llmProviderLocal:
			// Passthrough provider — Pi resolves connection via its own models.json.
			// No validation needed here.
		default:
			return fmt.Errorf("llm provider %q has unsupported type %q", name, provider.Type)
		}
		cfg.LLM.Providers[name] = provider
	}

	for name, profile := range cfg.LLM.Profiles {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("llm profile name must not be empty")
		}
		if strings.TrimSpace(profile.Provider) == "" {
			return fmt.Errorf("llm profile %q provider must not be empty", name)
		}
		if strings.TrimSpace(profile.Model) == "" {
			return fmt.Errorf("llm profile %q model must not be empty", name)
		}
		if _, ok := cfg.LLM.Providers[profile.Provider]; !ok {
			return fmt.Errorf("llm profile %q references unknown provider %q", name, profile.Provider)
		}
	}

	for name, route := range cfg.LLM.Routes {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("llm route name must not be empty")
		}
		if strings.TrimSpace(route.Profile) == "" {
			return fmt.Errorf("llm route %q profile must not be empty", name)
		}
		if _, ok := cfg.LLM.Profiles[route.Profile]; !ok {
			return fmt.Errorf("llm route %q references unknown profile %q", name, route.Profile)
		}
		for _, fallback := range route.FallbackProfiles {
			fallback = strings.TrimSpace(fallback)
			if fallback == "" {
				return fmt.Errorf("llm route %q has blank fallback profile", name)
			}
			if _, ok := cfg.LLM.Profiles[fallback]; !ok {
				return fmt.Errorf("llm route %q references unknown fallback profile %q", name, fallback)
			}
		}
		if route.LockProvider {
			primaryProvider := cfg.LLM.Profiles[route.Profile].Provider
			for _, fallback := range route.FallbackProfiles {
				if cfg.LLM.Profiles[fallback].Provider != primaryProvider {
					return fmt.Errorf("llm route %q lock_provider requires all fallbacks to use provider %q", name, primaryProvider)
				}
			}
		}
	}

	for _, route := range cfg.LLM.Startup.WarmupRoutes {
		route = strings.TrimSpace(route)
		if route == "" {
			return fmt.Errorf("llm.startup.warmup_routes contains empty route")
		}
		if _, ok := cfg.LLM.Routes[route]; !ok {
			return fmt.Errorf("llm.startup.warmup_routes references unknown route %q", route)
		}
	}

	return nil
}

func ApplyRouteEnvOverrides(cfg *LLMConfig, getenv func(string) string) {
	for routeName := range cfg.Routes {
		envKey := "LLM_ROUTE_" + strings.ToUpper(strings.ReplaceAll(routeName, ".", "_"))
		val := strings.TrimSpace(getenv(envKey))
		if val == "" {
			continue
		}
		if _, ok := cfg.Profiles[val]; !ok {
			log.Printf("llm: env override %s=%s: profile %q not found, skipping", envKey, val, val)
			continue
		}
		r := cfg.Routes[routeName]
		r.Profile = val
		cfg.Routes[routeName] = r
	}
}

func firstLLMProviderName(providers map[string]LLMProviderConfig) string {
	if len(providers) == 0 {
		return ""
	}
	keys := make([]string, 0, len(providers))
	for name := range providers {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys[0]
}
