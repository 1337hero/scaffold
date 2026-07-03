package api

import (
	"net/http"
)

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Webhook endpoint kept as health check — no task-creation or capture logic.
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
