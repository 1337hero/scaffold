package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"scaffold/db"
)

func newAuthTestServer(t *testing.T) (*Server, *db.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	hash, err := bcrypt.GenerateFromPassword([]byte("testpassword"), 4)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	return New(database, nil, testAPIToken, AuthConfig{
		AppUsername:          "testuser",
		AppPasswordHash:      string(hash),
		SessionTTL:           1 * time.Hour,
		CookieSecure:         false,
		LoginRateLimitWindow: 1 * time.Minute,
		LoginRateLimitMax:    5,
	}), database
}

func doLogin(t *testing.T, srv *Server, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)
	return rec
}

func sessionCookieFrom(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			return c.Value
		}
	}
	t.Fatal("no session cookie in response")
	return ""
}

func TestHandleLoginSuccess(t *testing.T) {
	srv, _ := newAuthTestServer(t)

	rec := doLogin(t, srv, "testuser", "testpassword")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var found bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			found = true
			if !c.HttpOnly {
				t.Error("session cookie should be HttpOnly")
			}
		}
	}
	if !found {
		t.Fatal("expected Set-Cookie header with session cookie")
	}
}

func TestHandleLoginBadPassword(t *testing.T) {
	srv, _ := newAuthTestServer(t)

	rec := doLogin(t, srv, "testuser", "wrongpassword")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleLoginRateLimited(t *testing.T) {
	srv, _ := newAuthTestServer(t)

	for i := 0; i < 5; i++ {
		body := strings.NewReader(`{"username":"testuser","password":"wrong"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/login", body)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, rec.Code)
		}
	}

	body := strings.NewReader(`{"username":"testuser","password":"wrong"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/login", body)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after rate limit, got %d", rec.Code)
	}
}

func TestProtectedRouteSessionCookieAuth(t *testing.T) {
	srv, database := newAuthTestServer(t)

	insertTodayDeskItem(t, database, "auth-desk-1", "Session auth task")

	loginRec := doLogin(t, srv, "testuser", "testpassword")
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d", loginRec.Code)
	}
	rawToken := sessionCookieFrom(t, loginRec)

	req := httptest.NewRequest(http.MethodGet, "/api/desk", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: rawToken})
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with session cookie, got %d", rec.Code)
	}
}

func TestProtectedRouteBearerTokenStillWorks(t *testing.T) {
	srv, _ := newAuthTestServer(t)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/desk", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with bearer token, got %d", rec.Code)
	}
}

func TestProtectedRouteNoAuth(t *testing.T) {
	srv, _ := newAuthTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/desk", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no auth, got %d", rec.Code)
	}
}

func TestHandleLogoutClearsSession(t *testing.T) {
	srv, _ := newAuthTestServer(t)

	loginRec := doLogin(t, srv, "testuser", "testpassword")
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d", loginRec.Code)
	}
	rawToken := sessionCookieFrom(t, loginRec)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logoutReq.Host = "localhost"
	logoutReq.Header.Set("Origin", "http://localhost")
	logoutReq.AddCookie(&http.Cookie{Name: "session", Value: rawToken})
	logoutRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout failed: %d", logoutRec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/desk", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: rawToken})
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", rec.Code)
	}
}

func TestAuthCheckAuthed(t *testing.T) {
	srv, _ := newAuthTestServer(t)

	loginRec := doLogin(t, srv, "testuser", "testpassword")
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d", loginRec.Code)
	}
	rawToken := sessionCookieFrom(t, loginRec)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: rawToken})
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp["authenticated"] {
		t.Fatal("expected authenticated: true")
	}
}

func TestAuthCheckNotAuthed(t *testing.T) {
	srv, _ := newAuthTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["authenticated"] {
		t.Fatal("expected authenticated: false")
	}
}

func TestMutatingCookieAuthOriginCheck(t *testing.T) {
	srv, _ := newAuthTestServer(t)

	loginRec := doLogin(t, srv, "testuser", "testpassword")
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d", loginRec.Code)
	}
	rawToken := sessionCookieFrom(t, loginRec)

	evilReq := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"text":"hello"}`))
	evilReq.Header.Set("Content-Type", "application/json")
	evilReq.Header.Set("Origin", "https://evil.example.com")
	evilReq.Host = "localhost"
	evilReq.AddCookie(&http.Cookie{Name: "session", Value: rawToken})
	evilRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(evilRec, evilReq)

	if evilRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for untrusted origin, got %d", evilRec.Code)
	}

	noOriginReq := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"text":"hello"}`))
	noOriginReq.Header.Set("Content-Type", "application/json")
	noOriginReq.Host = "localhost"
	noOriginReq.AddCookie(&http.Cookie{Name: "session", Value: rawToken})
	noOriginRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(noOriginRec, noOriginReq)

	if noOriginRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing origin on cookie auth, got %d", noOriginRec.Code)
	}

	goodReq := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"text":"hello"}`))
	goodReq.Header.Set("Content-Type", "application/json")
	goodReq.Header.Set("Origin", "http://localhost")
	goodReq.Host = "localhost"
	goodReq.AddCookie(&http.Cookie{Name: "session", Value: rawToken})
	goodRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(goodRec, goodReq)

	if goodRec.Code == http.StatusForbidden {
		t.Fatalf("expected non-403 for trusted origin, got 403")
	}

	proxiedReq := httptest.NewRequest(http.MethodPost, "/api/capture", strings.NewReader(`{"text":"hello"}`))
	proxiedReq.Header.Set("Content-Type", "application/json")
	proxiedReq.Header.Set("Origin", "http://localhost:4002")
	proxiedReq.Header.Set("X-Forwarded-Host", "localhost:4002")
	proxiedReq.Header.Set("X-Forwarded-Proto", "http")
	proxiedReq.Host = "127.0.0.1:46873"
	proxiedReq.AddCookie(&http.Cookie{Name: "session", Value: rawToken})
	proxiedRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(proxiedRec, proxiedReq)

	if proxiedRec.Code == http.StatusForbidden {
		t.Fatalf("expected non-403 for trusted proxied origin, got 403")
	}
}
