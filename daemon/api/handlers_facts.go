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

type factCreateRequest struct {
	Entity          string   `json:"entity"`
	Content         string   `json:"content"`
	Category        *string  `json:"category"` // user_pref|project|tool|general
	Tags            *string  `json:"tags"`     // comma-separated
	RelatedEntities []string `json:"related_entities"`
}

// --- CRUD ---

func (s *Server) handleFactsList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var entity *string
	if raw := strings.TrimSpace(q.Get("entity")); raw != "" {
		entity = &raw
	}
	var category *string
	if raw := strings.TrimSpace(q.Get("category")); raw != "" {
		category = &raw
	}
	var tag *string
	if raw := strings.TrimSpace(q.Get("tag")); raw != "" {
		tag = &raw
	}
	limit := 0
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
			return
		}
		limit = v
	}

	facts, err := s.db.ListFacts(entity, category, tag, limit)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, facts)
}

func (s *Server) handleFactCreate(w http.ResponseWriter, r *http.Request) {
	var req factCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.Entity) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "entity is required"})
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}

	f := db.Fact{
		ID:      uuid.New().String(),
		Entity:  req.Entity,
		Content: req.Content,
	}
	if req.Category != nil {
		f.Category = sql.NullString{String: *req.Category, Valid: true}
	}
	if req.Tags != nil {
		f.Tags = sql.NullString{String: *req.Tags, Valid: true}
	}

	if err := s.db.InsertFact(f, req.RelatedEntities); err != nil {
		if errors.Is(err, db.ErrInvalidEnum) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeInternalError(w, err)
		return
	}

	created, err := s.db.GetFact(f.ID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleFactGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	fact, err := s.db.GetFact(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if fact == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "fact not found"})
		return
	}
	writeJSON(w, http.StatusOK, fact)
}

func (s *Server) handleFactPatch(w http.ResponseWriter, r *http.Request) {
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
	if len(updates) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no fields to update"})
		return
	}

	if err := s.db.UpdateFact(id, updates); err != nil {
		if errors.Is(err, db.ErrInvalidEnum) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "fact not found"})
			return
		}
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleFactDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if err := s.db.SuppressFact(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "fact not found"})
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "suppressed": "true"})
}

// --- Operations ---

func (s *Server) handleFactsProbe(w http.ResponseWriter, r *http.Request) {
	entity := strings.TrimSpace(r.URL.Query().Get("entity"))
	if entity == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "entity is required"})
		return
	}

	facts, err := s.db.ProbeFacts(entity)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, facts)
}

func (s *Server) handleFactsReason(w http.ResponseWriter, r *http.Request) {
	var entities []string
	for _, raw := range strings.Split(r.URL.Query().Get("entities"), ",") {
		if e := strings.TrimSpace(raw); e != "" {
			entities = append(entities, e)
		}
	}
	if len(entities) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "entities requires at least two comma-separated values"})
		return
	}

	facts, err := s.db.ReasonFacts(entities)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, facts)
}

func (s *Server) handleFactsRelated(w http.ResponseWriter, r *http.Request) {
	entity := strings.TrimSpace(r.URL.Query().Get("entity"))
	if entity == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "entity is required"})
		return
	}

	entities, err := s.db.RelatedEntities(entity)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entities)
}

func (s *Server) handleFactsContradict(w http.ResponseWriter, r *http.Request) {
	entity := strings.TrimSpace(r.URL.Query().Get("entity"))
	if entity == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "entity is required"})
		return
	}

	facts, err := s.db.ContradictingFacts(entity)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, facts)
}

func (s *Server) handleFactFeedback(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	var req struct {
		Helpful *bool `json:"helpful"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if req.Helpful == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "helpful is required"})
		return
	}

	if err := s.db.FeedbackFact(id, *req.Helpful); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "fact not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	// Return the updated fact so callers see the new trust without a second call.
	fact, err := s.db.GetFact(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fact)
}
