package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"scaffold/api"
	"scaffold/brain"
	appconfig "scaffold/config"
	"scaffold/cortex"
	"scaffold/cron"
	"scaffold/db"
	signalcli "scaffold/signal"
)

func parseInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

const maxInFlightMessages = 4
const conversationHistoryLimit = 12

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
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

	b := brain.New(cfg.anthropicKey, database, brain.Config{
		AssistantName:    assistantName,
		UserName:         cfg.userName,
		SystemPrompt:     buildAgentSystemPrompt(appCfg),
		TriagePrompt:     appCfg.Triage.Prompt,
		RespondModel:     appCfg.Agent.Model,
		TriageModel:      appCfg.Triage.Model,
		RespondMaxTokens: appCfg.Agent.MaxResponseTokens,
		TriageMaxTokens:  appCfg.Triage.MaxTokens,
		Tools:            toolDefs,
	})

	cortexRuntime := cortex.New(database, cfg.anthropicKey, appCfg.Cortex)
	b.SetBulletinProvider(cortexRuntime.CurrentBulletin)

	client := signalcli.NewClient(cfg.signalURL, cfg.agentNumber)

	srv := api.New(database, b, cfg.apiToken, api.AuthConfig{
		AppUsername:          cfg.appUsername,
		AppPasswordHash:      cfg.appPasswordHash,
		SessionTTL:           time.Duration(cfg.sessionTTLHours) * time.Hour,
		CookieSecure:         cfg.cookieSecure,
		CookieDomain:         cfg.cookieDomain,
		LoginRateLimitWindow: time.Duration(cfg.loginRateLimitWindowSecs) * time.Second,
		LoginRateLimitMax:    cfg.loginRateLimitMax,
	})
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
	cron.Start(ctx, database, b)

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

	if _, err := database.InsertConversationEntry(msg.Sender, "user", msg.Message); err != nil {
		log.Printf("conversation insert error (user): %v", err)
	}

	history, err := database.ListRecentConversation(msg.Sender, conversationHistoryLimit)
	if err != nil {
		log.Printf("history query error: %v", err)
	}

	thread := historyToThread(history)
	thread = ensureCurrentUserMessage(thread, msg.Message)

	response, err := b.Respond(ctx, msg.Message, thread)
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
	anthropicKey             string
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
	required := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			log.Fatalf("%s is required", key)
		}
		return v
	}
	withDefault := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}
	apiPort := required("API_PORT")
	if p, err := strconv.Atoi(apiPort); err != nil || p < 1 || p > 65535 {
		log.Fatalf("API_PORT must be a valid port number, got %q", apiPort)
	}

	return config{
		configDir:                withDefault("CONFIG_DIR", "./config"),
		anthropicKey:             required("ANTHROPIC_API_KEY"),
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
		sessionTTLHours:          parseInt(withDefault("SESSION_TTL_HOURS", "168")),
		cookieSecure:             withDefault("COOKIE_SECURE", "true") == "true",
		cookieDomain:             withDefault("COOKIE_DOMAIN", ""),
		loginRateLimitWindowSecs: parseInt(withDefault("LOGIN_RATE_LIMIT_WINDOW_SECS", "300")),
		loginRateLimitMax:        parseInt(withDefault("LOGIN_RATE_LIMIT_MAX_ATTEMPTS", "5")),
	}
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
