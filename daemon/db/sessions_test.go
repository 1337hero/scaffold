package db

import (
	"testing"
)

func TestSessionHashStorageNotRaw(t *testing.T) {
	database := newTestDB(t)

	const tokenHash = "myhash"
	if err := database.CreateSession(tokenHash, 1*60*1000000000); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	var stored string
	err := database.conn.QueryRow(`SELECT token_hash FROM sessions WHERE token_hash = ?`, tokenHash).Scan(&stored)
	if err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	if stored != tokenHash {
		t.Fatalf("expected stored token_hash %q, got %q", tokenHash, stored)
	}
}

func TestValidateSessionExpired(t *testing.T) {
	database := newTestDB(t)

	_, err := database.conn.Exec(
		`INSERT INTO sessions (token_hash, created_at, expires_at) VALUES (?, ?, ?)`,
		"expiredhash", "2020-01-01T00:00:00Z", "2020-01-01T00:01:00Z",
	)
	if err != nil {
		t.Fatalf("insert expired session: %v", err)
	}

	valid, err := database.ValidateSession("expiredhash")
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if valid {
		t.Fatal("expected expired session to be invalid")
	}
}
