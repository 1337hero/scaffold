# Webhook Ingestion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `POST /api/webhook` — authenticated, rate-limited, generic webhook endpoint that routes external captures into the inbox via the existing triage pipeline.

**Architecture:** Named tokens in `config/webhooks.yaml` (gitignored). Each token maps to a source name; the handler validates the Bearer token, rate-limits per token name, formats the payload, and calls `capture.Ingest()` with source `"webhook:<name>"`. No changes to the existing `protected()` middleware — webhooks use their own auth path.

**Tech Stack:** Go stdlib, existing `capture.Ingest()`, existing `rateLimiter` struct from `daemon/api/auth.go`, `gopkg.in/yaml.v3`.

---

### Task 1: Webhook config loader

**Files:**
- Create: `daemon/config/webhooks.go`
- Create: `daemon/config/webhooks_test.go`

**Step 1: Write failing tests**

```go
// daemon/config/webhooks_test.go
package config

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoadWebhookConfigMissingFile(t *testing.T) {
    cfg, found, err := LoadWebhookConfig("/nonexistent/webhooks.yaml")
    if err != nil {
        t.Fatalf("expected no error for missing file, got: %v", err)
    }
    if found {
        t.Fatal("expected found=false for missing file")
    }
    if cfg != nil {
        t.Fatal("expected nil config for missing file")
    }
}

func TestLoadWebhookConfigValid(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "webhooks.yaml")
    content := `
rate_limit:
  max: 30
  window_minutes: 10
tokens:
  fitness: tok-abc123
  homelab: tok-def456
`
    if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
        t.Fatalf("write test file: %v", err)
    }

    cfg, found, err := LoadWebhookConfig(path)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !found {
        t.Fatal("expected found=true")
    }
    if cfg.RateLimit.Max != 30 {
        t.Errorf("expected max=30, got %d", cfg.RateLimit.Max)
    }
    if cfg.RateLimit.WindowMinutes != 10 {
        t.Errorf("expected window_minutes=10, got %d", cfg.RateLimit.WindowMinutes)
    }
    if cfg.Tokens["fitness"] != "tok-abc123" {
        t.Errorf("expected fitness token tok-abc123, got %q", cfg.Tokens["fitness"])
    }
    if cfg.Tokens["homelab"] != "tok-def456" {
        t.Errorf("expected homelab token tok-def456, got %q", cfg.Tokens["homelab"])
    }
}

func TestLoadWebhookConfigDefaults(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "webhooks.yaml")
    if err := os.WriteFile(path, []byte("tokens:\n  test: tok-xyz\n"), 0o600); err != nil {
        t.Fatalf("write: %v", err)
    }

    cfg, _, err := LoadWebhookConfig(path)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if cfg.RateLimit.Max != 60 {
        t.Errorf("expected default max=60, got %d", cfg.RateLimit.Max)
    }
    if cfg.RateLimit.WindowMinutes != 60 {
        t.Errorf("expected default window=60, got %d", cfg.RateLimit.WindowMinutes)
    }
}

func TestLoadWebhookConfigInvalidYAML(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "webhooks.yaml")
    if err := os.WriteFile(path, []byte(":::invalid yaml:::"), 0o600); err != nil {
        t.Fatalf("write: %v", err)
    }

    _, _, err := LoadWebhookConfig(path)
    if err == nil {
        t.Fatal("expected error for invalid YAML")
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
cd daemon && go test ./config/... -run TestLoadWebhook -v
```
Expected: FAIL — `LoadWebhookConfig` undefined.

**Step 3: Implement**

```go
// daemon/config/webhooks.go
package config

import (
    "errors"
    "fmt"
    "os"

    "gopkg.in/yaml.v3"
)

type WebhookConfig struct {
    RateLimit WebhookRateLimit  `yaml:"rate_limit"`
    Tokens    map[string]string `yaml:"tokens"`
}

type WebhookRateLimit struct {
    Max           int `yaml:"max"`
    WindowMinutes int `yaml:"window_minutes"`
}

// LoadWebhookConfig loads config/webhooks.yaml.
// Returns (nil, false, nil) if the file does not exist — webhooks are disabled.
// Returns (nil, false, err) on parse error.
func LoadWebhookConfig(path string) (*WebhookConfig, bool, error) {
    data, err := os.ReadFile(path)
    if errors.Is(err, os.ErrNotExist) {
        return nil, false, nil
    }
    if err != nil {
        return nil, false, fmt.Errorf("read webhook config: %w", err)
    }

    var cfg WebhookConfig
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, false, fmt.Errorf("parse webhook config: %w", err)
    }

    if cfg.RateLimit.Max == 0 {
        cfg.RateLimit.Max = 60
    }
    if cfg.RateLimit.WindowMinutes == 0 {
        cfg.RateLimit.WindowMinutes = 60
    }
    if cfg.Tokens == nil {
        cfg.Tokens = make(map[string]string)
    }

    return &cfg, true, nil
}
```

