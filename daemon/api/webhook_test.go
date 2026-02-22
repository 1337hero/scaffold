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
