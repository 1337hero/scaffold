package db

import (
	"database/sql"
	"errors"
	"time"
)

func (db *DB) CreateSession(tokenHash string, ttl time.Duration) error {
	n := now()
	exp := time.Now().UTC().Add(ttl).Format(time.RFC3339)
	_, err := db.conn.Exec(
		`INSERT INTO sessions (token_hash, created_at, expires_at) VALUES (?, ?, ?)`,
		tokenHash, n, exp,
	)
	return err
}

func (db *DB) ValidateSession(tokenHash string) (bool, error) {
	var dummy string
	err := db.conn.QueryRow(
		`SELECT token_hash FROM sessions WHERE token_hash = ? AND expires_at > ?`,
		tokenHash, now(),
	).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (db *DB) TouchSession(tokenHash string) error {
	_, err := db.conn.Exec(
		`UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?`,
		now(), tokenHash,
	)
	return err
}

func (db *DB) DeleteSession(tokenHash string) error {
	_, err := db.conn.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (db *DB) CleanExpiredSessions() error {
	_, err := db.conn.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now())
	return err
}