**Step 4: Run tests to verify they pass**

```bash
cd daemon && go test ./config/... -run TestLoadWebhook -v
```
Expected: PASS (4 tests).

**Step 5: Commit**

```bash
cd daemon && git add config/webhooks.go config/webhooks_test.go
git commit -m "feat: webhook config loader"
```

---

### Task 2: Config example file and gitignore

**Files:**
- Create: `config/webhooks.yaml.example`
- Modify: `.gitignore`

**Step 1: Create example file**

```yaml
# config/webhooks.yaml.example
# Copy to config/webhooks.yaml (gitignored) and fill in real tokens.
# Generate tokens with: openssl rand -hex 32

rate_limit:
  max: 60           # requests per window per token
  window_minutes: 60

tokens:
  fitness:  replace-with-random-token
  homelab:  replace-with-random-token
  github:   replace-with-random-token
```

**Step 2: Add to .gitignore**

In `.gitignore`, find the line `config/google.yaml` and add below it:

```
config/webhooks.yaml
```

**Step 3: Verify**

```bash
git check-ignore -v config/webhooks.yaml
```
Expected output includes `.gitignore` and `config/webhooks.yaml`.

**Step 4: Commit**

```bash
git add config/webhooks.yaml.example .gitignore
git commit -m "feat: webhook config example and gitignore"
```

---

### Task 3: Webhook handler

**Files:**
- Create: `daemon/api/webhook.go`
- Create: `daemon/api/webhook_test.go`

**Step 1: Write failing tests**

```go
// daemon/api/webhook_test.go
package api

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "testing"

    "scaffold/config"
)

const testWebhookToken = "webhook-test-token-abc"

func webhookConfig(max, windowMins int, tokens map[string]string) *config.WebhookConfig {
    return &config.WebhookConfig{
        RateLimit: config.WebhookRateLimit{Max: max, WindowMinutes: windowMins},
        Tokens:    tokens,
    }
}

func webhookRequest(method, token, body string) *http.Request {
    var buf bytes.Buffer
    if body != "" {
        buf.WriteString(body)
    }
    req := httptest.NewRequest(method, "/api/webhook", &buf)
    if token != "" {
        req.Header.Set("Authorization", "Bearer "+token)
    }
    req.Header.Set("Content-Type", "application/json")
    return req
}

func TestWebhookNotConfigured(t *testing.T) {
    srv, _ := newTestServer(t)
    // no webhook config set

    rec := httptest.NewRecorder()
    srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, testWebhookToken, `{"content":"hello"}`))

    if rec.Code != http.StatusServiceUnavailable {
        t.Fatalf("expected 503, got %d", rec.Code)
    }
}

func TestWebhookMissingToken(t *testing.T) {
    srv, _ := newTestServer(t)
    srv.SetWebhookConfig(webhookConfig(60, 60, map[string]string{"test": testWebhookToken}))

    rec := httptest.NewRecorder()
    srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, "", `{"content":"hello"}`))

    if rec.Code != http.StatusUnauthorized {
        t.Fatalf("expected 401, got %d", rec.Code)
    }
}

func TestWebhookInvalidToken(t *testing.T) {
    srv, _ := newTestServer(t)
    srv.SetWebhookConfig(webhookConfig(60, 60, map[string]string{"test": testWebhookToken}))

    rec := httptest.NewRecorder()
    srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, "wrong-token", `{"content":"hello"}`))

    if rec.Code != http.StatusUnauthorized {
        t.Fatalf("expected 401, got %d", rec.Code)
    }
}

func TestWebhookMissingContent(t *testing.T) {
    srv, _ := newTestServer(t)
    srv.SetWebhookConfig(webhookConfig(60, 60, map[string]string{"test": testWebhookToken}))

    rec := httptest.NewRecorder()
    srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, testWebhookToken, `{"title":"no content"}`))

    if rec.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", rec.Code)
    }
}

func TestWebhookRateLimited(t *testing.T) {
    srv, _ := newTestServer(t)
    srv.SetWebhookConfig(webhookConfig(2, 60, map[string]string{"test": testWebhookToken}))

    send := func() int {
        rec := httptest.NewRecorder()
        srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, testWebhookToken, `{"content":"ping"}`))
        return rec.Code
    }

    // First two succeed (brain is nil so Ingest will store but not triage)
    send()
    send()
    // Third should be rate limited
    code := send()
    if code != http.StatusTooManyRequests {
        t.Fatalf("expected 429 after limit, got %d", code)
    }
}

func TestWebhookSuccess(t *testing.T) {
    srv, _ := newTestServer(t)
    srv.SetWebhookConfig(webhookConfig(60, 60, map[string]string{"fitness": testWebhookToken}))

    rec := httptest.NewRecorder()
    srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, testWebhookToken,
        `{"title":"Weight logged","content":"185 lbs this morning"}`))

    if rec.Code != http.StatusAccepted {
        t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
    }
}

func TestWebhookSuccessNoTitle(t *testing.T) {
    srv, _ := newTestServer(t)
    srv.SetWebhookConfig(webhookConfig(60, 60, map[string]string{"homelab": testWebhookToken}))

    rec := httptest.NewRecorder()
    srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, testWebhookToken,
        `{"content":"disk usage at 87%"}`))

    if rec.Code != http.StatusAccepted {
        t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
cd daemon && go test ./api/... -run TestWebhook -v
```
Expected: FAIL — `SetWebhookConfig` undefined, route not registered.

