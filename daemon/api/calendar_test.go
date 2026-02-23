package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"scaffold/brain"
	"scaffold/db"
	googlecal "scaffold/google"
)

func TestHandleCalendarEventsReturnsEmptyWhenNoBrain(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/calendar/upcoming", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got []calendarEventDTO
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("expected empty array, got %d events", len(got))
	}
}

func TestHandleCalendarEventsDegradesWhenCalendarUnavailable(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	b := brain.NewWithDependencies(database, brain.Config{}, brain.Dependencies{})
	b.SetCalendarClient(&googlecal.CalendarClient{})

	srv := New(database, b, testAPIToken, AuthConfig{})

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/calendar/upcoming", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got []calendarEventDTO
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("expected empty array, got %d events", len(got))
	}
}
