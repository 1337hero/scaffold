package db

import "testing"

func TestOAuthToken_SaveAndGet(t *testing.T) {
	database := newTestDB(t)

	token := &OAuthToken{
		AccessToken:  "access123",
		RefreshToken: "refresh456",
		TokenType:    "Bearer",
		Expiry:       "2026-12-31T23:59:59Z",
	}

	if err := database.SaveOAuthToken("google", token); err != nil {
		t.Fatalf("SaveOAuthToken: %v", err)
	}

	got, err := database.GetOAuthToken("google")
	if err != nil {
		t.Fatalf("GetOAuthToken: %v", err)
	}
	if got == nil {
		t.Fatal("expected token, got nil")
	}
	if got.Provider != "google" {
		t.Errorf("provider = %q, want %q", got.Provider, "google")
	}
	if got.AccessToken != "access123" {
		t.Errorf("access_token = %q, want %q", got.AccessToken, "access123")
	}
	if got.RefreshToken != "refresh456" {
		t.Errorf("refresh_token = %q, want %q", got.RefreshToken, "refresh456")
	}
	if got.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want %q", got.TokenType, "Bearer")
	}
	if got.Expiry != "2026-12-31T23:59:59Z" {
		t.Errorf("expiry = %q, want %q", got.Expiry, "2026-12-31T23:59:59Z")
	}
	if got.CreatedAt == "" {
		t.Error("created_at should be set")
	}
	if got.UpdatedAt == "" {
		t.Error("updated_at should be set")
	}
}

func TestOAuthToken_Update(t *testing.T) {
	database := newTestDB(t)

	token := &OAuthToken{
		AccessToken:  "old_access",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		Expiry:       "2026-12-31T23:59:59Z",
	}
	if err := database.SaveOAuthToken("github", token); err != nil {
		t.Fatalf("SaveOAuthToken (first): %v", err)
	}

	first, _ := database.GetOAuthToken("github")

	token.AccessToken = "new_access"
	if err := database.SaveOAuthToken("github", token); err != nil {
		t.Fatalf("SaveOAuthToken (second): %v", err)
	}

	got, err := database.GetOAuthToken("github")
	if err != nil {
		t.Fatalf("GetOAuthToken: %v", err)
	}
	if got.AccessToken != "new_access" {
		t.Errorf("access_token = %q, want %q", got.AccessToken, "new_access")
	}
	if got.CreatedAt != first.CreatedAt {
		t.Errorf("created_at changed: %q -> %q", first.CreatedAt, got.CreatedAt)
	}
}

func TestOAuthToken_Delete(t *testing.T) {
	database := newTestDB(t)

	token := &OAuthToken{
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		Expiry:       "2026-12-31T23:59:59Z",
	}
	if err := database.SaveOAuthToken("spotify", token); err != nil {
		t.Fatalf("SaveOAuthToken: %v", err)
	}

	if err := database.DeleteOAuthToken("spotify"); err != nil {
		t.Fatalf("DeleteOAuthToken: %v", err)
	}

	got, err := database.GetOAuthToken("spotify")
	if err != nil {
		t.Fatalf("GetOAuthToken after delete: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestOAuthToken_NotFound(t *testing.T) {
	database := newTestDB(t)

	got, err := database.GetOAuthToken("nonexistent")
	if err != nil {
		t.Fatalf("GetOAuthToken: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent provider")
	}
}
