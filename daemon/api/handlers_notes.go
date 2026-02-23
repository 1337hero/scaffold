package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"scaffold/db"
)

type noteCreateRequest struct {
	Title    string  `json:"title"`
	DomainID *int64  `json:"domain_id"`
	GoalID   *string `json:"goal_id"`
	Content  *string `json:"content"`
	Tags     *string `json:"tags"`
}

func (s *Server) handleNotesList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tags := strings.TrimSpace(q.Get("tags"))

	var domainID *int
	if raw := q.Get("domain_id"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid domain_id"})
			return
		}
		domainID = &v
	}

	var goalID *string
	if raw := strings.TrimSpace(q.Get("goal_id")); raw != "" {
		goalID = &raw
	}

	notes, err := s.db.ListNotes(domainID, goalID, tags)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *Server) handleNoteGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	note, err := s.db.GetNote(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if note == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
		return
	}

	writeJSON(w, http.StatusOK, note)
}

func (s *Server) handleNoteCreate(w http.ResponseWriter, r *http.Request) {
	var req noteCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	n := db.Note{
		ID:    uuid.New().String(),
		Title: req.Title,
	}

	if req.DomainID != nil {
		n.DomainID = sql.NullInt64{Int64: *req.DomainID, Valid: true}
	}
	if req.GoalID != nil {
		n.GoalID = sql.NullString{String: *req.GoalID, Valid: true}
	}
	if req.Content != nil {
		n.Content = sql.NullString{String: *req.Content, Valid: true}
	}
	if req.Tags != nil {
		n.Tags = sql.NullString{String: *req.Tags, Valid: true}
	}

	if err := s.db.InsertNote(n); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) handleNoteUpdate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if err := s.db.UpdateNote(id, updates); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "updated": "true"})
}

func (s *Server) handleNoteDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if err := s.db.DeleteNote(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "deleted": "true"})
}
