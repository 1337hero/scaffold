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

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"scaffold/api"
	"scaffold/brain"
	"scaffold/db"
	signalcli "scaffold/signal"
)

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

	b := brain.New(cfg.anthropicKey, brain.Config{
		AssistantName: cfg.assistantName,
		UserName:      cfg.userName,
	})
	client := signalcli.NewClient(cfg.signalURL, cfg.agentNumber)

	srv := api.New(database, cfg.apiToken)
	apiAddr := net.JoinHostPort(cfg.apiHost, cfg.apiPort)
	httpServer := srv.NewHTTPServer(apiAddr)
	go func() {
		log.Printf("API server listening on %s", apiAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API server failed: %v", err)
		}
	}()

	if err := waitForSignal(client); err != nil {
		log.Fatalf("signal-cli not ready: %v", err)
	}
	log.Println("signal-cli connected")

	startupMsg := fmt.Sprintf("%s online. Brain active.", cfg.assistantName)
	if err := client.Send(context.Background(), cfg.userNumber, startupMsg); err != nil {
		log.Printf("warn: startup message failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	captureID, err := database.InsertCapture(msg.Message, "signal:user:"+msg.Sender)
	if err != nil {
		log.Printf("capture insert error: %v", err)
	}

	triage, err := b.Triage(ctx, msg.Message)
	if err != nil {
		log.Printf("triage error: %v", err)
	}

	if triage != nil && captureID != "" {
		log.Printf("triage: type=%s action=%s importance=%.1f", triage.Type, triage.Action, triage.Importance)

		memID := uuid.New().String()
		mem := db.Memory{
			ID:         memID,
			Type:       triage.Type,
			Content:    msg.Message,
			Title:      triage.Title,
			Importance: triage.Importance,
			Source:     "signal",
			Tags:       strings.Join(triage.Tags, ","),
		}
		if err := database.InsertMemory(mem); err != nil {
			log.Printf("memory insert error: %v", err)
		}

		if err := database.UpdateTriage(captureID, triage.Action, memID); err != nil {
			log.Printf("triage update error: %v", err)
		}
	}

	history, err := database.ListRecentBySender(msg.Sender, conversationHistoryLimit)
	if err != nil {
		log.Printf("history query error: %v", err)
	}

	response, err := b.Respond(ctx, msg.Message, historyToThread(history))
	if err != nil {
		log.Printf("brain error: %v", err)
		response = "Something went wrong on my end. Try again?"
	}

	if err := client.Send(ctx, msg.Sender, response); err != nil {
		log.Printf("failed to send response: %v", err)
	} else {
		log.Printf("-> %s (len=%d)", msg.Sender, len(response))
		if _, err := database.InsertProcessedCapture(response, "signal:assistant:"+msg.Sender, "responded"); err != nil {
			log.Printf("assistant capture insert error: %v", err)
		}
	}
}

type config struct {
	anthropicKey  string
	signalURL     string
	agentNumber   string
	userNumber    string
	assistantName string
	userName      string
	dbPath        string
	apiHost       string
	apiPort       string
	apiToken      string
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
		anthropicKey:  required("ANTHROPIC_API_KEY"),
		agentNumber:   required("AGENT_NUMBER"),
		userNumber:    required("USER_NUMBER"),
		signalURL:     required("SIGNAL_URL"),
		assistantName: withDefault("ASSISTANT_NAME", "Scaffold"),
		userName:      withDefault("USER_NAME", "User"),
		dbPath:        withDefault("DB_PATH", "./scaffold.db"),
		apiHost:       withDefault("API_HOST", "127.0.0.1"),
		apiPort:       apiPort,
		apiToken:      required("API_TOKEN"),
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

func historyToThread(history []db.Capture) []brain.ConversationTurn {
	if len(history) == 0 {
		return nil
	}

	// DB query returns newest-first; Anthropic expects chronological order.
	out := make([]brain.ConversationTurn, 0, len(history))
	for i := len(history) - 1; i >= 0; i-- {
		text := strings.TrimSpace(history[i].Raw)
		if text == "" {
			continue
		}

		role := "user"
		if strings.HasPrefix(history[i].Source, "signal:assistant:") {
			role = "assistant"
		}

		out = append(out, brain.ConversationTurn{
			Role:    role,
			Content: text,
		})
	}
	return out
}
