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

type projectCreateRequest struct {
	Name        string  `json:"name"`
	Type        *string `json:"type"`    // project|area|retainer
	Surface     *string `json:"surface"` // life|business
	DomainID    *int64  `json:"domain_id"`
	Status      *string `json:"status"`
	StartDate   *string `json:"start_date"`
	EndDate     *string `json:"end_date"`
	Description *string `json:"description"`
}

type milestoneCreateRequest struct {
	Title    string `json:"title"`
	Position *int   `json:"position"`
}

type checklistCreateRequest struct {
	Title string `json:"title"`
	Items string `json:"items"` // JSON array
}

type activityCreateRequest struct {
	Description string   `json:"description"`
	Hours       *float64 `json:"hours"`
}

// projectDetailResponse is the enriched GET response including sub-entities.
type projectDetailResponse struct {
	Project            db.Project     `json:"project"`
	Milestones         []db.Milestone `json:"milestones"`
	MilestoneCompleted int            `json:"milestone_completed"`
	MilestoneTotal     int            `json:"milestone_total"`
	Checklists         []db.Checklist `json:"checklists"`
	RecentActivity     []db.Activity  `json:"recent_activity"`
}

// --- Projects ---

func (s *Server) handleProjectsList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var typeFilter *string
	if raw := strings.TrimSpace(q.Get("type")); raw != "" {
		typeFilter = &raw
	}
	var surface *string
	if raw := strings.TrimSpace(q.Get("surface")); raw != "" {
		surface = &raw
	}
	var status *string
	if raw := strings.TrimSpace(q.Get("status")); raw != "" {
		status = &raw
	}
	var domainID *int64
	if raw := strings.TrimSpace(q.Get("domain_id")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid domain_id"})
			return
		}
		domainID = &v
	}

	projects, err := s.db.ListProjects(typeFilter, surface, status, domainID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (s *Server) handleProjectCreate(w http.ResponseWriter, r *http.Request) {
	var req projectCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	p := db.Project{
		ID:      uuid.New().String(),
		Name:    req.Name,
		Type:    "project",
		Surface: "life",
		Status:  "active",
	}
	if req.Type != nil {
		p.Type = *req.Type
	}
	if req.Surface != nil {
		p.Surface = *req.Surface
	}
	if req.DomainID != nil {
		p.DomainID = sql.NullInt64{Int64: *req.DomainID, Valid: true}
	}
	if req.Status != nil {
		p.Status = *req.Status
	}
	if req.StartDate != nil {
		p.StartDate = sql.NullString{String: *req.StartDate, Valid: true}
	}
	if req.EndDate != nil {
		p.EndDate = sql.NullString{String: *req.EndDate, Valid: true}
	}
	if req.Description != nil {
		p.Description = sql.NullString{String: *req.Description, Valid: true}
	}

	if err := s.db.InsertProject(p); err != nil {
		if errors.Is(err, db.ErrInvalidEnum) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleProjectGet(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	project, err := s.db.GetProject(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if project == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	// Fetch detail.
	milestones, err := s.db.ListMilestones(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	milestoneCompleted, milestoneTotal, err := s.db.MilestoneCompletion(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	checklists, err := s.db.ListChecklists(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	activity, err := s.db.ListActivity(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	// Cap recent activity at 5.
	if len(activity) > 5 {
		activity = activity[:5]
	}

	detail := projectDetailResponse{
		Project:            *project,
		Milestones:         milestones,
		MilestoneCompleted: milestoneCompleted,
		MilestoneTotal:     milestoneTotal,
		Checklists:         checklists,
		RecentActivity:     activity,
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleProjectPatch(w http.ResponseWriter, r *http.Request) {
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

	if err := s.db.UpdateProject(id, updates); err != nil {
		if errors.Is(err, db.ErrInvalidEnum) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
			return
		}
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProjectDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if err := s.db.SuppressProject(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "archived": "true"})
}

// --- Milestones ---

func (s *Server) handleMilestonesList(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is required"})
		return
	}

	milestones, err := s.db.ListMilestones(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, milestones)
}

func (s *Server) handleMilestoneCreate(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("id"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is required"})
		return
	}

	var req milestoneCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	m := db.Milestone{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		Title:     req.Title,
	}
	if req.Position != nil {
		m.Position = *req.Position
	}

	// Verify project exists before inserting.
	proj, err := s.db.GetProject(projectID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if proj == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	if err := s.db.InsertMilestone(m); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleMilestonePatch(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "milestone id is required"})
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

	if err := s.db.UpdateMilestone(id, updates); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "milestone not found"})
			return
		}
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMilestoneDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "milestone id is required"})
		return
	}

	if err := s.db.DeleteMilestone(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "milestone not found"})
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "deleted": "true"})
}

// --- Checklists ---

func (s *Server) handleChecklistsList(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is required"})
		return
	}

	checklists, err := s.db.ListChecklists(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, checklists)
}

func (s *Server) handleChecklistCreate(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("id"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is required"})
		return
	}

	var req checklistCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}
	if strings.TrimSpace(req.Items) == "" {
		req.Items = "[]"
	}

	c := db.Checklist{
		ID:        uuid.New().String(),
		ProjectID: sql.NullString{String: projectID, Valid: true},
		Title:     req.Title,
		Items:     req.Items,
	}

	// Verify project exists before inserting.
	proj, err := s.db.GetProject(projectID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if proj == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	if err := s.db.InsertChecklist(c); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleChecklistClone(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("id"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is required"})
		return
	}

	var req struct {
		TemplateID string `json:"template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.TemplateID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "template_id is required"})
		return
	}

	cloned, err := s.db.CloneChecklist(req.TemplateID, projectID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if cloned == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}
	writeJSON(w, http.StatusCreated, cloned)
}

func (s *Server) handleChecklistPatch(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "checklist id is required"})
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

	if err := s.db.UpdateChecklist(id, updates); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "checklist not found"})
			return
		}
		writeInternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleChecklistTemplatesList(w http.ResponseWriter, r *http.Request) {
	templates, err := s.db.ListChecklistTemplates()
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

// --- Activity ---

func (s *Server) handleActivityList(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is required"})
		return
	}

	activity, err := s.db.ListActivity(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, activity)
}

func (s *Server) handleActivityCreate(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("id"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is required"})
		return
	}

	var req activityCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "description is required"})
		return
	}

	// Verify project exists before inserting.
	project, err := s.db.GetProject(projectID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if project == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	a := db.Activity{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		Description: req.Description,
	}
	if req.Hours != nil {
		a.Hours = sql.NullFloat64{Float64: *req.Hours, Valid: true}
	}

	if err := s.db.InsertActivity(a); err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

// --- Slipping ---

func (s *Server) handleProjectsSlipping(w http.ResponseWriter, r *http.Request) {
	projects, err := s.db.ProjectsSlipping()
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (s *Server) handleAreasSlipping(w http.ResponseWriter, r *http.Request) {
	areas, err := s.db.AreasSlipping()
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, areas)
}
