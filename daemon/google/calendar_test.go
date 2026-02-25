package google

import (
	"testing"

	"google.golang.org/api/calendar/v3"
)

func TestFormatEvents_Empty(t *testing.T) {
	if got := FormatEvents(nil); got != "No events found." {
		t.Errorf("nil input: got %q, want %q", got, "No events found.")
	}
	if got := FormatEvents([]Event{}); got != "No events found." {
		t.Errorf("empty slice: got %q, want %q", got, "No events found.")
	}
}

func TestFormatEvents_TimedEvents(t *testing.T) {
	events := []Event{
		{
			Title: "Standup",
			Start: "2026-02-20T09:00:00-07:00",
			End:   "2026-02-20T09:30:00-07:00",
		},
		{
			Title:    "Lunch",
			Start:    "2026-02-20T12:00:00-07:00",
			End:      "2026-02-20T13:00:00-07:00",
			Location: "Cafe",
		},
	}
	got := FormatEvents(events)

	if want := "2 event(s):\n"; !contains(got, want) {
		t.Errorf("missing count line: got %q", got)
	}
	if !contains(got, "9:00 AM") || !contains(got, "9:30 AM") {
		t.Errorf("missing time range for Standup: got %q", got)
	}
	if !contains(got, "Standup") {
		t.Errorf("missing title Standup: got %q", got)
	}
	if !contains(got, "@ Cafe") {
		t.Errorf("missing location: got %q", got)
	}
}

func TestFormatEvents_AllDayEvent(t *testing.T) {
	events := []Event{
		{
			Title: "PTO",
			Start: "2026-02-20",
			End:   "2026-02-21",
			AllDay: true,
		},
	}
	got := FormatEvents(events)

	if !contains(got, "[All day]") {
		t.Errorf("missing [All day] marker: got %q", got)
	}
	if !contains(got, "PTO") {
		t.Errorf("missing title: got %q", got)
	}
}

func TestFormatEvents_Mixed(t *testing.T) {
	events := []Event{
		{
			Title:  "PTO",
			Start:  "2026-02-20",
			End:    "2026-02-21",
			AllDay: true,
		},
		{
			Title: "Standup",
			Start: "2026-02-20T09:00:00-07:00",
			End:   "2026-02-20T09:30:00-07:00",
		},
		{
			Title:    "Review",
			Start:    "2026-02-20T14:00:00-07:00",
			End:      "2026-02-20T15:00:00-07:00",
			Location: "Room 4",
		},
	}
	got := FormatEvents(events)

	if !contains(got, "3 event(s):") {
		t.Errorf("wrong count: got %q", got)
	}
}

func TestConvertEvents_FiltersCancelled(t *testing.T) {
	items := []*calendar.Event{
		{
			Summary: "Cancelled Meeting",
			Status:  "cancelled",
			Start:   &calendar.EventDateTime{DateTime: "2026-02-20T10:00:00-07:00"},
			End:     &calendar.EventDateTime{DateTime: "2026-02-20T11:00:00-07:00"},
		},
	}
	got := convertEvents(items)
	if len(got) != 0 {
		t.Errorf("expected 0 events, got %d", len(got))
	}
}

func TestConvertEvents_FiltersDeclined(t *testing.T) {
	items := []*calendar.Event{
		{
			Summary: "Declined Meeting",
			Status:  "confirmed",
			Start:   &calendar.EventDateTime{DateTime: "2026-02-20T10:00:00-07:00"},
			End:     &calendar.EventDateTime{DateTime: "2026-02-20T11:00:00-07:00"},
			Attendees: []*calendar.EventAttendee{
				{Self: true, ResponseStatus: "declined"},
			},
		},
	}
	got := convertEvents(items)
	if len(got) != 0 {
		t.Errorf("expected 0 events, got %d", len(got))
	}
}

func TestConvertEvents_KeepsAccepted(t *testing.T) {
	items := []*calendar.Event{
		{
			Summary: "Team Sync",
			Status:  "confirmed",
			Start:   &calendar.EventDateTime{DateTime: "2026-02-20T10:00:00-07:00"},
			End:     &calendar.EventDateTime{DateTime: "2026-02-20T11:00:00-07:00"},
			Attendees: []*calendar.EventAttendee{
				{Self: true, ResponseStatus: "accepted"},
			},
		},
		{
			Summary: "No Attendees",
			Status:  "confirmed",
			Start:   &calendar.EventDateTime{Date: "2026-02-20"},
			End:     &calendar.EventDateTime{Date: "2026-02-21"},
		},
	}
	got := convertEvents(items)
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].Title != "Team Sync" {
		t.Errorf("first event title: got %q, want %q", got[0].Title, "Team Sync")
	}
	if got[0].AllDay {
		t.Error("first event should not be all-day")
	}
	if !got[1].AllDay {
		t.Error("second event should be all-day")
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026-02-20T14:30:00-07:00", "2:30 PM"},
		{"invalid", "invalid"},
	}
	for _, tt := range tests {
		got := formatTime(tt.input)
		if got != tt.want {
			t.Errorf("formatTime(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEventToGCal_TimedEvent(t *testing.T) {
	e := Event{
		Title:       "Standup",
		Start:       "2026-02-20T09:00:00-07:00",
		End:         "2026-02-20T09:30:00-07:00",
		Location:    "Zoom",
		Description: "Daily sync",
		AllDay:      false,
	}
	gcal := eventToGCal(e)

	if gcal.Summary != "Standup" {
		t.Errorf("Summary: got %q, want %q", gcal.Summary, "Standup")
	}
	if gcal.Location != "Zoom" {
		t.Errorf("Location: got %q, want %q", gcal.Location, "Zoom")
	}
	if gcal.Description != "Daily sync" {
		t.Errorf("Description: got %q, want %q", gcal.Description, "Daily sync")
	}
	if gcal.Start == nil || gcal.Start.DateTime != e.Start {
		t.Errorf("Start.DateTime: got %v, want %q", gcal.Start, e.Start)
	}
	if gcal.End == nil || gcal.End.DateTime != e.End {
		t.Errorf("End.DateTime: got %v, want %q", gcal.End, e.End)
	}
	if gcal.Start.Date != "" {
		t.Errorf("Start.Date should be empty for timed event, got %q", gcal.Start.Date)
	}
}

func TestEventToGCal_AllDayEvent(t *testing.T) {
	e := Event{
		Title:  "PTO",
		Start:  "2026-02-20",
		End:    "2026-02-21",
		AllDay: true,
	}
	gcal := eventToGCal(e)

	if gcal.Summary != "PTO" {
		t.Errorf("Summary: got %q, want %q", gcal.Summary, "PTO")
	}
	if gcal.Start == nil || gcal.Start.Date != e.Start {
		t.Errorf("Start.Date: got %v, want %q", gcal.Start, e.Start)
	}
	if gcal.End == nil || gcal.End.Date != e.End {
		t.Errorf("End.Date: got %v, want %q", gcal.End, e.End)
	}
	if gcal.Start.DateTime != "" {
		t.Errorf("Start.DateTime should be empty for all-day event, got %q", gcal.Start.DateTime)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
