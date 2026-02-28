package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	"scaffold/agents"
	"scaffold/api"
	"scaffold/brain"
	appconfig "scaffold/config"
	"scaffold/cortex"
	"scaffold/db"
	"scaffold/embedding"
	googleauth "scaffold/google"
	"scaffold/ingestion"
	"scaffold/llm"
	"scaffold/sessionbus"
	signalcli "scaffold/signal"
)

const maxInFlightMessages = 4
const conversationHistoryLimit = 12
const signalNonTextSupportMessage = "I got your Signal message, but I currently can't view images, open attachments, or transcribe audio. Please send text, or paste a transcript/description."

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "auth" {
		handleAuthSubcommand(os.Args[2:])
		return
	}

	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Printf("timezone: %s", time.Now().Location())
	log.Println("scaffold daemon starting")

	if err := secureFileIfExists(".env"); err != nil {
		log.Fatalf("failed to secure .env: %v", err)
	}
	_ = godotenv.Load()

	cfg := loadConfig()

	database, err := db.Open(cfg.dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()
	if err := secureFileIfExists(cfg.dbPath); err != nil {
		log.Fatalf("failed to secure database file: %v", err)
	}
	log.Println("database open")

	appCfg, err := appconfig.Load(cfg.configDir, cfg.userName)
	if err != nil {
		log.Fatalf("failed to load config from %s: %v", cfg.configDir, err)
	}
	log.Printf("config loaded: agent=%s, %d tools, cortex bulletin every %dm",
		appCfg.Agent.Name, len(appCfg.Tools.Tools), appCfg.Cortex.Bulletin.IntervalMinutes)

	assistantName := cfg.assistantName
	if strings.TrimSpace(appCfg.Agent.Name) != "" {
		assistantName = appCfg.Agent.Name
	}

	toolDefs := make([]brain.ToolDefinition, 0, len(appCfg.Tools.Tools))
	for _, tool := range appCfg.Tools.Tools {
		toolDefs = append(toolDefs, brain.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	llmFactories := map[string]llm.ProviderFactory{
		"anthropic": func(apiKey string) (llm.ToolUseResponder, llm.CompletionClient) {
			return brain.NewAnthropicResponder(apiKey), brain.NewAnthropicCompletionClient(apiKey)
		},
	}
	llmRuntime, err := llm.NewRuntime(appCfg.LLM, llmFactories)
	if err != nil {
		log.Fatalf("failed to initialize llm runtime: %v", err)
	}
	if err := llmRuntime.VerifyStartup(context.Background()); err != nil {
		log.Fatalf("llm startup verification failed: %v", err)
	}

	respondResponder, respondModel, err := llmRuntime.BindResponder(appconfig.LLMRouteBrainRespond)
	if err != nil {
		log.Fatalf("bind llm route %s: %v", appconfig.LLMRouteBrainRespond, err)
	}
	triageCompletion, triageModel, err := llmRuntime.BindCompletion(appconfig.LLMRouteBrainTriage)
	if err != nil {
		log.Fatalf("bind llm route %s: %v", appconfig.LLMRouteBrainTriage, err)
	}
	prioritizeCompletion, prioritizeModel, err := llmRuntime.BindCompletion(appconfig.LLMRouteBrainPrioritize)
	if err != nil {
		log.Fatalf("bind llm route %s: %v", appconfig.LLMRouteBrainPrioritize, err)
	}
	bulletinCompletion, bulletinModel, err := llmRuntime.BindCompletion(appconfig.LLMRouteCortexBulletin)
	if err != nil {
		log.Fatalf("bind llm route %s: %v", appconfig.LLMRouteCortexBulletin, err)
	}
	semanticCompletion, semanticModel, err := llmRuntime.BindCompletion(appconfig.LLMRouteCortexSemantic)
	if err != nil {
		log.Fatalf("bind llm route %s: %v", appconfig.LLMRouteCortexSemantic, err)
	}
	observationsCompletion, observationsModel, err := llmRuntime.BindCompletion(appconfig.LLMRouteCortexObservations)
	if err != nil {
		log.Fatalf("bind llm route %s: %v", appconfig.LLMRouteCortexObservations, err)
	}

	coderCfg := loadCoderConfig(cfg.configDir, appCfg.LLM)

	b := brain.NewWithDependencies(database, brain.Config{
		AssistantName:    assistantName,
		UserName:         cfg.userName,
		SystemPrompt:     buildAgentSystemPrompt(appCfg),
		TriagePrompt:     appCfg.Triage.Prompt,
		RespondModel:     respondModel,
		TriageModel:      triageModel,
		PrioritizeModel:  prioritizeModel,
		RespondMaxTokens: appCfg.Agent.MaxResponseTokens,
		TriageMaxTokens:  appCfg.Triage.MaxTokens,
		Tools:            toolDefs,
		CodeDispatchCWD:  coderCfg.DefaultCWD,
	}, brain.Dependencies{
		Responder:            respondResponder,
		TriageCompletion:     triageCompletion,
		PrioritizeCompletion: prioritizeCompletion,
	})

	sessionBus := sessionbus.New(sessionbus.Config{
		SessionTTL:      15 * time.Minute,
		MaxQueuePerSess: 128,
		MaxMessageBytes: 32 * 1024,
	})
	b.SetSessionBus(sessionBus)
	if _, err := sessionBus.Register(context.Background(), sessionbus.RegisterRequest{
		SessionID: "scaffold-agent",
		Provider:  "scaffold",
		Name:      assistantName,
	}); err != nil {
		log.Printf("warn: session bus register scaffold-agent failed: %v", err)
	}

	coderSvc := agents.New(sessionBus, coderCfg)

	var embedder embedding.Embedder
	switch strings.ToLower(strings.TrimSpace(appCfg.Embedding.Provider)) {
	case "ollama":
		embedder = embedding.NewOllamaClient(
			appCfg.Embedding.URL,
			appCfg.Embedding.Model,
			appCfg.Embedding.Dimensions,
		)
	default:
		log.Fatalf("unsupported embedding provider %q", appCfg.Embedding.Provider)
	}
	b.SetEmbedder(embedder)

	var googleGmailClient *googleauth.GmailClient
	if gcfg := appCfg.Google; gcfg.ClientID != "" {
		store := &googleauth.DBTokenStore{DB: database, Provider: "google"}
		existing, err := store.Get()
		if err != nil {
			log.Printf("warn: failed to check Google token: %v", err)
		} else if existing == nil {
			log.Println("Google configured but not authenticated. Run: scaffold-daemon auth google")
		} else {
			oauthCfg := googleauth.NewOAuth2Config(gcfg)
			tokenSource := googleauth.TokenSource(oauthCfg, store)
			calClient, err := googleauth.NewCalendarClient(context.Background(), tokenSource, gcfg.CalendarID)
			if err != nil {
				log.Printf("warn: Google Calendar client failed: %v", err)
			} else {
				b.SetCalendarClient(calClient)
				log.Printf("Google Calendar connected (calendar=%s)", gcfg.CalendarID)
			}
			gmailClient, err := googleauth.NewGmailClient(context.Background(), tokenSource)
			if err != nil {
				log.Printf("warn: Gmail client failed: %v", err)
			} else {
				b.SetGmailClient(gmailClient)
				googleGmailClient = gmailClient
				log.Println("Gmail client connected")
			}
		}
	} else {
		log.Println("Google not configured, skipping")
	}

	cortexRuntime := cortex.NewWithLLM(database, b, appCfg.Cortex, embedder, cortex.LLMRoutes{
		Bulletin: cortex.LLMRoute{
			Client: bulletinCompletion,
			Model:  bulletinModel,
		},
		Semantic: cortex.LLMRoute{
			Client: semanticCompletion,
			Model:  semanticModel,
		},
		Observations: cortex.LLMRoute{
			Client: observationsCompletion,
			Model:  observationsModel,
		},
	})
	b.SetBulletinProvider(cortexRuntime.CurrentBulletin)
	cortexRuntime.SetSessionBus(sessionBus)
	cortexRuntime.SetNotificationsConfig(&appCfg.Notifications)
	if googleGmailClient != nil {
		cortexRuntime.SetGmailClient(googleGmailClient)
	}
	if gmailCfg := loadGmailConfig(cfg.configDir); gmailCfg != nil {
		cortexRuntime.SetGmailConfig(gmailCfg)
	}

	client := signalcli.NewClient(cfg.signalURL, cfg.agentNumber)
	var ingestService *ingestion.Service
	if svc, err := ingestion.New(database, b, cfg.ingestDir, time.Duration(cfg.ingestPollSecs)*time.Second); err != nil {
		log.Printf("ingestion disabled: %v", err)
	} else {
		ingestService = svc
		log.Printf("ingestion enabled: dir=%s poll=%ds", ingestService.Directory(), cfg.ingestPollSecs)
	}

	srv := api.New(database, b, cfg.apiToken, api.AuthConfig{
		AppUsername:          cfg.appUsername,
		AppPasswordHash:      cfg.appPasswordHash,
		SessionTTL:           time.Duration(cfg.sessionTTLHours) * time.Hour,
		CookieSecure:         cfg.cookieSecure,
		CookieDomain:         cfg.cookieDomain,
		LoginRateLimitWindow: time.Duration(cfg.loginRateLimitWindowSecs) * time.Second,
		LoginRateLimitMax:    cfg.loginRateLimitMax,
	})
	srv.SetSessionBus(sessionBus)
	srv.SetAgents(coderSvc)
	if ingestService != nil {
		srv.SetIngestor(ingestService)
	}
	webhookCfgPath := filepath.Join(cfg.configDir, "webhooks.yaml")
	webhookCfg, webhookFound, err := appconfig.LoadWebhookConfig(webhookCfgPath)
	if err != nil {
		log.Fatalf("webhook config: %v", err)
	}
	if webhookFound {
		srv.SetWebhookConfig(webhookCfg)
		log.Printf("webhooks: enabled (%d tokens configured)", len(webhookCfg.Tokens))
	} else {
		log.Printf("webhooks: disabled (config/webhooks.yaml not found)")
	}
	if err := srv.EnableFrontendServing(cfg.frontendDistDir); err != nil {
		log.Printf("frontend static serving disabled: %v", err)
	} else {
		log.Printf("frontend static serving from %s", cfg.frontendDistDir)
	}
	apiAddr := net.JoinHostPort(cfg.apiHost, cfg.apiPort)
	httpServer := srv.NewHTTPServer(apiAddr)
	go func() {
		log.Printf("API server listening on %s", apiAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API server failed: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cortexRuntime.Start(ctx)
	coderSvc.Start(ctx)
	if ingestService != nil {
		ingestService.Start(ctx)
	}

	go runProactiveNotifier(ctx, sessionBus, client, cfg.userNumber)

	if err := waitForSignal(client); err != nil {
		log.Fatalf("signal-cli not ready: %v", err)
	}
	log.Println("signal-cli connected")

	startupMsg := fmt.Sprintf("%s online. Brain active.", assistantName)
	if err := client.Send(context.Background(), cfg.userNumber, startupMsg); err != nil {
		log.Printf("warn: startup message failed: %v", err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("shutting down")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
			log.Printf("warn: API shutdown error: %v", err)
		}
	}()

	log.Printf("listening for messages on %s", cfg.agentNumber)
	slots := make(chan struct{}, maxInFlightMessages)
	for {
		err := client.StreamEvents(ctx, func(event signalcli.SSEEvent) {
			msg := signalcli.ParseInbound(event)
			if msg == nil {
				return
			}
			if msg.Sender != cfg.userNumber {
				log.Printf("ignoring message from %s", msg.Sender)
				return
			}
			log.Printf("<- %s (len=%d)", msg.Sender, len(msg.Message))
			slots <- struct{}{}
			go func(msg *signalcli.InboundMessage) {
				defer func() { <-slots }()
				handleMessage(client, b, database, msg)
			}(msg)
		})
		if ctx.Err() != nil {
			break
		}
		if err != nil {
			log.Printf("SSE stream error: %v -- reconnecting in 3s", err)
			time.Sleep(3 * time.Second)
		}
	}

	log.Println("daemon stopped")
}

func handleMessage(client *signalcli.Client, b *brain.Brain, database *db.DB, msg *signalcli.InboundMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userText := strings.TrimSpace(msg.Message)
	nonTextSummary := msg.NonTextContentSummary()

	if userText == "" && nonTextSummary != "" {
		userEntry := fmt.Sprintf("[Signal non-text content: %s]", nonTextSummary)
		if _, err := database.InsertConversationEntry(msg.Sender, "user", userEntry); err != nil {
			log.Printf("conversation insert error (user non-text): %v", err)
		}

		response := signalNonTextSupportMessage
		if err := client.Send(ctx, msg.Sender, response); err != nil {
			log.Printf("failed to send non-text capability response: %v", err)
		} else {
			log.Printf("-> %s (len=%d)", msg.Sender, len(response))
			if _, err := database.InsertConversationEntry(msg.Sender, "assistant", response); err != nil {
				log.Printf("conversation insert error (assistant non-text): %v", err)
			}
		}
		return
	}

	if userText == "" {
		log.Printf("ignoring empty inbound message from %s", msg.Sender)
		return
	}

	if _, err := database.InsertConversationEntry(msg.Sender, "user", userText); err != nil {
		log.Printf("conversation insert error (user): %v", err)
	}

	history, err := database.ListRecentConversation(msg.Sender, conversationHistoryLimit)
	if err != nil {
		log.Printf("history query error: %v", err)
	}

	brainMessage := annotateUserMessageWithSignalMetadata(userText, nonTextSummary)
	thread := historyToThread(history)
	if len(thread) > 0 {
		last := thread[len(thread)-1]
		if last.Role == "user" && strings.TrimSpace(last.Content) == userText {
			thread[len(thread)-1].Content = brainMessage
		}
	}
	thread = ensureCurrentUserMessage(thread, brainMessage)

	response, err := b.Respond(ctx, brainMessage, thread)
	if err != nil {
		log.Printf("brain error: %v", err)
		response = "Something went wrong on my end. Try again?"
	}

	if err := client.Send(ctx, msg.Sender, response); err != nil {
		log.Printf("failed to send response: %v", err)
	} else {
		log.Printf("-> %s (len=%d)", msg.Sender, len(response))
		if _, err := database.InsertConversationEntry(msg.Sender, "assistant", response); err != nil {
			log.Printf("conversation insert error (assistant): %v", err)
		}
	}
}

type config struct {
	configDir                string
	frontendDistDir          string
	ingestDir                string
	ingestPollSecs           int
	signalURL                string
	agentNumber              string
	userNumber               string
	assistantName            string
	userName                 string
	dbPath                   string
	apiHost                  string
	apiPort                  string
	apiToken                 string
	appUsername              string
	appPasswordHash          string
	sessionTTLHours          int
	cookieSecure             bool
	cookieDomain             string
	loginRateLimitWindowSecs int
	loginRateLimitMax        int
}

func loadConfig() config {
	sanitizeEnvValue := func(v string) string {
		v = strings.TrimSpace(v)
		if i := strings.Index(v, " #"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		return v
	}
	required := func(key string) string {
		v := sanitizeEnvValue(os.Getenv(key))
		if v == "" {
			log.Fatalf("%s is required", key)
		}
		return v
	}
	withDefault := func(key, def string) string {
		if v := sanitizeEnvValue(os.Getenv(key)); v != "" {
			return v
		}
		return def
	}
	parsePositiveInt := func(key, raw string, min int) int {
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || n < min {
			log.Fatalf("%s must be an integer >= %d, got %q", key, min, raw)
		}
		return n
	}
	parseBool := func(key, raw string) bool {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		default:
			log.Fatalf("%s must be a boolean (true/false), got %q", key, raw)
			return false
		}
	}

	configDir := withDefault("CONFIG_DIR", "./config")
	ingestDir := withDefault("INGEST_DIR", defaultIngestDir(configDir))
	apiPort := required("API_PORT")
	if p, err := strconv.Atoi(apiPort); err != nil || p < 1 || p > 65535 {
		log.Fatalf("API_PORT must be a valid port number, got %q", apiPort)
	}
	sessionTTLHours := parsePositiveInt("SESSION_TTL_HOURS", withDefault("SESSION_TTL_HOURS", "168"), 1)
	loginRateLimitWindowSecs := parsePositiveInt("LOGIN_RATE_LIMIT_WINDOW_SECS", withDefault("LOGIN_RATE_LIMIT_WINDOW_SECS", "300"), 1)
	loginRateLimitMax := parsePositiveInt("LOGIN_RATE_LIMIT_MAX_ATTEMPTS", withDefault("LOGIN_RATE_LIMIT_MAX_ATTEMPTS", "5"), 1)
	ingestPollSecs := parsePositiveInt("INGEST_POLL_SECS", withDefault("INGEST_POLL_SECS", "30"), 1)
	cookieSecure := parseBool("COOKIE_SECURE", withDefault("COOKIE_SECURE", "true"))

	return config{
		configDir:                configDir,
		frontendDistDir:          withDefault("FRONTEND_DIST_DIR", "../app/dist"),
		ingestDir:                ingestDir,
		ingestPollSecs:           ingestPollSecs,
		agentNumber:              required("AGENT_NUMBER"),
		userNumber:               required("USER_NUMBER"),
		signalURL:                required("SIGNAL_URL"),
		assistantName:            withDefault("ASSISTANT_NAME", "Scaffold"),
		userName:                 withDefault("USER_NAME", "User"),
		dbPath:                   withDefault("DB_PATH", "./scaffold.db"),
		apiHost:                  withDefault("API_HOST", "127.0.0.1"),
		apiPort:                  apiPort,
		apiToken:                 required("API_TOKEN"),
		appUsername:              required("APP_USERNAME"),
		appPasswordHash:          required("APP_PASSWORD_HASH"),
		sessionTTLHours:          sessionTTLHours,
		cookieSecure:             cookieSecure,
		cookieDomain:             withDefault("COOKIE_DOMAIN", ""),
		loginRateLimitWindowSecs: loginRateLimitWindowSecs,
		loginRateLimitMax:        loginRateLimitMax,
	}
}

func loadCoderConfig(configDir string, llmCfg appconfig.LLMConfig) agents.Config {
	type stepYAML struct {
		Name     string   `yaml:"name"`
		Prompt   string   `yaml:"prompt"`
		Tools    []string `yaml:"tools"`
		Thinking string   `yaml:"thinking"`
	}
	type chainYAML struct {
		Steps []stepYAML `yaml:"steps"`
	}
	type coderYAML struct {
		AllowedPaths  []string             `yaml:"allowed_paths"`
		DefaultCWD    string               `yaml:"default_cwd"`
		MaxConcurrent int                  `yaml:"max_concurrent"`
		Chains        map[string]chainYAML `yaml:"chains"`
	}

	cfg := agents.Config{
		PromptsDir:    filepath.Join(configDir, "coder-prompts"),
		SkillPath:     filepath.Join(configDir, "coder-skill.md"),
		StepSkillsDir: filepath.Join(configDir, "coder-skills"),
		Chains:        make(map[string]agents.Chain),
	}

	data, err := os.ReadFile(filepath.Join(configDir, "coder.yaml"))
	if err != nil {
		log.Printf("agents: no coder.yaml found, using defaults")
	} else {
		var y coderYAML
		if err := yaml.Unmarshal(data, &y); err != nil {
			log.Printf("agents: parse coder.yaml: %v", err)
		} else {
			cfg.AllowedPaths = y.AllowedPaths
			cfg.DefaultCWD = y.DefaultCWD
			cfg.MaxConcurrent = y.MaxConcurrent

			for name, ch := range y.Chains {
				steps := make([]agents.Step, len(ch.Steps))
				for i, s := range ch.Steps {
					steps[i] = agents.Step{
						Name:       s.Name,
						PromptFile: s.Prompt,
						Tools:      s.Tools,
						Thinking:   s.Thinking,
					}
				}
				cfg.Chains[name] = agents.Chain{Steps: steps}
			}
		}
	}

	if len(cfg.Chains) == 0 {
		log.Printf("agents: no chains defined in coder.yaml")
	} else {
		log.Printf("agents: loaded %d chains from config", len(cfg.Chains))
	}

	// Resolve coder.worker LLM route → pi provider/model
	cfg.Provider, cfg.Model, cfg.APIKeyEnv = resolveLLMRoute("coder.worker", llmCfg)
	if cfg.Provider == "" {
		cfg.Provider = "anthropic"
		cfg.Model = "claude-sonnet-4-6"
		cfg.APIKeyEnv = "ANTHROPIC_API_KEY"
		log.Printf("agents: no coder.worker route, defaulting to %s/%s", cfg.Provider, cfg.Model)
	} else {
		log.Printf("agents: using %s/%s via %s", cfg.Provider, cfg.Model, cfg.APIKeyEnv)
	}

	return cfg
}

func resolveLLMRoute(routeName string, llmCfg appconfig.LLMConfig) (provider, model, apiKeyEnv string) {
	route, ok := llmCfg.Routes[routeName]
	if !ok {
		return "", "", ""
	}
	profile, ok := llmCfg.Profiles[route.Profile]
	if !ok {
		return "", "", ""
	}
	prov, ok := llmCfg.Providers[profile.Provider]
	if !ok {
		return "", "", ""
	}
	return prov.Type, profile.Model, prov.APIKeyEnv
}

func defaultIngestDir(configDir string) string {
	absConfigDir, err := filepath.Abs(configDir)
	if err != nil {
		return "./ingest"
	}

	workspaceDir := filepath.Dir(absConfigDir)
	if workspaceDir == "." || workspaceDir == string(filepath.Separator) || workspaceDir == "" {
		return "./ingest"
	}
	return filepath.Join(workspaceDir, "ingest")
}

func waitForSignal(client *signalcli.Client) error {
	var lastErr error
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := client.Check(ctx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if i < 9 {
			log.Printf("waiting for signal-cli... (%v)", err)
			time.Sleep(2 * time.Second)
		}
	}
	return lastErr
}

func secureFileIfExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	if info.Mode().Perm() == 0o600 {
		return nil
	}
	return os.Chmod(path, 0o600)
}

func historyToThread(history []db.ConversationEntry) []brain.ConversationTurn {
	if len(history) == 0 {
		return nil
	}

	// conversation_log query already returns chronological order.
	out := make([]brain.ConversationTurn, 0, len(history))
	for i := 0; i < len(history); i++ {
		text := strings.TrimSpace(history[i].Content)
		if text == "" {
			continue
		}

		role := "user"
		if strings.EqualFold(strings.TrimSpace(history[i].Role), "assistant") {
			role = "assistant"
		}

		out = append(out, brain.ConversationTurn{
			Role:    role,
			Content: text,
		})
	}
	return out
}

func ensureCurrentUserMessage(thread []brain.ConversationTurn, message string) []brain.ConversationTurn {
	text := strings.TrimSpace(message)
	if text == "" {
		return thread
	}
	if len(thread) > 0 {
		last := thread[len(thread)-1]
		if last.Role == "user" && strings.TrimSpace(last.Content) == text {
			return thread
		}
	}
	return append(thread, brain.ConversationTurn{Role: "user", Content: text})
}

func annotateUserMessageWithSignalMetadata(message, nonTextSummary string) string {
	text := strings.TrimSpace(message)
	if text == "" || strings.TrimSpace(nonTextSummary) == "" {
		return text
	}

	return fmt.Sprintf("%s\n\n[Signal metadata: user also sent %s. You cannot access images, attachments, or audio. Ask for text description or transcript if needed.]", text, nonTextSummary)
}

func buildAgentSystemPrompt(cfg *appconfig.Config) string {
	if cfg == nil {
		return ""
	}

	base := strings.TrimSpace(cfg.Agent.Personality)
	rules := make([]string, 0, len(cfg.Agent.Rules))
	for _, rule := range cfg.Agent.Rules {
		rule = strings.TrimSpace(rule)
		if rule != "" {
			rules = append(rules, rule)
		}
	}
	var b strings.Builder
	if base != "" {
		b.WriteString(base)
	}
	if len(rules) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("Rules:")
		for _, rule := range rules {
			b.WriteString("\n- ")
			b.WriteString(rule)
		}
	}

	b.WriteString("\n\n## Current Context")
	b.WriteString("\n{{cortex_bulletin}}")

	return b.String()
}

// runProactiveNotifier drains the scaffold-agent session bus queue and sends
// messages to the user via Signal. Cortex tasks and agent results push to this
// queue; the notifier is the single outbound path to Signal.
func runProactiveNotifier(ctx context.Context, bus *sessionbus.Bus, client *signalcli.Client, userNumber string) {
	const pollTimeout = 30 * time.Second
	const sessionID = "scaffold-agent"

	for {
		if ctx.Err() != nil {
			return
		}

		msgs, err := bus.Poll(ctx, sessionID, 10, pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("notifier: poll error: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		for _, msg := range msgs {
			text := strings.TrimSpace(msg.Message)
			if text == "" {
				continue
			}

			// Try typed message parsing
			var typed struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal([]byte(text), &typed); err == nil {
				switch typed.Type {
				case "coder_result":
					var result agents.CoderResultMessage
					if err := json.Unmarshal([]byte(text), &result); err == nil {
						text = formatCoderResult(result)
					}
				case "notification":
					var notif struct {
						SubType string `json:"sub_type"`
						Text    string `json:"text"`
					}
					if err := json.Unmarshal([]byte(text), &notif); err == nil && notif.Text != "" {
						text = notif.Text
					}
				}
			}

			sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := client.Send(sendCtx, userNumber, text); err != nil {
				log.Printf("notifier: send failed: %v", err)
			} else {
				log.Printf("notifier: -> %s (len=%d)", userNumber, len(text))
			}
			cancel()
		}
	}
}

func formatCoderResult(result agents.CoderResultMessage) string {
	statusIcon := "✓"
	if result.Status == "failed" || result.Status == "cancelled" {
		statusIcon = "✗"
	}

	var elapsedStr string
	if result.ElapsedS > 60 {
		mins := int(result.ElapsedS) / 60
		secs := int(result.ElapsedS) % 60
		elapsedStr = fmt.Sprintf("%d m %02d s", mins, secs)
	} else {
		elapsedStr = fmt.Sprintf("%.1fs", result.ElapsedS)
	}

	var sb strings.Builder
	statusText := "done"
	if result.Status == "failed" {
		statusText = "failed"
	} else if result.Status == "cancelled" {
		statusText = "cancelled"
	}
	sb.WriteString(fmt.Sprintf("%s Chain %s: %s (%s)", statusIcon, statusText, result.Chain, elapsedStr))

	if result.TaskDesc != "" {
		desc := result.TaskDesc
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		sb.WriteString(fmt.Sprintf("\n\"%s\"", desc))
	}
	sb.WriteString("\n")

	if result.Status == "failed" {
		if result.FailedStep != "" {
			sb.WriteString(fmt.Sprintf("→ Failed at step: %s", result.FailedStep))
		}
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("\n→ %s", result.Error))
		}
	} else if result.Summary != "" {
		summary := result.Summary
		if len(summary) > 200 {
			summary = summary[:197] + "..."
		}
		sb.WriteString(fmt.Sprintf("\n→ %s", summary))
	}

	return sb.String()
}

func loadGmailConfig(configDir string) *googleauth.GmailConfig {
	data, err := os.ReadFile(filepath.Join(configDir, "gmail.yaml"))
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("warn: gmail.yaml read error: %v", err)
		}
		return nil
	}
	var cfg googleauth.GmailConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Printf("warn: gmail.yaml parse error: %v", err)
		return nil
	}
	log.Printf("Gmail config loaded (%d prefilter rules)", len(cfg.Prefilter.KnownFilers))
	return &cfg
}

