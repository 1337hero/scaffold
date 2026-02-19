package api

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"scaffold/db"
)

type Server struct {
	db       *db.DB
	mux      *http.ServeMux
	apiToken string
}

func New(database *db.DB, apiToken string) *Server {
	s := &Server{db: database, mux: http.NewServeMux(), apiToken: apiToken}
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/inbox", s.protected(s.handleInbox))
	s.mux.HandleFunc("GET /api/memories", s.protected(s.handleMemories))
	s.mux.HandleFunc("GET /api/desk", s.protected(s.handleDesk))
	s.mux.HandleFunc("PATCH /api/desk/{id}", s.protected(s.handleDeskPatch))
	s.mux.HandleFunc("POST /api/desk/{id}/defer", s.protected(s.handleDeskDefer))
	return s
}

func (s *Server) NewHTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func (s *Server) ListenAndServe(addr string) error {
	log.Printf("API server listening on %s", addr)
	server := s.NewHTTPServer(addr)
	return server.ListenAndServe()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) protected(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *Server) authorized(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok {
		return false
	}
	token = strings.TrimSpace(token)
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.apiToken)) == 1
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeInternalError(w http.ResponseWriter, err error) {
	log.Printf("internal API error: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
