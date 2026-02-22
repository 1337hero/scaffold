package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type calendarEventDTO struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Time    string `json:"time"`
	AllDay  bool   `json:"all_day"`
}

func (s *Server) handleCalendarEvents(w http.ResponseWriter, r *http.Request) {
	if s.brain == nil {
		writeJSON(w, http.StatusOK, []calendarEventDTO{})
		return
	}

	events, err := s.brain.CalendarUpcoming(r.Context(), 3)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if events == nil {
		writeJSON(w, http.StatusOK, []calendarEventDTO{})
		return
	}

	out := make([]calendarEventDTO, 0, len(events))
	for i, e := range events {
		eventID := strings.TrimSpace(e.ID)
		if eventID == "" {
			eventID = fmt.Sprintf("%s|%s|%d", e.Start, e.Title, i)
		}
		dto := calendarEventDTO{
			ID:      eventID,
			Summary: e.Title,
			AllDay:  e.AllDay,
		}
		if e.AllDay {
			dto.Time = "All day"
		} else {
			dto.Time = formatEventTime(e.Start)
		}
		out = append(out, dto)
	}

	writeJSON(w, http.StatusOK, out)
}

func formatEventTime(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return t.Local().Format("3:04 PM")
}
