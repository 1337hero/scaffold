package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLegacyRoutesRemoved(t *testing.T) {
	srv, _ := newTestServer(t)

	cases := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/inbox"},
		{method: http.MethodGet, path: "/memories"},
	}

	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		srv.mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s: expected 404, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestAPIPrefixRoutesRemainAvailable(t *testing.T) {
	srv, _ := newTestServer(t)

	cases := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/inbox"},
		{method: http.MethodGet, path: "/api/memories"},
	}

	for _, tc := range cases {
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, authedRequest(tc.method, tc.path, ""))

		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: expected 200, got %d", tc.method, tc.path, rec.Code)
		}
	}
}