**Step 3: Implement**

```go
// daemon/api/webhook.go
package api

import (
    "crypto/subtle"
    "encoding/json"
    "net/http"
    "strconv"
    "strings"
    "time"

    "scaffold/capture"
    "scaffold/config"
)

type webhookRequest struct {
    Title   string `json:"title"`
    Content string `json:"content"`
}

type webhookResponse struct {
    ID     string `json:"id"`
    Source string `json:"source"`
}

func (s *Server) SetWebhookConfig(cfg *config.WebhookConfig) {
    s.webhookCfg = cfg
    s.webhookLimiter = newRateLimiter(
        time.Duration(cfg.RateLimit.WindowMinutes)*time.Minute,
        cfg.RateLimit.Max,
    )
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
    if s.webhookCfg == nil {
        writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "webhooks not configured"})
        return
    }

    bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
    if bearer == "" {
        writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer token required"})
        return
    }

    tokenName := ""
    for name, token := range s.webhookCfg.Tokens {
        if subtle.ConstantTimeCompare([]byte(bearer), []byte(token)) == 1 {
            tokenName = name
            break
        }
    }
    if tokenName == "" {
        writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
        return
    }

    if !s.webhookLimiter.allow(tokenName) {
        retryAfter := strconv.Itoa(s.webhookCfg.RateLimit.WindowMinutes * 60)
        w.Header().Set("Retry-After", retryAfter)
        writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
        return
    }
    s.webhookLimiter.record(tokenName)

    var req webhookRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Content) == "" {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
        return
    }

    text := strings.TrimSpace(req.Content)
    if title := strings.TrimSpace(req.Title); title != "" {
        text = title + "\n\n" + text
    }

    source := "webhook:" + tokenName
    captureID, _, _, err := capture.Ingest(r.Context(), s.db, s.brain, text, source)
    if err != nil {
        writeInternalError(w, err)
        return
    }

    writeJSON(w, http.StatusAccepted, webhookResponse{ID: captureID, Source: tokenName})
}
```

**Step 4: Add fields to Server struct and register route**

In `daemon/api/server.go`, add two fields to the `Server` struct:

```go
// Add to Server struct after loginLimiter:
webhookCfg     *config.WebhookConfig
webhookLimiter  *rateLimiter
```

Add the import `"scaffold/config"` to server.go's import block.

Register route in `New()`, after the `POST /api/capture` line:

```go
s.mux.HandleFunc("POST /api/webhook", s.handleWebhook)
```

