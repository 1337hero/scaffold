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

type goalCreateRequest struct {
	Title        string   `json:"title"`
	DomainID     *int64   `json:"domain_id"`
	Context      *string  `json:"context"`
	DueDate      *string  `json:"due_date"`
	Type         string   `json:"type"`
	TargetValue  *float64 `json:"target_value"`
	CurrentValue *float64 `json:"current_value"`
	HabitType    *string  `json:"habit_type"`
	ScheduleDays *string  `json:"schedule_days"`
	Notify       *int     `json:"notify"`
}

type goalDetailResponse struct {
	Goal  *db.Goal   `json:"goal"`
	Tasks []db.Task  `json:"tasks"`
	Notes []db.Note  `json:"notes"`
}

func (s *Server) handleGoalsList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))

	var domainID *int
	if raw := q.Get("domain_id"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid domain_id"})
			return
		}
		domainID = &v
	}

	if status != "" {
		goals, err := s.db.ListGoals(domainID, status)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, goals)
		return
	}

	goals, err := s.db.GoalsWithProgress(domainID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, goals)
}

func (s *Server) handleGoalGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	goal, err := s.db.GetGoal(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if goal == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "goal not found"})
		return
	}

	tasks, err := s.db.TasksByGoal(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	notes, err := s.db.ListNotes(nil, &id, "")
	if err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, goalDetailResponse{
		Goal:  goal,
		Tasks: tasks,
		Notes: notes,
	})
}

func (s *Server) handleGoalCreate(w http.ResponseWriter, r *http.Request) {
	var req goalCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	g := db.Goal{
		ID:    uuid.New().String(),
		Title: req.Title,
		Type:  req.Type,
	}

	if req.DomainID != nil {
		g.DomainID = sql.NullInt64{Int64: *req.DomainID, Valid: true}
	}
	if req.Context != nil {
		g.Context = sql.NullString{String: *req.Context, Valid: true}
	}
	if req.DueDate != nil {
		g.DueDate = sql.NullString{String: *req.DueDate, Valid: true}
	}
	if req.TargetValue != nil {
		g.TargetValue = sql.NullFloat64{Float64: *req.TargetValue, Valid: true}
	}
	if req.CurrentValue != nil {
		g.CurrentValue = sql.NullFloat64{Float64: *req.CurrentValue, Valid: true}
	}
	if req.HabitType != nil {
		g.HabitType = sql.NullString{String: *req.HabitType, Valid: true}
	}
	if req.ScheduleDays != nil {
		g.ScheduleDays = sql.NullString{String: *req.ScheduleDays, Valid: true}
	}
	if req.Notify != nil {
		g.Notify = *req.Notify
	}

	if err := s.db.InsertGoal(g); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleGoalUpdate(w http.ResponseWriter, r *http.Request) {
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

	if err := s.db.UpdateGoal(id, updates); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "goal not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "updated": "true"})
}

func (s *Server) handleGoalDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if err := s.db.SoftDeleteGoal(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "goal not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "deleted": "true"})
}
