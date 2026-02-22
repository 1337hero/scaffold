package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"scaffold/sessionbus"
)

type sessionBusRegisterRequest struct {
	SessionID string `json:"session_id"`
	Provider  string `json:"provider"`
	Name      string `json:"name"`
}

type sessionBusRegisterResponse struct {
	Session sessionbus.Session `json:"session"`
}

type sessionBusSendRequest struct {
	FromSessionID string `json:"from_session_id"`
	ToSessionID   string `json:"to_session_id"`
	Mode          string `json:"mode"`
	Message       string `json:"message"`
}

type sessionBusSendResponse struct {
	Message sessionbus.Envelope `json:"message"`
}

type sessionBusPollRequest struct {
	SessionID   string `json:"session_id"`
	Limit       int    `json:"limit"`
	WaitSeconds int    `json:"wait_seconds"`
}

type sessionBusPollResponse struct {
	Messages []sessionbus.Envelope `json:"messages"`
}

func (s *Server) handleSessionBusRegister(w http.ResponseWriter, r *http.Request) {
	if s.sessionBus == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session bus is not configured"})
		return
	}

	var req sessionBusRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	session, err := s.sessionBus.Register(r.Context(), sessionbus.RegisterRequest{
		SessionID: req.SessionID,
		Provider:  req.Provider,
		Name:      req.Name,
	})
	if err != nil {
		writeSessionBusError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionBusRegisterResponse{Session: session})
}

func (s *Server) handleSessionBusSessions(w http.ResponseWriter, r *http.Request) {
	if s.sessionBus == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session bus is not configured"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.sessionBus.List(r.Context())})
}

func (s *Server) handleSessionBusSend(w http.ResponseWriter, r *http.Request) {
	if s.sessionBus == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session bus is not configured"})
		return
	}

	var req sessionBusSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	msg, err := s.sessionBus.Send(r.Context(), sessionbus.SendRequest{
		FromSessionID: req.FromSessionID,
		ToSessionID:   req.ToSessionID,
		Mode:          req.Mode,
		Message:       req.Message,
	})
	if err != nil {
		writeSessionBusError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, sessionBusSendResponse{Message: msg})
}

func (s *Server) handleSessionBusPoll(w http.ResponseWriter, r *http.Request) {
	if s.sessionBus == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session bus is not configured"})
		return
	}

	var req sessionBusPollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if req.WaitSeconds < 0 {
		req.WaitSeconds = 0
	}
	if req.WaitSeconds > 120 {
		req.WaitSeconds = 120
	}

	messages, err := s.sessionBus.Poll(r.Context(), strings.TrimSpace(req.SessionID), req.Limit, time.Duration(req.WaitSeconds)*time.Second)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			return
		}
		writeSessionBusError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionBusPollResponse{Messages: messages})
}

func writeSessionBusError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sessionbus.ErrUnknownSession):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
	case errors.Is(err, sessionbus.ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeInternalError(w, err)
	}
}
