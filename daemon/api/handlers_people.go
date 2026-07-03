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

type personCreateRequest struct {
	Name               string   `json:"name"`
	Surface            *string  `json:"surface"`
	DomainID           *int64   `json:"domain_id"`
	Relationship       *string  `json:"relationship"`
	Birthday           *string  `json:"birthday"`
	Anniversary        *string  `json:"anniversary"`
	Spouse             *string  `json:"spouse"`
	Kids               []db.Kid `json:"kids"`
	Notes              *string  `json:"notes"`
	ContactCadenceDays *int64   `json:"contact_cadence_days"`
}

type interactionCreateRequest struct {
	Date         *string `json:"date"`
	Summary      string  `json:"summary"`
	FollowUp     *string `json:"follow_up"`
	FollowUpDate *string `json:"follow_up_date"`
}

func (s *Server) handlePeopleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var surface *string
	if raw := strings.TrimSpace(q.Get("surface")); raw != "" {
		surface = &raw
	}
	var relationship *string
	if raw := strings.TrimSpace(q.Get("relationship")); raw != "" {
		relationship = &raw
	}

	people, err := s.db.ListPeople(surface, relationship)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, people)
}

func (s *Server) handlePersonCreate(w http.ResponseWriter, r *http.Request) {
	var req personCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	kids, err := db.MarshalKids(req.Kids)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid kids"})
		return
	}

	p := db.Person{
		ID:      uuid.New().String(),
		Name:    req.Name,
		Surface: "life",
		Kids:    kids,
	}
	if req.Surface != nil {
		p.Surface = *req.Surface
	}
	if req.DomainID != nil {
		p.DomainID = sql.NullInt64{Int64: *req.DomainID, Valid: true}
	}
	if req.Relationship != nil {
		p.Relationship = sql.NullString{String: *req.Relationship, Valid: true}
	}
	if req.Birthday != nil {
		p.Birthday = sql.NullString{String: *req.Birthday, Valid: true}
	}
	if req.Anniversary != nil {
		p.Anniversary = sql.NullString{String: *req.Anniversary, Valid: true}
	}
	if req.Spouse != nil {
		p.Spouse = sql.NullString{String: *req.Spouse, Valid: true}
	}
	if req.Notes != nil {
		p.Notes = sql.NullString{String: *req.Notes, Valid: true}
	}
	if req.ContactCadenceDays != nil {
		p.ContactCadenceDays = sql.NullInt64{Int64: *req.ContactCadenceDays, Valid: true}
	}

	if err := s.db.InsertPerson(p); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handlePersonGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	person, err := s.db.GetPerson(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if person == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "person not found"})
		return
	}
	writeJSON(w, http.StatusOK, person)
}

func (s *Server) handlePersonPatch(w http.ResponseWriter, r *http.Request) {
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

	// kids arrives as JSON; store it as a marshalled string column.
	if raw, ok := updates["kids"]; ok {
		b, err := json.Marshal(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid kids"})
			return
		}
		updates["kids"] = string(b)
	}

	if err := s.db.UpdatePerson(id, updates); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "person not found"})
			return
		}
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePersonDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if err := s.db.SuppressPerson(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "person not found"})
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "suppressed": "true"})
}

func (s *Server) handleInteractionsList(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	interactions, err := s.db.ListInteractions(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, interactions)
}

func (s *Server) handleInteractionCreate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	var req interactionCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.Summary) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "summary is required"})
		return
	}

	person, err := s.db.GetPerson(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if person == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "person not found"})
		return
	}

	i := db.Interaction{
		ID:       uuid.New().String(),
		PersonID: id,
		Summary:  req.Summary,
	}
	if req.Date != nil {
		i.Date = *req.Date
	}
	if req.FollowUp != nil {
		i.FollowUp = sql.NullString{String: *req.FollowUp, Valid: true}
	}
	if req.FollowUpDate != nil {
		i.FollowUpDate = sql.NullString{String: *req.FollowUpDate, Valid: true}
	}

	if err := s.db.InsertInteraction(i); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "person not found"})
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, i)
}

func (s *Server) handleBirthdays(w http.ResponseWriter, r *http.Request) {
	days := 7
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid days"})
			return
		}
		days = v
	}

	hits, err := s.db.UpcomingBirthdays(days)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, hits)
}
