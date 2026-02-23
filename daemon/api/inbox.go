package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"scaffold/db"
)

type inboxOverrideRequest struct {
	Type       string   `json:"type"`
	Action     string   `json:"action"`
	Importance *float64 `json:"importance"`
	Tags       []string `json:"tags"`
}

type inboxCaptureResponse struct {
	ID           string         `json:"ID"`
	Raw          string         `json:"Raw"`
	Source       string         `json:"Source"`
	Processed    int            `json:"Processed"`
	TriageAction sql.NullString `json:"TriageAction"`
	MemoryID     sql.NullString `json:"MemoryID"`
	CreatedAt    string         `json:"CreatedAt"`
	Confirmed    int            `json:"Confirmed"`
	DomainID     sql.NullInt64  `json:"DomainID"`
	Title        string         `json:"Title"`
	Summary      string         `json:"Summary"`
	Type         string         `json:"Type"`
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	captures, err := s.db.ListInboxCaptures(50)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	out := make([]inboxCaptureResponse, 0, len(captures))
	for _, c := range captures {
		action := ""
		if c.TriageAction.Valid {
			action = c.TriageAction.String
		}

		title, summary := splitCaptureText(c.Raw, c.MemoryTitle.String)
		captureType := inferCaptureType(c.Raw, c.Source, action, c.MemoryType.String)

		out = append(out, inboxCaptureResponse{
			ID:           c.ID,
			Raw:          c.Raw,
			Source:       c.Source,
			Processed:    c.Processed,
			TriageAction: c.TriageAction,
			MemoryID:     c.MemoryID,
			CreatedAt:    c.CreatedAt,
			Confirmed:    c.Confirmed,
			DomainID:     c.DomainID,
			Title:        title,
			Summary:      summary,
			Type:         captureType,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func splitCaptureText(raw, memoryTitle string) (title, summary string) {
	if memoryTitle != "" {
		return memoryTitle, truncateOrDefault(raw, memoryTitle)
	}

	text := strings.TrimSpace(compactWhitespace(raw))
	if text == "" {
		return "Untitled capture", "Captured from inbox."
	}
	if len(text) <= 84 {
		return text, "Captured from inbox. Ready to triage."
	}

	window := text[:min(len(text), 108)]
	cut := maxIndex(
		strings.LastIndex(window, ". "),
		strings.LastIndex(window, " - "),
		strings.LastIndex(window, " \u2014 "),
		strings.LastIndex(window, "; "),
		strings.LastIndex(window, ", "),
	)
	if cut < 40 {
		cut = strings.LastIndex(window, " ")
	}
	if cut < 40 {
		cut = 84
	}

	title = strings.TrimSpace(text[:cut])
	remainder := strings.TrimSpace(text[cut:])
	remainder = strings.TrimLeft(remainder, ",.;:- ")
	if remainder == "" {
		remainder = "Captured from inbox. Ready to triage."
	}
	return title, remainder
}

var (
	rxURL     = regexp.MustCompile(`(?i)https?://|www\.`)
	rxVideo   = regexp.MustCompile(`(?i)\b(video|youtube|watch|vimeo)\b`)
	rxIdea    = regexp.MustCompile(`(?i)\b(idea|what if|maybe)\b`)
	rxArticle = regexp.MustCompile(`(?i)\b(article|research|paper|spec)\b`)
)

var actionTypeMap = map[string]string{
	"do":        "task",
	"explore":   "idea",
	"reference": "note",
}

func inferCaptureType(raw, source, action, memoryType string) string {
	if strings.EqualFold(memoryType, "Goal") {
		return "goal"
	}
	if rxURL.MatchString(raw) {
		return "link"
	}
	if rxVideo.MatchString(raw) {
		return "video"
	}
	if rxIdea.MatchString(raw) {
		return "idea"
	}
	if rxArticle.MatchString(raw) {
		return "article"
	}
	if strings.HasPrefix(source, "signal") {
		return "note"
	}
	if t, ok := actionTypeMap[action]; ok {
		return t
	}
	return "note"
}

func compactWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func truncateOrDefault(raw, title string) string {
	text := strings.TrimSpace(compactWhitespace(raw))
	remainder := strings.TrimPrefix(text, title)
	remainder = strings.TrimSpace(remainder)
	remainder = strings.TrimLeft(remainder, ",.;:- ")
	if remainder == "" {
		return "Captured from inbox. Ready to triage."
	}
	return remainder
}

func maxIndex(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
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

type inboxProcessRequest struct {
	Type     string  `json:"type"`
	Title    string  `json:"title"`
	DomainID *int64  `json:"domain_id"`
	GoalID   *string `json:"goal_id"`
	Context  *string `json:"context"`
	DueDate  *string `json:"due_date"`
	Priority *string `json:"priority"`
	Recurring *string `json:"recurring"`
	Content  *string `json:"content"`
	Tags     *string `json:"tags"`

	GoalType     *string  `json:"goal_type"`
	TargetValue  *float64 `json:"target_value"`
	CurrentValue *float64 `json:"current_value"`
	HabitType    *string  `json:"habit_type"`
	ScheduleDays *string  `json:"schedule_days"`
}

func (s *Server) handleInboxProcess(w http.ResponseWriter, r *http.Request) {
	captureID := strings.TrimSpace(r.PathValue("id"))
	if captureID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "capture id is required"})
		return
	}

	var req inboxProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
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

	var createdID string

	switch req.Type {
	case "goal":
		g := db.Goal{
			ID:    uuid.New().String(),
			Title: req.Title,
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
		if req.GoalType != nil {
			g.Type = *req.GoalType
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
		if err := s.db.InsertGoal(g); err != nil {
			writeInternalError(w, err)
			return
		}
		createdID = g.ID

	case "task":
		t := db.Task{
			ID:    uuid.New().String(),
			Title: req.Title,
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
		if req.Priority != nil {
			t.Priority = *req.Priority
		}
		if req.Recurring != nil {
			t.Recurring = sql.NullString{String: *req.Recurring, Valid: true}
		}
		if err := s.db.InsertTask(t); err != nil {
			writeInternalError(w, err)
			return
		}
		createdID = t.ID

	case "note":
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
		createdID = n.ID

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type must be goal, task, or note"})
		return
	}

	if err := s.db.MarkCaptureProcessed(captureID, "processed:"+req.Type); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"capture_id": captureID,
		"created_id": createdID,
		"type":       req.Type,
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