func handleAuthSubcommand(args []string) {
	if len(args) == 0 {
		log.Fatal("usage: scaffold-daemon auth google")
	}

	switch args[0] {
	case "google":
		handleGoogleAuth()
	default:
		log.Fatalf("unknown auth provider: %s", args[0])
	}
}

func handleGoogleAuth() {
	_ = godotenv.Load()

	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "./config"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./scaffold.db"
	}

	var googleCfg appconfig.GoogleConfig
	data, err := os.ReadFile(filepath.Join(configDir, "google.yaml"))
	if err != nil {
		log.Fatalf("failed to read google.yaml: %v (run from daemon/ directory or set CONFIG_DIR)", err)
	}
	if err := yaml.Unmarshal(data, &googleCfg); err != nil {
		log.Fatalf("failed to parse google.yaml: %v", err)
	}

	if googleCfg.ClientID == "" || googleCfg.ClientSecret == "" {
		log.Fatal("google.yaml: client_id and client_secret are required. See file for setup instructions.")
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	oauthCfg := googleauth.NewOAuth2Config(googleCfg)
	store := &googleauth.DBTokenStore{DB: database, Provider: "google"}

	if err := googleauth.RunConsentFlow(oauthCfg, store); err != nil {
		log.Fatalf("auth flow failed: %v", err)
	}
}
