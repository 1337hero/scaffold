package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnableFrontendServingRequiresIndex(t *testing.T) {
	srv, _ := newTestServer(t)
	distDir := t.TempDir()

	if err := srv.EnableFrontendServing(distDir); err == nil {
		t.Fatal("expected error when index.html is missing")
	}
}

func TestFrontendServingStaticAndSPAFallback(t *testing.T) {
	srv, _ := newTestServer(t)

	distDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte("<html>shell</html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(distDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "assets", "app.js"), []byte("console.log('ok');"), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	if err := srv.EnableFrontendServing(distDir); err != nil {
		t.Fatalf("enable frontend serving: %v", err)
	}

	handler := srv.httpHandler()

	rootRec := httptest.NewRecorder()
	handler.ServeHTTP(rootRec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rootRec.Code != http.StatusOK {
		t.Fatalf("GET / expected 200, got %d", rootRec.Code)
	}
	if !strings.Contains(rootRec.Body.String(), "shell") {
		t.Fatalf("GET / expected index body, got %q", rootRec.Body.String())
	}

	assetRec := httptest.NewRecorder()
	handler.ServeHTTP(assetRec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if assetRec.Code != http.StatusOK {
		t.Fatalf("GET /assets/app.js expected 200, got %d", assetRec.Code)
	}
	if !strings.Contains(assetRec.Body.String(), "console.log('ok');") {
		t.Fatalf("GET /assets/app.js expected asset body, got %q", assetRec.Body.String())
	}

	spaRec := httptest.NewRecorder()
	handler.ServeHTTP(spaRec, httptest.NewRequest(http.MethodGet, "/inbox", nil))
	if spaRec.Code != http.StatusOK {
		t.Fatalf("GET /inbox expected 200 via SPA fallback, got %d", spaRec.Code)
	}
	if !strings.Contains(spaRec.Body.String(), "shell") {
		t.Fatalf("GET /inbox expected index body, got %q", spaRec.Body.String())
	}

	missingAssetRec := httptest.NewRecorder()
	handler.ServeHTTP(missingAssetRec, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))
	if missingAssetRec.Code != http.StatusNotFound {
		t.Fatalf("GET /assets/missing.js expected 404, got %d", missingAssetRec.Code)
	}

	apiRec := httptest.NewRecorder()
	handler.ServeHTTP(apiRec, authedRequest(http.MethodGet, "/api/inbox", ""))
	if apiRec.Code != http.StatusOK {
		t.Fatalf("GET /api/inbox expected 200, got %d", apiRec.Code)
	}
}
