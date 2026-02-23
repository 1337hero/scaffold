package llm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	appconfig "scaffold/config"
)

type ProviderFactory func(apiKey string) (ToolUseResponder, CompletionClient)

type routeRequirement struct {
	toolUse        bool
	completionJSON bool
	completionText bool
}

type Runtime struct {
	cfg       appconfig.LLMConfig
	providers map[string]provider
}

type provider interface {
	SupportsToolUse() bool
	SupportsCompletionJSON() bool
	SupportsCompletionText() bool
	NewResponder() (ToolUseResponder, error)
	NewCompletion() (CompletionClient, error)
	HealthCheck() HealthChecker
}

type externalProvider struct {
	apiKey        string
	factory       ProviderFactory
	healthChecker HealthChecker
}

func (p *externalProvider) SupportsToolUse() bool        { return true }
func (p *externalProvider) SupportsCompletionJSON() bool { return true }
func (p *externalProvider) SupportsCompletionText() bool { return true }

func (p *externalProvider) NewResponder() (ToolUseResponder, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return nil, fmt.Errorf("api key is empty")
	}
	r, _ := p.factory(p.apiKey)
	if r == nil {
		return nil, fmt.Errorf("factory returned nil responder")
	}
	return r, nil
}

func (p *externalProvider) NewCompletion() (CompletionClient, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return nil, fmt.Errorf("api key is empty")
	}
	_, c := p.factory(p.apiKey)
	if c == nil {
		return nil, fmt.Errorf("factory returned nil completion client")
	}
	return c, nil
}

func (p *externalProvider) HealthCheck() HealthChecker {
	if p.healthChecker != nil {
		return p.healthChecker
	}
	return &noopHealthChecker{}
}

type openAIProvider struct {
	baseURL                 string
	apiKey                  string
	timeout                 time.Duration
	supportsToolUse         bool
	nativeJSONFormat        bool
	useMaxCompletionTokens  bool
}

func (p *openAIProvider) SupportsToolUse() bool {
	return p.supportsToolUse
}

func (p *openAIProvider) SupportsCompletionJSON() bool {
	return true
}

func (p *openAIProvider) SupportsCompletionText() bool {
	return true
}

func (p *openAIProvider) NewResponder() (ToolUseResponder, error) {
	if !p.supportsToolUse {
		return nil, fmt.Errorf("provider does not support tool use")
	}
	client := newOpenAIClient(p.baseURL, p.apiKey, p.timeout, p.supportsToolUse, p.nativeJSONFormat, p.useMaxCompletionTokens)
	return client, nil
}

func (p *openAIProvider) NewCompletion() (CompletionClient, error) {
	client := newOpenAIClient(p.baseURL, p.apiKey, p.timeout, p.supportsToolUse, p.nativeJSONFormat, p.useMaxCompletionTokens)
	return client, nil
}

func (p *openAIProvider) HealthCheck() HealthChecker {
	return &openAIHealthChecker{baseURL: p.baseURL, apiKey: p.apiKey}
}

func NewRuntime(cfg appconfig.LLMConfig, factories map[string]ProviderFactory) (*Runtime, error) {
	return NewRuntimeWithEnv(cfg, factories, os.Getenv)
}

func NewRuntimeWithEnv(cfg appconfig.LLMConfig, factories map[string]ProviderFactory, getenv func(string) string) (*Runtime, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if factories == nil {
		factories = map[string]ProviderFactory{}
	}

	runtime := &Runtime{
		cfg:       cfg,
		providers: map[string]provider{},
	}
	for name, providerCfg := range cfg.Providers {
		provider, err := newProvider(providerCfg, factories, getenv)
		if err != nil {
			return nil, fmt.Errorf("llm provider %q: %w", name, err)
		}
		runtime.providers[name] = provider
	}

	appconfig.ApplyRouteEnvOverrides(&cfg, getenv)
	runtime.cfg = cfg

	return runtime, nil
}

