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

type taskCreateRequest struct {
	Title      string  `json:"title"`
	DomainID   *int64  `json:"domain_id"`
	GoalID     *string `json:"goal_id"`
	Context    *string `json:"context"`
	DueDate    *string `json:"due_date"`
	Recurring  *string `json:"recurring"`
	Priority   string  `json:"priority"`
	MicroSteps *string `json:"micro_steps"`
	Notify     *int    `json:"notify"`
	Position   *int    `json:"position"`
}

type reorderRequest struct {
	Position int `json:"position"`
}

func (s *Server) handleTasksList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	due := strings.TrimSpace(q.Get("due"))

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

	tasks, err := s.db.ListTasks(domainID, goalID, status, due)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleTaskCreate(w http.ResponseWriter, r *http.Request) {
	var req taskCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	t := db.Task{
		ID:       uuid.New().String(),
		Title:    req.Title,
		Priority: req.Priority,
	}

	if req.DomainID != nil {
		t.DomainID = sql.NullInt64{Int64: *req.DomainID, Valid: true}
	}
	if req.GoalID != nil {
		t.GoalID = sql.NullString{String: *req.GoalID, Valid: true}
	}
	if req.Context != nil {
		t.Context = sql.NullString{String: *req.Context, Valid: true}
	}
	if req.DueDate != nil {
		t.DueDate = sql.NullString{String: *req.DueDate, Valid: true}
	}
	if req.Recurring != nil {
		t.Recurring = sql.NullString{String: *req.Recurring, Valid: true}
	}
	if req.MicroSteps != nil {
		t.MicroSteps = sql.NullString{String: *req.MicroSteps, Valid: true}
	}
	if req.Notify != nil {
		t.Notify = *req.Notify
	}
	if req.Position != nil {
		t.Position = *req.Position
	}

	if err := s.db.InsertTask(t); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleTaskUpdate(w http.ResponseWriter, r *http.Request) {
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

	if err := s.db.UpdateTask(id, updates); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "updated": "true"})
}

func (s *Server) handleTaskComplete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if err := s.db.CompleteTask(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "completed": "true"})
}

func (s *Server) handleTaskReorder(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	var req reorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if err := s.db.ReorderTask(id, req.Position); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "position": strconv.Itoa(req.Position)})
}

func (s *Server) handleTaskSetFocus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if err := s.db.SetFocus(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found or not pending"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "focus": "true"})
}

func (s *Server) handleTaskClearFocus(w http.ResponseWriter, r *http.Request) {
	if err := s.db.ClearFocus(); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"focus": "cleared"})
}

func (s *Server) handleTaskDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if err := s.db.SoftDeleteTask(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "deleted": "true"})
}
