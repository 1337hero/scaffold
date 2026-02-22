package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"scaffold/db"
)

type domainResponse struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Importance     int    `json:"importance"`
	LastTouchedAt  string `json:"last_touched_at,omitempty"`
	StatusLine     string `json:"status_line,omitempty"`
	Briefing       string `json:"briefing,omitempty"`
	DaysSinceTouch int    `json:"days_since_touch"`
	DriftState     string `json:"drift_state"`
	DriftLabel     string `json:"drift_label"`
	OpenTaskCount  int    `json:"open_task_count"`
}

type domainDetailResponse struct {
	Domain         domainResponse `json:"domain"`
	DeskItems      []db.DeskItem  `json:"desk_items"`
	OpenCaptures   []db.Capture   `json:"open_captures"`
	RecentMemories []db.Memory    `json:"recent_memories"`
	DriftState     string         `json:"drift_state"`
	DriftLabel     string         `json:"drift_label"`
}

type domainPatchRequest struct {
	StatusLine *string `json:"status_line"`
	Briefing   *string `json:"briefing"`
	Importance *int    `json:"importance"`
}

type dumpResponse struct {
	Count    int          `json:"count"`
	Captures []db.Capture `json:"captures"`
	Memories []db.Memory  `json:"memories"`
}

func (s *Server) handleDomains(w http.ResponseWriter, r *http.Request) {
	drifts, err := s.db.ComputeDriftStates()
	if err != nil {
		writeInternalError(w, err)
		return
	}

	out := make([]domainResponse, 0, len(drifts)+1)
	for _, d := range drifts {
		resp := domainResponse{
			ID:             d.ID,
			Name:           d.Name,
			Importance:     d.Importance,
			LastTouchedAt:  d.LastTouchedAt,
			DaysSinceTouch: d.DaysSinceTouch,
			DriftState:     d.State,
			DriftLabel:     d.Label,
			OpenTaskCount:  d.OpenTaskCount,
		}
		if d.StatusLine.Valid {
			resp.StatusLine = d.StatusLine.String
		}
		if d.Briefing.Valid {
			resp.Briefing = d.Briefing.String
		}
		out = append(out, resp)
	}

	count, err := s.db.CountDumpItems()
	if err != nil {
		writeInternalError(w, err)
		return
	}
	out = append(out, domainResponse{
		ID:            0,
		Name:          "The Dump",
		Importance:    1,
		DriftState:    "cold",
		DriftLabel:    strconv.Itoa(count) + " items",
		OpenTaskCount: count,
	})

	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDomainDetail(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSpace(r.PathValue("id"))
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid domain id"})
		return
	}

	detail, err := s.db.DomainDetailByID(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if detail == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
		return
	}

	var driftState, driftLabel string
	drifts, err := s.db.ComputeDriftStates()
	if err == nil {
		for _, d := range drifts {
			if d.ID == id {
				driftState = d.State
				driftLabel = d.Label
				break
			}
		}
	}

	domResp := domainResponse{
		ID:            detail.ID,
		Name:          detail.Name,
		Importance:    detail.Importance,
		LastTouchedAt: detail.LastTouchedAt,
	}
	if detail.StatusLine.Valid {
		domResp.StatusLine = detail.StatusLine.String
	}
	if detail.Briefing.Valid {
		domResp.Briefing = detail.Briefing.String
	}

	resp := domainDetailResponse{
		Domain:         domResp,
		DeskItems:      emptyIfNilDesk(detail.DeskItems),
		OpenCaptures:   emptyIfNilCaptures(detail.OpenCaptures),
		RecentMemories: emptyIfNilMemories(detail.RecentMemories),
		DriftState:     driftState,
		DriftLabel:     driftLabel,
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDomainPatch(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSpace(r.PathValue("id"))
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid domain id"})
		return
	}

	var req domainPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if req.StatusLine == nil && req.Briefing == nil && req.Importance == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no fields to update"})
		return
	}
	if req.Importance != nil && (*req.Importance < 1 || *req.Importance > 5) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "importance must be between 1 and 5"})
		return
	}

	if err := s.db.UpdateDomain(id, req.StatusLine, req.Briefing, req.Importance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
			return
		}
		writeInternalError(w, err)
		return
	}

	if req.StatusLine != nil || req.Briefing != nil {
		if err := s.db.TouchDomain(id); err != nil {
			writeInternalError(w, fmt.Errorf("touch domain %d: %w", id, err))
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDomainsDump(w http.ResponseWriter, r *http.Request) {
	captures, err := s.db.DumpItems(50)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	memories, err := s.db.DumpMemories(50)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	count, err := s.db.CountDumpItems()
	if err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, dumpResponse{
		Count:    count,
		Captures: emptyIfNilCaptures(captures),
		Memories: emptyIfNilMemories(memories),
	})
}

func emptyIfNilDesk(items []db.DeskItem) []db.DeskItem {
	if items == nil {
		return []db.DeskItem{}
	}
	return items
}

func emptyIfNilCaptures(items []db.Capture) []db.Capture {
	if items == nil {
		return []db.Capture{}
	}
	return items
}

func emptyIfNilMemories(items []db.Memory) []db.Memory {
	if items == nil {
		return []db.Memory{}
	}
	return items
}
