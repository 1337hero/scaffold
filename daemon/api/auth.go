package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// rateLimiter tracks failed login attempts per IP.
type rateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
	window  time.Duration
	max     int
}

func newRateLimiter(window time.Duration, max int) *rateLimiter {
	return &rateLimiter{
		windows: make(map[string][]time.Time),
		window:  window,
		max:     max,
	}
}

// allow returns true if the IP is under the rate limit.
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	attempts := rl.windows[ip]
	valid := make([]time.Time, 0, len(attempts))
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.windows[ip] = valid
	return len(valid) < rl.max
}

// record adds a failed attempt for the IP.
func (rl *rateLimiter) record(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.windows[ip] = append(rl.windows[ip], time.Now())
}

// allowAndRecord returns true and records the attempt if under the limit.
// It is atomic — check and record happen under a single lock.
func (rl *rateLimiter) allowAndRecord(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	attempts := rl.windows[key]
	valid := make([]time.Time, 0, len(attempts))
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= rl.max {
		rl.windows[key] = valid
		return false
	}
	rl.windows[key] = append(valid, time.Now())
	return true
}

// hashToken returns the hex-encoded SHA-256 of the raw token.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// generateToken creates a cryptographically random 32-byte hex token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// clientIP extracts the remote IP from the request (no port).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.loginLimiter.allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, try again later"})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	// Constant-time username compare
	usernameMatch := subtle.ConstantTimeCompare([]byte(req.Username), []byte(s.appUsername)) == 1
	// bcrypt compare (always call to avoid timing oracle even on username mismatch)
	pwErr := bcrypt.CompareHashAndPassword([]byte(s.appPasswordHash), []byte(req.Password))

	if !usernameMatch || pwErr != nil {
		s.loginLimiter.record(ip)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	rawToken, err := generateToken()
	if err != nil {
		writeInternalError(w, err)
		return
	}
	tokenHash := hashToken(rawToken)

	if err := s.db.CreateSession(tokenHash, s.sessionTTL); err != nil {
		writeInternalError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    rawToken,
		Path:     "/",
		MaxAge:   int(s.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Domain:   s.cookieDomain,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		_ = s.db.DeleteSession(hashToken(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Domain:   s.cookieDomain,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]bool{"authenticated": false})
		return
	}
	valid, err := s.db.ValidateSession(hashToken(cookie.Value))
	if err != nil || !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]bool{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}
