package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleDashboardReturnsArraysNotNull(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/dashboard", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	keys := []string{
		"TodaysTasks",
		"OverdueTasks",
		"GoalsWithProgress",
		"DomainHealth",
		"DoneToday",
	}
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			t.Fatalf("expected %s key in response", key)
		}
		if string(value) == "null" {
			t.Fatalf("expected %s to be [] when empty, got null", key)
		}
	}
}

func TestListEndpointsReturnArraysNotNull(t *testing.T) {
	srv, _ := newTestServer(t)

	cases := []string{
		"/api/goals",
		"/api/tasks",
		"/api/notes",
		"/api/search?q=test",
	}

	for _, path := range cases {
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, path, ""))

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, rec.Code)
		}

		var raw json.RawMessage
		if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
			t.Fatalf("%s: decode: %v", path, err)
		}
		if string(raw) == "null" {
			t.Fatalf("%s: expected [] when empty, got null", path)
		}
		if len(raw) == 0 || raw[0] != '[' {
			t.Fatalf("%s: expected JSON array response, got %s", path, string(raw))
		}
	}
}