Note: No `protected()` wrapper — webhook uses its own auth.

**Step 5: Run tests**

```bash
cd daemon && go test ./api/... -run TestWebhook -v
```
Expected: PASS (7 tests).

Run full API suite to confirm no regressions:

```bash
cd daemon && go test ./api/... -v
```

**Step 6: Commit**

```bash
git add daemon/api/webhook.go daemon/api/webhook_test.go daemon/api/server.go
git commit -m "feat: webhook handler with per-token auth and rate limiting"
```

---

### Task 4: Wire into main.go

**Files:**
- Modify: `daemon/main.go`

**Step 1: Read main.go to understand wiring pattern**

Look for where `config.Load()` is called and where `srv.SetIngestor()` is called — the webhook config wires in the same way.

**Step 2: Add webhook config loading**

Find the config loading section in `main.go`. After `config.Load()` succeeds, add:

```go
webhookCfgPath := filepath.Join(cfg.ConfigDir, "webhooks.yaml") // or wherever CONFIG_DIR resolves
webhookCfg, webhookFound, err := config.LoadWebhookConfig(webhookCfgPath)
if err != nil {
    log.Fatalf("webhook config: %v", err)
}
if webhookFound {
    srv.SetWebhookConfig(webhookCfg)
    log.Printf("webhooks: enabled (%d tokens configured)", len(webhookCfg.Tokens))
} else {
    log.Printf("webhooks: disabled (config/webhooks.yaml not found)")
}
```

You'll need to determine the config dir path — look at how the existing code derives `CONFIG_DIR` from env. Likely something like:

```go
configDir := os.Getenv("CONFIG_DIR")
if configDir == "" {
    configDir = "./config"
}
```

Use `filepath.Join(configDir, "webhooks.yaml")` as the path.

**Step 3: Build**

```bash
cd daemon && go build ./...
```
Expected: success, no errors.

**Step 4: Run all tests**

```bash
cd daemon && go test ./...
```
Expected: all pass.

**Step 5: Commit**

```bash
git add daemon/main.go
git commit -m "feat: wire webhook config into daemon startup"
```

---

### Task 5: Smoke test

**Step 1: Create a local webhooks.yaml**

```bash
cp config/webhooks.yaml.example config/webhooks.yaml
```

Edit `config/webhooks.yaml` — set a real token for one entry:

```yaml
rate_limit:
  max: 60
  window_minutes: 60
tokens:
  test: my-local-test-token-123
```

**Step 2: Build and restart**

```bash
cd daemon && go build -o bin/scaffold-daemon . && systemctl --user restart scaffold-daemon.service
```

**Step 3: Check logs confirm webhooks enabled**

```bash
journalctl --user -u scaffold-daemon.service -n 20
```
Expected: `webhooks: enabled (1 tokens configured)`

**Step 4: Fire a test webhook**

```bash
curl -s -X POST http://127.0.0.1:46873/api/webhook \
  -H "Authorization: Bearer my-local-test-token-123" \
  -H "Content-Type: application/json" \
  -d '{"title":"Smoke test","content":"Webhook integration working"}' | jq .
```
Expected: `{"id":"...","source":"test"}`

**Step 5: Confirm it landed in inbox**

```bash
curl -s http://127.0.0.1:46873/api/inbox \
  -H "Authorization: Bearer $API_TOKEN" | jq '.[] | select(.source | startswith("webhook"))'
```
Expected: the capture appears with `source: "webhook:test"`.

**Step 6: Test 401**

```bash
curl -s -o /dev/null -w "%{http_code}" -X POST http://127.0.0.1:46873/api/webhook \
  -H "Authorization: Bearer wrong-token" \
  -H "Content-Type: application/json" \
  -d '{"content":"should fail"}'
```
Expected: `401`

**Step 7: Final commit if any smoke-test fixes needed**

```bash
git add -p
git commit -m "fix: webhook smoke test corrections"
```

---

## Route Summary

```
POST /api/webhook
  Authorization: Bearer <token-from-webhooks.yaml>
  {"title": "optional", "content": "required"}

  → 202 {"id": "...", "source": "fitness"}
  → 400 missing content
  → 401 bad/missing token
  → 429 rate limited (Retry-After header)
  → 503 webhooks.yaml not present
```