func newProvider(cfg appconfig.LLMProviderConfig, factories map[string]ProviderFactory, getenv func(string) string) (provider, error) {
	providerType := strings.ToLower(strings.TrimSpace(cfg.Type))
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" && strings.TrimSpace(cfg.APIKeyEnv) != "" {
		apiKey = strings.TrimSpace(getenv(cfg.APIKeyEnv))
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if factory, ok := factories[providerType]; ok {
		ep := &externalProvider{apiKey: apiKey, factory: factory}
		switch providerType {
		case "anthropic":
			ep.healthChecker = &anthropicHealthChecker{apiKey: apiKey}
		}
		return ep, nil
	}

	switch providerType {
	case "openai":
		baseURL := strings.TrimSpace(cfg.BaseURL)
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		if apiKey == "" {
			return nil, fmt.Errorf("api key is required for provider type %q", providerType)
		}
		return &openAIProvider{
			baseURL:                baseURL,
			apiKey:                 apiKey,
			timeout:                timeout,
			supportsToolUse:        true,
			nativeJSONFormat:       true,
			useMaxCompletionTokens: true,
		}, nil
	case "openai_compatible":
		baseURL := strings.TrimSpace(cfg.BaseURL)
		if baseURL == "" {
			return nil, fmt.Errorf("base_url is required for provider type %q", providerType)
		}
		return &openAIProvider{
			baseURL:          baseURL,
			apiKey:           apiKey,
			timeout:          timeout,
			supportsToolUse:  cfg.SupportsToolUse,
			nativeJSONFormat: false,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported provider type %q", providerType)
	}
}

func (r *Runtime) VerifyStartup(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("runtime is nil")
	}
	if !r.cfg.Startup.VerifyRequiredRoutes {
		return nil
	}

	for routeName, route := range r.cfg.Routes {
		if !route.Required {
			continue
		}
		requirement := requirementForRoute(routeName)
		if err := r.verifyRoute(routeName, requirement); err != nil {
			return fmt.Errorf("required route %q: %w", routeName, err)
		}
	}
	for _, routeName := range r.cfg.Startup.WarmupRoutes {
		requirement := requirementForRoute(routeName)
		if err := r.verifyRoute(routeName, requirement); err != nil {
			return fmt.Errorf("warmup route %q: %w", routeName, err)
		}

		var selectedProvider provider
		switch {
		case requirement.toolUse:
			candidate, _, err := r.bindResponderCandidate(routeName)
			if err != nil {
				return fmt.Errorf("warmup route %q: bind responder for health check: %w", routeName, err)
			}
			selectedProvider = candidate.provider
		case requirement.completionJSON || requirement.completionText:
			candidate, _, err := r.bindCompletionCandidate(routeName)
			if err != nil {
				return fmt.Errorf("warmup route %q: bind completion for health check: %w", routeName, err)
			}
			selectedProvider = candidate.provider
		default:
			_, prov, err := r.resolveRoute(routeName, requirement)
			if err != nil {
				return fmt.Errorf("warmup route %q: resolve provider for health check: %w", routeName, err)
			}
			selectedProvider = prov
		}

		hCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := selectedProvider.HealthCheck().Check(hCtx); err != nil {
			cancel()
			return fmt.Errorf("warmup route %q health check: %w", routeName, err)
		}
		cancel()
	}
	return nil
}

func (r *Runtime) verifyRoute(routeName string, requirement routeRequirement) error {
	if requirement.toolUse {
		_, _, err := r.bindResponderCandidate(routeName)
		return err
	}
	if requirement.completionJSON || requirement.completionText {
		_, _, err := r.bindCompletionCandidate(routeName)
		return err
	}
	_, _, err := r.resolveRoute(routeName, requirement)
	return err
}

func (r *Runtime) BindResponder(routeName string) (ToolUseResponder, string, error) {
	candidate, responder, err := r.bindResponderCandidate(routeName)
	if err != nil {
		return nil, "", err
	}
	return responder, candidate.binding.Model, nil
}

func (r *Runtime) bindResponderCandidate(routeName string) (routeCandidate, ToolUseResponder, error) {
	candidates, _, err := r.routeCandidates(routeName, routeRequirement{toolUse: true})
	if err != nil {
		return routeCandidate{}, nil, err
	}
	buildErrors := make([]string, 0)
	for _, candidate := range candidates {
		responder, err := candidate.provider.NewResponder()
		if err != nil {
			buildErrors = append(buildErrors, fmt.Sprintf("profile %q provider %q responder: %v", candidate.binding.Profile, candidate.binding.Provider, err))
			continue
		}
		return candidate, responder, nil
	}
	if len(buildErrors) > 0 {
		return routeCandidate{}, nil, fmt.Errorf("%s", strings.Join(buildErrors, "; "))
	}
	return routeCandidate{}, nil, fmt.Errorf("no valid provider candidates for route %q", routeName)
}

func (r *Runtime) BindCompletion(routeName string) (CompletionClient, string, error) {
	candidate, completion, err := r.bindCompletionCandidate(routeName)
	if err != nil {
		return nil, "", err
	}
	return completion, candidate.binding.Model, nil
}

