package main

import (
	"context"
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

	"scaffold/api"
	appconfig "scaffold/config"
	"scaffold/db"
	googleauth "scaffold/google"
)

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

	appCfg, err := appconfig.Load(cfg.configDir, "")
	if err != nil {
		log.Fatalf("failed to load config from %s: %v", cfg.configDir, err)
	}
	log.Printf("config loaded from %s", cfg.configDir)

	var calendarClient *googleauth.CalendarClient
	calendarClient, err = initGoogleCalendarClient(context.Background(), database, appCfg.Google)
	if err != nil {
		log.Printf("warn: google calendar unavailable: %v", err)
		calendarClient = nil
	} else if calendarClient != nil {
		log.Printf("google calendar connected: %s", calendarClient.CalendarID)
	} else {
		log.Printf("google calendar not configured; run scaffold-daemon auth google")
	}

	srv := api.New(database, calendarClient, cfg.apiToken, api.AuthConfig{
		AppUsername:          cfg.appUsername,
		AppPasswordHash:      cfg.appPasswordHash,
		SessionTTL:           time.Duration(cfg.sessionTTLHours) * time.Hour,
		CookieSecure:         cfg.cookieSecure,
		CookieDomain:         cfg.cookieDomain,
		LoginRateLimitWindow: time.Duration(cfg.loginRateLimitWindowSecs) * time.Second,
		LoginRateLimitMax:    cfg.loginRateLimitMax,
	})
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

	startMaintenance(ctx, database)

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

	<-ctx.Done()
	log.Println("daemon stopped")
}

// startMaintenance launches background goroutines that decay and prune memories
// on fixed intervals. These are pure SQL operations against the db package.
func startMaintenance(ctx context.Context, database *db.DB) {
	const decayInterval = 24 * time.Hour
	const pruneInterval = 24 * time.Hour

	runOnce := func() {
		if _, err := database.DecayMemories(0.95, nil, 0.1, 30); err != nil {
			log.Printf("warn: decay memories failed: %v", err)
		}
		if _, err := database.PruneSuppressedMemories(30); err != nil {
			log.Printf("warn: prune suppressed memories failed: %v", err)
		}
	}

	// Run once at startup, then on a ticker.
	go runOnce()

	decayTicker := time.NewTicker(decayInterval)
	pruneTicker := time.NewTicker(pruneInterval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				decayTicker.Stop()
				return
			case <-decayTicker.C:
				if _, err := database.DecayMemories(0.95, nil, 0.1, 30); err != nil {
					log.Printf("warn: decay memories failed: %v", err)
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				pruneTicker.Stop()
				return
			case <-pruneTicker.C:
				if _, err := database.PruneSuppressedMemories(30); err != nil {
					log.Printf("warn: prune suppressed memories failed: %v", err)
				}
			}
		}
	}()
}

func initGoogleCalendarClient(ctx context.Context, database *db.DB, cfg appconfig.GoogleConfig) (*googleauth.CalendarClient, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, nil
	}
	store := &googleauth.DBTokenStore{DB: database, Provider: "google"}
	token, err := store.Get()
	if err != nil {
		return nil, fmt.Errorf("load google token: %w", err)
	}
	if token == nil {
		return nil, nil
	}
	oauthCfg := googleauth.NewOAuth2Config(cfg)
	return googleauth.NewCalendarClient(ctx, oauthCfg.TokenSource(ctx, token), cfg.CalendarID)
}

type config struct {
	configDir                string
	frontendDistDir          string
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
	withDefault := func(key, def string) string {
		if v := sanitizeEnvValue(os.Getenv(key)); v != "" {
			return v
		}
		return def
	}
	required := func(key string) string {
		v := sanitizeEnvValue(os.Getenv(key))
		if v == "" {
			log.Fatalf("%s is required", key)
		}
		return v
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
	apiPort := required("API_PORT")
	if p, err := strconv.Atoi(apiPort); err != nil || p < 1 || p > 65535 {
		log.Fatalf("API_PORT must be a valid port number, got %q", apiPort)
	}
	sessionTTLHours := parsePositiveInt("SESSION_TTL_HOURS", withDefault("SESSION_TTL_HOURS", "168"), 1)
	loginRateLimitWindowSecs := parsePositiveInt("LOGIN_RATE_LIMIT_WINDOW_SECS", withDefault("LOGIN_RATE_LIMIT_WINDOW_SECS", "300"), 1)
	loginRateLimitMax := parsePositiveInt("LOGIN_RATE_LIMIT_MAX_ATTEMPTS", withDefault("LOGIN_RATE_LIMIT_MAX_ATTEMPTS", "5"), 1)
	cookieSecure := parseBool("COOKIE_SECURE", withDefault("COOKIE_SECURE", "true"))

	return config{
		configDir:                configDir,
		frontendDistDir:          withDefault("FRONTEND_DIST_DIR", "../app/dist"),
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
