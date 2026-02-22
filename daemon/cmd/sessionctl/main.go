package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type commonOptions struct {
	baseURL  string
	apiToken string
	timeout  time.Duration
	asJSON   bool
}

type session struct {
	SessionID  string    `json:"session_id"`
	Provider   string    `json:"provider"`
	Name       string    `json:"name,omitempty"`
	QueueDepth int       `json:"queue_depth"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type envelope struct {
	ID            string    `json:"id"`
	FromSessionID string    `json:"from_session_id"`
	ToSessionID   string    `json:"to_session_id"`
	Mode          string    `json:"mode"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"created_at"`
}

func main() {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	command := strings.TrimSpace(os.Args[1])
	var err error

	switch command {
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return
	case "register":
		err = runRegister(os.Args[2:])
	case "list":
		err = runList(os.Args[2:])
	case "send":
		err = runSend(os.Args[2:])
	case "poll":
		err = runPoll(os.Args[2:])
	default:
		printUsage(os.Stderr)
		err = fmt.Errorf("unknown command %q", command)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runRegister(args []string) error {
	opts := defaultCommonOptions()
	fs := newFlagSet("register")
	addCommonFlags(fs, &opts)

	sessionID := fs.String("session-id", "", "Session ID to register")
	provider := fs.String("provider", "unknown", "Provider label (codex, gemini, anthropic, scaffold)")
	name := fs.String("name", "", "Optional display name")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sessionID) == "" {
		return errors.New("--session-id is required")
	}

	client := newHTTPClient(opts.timeout)
	ctx := context.Background()

	var response struct {
		Session session `json:"session"`
	}
	err := requestJSON(ctx, client, http.MethodPost, opts, "/api/session-bus/register", map[string]any{
		"session_id": *sessionID,
		"provider":   *provider,
		"name":       *name,
	}, &response)
	if err != nil {
		return err
	}

	if opts.asJSON {
		return writeJSON(os.Stdout, response)
	}

	fmt.Printf("registered %s [%s]", response.Session.SessionID, response.Session.Provider)
	if strings.TrimSpace(response.Session.Name) != "" {
		fmt.Printf(" %s", response.Session.Name)
	}
	fmt.Println()
	return nil
}

func runList(args []string) error {
	opts := defaultCommonOptions()
	fs := newFlagSet("list")
	addCommonFlags(fs, &opts)
	if err := fs.Parse(args); err != nil {
		return err
	}

	client := newHTTPClient(opts.timeout)
	ctx := context.Background()

	var response struct {
		Sessions []session `json:"sessions"`
	}
	err := requestJSON(ctx, client, http.MethodGet, opts, "/api/session-bus/sessions", nil, &response)
	if err != nil {
		return err
	}

	sort.Slice(response.Sessions, func(i, j int) bool {
		return response.Sessions[i].SessionID < response.Sessions[j].SessionID
	})

	if opts.asJSON {
		return writeJSON(os.Stdout, response)
	}

	if len(response.Sessions) == 0 {
		fmt.Println("no active sessions")
		return nil
	}

	fmt.Printf("active sessions: %d\n", len(response.Sessions))
	for _, s := range response.Sessions {
		fmt.Printf("- %s [%s]", s.SessionID, s.Provider)
		if strings.TrimSpace(s.Name) != "" {
			fmt.Printf(" %s", s.Name)
		}
		fmt.Printf(" queue=%d last_seen=%s\n", s.QueueDepth, s.LastSeenAt.Format(time.RFC3339))
	}
	return nil
}

func runSend(args []string) error {
	opts := defaultCommonOptions()
	fs := newFlagSet("send")
	addCommonFlags(fs, &opts)

	from := fs.String("from", "", "Sender session ID")
	to := fs.String("to", "", "Target session ID")
	mode := fs.String("mode", "steer", "Mode label: steer or follow_up")
	message := fs.String("message", "", "Message content")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*from) == "" {
		return errors.New("--from is required")
	}
	if strings.TrimSpace(*to) == "" {
		return errors.New("--to is required")
	}
	if strings.TrimSpace(*message) == "" {
		return errors.New("--message is required")
	}

	client := newHTTPClient(opts.timeout)
	ctx := context.Background()

	var response struct {
		Message envelope `json:"message"`
	}
	err := requestJSON(ctx, client, http.MethodPost, opts, "/api/session-bus/send", map[string]any{
		"from_session_id": *from,
		"to_session_id":   *to,
		"mode":            *mode,
		"message":         *message,
	}, &response)
	if err != nil {
		return err
	}

	if opts.asJSON {
		return writeJSON(os.Stdout, response)
	}

	fmt.Printf("sent %s: %s -> %s (%s)\n", response.Message.ID, response.Message.FromSessionID, response.Message.ToSessionID, response.Message.Mode)
	return nil
}

func runPoll(args []string) error {
	opts := defaultCommonOptions()
	fs := newFlagSet("poll")
	addCommonFlags(fs, &opts)

	sessionID := fs.String("session-id", "", "Session ID to poll")
	limit := fs.Int("limit", 10, "Maximum messages to receive")
	waitSeconds := fs.Int("wait-seconds", 0, "Long-poll wait in seconds (0-120)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sessionID) == "" {
		return errors.New("--session-id is required")
	}
	if *limit <= 0 {
		return errors.New("--limit must be > 0")
	}
	if *waitSeconds < 0 || *waitSeconds > 120 {
		return errors.New("--wait-seconds must be between 0 and 120")
	}

	effectiveTimeout := opts.timeout
	minTimeout := time.Duration(*waitSeconds+5) * time.Second
	if minTimeout > effectiveTimeout {
		effectiveTimeout = minTimeout
	}
	client := newHTTPClient(effectiveTimeout)
	ctx := context.Background()

	var response struct {
		Messages []envelope `json:"messages"`
	}
	err := requestJSON(ctx, client, http.MethodPost, opts, "/api/session-bus/poll", map[string]any{
		"session_id":   *sessionID,
		"limit":        *limit,
		"wait_seconds": *waitSeconds,
	}, &response)
	if err != nil {
		return err
	}

	if opts.asJSON {
		return writeJSON(os.Stdout, response)
	}

	if len(response.Messages) == 0 {
		fmt.Println("no messages")
		return nil
	}

	for i, msg := range response.Messages {
		if i > 0 {
			fmt.Println("---")
		}
		fmt.Printf("[%s] %s -> %s (%s) id=%s\n", msg.CreatedAt.Format(time.RFC3339), msg.FromSessionID, msg.ToSessionID, msg.Mode, msg.ID)
		fmt.Println(msg.Message)
	}
	return nil
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		switch name {
		case "register":
			fmt.Fprintf(fs.Output(), "usage: sessionctl register --session-id <id> [--provider <provider>] [--name <name>] [common flags]\n")
		case "list":
			fmt.Fprintf(fs.Output(), "usage: sessionctl list [common flags]\n")
		case "send":
			fmt.Fprintf(fs.Output(), "usage: sessionctl send --from <id> --to <id> --message <text> [--mode steer|follow_up] [common flags]\n")
		case "poll":
			fmt.Fprintf(fs.Output(), "usage: sessionctl poll --session-id <id> [--limit 10] [--wait-seconds 0..120] [common flags]\n")
		}
		printCommonFlags(fs.Output())
	}
	return fs
}

func defaultCommonOptions() commonOptions {
	return commonOptions{
		baseURL:  defaultBaseURL(),
		apiToken: strings.TrimSpace(os.Getenv("API_TOKEN")),
		timeout:  20 * time.Second,
		asJSON:   false,
	}
}

func addCommonFlags(fs *flag.FlagSet, opts *commonOptions) {
	fs.StringVar(&opts.baseURL, "base-url", opts.baseURL, "Daemon base URL")
	fs.StringVar(&opts.apiToken, "api-token", opts.apiToken, "Bearer token (defaults to API_TOKEN)")
	fs.DurationVar(&opts.timeout, "timeout", opts.timeout, "HTTP timeout (e.g. 20s)")
	fs.BoolVar(&opts.asJSON, "json", opts.asJSON, "Print JSON response")
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "sessionctl: tiny CLI for Scaffold session bus")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  register  Register a session")
	fmt.Fprintln(w, "  list      List active sessions")
	fmt.Fprintln(w, "  send      Send a message")
	fmt.Fprintln(w, "  poll      Poll messages for a session")
	fmt.Fprintln(w, "")
	printCommonFlags(w)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "examples:")
	fmt.Fprintln(w, "  sessionctl register --session-id codex-main --provider codex --name \"Codex Main\"")
	fmt.Fprintln(w, "  sessionctl list")
	fmt.Fprintln(w, "  sessionctl send --from codex-main --to gemini-worker --mode steer --message \"Summarize docs\"")
	fmt.Fprintln(w, "  sessionctl poll --session-id codex-main --wait-seconds 30")
}

func printCommonFlags(w io.Writer) {
	fmt.Fprintln(w, "common flags:")
	fmt.Fprintln(w, "  --base-url <url>   Daemon URL (default: $SCAFFOLD_API_BASE_URL or http://127.0.0.1:$API_PORT)")
	fmt.Fprintln(w, "  --api-token <tok>  API bearer token (default: $API_TOKEN)")
	fmt.Fprintln(w, "  --timeout <dur>    HTTP timeout (default: 20s)")
	fmt.Fprintln(w, "  --json             Print JSON response")
}

func defaultBaseURL() string {
	if explicit := strings.TrimSpace(os.Getenv("SCAFFOLD_API_BASE_URL")); explicit != "" {
		return strings.TrimRight(explicit, "/")
	}
	port := strings.TrimSpace(os.Getenv("API_PORT"))
	if port == "" {
		port = "46873"
	}
	return fmt.Sprintf("http://127.0.0.1:%s", port)
}

func newHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

func requestJSON(ctx context.Context, client *http.Client, method string, opts commonOptions, apiPath string, payload any, out any) error {
	if strings.TrimSpace(opts.apiToken) == "" {
		return errors.New("missing API token; set API_TOKEN or pass --api-token")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(opts.baseURL), "/")
	if baseURL == "" {
		return errors.New("missing base URL")
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+apiPath, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(opts.apiToken))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s failed: %w", method, apiPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := readAPIError(resp.Body)
		if apiErr == "" {
			apiErr = resp.Status
		}
		return fmt.Errorf("request %s %s failed (%d): %s", method, apiPath, resp.StatusCode, apiErr)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func readAPIError(r io.Reader) string {
	if r == nil {
		return ""
	}
	var body map[string]any
	if err := json.NewDecoder(r).Decode(&body); err != nil {
		return ""
	}
	if raw, ok := body["error"]; ok {
		if text, ok := raw.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
