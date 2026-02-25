package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"scaffold/capture"
	"scaffold/config"
	"scaffold/webhook"
)

type webhookPayload struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type webhookResponse struct {
	ID     string `json:"id"`
	Source string `json:"source"`
}

type extractorResponse struct {
	EventCount int    `json:"event_count"`
	Source     string `json:"source"`
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
		bearer = r.URL.Query().Get("token")
	}
	if bearer == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bearer token required"})
		return
	}

	var tokenName string
	var tok *config.WebhookToken
	for name, t := range s.webhookCfg.Tokens {
		if subtle.ConstantTimeCompare([]byte(bearer), []byte(t.Token)) == 1 {
			tokenName = name
			tok = t
			break
		}
	}
	if tokenName == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}

	if !s.webhookLimiter.allowAndRecord(tokenName) {
		retryAfter := strconv.Itoa(s.webhookCfg.RateLimit.WindowMinutes * 60)
		w.Header().Set("Retry-After", retryAfter)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}

	// Extractor-typed tokens get the structured pipeline
	if tok.Type != "" {
		s.handleExtractorWebhook(w, r, tokenName, tok)
		return
	}

	// Generic path: {title, content} → capture.Ingest
	s.handleGenericWebhook(w, r, tokenName)
}

func (s *Server) handleGenericWebhook(w http.ResponseWriter, r *http.Request, tokenName string) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req webhookPayload
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

	writeJSON(w, http.StatusAccepted, webhookResponse{ID: captureID, Source: source})
}

func (s *Server) handleExtractorWebhook(w http.ResponseWriter, r *http.Request, tokenName string, tok *config.WebhookToken) {
	// Larger body limit for structured payloads (GitHub push can be large)
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	ext, ok := webhook.Get(tok.Type)
	if !ok {
		log.Printf("webhook: no extractor registered for type %q", tok.Type)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported webhook type"})
		return
	}

	if err := ext.Verify(tok.Secret, r.Header, body); err != nil {
		log.Printf("webhook: signature verification failed for %s: %v", tokenName, err)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "signature verification failed"})
		return
	}

	events, err := ext.Extract(r.Header, body)
	if err != nil {
		log.Printf("webhook: extract failed for %s: %v", tokenName, err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse webhook payload"})
		return
	}

	if len(events) == 0 {
		writeJSON(w, http.StatusAccepted, extractorResponse{EventCount: 0, Source: "webhook:" + tokenName})
		return
	}

	source := "webhook:" + tokenName
	for _, event := range events {
		s.processWebhookEvent(r.Context(), event, source)
	}

	writeJSON(w, http.StatusAccepted, extractorResponse{EventCount: len(events), Source: source})
}

func (s *Server) processWebhookEvent(ctx context.Context, event webhook.Event, source string) {
	// Build structured message for agent
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("Webhook event from %s:\n", event.Source))
	msg.WriteString(fmt.Sprintf("Type: %s\n", event.EventType))
	msg.WriteString(fmt.Sprintf("Repo: %s\n", event.Repo))
	msg.WriteString(fmt.Sprintf("Title: %s\n", event.Title))
	msg.WriteString(fmt.Sprintf("Author: %s\n", event.Author))
	if event.URL != "" {
		msg.WriteString(fmt.Sprintf("URL: %s\n", event.URL))
	}
	if event.Body != "" {
		body := event.Body
		if len(body) > 500 {
			body = body[:500] + "..."
		}
		msg.WriteString(fmt.Sprintf("\nBody:\n%s\n", body))
	}

	// Dedup: check if task already exists for this source_ref
	if event.URL != "" {
		existing, err := s.db.TaskBySourceRef(event.URL)
		if err != nil {
			log.Printf("webhook: dedup lookup failed: %v", err)
		} else if existing != nil {
			msg.WriteString(fmt.Sprintf("\nNote: A task already exists for this URL (id=%s, status=%s, title=%q). Consider adding a note instead of creating a duplicate.\n", existing.ID, existing.Status, existing.Title))
		}
	}

	message := msg.String()

	// Try agent triage if brain is available
	if s.brain != nil {
		agentCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()

		response, err := s.brain.Respond(agentCtx, message, nil)
		if err != nil {
			log.Printf("webhook: agent triage failed for %s event, falling back to capture: %v", event.EventType, err)
			s.fallbackCapture(ctx, event, source)
			return
		}

		// Log the exchange
		s.db.InsertConversationEntry(source, "user", message)
		s.db.InsertConversationEntry(source, "assistant", response)
		return
	}

	// No brain: fall back to capture
	s.fallbackCapture(ctx, event, source)
}

func (s *Server) fallbackCapture(ctx context.Context, event webhook.Event, source string) {
	text := event.Title
	if event.Body != "" {
		text += "\n\n" + event.Body
	}
	if event.URL != "" {
		text += "\n\nSource: " + event.URL
	}

	if _, _, _, err := capture.Ingest(ctx, s.db, s.brain, text, source); err != nil {
		log.Printf("webhook: fallback capture failed: %v", err)
	}
}
