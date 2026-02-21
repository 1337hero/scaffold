package google

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"scaffold/config"
	"scaffold/db"
)

type TokenStore interface {
	Save(token *oauth2.Token) error
	Get() (*oauth2.Token, error)
}

type DBTokenStore struct {
	DB       *db.DB
	Provider string
}

func (s *DBTokenStore) Save(token *oauth2.Token) error {
	t := &db.OAuthToken{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry.Format(time.RFC3339),
	}
	return s.DB.SaveOAuthToken(s.Provider, t)
}

func (s *DBTokenStore) Get() (*oauth2.Token, error) {
	t, err := s.DB.GetOAuthToken(s.Provider)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, nil
	}
	expiry, err := time.Parse(time.RFC3339, t.Expiry)
	if err != nil {
		return nil, fmt.Errorf("parse token expiry: %w", err)
	}
	return &oauth2.Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		TokenType:    t.TokenType,
		Expiry:       expiry,
	}, nil
}

func NewOAuth2Config(cfg config.GoogleConfig) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8085/callback",
		Scopes:       cfg.Scopes,
	}
}

func RunConsentFlow(cfg *oauth2.Config, store TokenStore) error {
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return fmt.Errorf("generate state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)

	ln, err := net.Listen("tcp", ":8085")
	if err != nil {
		return runManualFlow(cfg, authURL, store)
	}

	return runServerFlow(cfg, ln, authURL, state, store)
}

func runManualFlow(cfg *oauth2.Config, authURL string, store TokenStore) error {
	fmt.Println("Visit this URL to authorize:")
	fmt.Println(authURL)
	fmt.Println()
	fmt.Print("Paste the authorization code: ")

	var code string
	if _, err := fmt.Scanln(&code); err != nil {
		return fmt.Errorf("read code: %w", err)
	}

	token, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	return store.Save(token)
}

func runServerFlow(cfg *oauth2.Config, ln net.Listener, authURL, state string, store TokenStore) error {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			http.Error(w, "authorization failed: "+errMsg, http.StatusBadRequest)
			errCh <- fmt.Errorf("authorization denied: %s", errMsg)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "<html><body><h1>Authorization successful!</h1><p>You can close this tab.</p></body></html>")
		codeCh <- code
	})

	server := &http.Server{Handler: mux}

	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	fmt.Println("Opening browser for Google authorization...")
	fmt.Printf("If browser doesn't open, visit: %s\n", authURL)

	_ = exec.Command("xdg-open", authURL).Start()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		_ = server.Close()
		return err
	}

	token, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		_ = server.Close()
		return fmt.Errorf("exchange code: %w", err)
	}

	if err := store.Save(token); err != nil {
		_ = server.Close()
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)

	fmt.Println("Authorization successful! Token saved.")
	return nil
}

type persistingTokenSource struct {
	src   oauth2.TokenSource
	store TokenStore
	mu    sync.Mutex
	last  *oauth2.Token
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	tok, err := p.src.Token()
	if err != nil {
		return nil, err
	}

	if p.last == nil || p.last.AccessToken != tok.AccessToken {
		toSave := &oauth2.Token{
			AccessToken:  tok.AccessToken,
			TokenType:    tok.TokenType,
			RefreshToken: tok.RefreshToken,
			Expiry:       tok.Expiry,
		}
		if toSave.RefreshToken == "" {
			existing, err := p.store.Get()
			if err == nil && existing != nil {
				toSave.RefreshToken = existing.RefreshToken
			}
		}
		if err := p.store.Save(toSave); err != nil {
			log.Printf("warn: failed to persist refreshed token: %v", err)
		}
		p.last = tok
	}

	return tok, nil
}

func TokenSource(cfg *oauth2.Config, store TokenStore) oauth2.TokenSource {
	existing, _ := store.Get()
	underlying := cfg.TokenSource(context.Background(), existing)
	reuse := oauth2.ReuseTokenSource(existing, underlying)
	return &persistingTokenSource{
		src:   reuse,
		store: store,
	}
}
