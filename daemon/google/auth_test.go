package google

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"scaffold/db"
)

func TestDBTokenStore_RoundTrip(t *testing.T) {
	database := openTestDB(t)
	store := &DBTokenStore{DB: database, Provider: "google"}

	expiry := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	original := &oauth2.Token{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-xyz",
		TokenType:    "Bearer",
		Expiry:       expiry,
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected token, got nil")
	}
	if got.AccessToken != original.AccessToken {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, original.AccessToken)
	}
	if got.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, original.RefreshToken)
	}
	if got.TokenType != original.TokenType {
		t.Errorf("TokenType = %q, want %q", got.TokenType, original.TokenType)
	}
	if !got.Expiry.Equal(expiry) {
		t.Errorf("Expiry = %v, want %v", got.Expiry, expiry)
	}
}

func TestDBTokenStore_NotFound(t *testing.T) {
	database := openTestDB(t)
	store := &DBTokenStore{DB: database, Provider: "nonexistent"}

	got, err := store.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestPersistingTokenSource_PreservesRefreshToken(t *testing.T) {
	store := &memTokenStore{}
	store.Save(&oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "original-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	})

	newTok := &oauth2.Token{
		AccessToken:  "new-access",
		RefreshToken: "",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	pts := &persistingTokenSource{
		src:   &staticTokenSource{tok: newTok},
		store: store,
	}

	got, err := pts.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "new-access")
	}

	saved, _ := store.Get()
	if saved.RefreshToken != "original-refresh" {
		t.Errorf("saved RefreshToken = %q, want %q", saved.RefreshToken, "original-refresh")
	}
}

func TestPersistingTokenSource_UpdatesRefreshToken(t *testing.T) {
	store := &memTokenStore{}
	store.Save(&oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	})

	newTok := &oauth2.Token{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	pts := &persistingTokenSource{
		src:   &staticTokenSource{tok: newTok},
		store: store,
	}

	got, err := pts.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if got.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "new-access")
	}

	saved, _ := store.Get()
	if saved.RefreshToken != "new-refresh" {
		t.Errorf("saved RefreshToken = %q, want %q", saved.RefreshToken, "new-refresh")
	}
}

type memTokenStore struct {
	mu  sync.Mutex
	tok *oauth2.Token
}

func (m *memTokenStore) Save(token *oauth2.Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *token
	m.tok = &cp
	return nil
}

func (m *memTokenStore) Get() (*oauth2.Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tok == nil {
		return nil, nil
	}
	cp := *m.tok
	return &cp, nil
}

type staticTokenSource struct {
	tok *oauth2.Token
}

func (s *staticTokenSource) Token() (*oauth2.Token, error) {
	return s.tok, nil
}

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
