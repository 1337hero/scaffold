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

type webhookPayload struct {
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

	if !s.webhookLimiter.allowAndRecord(tokenName) {
		retryAfter := strconv.Itoa(s.webhookCfg.RateLimit.WindowMinutes * 60)
		w.Header().Set("Retry-After", retryAfter)
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}

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
