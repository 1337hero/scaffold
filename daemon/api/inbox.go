package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"scaffold/db"
)

type inboxOverrideRequest struct {
	Type       string   `json:"type"`
	Action     string   `json:"action"`
	Importance *float64 `json:"importance"`
	Tags       []string `json:"tags"`
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	captures, err := s.db.ListRecent(50)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	filtered := captures[:0]
	for _, capture := range captures {
		if strings.EqualFold(strings.TrimSpace(capture.Source), "user:archive") {
			continue
		}
		filtered = append(filtered, capture)
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (s *Server) handleInboxConfirm(w http.ResponseWriter, r *http.Request) {
	captureID := strings.TrimSpace(r.PathValue("id"))
	if captureID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "capture id is required"})
		return
	}

	if err := s.db.ConfirmCapture(captureID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "capture not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": captureID, "confirmed": true})
}

func (s *Server) handleInboxArchive(w http.ResponseWriter, r *http.Request) {
	captureID := strings.TrimSpace(r.PathValue("id"))
	if captureID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "capture id is required"})
		return
	}

	capture, err := s.db.GetCapture(captureID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if capture == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "capture not found"})
		return
	}
	if !capture.MemoryID.Valid || strings.TrimSpace(capture.MemoryID.String) == "" {
		if err := s.db.UpdateCaptureSource(captureID, "user:archive"); err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":             captureID,
			"memory_missing": true,
			"archived":       true,
		})
		return
	}

	memoryID := strings.TrimSpace(capture.MemoryID.String)
	memoryMissing := false
	if err := s.db.SuppressMemory(memoryID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			writeInternalError(w, err)
			return
		}
		memoryMissing = true
	}

	if err := s.db.UpdateCaptureSource(captureID, "user:archive"); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":             captureID,
		"memory_id":      memoryID,
		"memory_missing": memoryMissing,
		"archived":       true,
	})
}

func (s *Server) handleInboxOverride(w http.ResponseWriter, r *http.Request) {
	captureID := strings.TrimSpace(r.PathValue("id"))
	if captureID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "capture id is required"})
		return
	}

	var req inboxOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	overrideType, ok := canonicalMemoryType(req.Type)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid type"})
		return
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if _, ok := validTriageActions[action]; !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid action"})
		return
	}

	if req.Importance == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "importance is required"})
		return
	}
	importance := *req.Importance
	if importance < 0 || importance > 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "importance must be between 0 and 1"})
		return
	}

	capture, err := s.db.GetCapture(captureID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if capture == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "capture not found"})
		return
	}
	if !capture.MemoryID.Valid || strings.TrimSpace(capture.MemoryID.String) == "" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "capture has no linked memory"})
		return
	}
	memoryID := strings.TrimSpace(capture.MemoryID.String)

	params := db.ReclassifyParams{
		Type:       overrideType,
		Action:     action,
		Tags:       strings.Join(cleanTags(req.Tags), ","),
		Importance: importance,
	}
	if err := s.db.ReclassifyMemory(memoryID, params); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "memory not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	if err := s.db.UpdateCaptureSource(captureID, "user:override"); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         captureID,
		"memory_id":  memoryID,
		"type":       overrideType,
		"action":     action,
		"importance": importance,
		"tags":       cleanTags(req.Tags),
	})
}

func (s *Server) handleMemories(w http.ResponseWriter, r *http.Request) {
	memories, err := s.db.ListByImportance(50)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, memories)
}

func cleanTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			out = append(out, tag)
		}
	}
	return out
}

var memoryTypeMap = map[string]string{
	"identity":    "Identity",
	"goal":        "Goal",
	"decision":    "Decision",
	"todo":        "Todo",
	"idea":        "Idea",
	"preference":  "Preference",
	"fact":        "Fact",
	"event":       "Event",
	"observation": "Observation",
}

var validTriageActions = map[string]struct{}{
	"do":        {},
	"explore":   {},
	"reference": {},
	"waiting":   {},
}

func canonicalMemoryType(memoryType string) (string, bool) {
	canonical, ok := memoryTypeMap[strings.ToLower(strings.TrimSpace(memoryType))]
	return canonical, ok
}
