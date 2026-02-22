package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scaffold/sessionbus"
)

func TestSessionBusUnavailable(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/session-bus/sessions", ""))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestSessionBusRegisterSendPollFlow(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetSessionBus(sessionbus.New(sessionbus.Config{SessionTTL: time.Hour}))

	register := func(sessionID, provider, name string) {
		body := `{"session_id":"` + sessionID + `","provider":"` + provider + `","name":"` + name + `"}`
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/session-bus/register", body))
		if rec.Code != http.StatusOK {
			t.Fatalf("register %s: expected 200 got %d", sessionID, rec.Code)
		}
	}

	register("codex-main", "codex", "Codex")
	register("gemini-worker", "gemini", "Gemini")

	sessionsRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(sessionsRec, authedRequest(http.MethodGet, "/api/session-bus/sessions", ""))
	if sessionsRec.Code != http.StatusOK {
		t.Fatalf("sessions: expected 200 got %d", sessionsRec.Code)
	}

	var listed struct {
		Sessions []sessionbus.Session `json:"sessions"`
	}
	if err := json.NewDecoder(sessionsRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	if len(listed.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(listed.Sessions))
	}

	sendBody := `{"from_session_id":"codex-main","to_session_id":"gemini-worker","mode":"steer","message":"hello from codex"}`
	sendRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(sendRec, authedRequest(http.MethodPost, "/api/session-bus/send", sendBody))
	if sendRec.Code != http.StatusAccepted {
		t.Fatalf("send: expected 202 got %d", sendRec.Code)
	}

	pollBody := `{"session_id":"gemini-worker","limit":10,"wait_seconds":0}`
	pollRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(pollRec, authedRequest(http.MethodPost, "/api/session-bus/poll", pollBody))
	if pollRec.Code != http.StatusOK {
		t.Fatalf("poll: expected 200 got %d", pollRec.Code)
	}

	var polled struct {
		Messages []sessionbus.Envelope `json:"messages"`
	}
	if err := json.NewDecoder(pollRec.Body).Decode(&polled); err != nil {
		t.Fatalf("decode poll: %v", err)
	}
	if len(polled.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(polled.Messages))
	}
	if polled.Messages[0].Message != "hello from codex" {
		t.Fatalf("unexpected message: %q", polled.Messages[0].Message)
	}
}

func TestSessionBusSendUnknownSession(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetSessionBus(sessionbus.New(sessionbus.Config{SessionTTL: time.Hour}))

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/session-bus/register", `{"session_id":"sender","provider":"codex"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("register sender: expected 200 got %d", rec.Code)
	}

	sendBody := strings.NewReader(`{"from_session_id":"sender","to_session_id":"missing","message":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/session-bus/send", sendBody)
	req.Header.Set("Authorization", "Bearer "+testAPIToken)
	req.Header.Set("Content-Type", "application/json")
	sendRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(sendRec, req)
	if sendRec.Code != http.StatusNotFound {
		t.Fatalf("send unknown: expected 404 got %d", sendRec.Code)
	}
}
