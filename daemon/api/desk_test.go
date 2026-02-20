package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"scaffold/db"
)

const testAPIToken = "test-token"

func TestHandleDesk(t *testing.T) {
	srv, database := newTestServer(t)
	insertTodayDeskItem(t, database, "desk-1", "Task one")

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/desk", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got []db.DeskItem
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 desk item, got %d", len(got))
	}
	if got[0].ID != "desk-1" {
		t.Fatalf("expected id desk-1, got %s", got[0].ID)
	}
}

func TestHandleDeskPatch(t *testing.T) {
	srv, database := newTestServer(t)
	insertTodayDeskItem(t, database, "desk-2", "Task two")

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPatch, "/api/desk/desk-2", `{"status":"done"}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	items, err := database.TodaysDesk()
	if err != nil {
		t.Fatalf("query desk: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 desk item, got %d", len(items))
	}
	if items[0].Status != "done" {
		t.Fatalf("expected status done, got %s", items[0].Status)
	}
	if !items[0].CompletedAt.Valid {
		t.Fatal("expected completed_at to be set")
	}
}

func TestHandleDeskPatchNotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPatch, "/api/desk/missing-id", `{"status":"done"}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleDeskDefer(t *testing.T) {
	srv, database := newTestServer(t)
	insertTodayDeskItem(t, database, "desk-3", "Task three")

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/desk/desk-3/defer", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	items, err := database.TodaysDesk()
	if err != nil {
		t.Fatalf("query desk: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected deferred task to be removed from today's desk, still got %d items", len(items))
	}
}

func TestHandleDeskRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/desk", nil)
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func newTestServer(t *testing.T) (*Server, *db.DB) {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	return New(database, nil, testAPIToken, AuthConfig{}), database
}

func insertTodayDeskItem(t *testing.T, database *db.DB, id, title string) {
	t.Helper()

	err := database.InsertDeskItem(db.DeskItem{
		ID:       id,
		Title:    title,
		Position: 1,
		Status:   "active",
		Date:     time.Now().Format("2006-01-02"),
	})
	if err != nil {
		t.Fatalf("insert desk item: %v", err)
	}
}

func authedRequest(method, target, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Authorization", "Bearer "+testAPIToken)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}
