package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"scaffold/config"
	_ "scaffold/webhook" // register github extractor
)

const testWebhookToken = "webhook-test-token-abc"

func webhookConfig(max, windowMins int, tokens map[string]*config.WebhookToken) *config.WebhookConfig {
	return &config.WebhookConfig{
		RateLimit: config.WebhookRateLimit{Max: max, WindowMinutes: windowMins},
		Tokens:    tokens,
	}
}

func simpleTokens(pairs ...string) map[string]*config.WebhookToken {
	m := make(map[string]*config.WebhookToken, len(pairs)/2)
	for i := 0; i < len(pairs)-1; i += 2 {
		m[pairs[i]] = &config.WebhookToken{Token: pairs[i+1]}
	}
	return m
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

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, testWebhookToken, `{"content":"hello"}`))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestWebhookMissingToken(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetWebhookConfig(webhookConfig(60, 60, simpleTokens("test", testWebhookToken)))

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, "", `{"content":"hello"}`))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestWebhookInvalidToken(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetWebhookConfig(webhookConfig(60, 60, simpleTokens("test", testWebhookToken)))

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, "wrong-token", `{"content":"hello"}`))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestWebhookMissingContent(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetWebhookConfig(webhookConfig(60, 60, simpleTokens("test", testWebhookToken)))

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, testWebhookToken, `{"title":"no content"}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestWebhookRateLimited(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetWebhookConfig(webhookConfig(2, 60, simpleTokens("test", testWebhookToken)))

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
	srv.SetWebhookConfig(webhookConfig(60, 60, simpleTokens("fitness", testWebhookToken)))

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, testWebhookToken,
		`{"title":"Weight logged","content":"185 lbs this morning"}`))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookSuccessNoTitle(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetWebhookConfig(webhookConfig(60, 60, simpleTokens("homelab", testWebhookToken)))

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, webhookRequest(http.MethodPost, testWebhookToken,
		`{"content":"disk usage at 87%"}`))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

func ghSign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func githubWebhookRequest(token string, eventType string, body []byte, secret string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/webhook", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", eventType)
	if secret != "" {
		req.Header.Set("X-Hub-Signature-256", ghSign(secret, body))
	}
	return req
}

func TestWebhookExtractorIssueOpened(t *testing.T) {
	srv, _ := newTestServer(t)
	secret := "test-hmac-secret"
	tokens := map[string]*config.WebhookToken{
		"github": {Token: testWebhookToken, Type: "github", Secret: secret},
	}
	srv.SetWebhookConfig(webhookConfig(60, 60, tokens))

	body := []byte(`{
		"action": "opened",
		"issue": {"title": "Bug report", "body": "Steps...", "html_url": "https://github.com/o/r/issues/1", "number": 1, "user": {"login": "alice"}},
		"repository": {"full_name": "owner/repo"}
	}`)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, githubWebhookRequest(testWebhookToken, "issues", body, secret))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp extractorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.EventCount != 1 {
		t.Fatalf("expected event_count=1, got %d", resp.EventCount)
	}
}

func TestWebhookExtractorHMACFailure(t *testing.T) {
	srv, _ := newTestServer(t)
	tokens := map[string]*config.WebhookToken{
		"github": {Token: testWebhookToken, Type: "github", Secret: "real-secret"},
	}
	srv.SetWebhookConfig(webhookConfig(60, 60, tokens))

	body := []byte(`{"action":"opened","issue":{"title":"X","body":"","html_url":"","number":1,"user":{"login":"u"}},"repository":{"full_name":"o/r"}}`)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, githubWebhookRequest(testWebhookToken, "issues", body, "wrong-secret"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad HMAC, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookExtractorIgnoredEvent(t *testing.T) {
	srv, _ := newTestServer(t)
	tokens := map[string]*config.WebhookToken{
		"github": {Token: testWebhookToken, Type: "github"},
	}
	srv.SetWebhookConfig(webhookConfig(60, 60, tokens))

	body := []byte(`{"action":"created"}`)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, githubWebhookRequest(testWebhookToken, "star", body, ""))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp extractorResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.EventCount != 0 {
		t.Fatalf("expected event_count=0 for ignored event, got %d", resp.EventCount)
	}
}

func TestWebhookExtractorFallbackNoBrain(t *testing.T) {
	srv, _ := newTestServer(t)
	tokens := map[string]*config.WebhookToken{
		"github": {Token: testWebhookToken, Type: "github"},
	}
	srv.SetWebhookConfig(webhookConfig(60, 60, tokens))

	body := []byte(`{
		"action": "opened",
		"issue": {"title": "Test issue", "body": "Body text", "html_url": "https://github.com/o/r/issues/99", "number": 99, "user": {"login": "mike"}},
		"repository": {"full_name": "owner/repo"}
	}`)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, githubWebhookRequest(testWebhookToken, "issues", body, ""))

	// Should still succeed (falls back to capture.Ingest since brain is nil)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}
