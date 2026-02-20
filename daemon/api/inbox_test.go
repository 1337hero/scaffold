package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"scaffold/db"
)

func TestHandleInboxConfirm(t *testing.T) {
	srv, database := newTestServer(t)

	captureID, err := database.InsertCapture("capture text", "web")
	if err != nil {
		t.Fatalf("insert capture: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/inbox/"+captureID+"/confirm", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	capture, err := database.GetCapture(captureID)
	if err != nil {
		t.Fatalf("get capture: %v", err)
	}
	if capture == nil {
		t.Fatal("expected capture to exist")
	}
	if capture.Confirmed != 1 {
		t.Fatalf("expected confirmed=1, got %d", capture.Confirmed)
	}
}

func TestHandleInboxConfirmNotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/inbox/missing/confirm", ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleInboxArchive(t *testing.T) {
	srv, database := newTestServer(t)
	captureID, memoryID := insertLinkedCapture(t, database)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/inbox/"+captureID+"/archive", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	memory, err := database.GetMemory(memoryID)
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if memory == nil {
		t.Fatal("expected memory to exist")
	}
	if !memory.SuppressedAt.Valid {
		t.Fatal("expected memory to be suppressed")
	}
}

func TestHandleInboxArchiveWithoutLinkedMemory(t *testing.T) {
	srv, database := newTestServer(t)

	captureID, err := database.InsertCapture("capture text", "web")
	if err != nil {
		t.Fatalf("insert capture: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/inbox/"+captureID+"/archive", ""))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestHandleInboxOverride(t *testing.T) {
	srv, database := newTestServer(t)
	captureID, memoryID := insertLinkedCapture(t, database)

	body := `{"type":"Todo","action":"do","importance":0.91,"tags":["urgent","focus"]}`
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/inbox/"+captureID+"/override", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	memory, err := database.GetMemory(memoryID)
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if memory == nil {
		t.Fatal("expected memory to exist")
	}
	if memory.Type != "Todo" {
		t.Fatalf("expected type Todo, got %s", memory.Type)
	}
	if memory.Importance != 0.91 {
		t.Fatalf("expected importance 0.91, got %.2f", memory.Importance)
	}
	if memory.Tags != "urgent,focus" {
		t.Fatalf("expected tags urgent,focus, got %q", memory.Tags)
	}

	capture, err := database.GetCapture(captureID)
	if err != nil {
		t.Fatalf("get capture: %v", err)
	}
	if capture == nil {
		t.Fatal("expected capture to exist")
	}
	if capture.Source != "user:override" {
		t.Fatalf("expected source user:override, got %q", capture.Source)
	}
	if !capture.TriageAction.Valid || capture.TriageAction.String != "do" {
		t.Fatalf("expected triage_action do, got %#v", capture.TriageAction)
	}
}

func TestHandleInboxOverrideValidation(t *testing.T) {
	srv, database := newTestServer(t)
	captureID, _ := insertLinkedCapture(t, database)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/inbox/"+captureID+"/override", `{"type":"Invalid","action":"do","importance":0.8}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid type, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/inbox/"+captureID+"/override", `{"type":"Todo","action":"do"}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing importance, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/inbox/"+captureID+"/override", `{"type":"Todo","action":"do","importance":1.8}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-range importance, got %d", rec.Code)
	}
}

func TestHandleInboxOverrideWithoutLinkedMemory(t *testing.T) {
	srv, database := newTestServer(t)

	captureID, err := database.InsertCapture("capture text", "web")
	if err != nil {
		t.Fatalf("insert capture: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/inbox/"+captureID+"/override", `{"type":"Todo","action":"do","importance":0.8}`))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestHandleInboxListReturnsCaptures(t *testing.T) {
	srv, database := newTestServer(t)

	if _, err := database.InsertCapture("first", "web"); err != nil {
		t.Fatalf("insert first capture: %v", err)
	}
	if _, err := database.InsertCapture("second", "web"); err != nil {
		t.Fatalf("insert second capture: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/inbox", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var captures []db.Capture
	if err := json.NewDecoder(rec.Body).Decode(&captures); err != nil {
		t.Fatalf("decode captures: %v", err)
	}
	if len(captures) != 2 {
		t.Fatalf("expected 2 captures, got %d", len(captures))
	}
}

func insertLinkedCapture(t *testing.T, database *db.DB) (captureID string, memoryID string) {
	t.Helper()

	memoryID = fmt.Sprintf("mem-%d", time.Now().UnixNano())
	err := database.InsertMemory(db.Memory{
		ID:         memoryID,
		Type:       "Fact",
		Title:      "Original title",
		Content:    "Original content",
		Importance: 0.6,
		Source:     "test",
		Tags:       "old",
	})
	if err != nil {
		t.Fatalf("insert memory: %v", err)
	}

	captureID, err = database.InsertCapture("capture text", "web")
	if err != nil {
		t.Fatalf("insert capture: %v", err)
	}

	if err := database.UpdateTriage(captureID, "reference", memoryID); err != nil {
		t.Fatalf("update triage: %v", err)
	}

	capture, err := database.GetCapture(captureID)
	if err != nil {
		t.Fatalf("get capture: %v", err)
	}
	if capture == nil {
		t.Fatal("expected capture")
	}
	if capture.MemoryID == (sql.NullString{}) {
		t.Fatal("expected capture to have linked memory")
	}

	return captureID, memoryID
}
