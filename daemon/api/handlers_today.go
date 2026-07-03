package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"scaffold/db"
)

type slippingResponse struct {
	Projects []db.Project      `json:"projects"`
	Tasks    []db.SlippingTask `json:"tasks"`
	People   []db.Person       `json:"people"`
	Areas    []db.Project      `json:"areas"`
}

type todayResponse struct {
	Top3          []db.Task              `json:"top3"`
	Calendar      []calendarEventDTO     `json:"calendar"`
	Slipping      slippingResponse       `json:"slipping"`
	Notifications []db.TodayNotification `json:"notifications"`
}

func surfaceParam(r *http.Request) *string {
	if raw := strings.TrimSpace(r.URL.Query().Get("surface")); raw != "" {
		return &raw
	}
	return nil
}

func toSlippingResponse(s *db.Slipping) slippingResponse {
	return slippingResponse{Projects: s.Projects, Tasks: s.Tasks, People: s.People, Areas: s.Areas}
}

// todaysCalendar returns today's events, degrading to empty when the calendar
// is unreachable — the Today page must never 500 over a missing token.
func (s *Server) todaysCalendar(r *http.Request) []calendarEventDTO {
	out := make([]calendarEventDTO, 0)
	if s.brain == nil {
		return out
	}
	events, err := s.brain.CalendarToday(r.Context())
	if err != nil {
		log.Printf("warn: today calendar unavailable: %v", err)
		return out
	}

	for i, e := range events {
		eventID := strings.TrimSpace(e.ID)
		if eventID == "" {
			eventID = fmt.Sprintf("%s|%s|%d", e.Start, e.Title, i)
		}
		dto := calendarEventDTO{ID: eventID, Summary: e.Title, AllDay: e.AllDay}
		if e.AllDay {
			dto.Time = "All day"
		} else {
			dto.Time = formatEventTime(e.Start)
		}
		out = append(out, dto)
	}
	return out
}

func (s *Server) handleToday(w http.ResponseWriter, r *http.Request) {
	surface := surfaceParam(r)

	top3, err := s.db.GetTop3Tasks(surface)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	slipping, err := s.db.SlippingAll(surface)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	notifications, err := s.db.TodayNotifications(surface)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, todayResponse{
		Top3:          top3,
		Calendar:      s.todaysCalendar(r),
		Slipping:      toSlippingResponse(slipping),
		Notifications: notifications,
	})
}

func (s *Server) handleTop3Set(w http.ResponseWriter, r *http.Request) {
	var taskIDs []string
	if err := json.NewDecoder(r.Body).Decode(&taskIDs); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be a JSON array of task ids"})
		return
	}
	if len(taskIDs) > 3 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "top3 accepts at most 3 task ids"})
		return
	}

	if err := s.db.SetTop3Tasks(taskIDs); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeInternalError(w, err)
		return
	}

	top3, err := s.db.GetTop3Tasks(nil)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, top3)
}

func (s *Server) handleSlipping(w http.ResponseWriter, r *http.Request) {
	slipping, err := s.db.SlippingAll(surfaceParam(r))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toSlippingResponse(slipping))
}
