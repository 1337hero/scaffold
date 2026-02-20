package api

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"scaffold/brain"
	"scaffold/db"
)

// AuthConfig holds all auth-related configuration for the API server.
type AuthConfig struct {
	AppUsername          string
	AppPasswordHash      string
	SessionTTL           time.Duration
	CookieSecure         bool
	CookieDomain         string
	LoginRateLimitWindow time.Duration
	LoginRateLimitMax    int
}

type Server struct {
	db              *db.DB
	brain           *brain.Brain
	mux             *http.ServeMux
	apiToken        string
	appUsername     string
	appPasswordHash string
	sessionTTL      time.Duration
	cookieSecure    bool
	cookieDomain    string
	loginLimiter    *rateLimiter
}

func New(database *db.DB, b *brain.Brain, apiToken string, authCfg AuthConfig) *Server {
	if authCfg.SessionTTL == 0 {
		authCfg.SessionTTL = 7 * 24 * time.Hour
	}
	if authCfg.LoginRateLimitWindow == 0 {
		authCfg.LoginRateLimitWindow = 5 * time.Minute
	}
	if authCfg.LoginRateLimitMax == 0 {
		authCfg.LoginRateLimitMax = 5
	}

	s := &Server{
		db:              database,
		brain:           b,
		mux:             http.NewServeMux(),
		apiToken:        apiToken,
		appUsername:     authCfg.AppUsername,
		appPasswordHash: authCfg.AppPasswordHash,
		sessionTTL:      authCfg.SessionTTL,
		cookieSecure:    authCfg.CookieSecure,
		cookieDomain:    authCfg.CookieDomain,
		loginLimiter:    newRateLimiter(authCfg.LoginRateLimitWindow, authCfg.LoginRateLimitMax),
	}

	// Unauthenticated routes
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/login", s.handleLogin)
	s.mux.HandleFunc("GET /api/auth/check", s.handleAuthCheck)

	// Authenticated routes
	s.mux.HandleFunc("POST /api/logout", s.protected(s.handleLogout))
	s.mux.HandleFunc("GET /api/inbox", s.protected(s.handleInbox))
	s.mux.HandleFunc("POST /api/inbox/{id}/confirm", s.protected(s.handleInboxConfirm))
	s.mux.HandleFunc("POST /api/inbox/{id}/override", s.protected(s.handleInboxOverride))
	s.mux.HandleFunc("POST /api/inbox/{id}/archive", s.protected(s.handleInboxArchive))
	s.mux.HandleFunc("GET /api/memories", s.protected(s.handleMemories))
	s.mux.HandleFunc("GET /api/desk", s.protected(s.handleDesk))
	s.mux.HandleFunc("PATCH /api/desk/{id}", s.protected(s.handleDeskPatch))
	s.mux.HandleFunc("POST /api/desk/{id}/defer", s.protected(s.handleDeskDefer))
	s.mux.HandleFunc("POST /api/capture", s.protected(s.handleCapture))
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
		authedViaCookie, err := s.authorizedByCookie(r)
		if err != nil {
			writeInternalError(w, err)
			return
		}

		if authedViaCookie {
			// For cookie-auth, mutating methods require Origin to match Host
			if isMutating(r.Method) && !s.originTrusted(r) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
			next(w, r)
			return
		}

		if s.authorizedByBearer(r) {
			next(w, r)
			return
		}

		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
}

// authorizedByCookie validates the session cookie and touches it on success.
func (s *Server) authorizedByCookie(r *http.Request) (bool, error) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return false, nil
	}
	tokenHash := hashToken(cookie.Value)
	valid, err := s.db.ValidateSession(tokenHash)
	if err != nil {
		return false, err
	}
	if valid {
		_ = s.db.TouchSession(tokenHash)
	}
	return valid, nil
}

// authorizedByBearer validates the Authorization: Bearer <token> header.
func (s *Server) authorizedByBearer(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok {
		return false
	}
	token = strings.TrimSpace(token)
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.apiToken)) == 1
}

// originTrusted checks that the request Origin (or Referer) host matches the request Host.
func (s *Server) originTrusted(r *http.Request) bool {
	raw := strings.TrimSpace(r.Header.Get("Origin"))
	if raw == "" {
		raw = strings.TrimSpace(r.Header.Get("Referer"))
	}
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	originScheme := strings.ToLower(u.Scheme)
	originHost := canonicalHostPort(u.Hostname(), u.Port(), originScheme)
	if originScheme == "" || originHost == "" {
		return false
	}

	reqScheme := requestScheme(r)
	reqHost, reqPort := splitHostPort(r.Host)
	expectedHost := canonicalHostPort(reqHost, reqPort, reqScheme)
	if reqScheme == "" || expectedHost == "" {
		return false
	}

	return originScheme == reqScheme && originHost == expectedHost
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

func requestScheme(r *http.Request) string {
	if xfProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); xfProto != "" {
		return strings.ToLower(strings.TrimSpace(strings.Split(xfProto, ",")[0]))
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func splitHostPort(host string) (string, string) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", ""
	}

	if parsed, err := url.Parse("http://" + host); err == nil {
		return strings.ToLower(parsed.Hostname()), parsed.Port()
	}

	if h, p, err := net.SplitHostPort(host); err == nil {
		return strings.ToLower(h), p
	}
	return strings.ToLower(host), ""
}

func canonicalHostPort(host, port, scheme string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}

	if port == "" {
		port = defaultPortForScheme(scheme)
	}
	if port == "" {
		return host
	}
	return strings.ToLower(net.JoinHostPort(host, port))
}

func defaultPortForScheme(scheme string) string {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
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
