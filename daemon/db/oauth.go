package db

import (
	"database/sql"
	"errors"
)

type OAuthToken struct {
	Provider     string
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       string
	CreatedAt    string
	UpdatedAt    string
}

func (db *DB) SaveOAuthToken(provider string, token *OAuthToken) error {
	ts := now()
	_, err := db.conn.Exec(
		`INSERT INTO oauth_tokens (provider, access_token, refresh_token, token_type, expiry, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(provider) DO UPDATE SET
		   access_token  = excluded.access_token,
		   refresh_token = excluded.refresh_token,
		   token_type    = excluded.token_type,
		   expiry        = excluded.expiry,
		   updated_at    = excluded.updated_at`,
		provider, token.AccessToken, token.RefreshToken, token.TokenType, token.Expiry, ts, ts,
	)
	return err
}

func (db *DB) GetOAuthToken(provider string) (*OAuthToken, error) {
	var t OAuthToken
	err := db.conn.QueryRow(
		`SELECT provider, access_token, refresh_token, token_type, expiry, created_at, updated_at
		 FROM oauth_tokens WHERE provider = ?`, provider,
	).Scan(&t.Provider, &t.AccessToken, &t.RefreshToken, &t.TokenType, &t.Expiry, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (db *DB) DeleteOAuthToken(provider string) error {
	_, err := db.conn.Exec(`DELETE FROM oauth_tokens WHERE provider = ?`, provider)
	return err
}