func (r *Runtime) bindCompletionCandidate(routeName string) (routeCandidate, CompletionClient, error) {
	req := requirementForRoute(routeName)
	if !req.completionJSON && !req.completionText {
		req.completionJSON = true
	}
	candidates, _, err := r.routeCandidates(routeName, req)
	if err != nil {
		return routeCandidate{}, nil, err
	}
	buildErrors := make([]string, 0)
	for _, candidate := range candidates {
		completion, err := candidate.provider.NewCompletion()
		if err != nil {
			buildErrors = append(buildErrors, fmt.Sprintf("profile %q provider %q completion: %v", candidate.binding.Profile, candidate.binding.Provider, err))
			continue
		}
		return candidate, completion, nil
	}
	if len(buildErrors) > 0 {
		return routeCandidate{}, nil, fmt.Errorf("%s", strings.Join(buildErrors, "; "))
	}
	return routeCandidate{}, nil, fmt.Errorf("no valid provider candidates for route %q", routeName)
}

type resolvedBinding struct {
	Route    string
	Profile  string
	Provider string
	Model    string
}

type routeCandidate struct {
	binding  resolvedBinding
	provider provider
}

func (r *Runtime) resolveRoute(routeName string, requirement routeRequirement) (resolvedBinding, provider, error) {
	candidates, _, err := r.routeCandidates(routeName, requirement)
	if err != nil {
		return resolvedBinding{}, nil, err
	}
	if len(candidates) == 0 {
		return resolvedBinding{}, nil, fmt.Errorf("no valid profiles available for route")
	}
	return candidates[0].binding, candidates[0].provider, nil
}

func (r *Runtime) routeCandidates(routeName string, requirement routeRequirement) ([]routeCandidate, []string, error) {
	if r == nil {
		return nil, nil, fmt.Errorf("runtime is nil")
	}

	route, ok := r.cfg.Routes[routeName]
	if !ok {
		return nil, nil, fmt.Errorf("route not configured")
	}

	profileNames := make([]string, 0, 1+len(route.FallbackProfiles))
	profileNames = append(profileNames, strings.TrimSpace(route.Profile))
	for _, fallback := range route.FallbackProfiles {
		profileNames = append(profileNames, strings.TrimSpace(fallback))
	}

	if route.LockProvider {
		expected := ""
		for _, profileName := range profileNames {
			profile, ok := r.cfg.Profiles[profileName]
			if !ok {
				continue
			}
			if expected == "" {
				expected = profile.Provider
				continue
			}
			if profile.Provider != expected {
				return nil, nil, fmt.Errorf("lock_provider requires all profiles to use provider %q", expected)
			}
		}
	}

	candidates := make([]routeCandidate, 0, len(profileNames))
	reasons := make([]string, 0)
	for _, profileName := range profileNames {
		if profileName == "" {
			continue
		}
		profile, ok := r.cfg.Profiles[profileName]
		if !ok {
			reasons = append(reasons, fmt.Sprintf("profile %q missing", profileName))
			continue
		}

		provider, ok := r.providers[profile.Provider]
		if !ok {
			reasons = append(reasons, fmt.Sprintf("provider %q missing for profile %q", profile.Provider, profileName))
			continue
		}
		if requirement.toolUse && !provider.SupportsToolUse() {
			reasons = append(reasons, fmt.Sprintf("profile %q provider %q does not support tool_use", profileName, profile.Provider))
			continue
		}
		if requirement.completionJSON && !provider.SupportsCompletionJSON() {
			reasons = append(reasons, fmt.Sprintf("profile %q provider %q does not support completion_json", profileName, profile.Provider))
			continue
		}
		if requirement.completionText && !provider.SupportsCompletionText() {
			reasons = append(reasons, fmt.Sprintf("profile %q provider %q does not support completion_text", profileName, profile.Provider))
			continue
		}

		candidates = append(candidates, routeCandidate{
			binding: resolvedBinding{
				Route:    routeName,
				Profile:  profileName,
				Provider: profile.Provider,
				Model:    profile.Model,
			},
			provider: provider,
		})
	}

	if len(candidates) == 0 {
		if len(reasons) == 0 {
			return nil, nil, fmt.Errorf("no valid profiles available for route")
		}
		return nil, reasons, fmt.Errorf("%s", strings.Join(reasons, "; "))
	}
	return candidates, reasons, nil
}

func requirementForRoute(routeName string) routeRequirement {
	switch routeName {
	case appconfig.LLMRouteBrainRespond:
		return routeRequirement{toolUse: true}
	case appconfig.LLMRouteBrainTriage:
		return routeRequirement{completionJSON: true}
	case appconfig.LLMRouteBrainPrioritize:
		return routeRequirement{completionJSON: true}
	case appconfig.LLMRouteCortexBulletin:
		return routeRequirement{completionText: true}
	case appconfig.LLMRouteCortexSemantic:
		return routeRequirement{completionJSON: true}
	case appconfig.LLMRouteCortexObservations:
		return routeRequirement{completionJSON: true}
	default:
		return routeRequirement{}
	}
}
