package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type deskPatchRequest struct {
	Status string `json:"status"`
}

func (s *Server) handleDesk(w http.ResponseWriter, r *http.Request) {
	items, err := s.db.TodaysDesk()
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDeskPatch(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "desk id is required"})
		return
	}

	var req deskPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if _, ok := validDeskStatuses[status]; !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be one of: active, done, deferred"})
		return
	}

	if err := s.db.UpdateDeskStatus(id, status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "desk item not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": status})
}

func (s *Server) handleDeskDefer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "desk id is required"})
		return
	}

	if err := s.db.DeferDeskItem(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "desk item not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "deferred"})
}

var validDeskStatuses = map[string]struct{}{
	"active":   {},
	"done":     {},
	"deferred": {},
}
