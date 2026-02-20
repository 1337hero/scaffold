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
		method       string
		path         string
		body         string
		expectedCode int
	}{
		{method: http.MethodGet, path: "/api/inbox", expectedCode: http.StatusOK},
		{method: http.MethodGet, path: "/api/memories", expectedCode: http.StatusOK},
		{method: http.MethodPost, path: "/api/inbox/missing/confirm", expectedCode: http.StatusNotFound},
		{method: http.MethodPost, path: "/api/inbox/missing/archive", expectedCode: http.StatusNotFound},
		{method: http.MethodPost, path: "/api/inbox/missing/override", body: `{}`, expectedCode: http.StatusBadRequest},
	}

	for _, tc := range cases {
		rec := httptest.NewRecorder()
		srv.mux.ServeHTTP(rec, authedRequest(tc.method, tc.path, tc.body))

		if rec.Code != tc.expectedCode {
			t.Fatalf("%s %s: expected %d, got %d", tc.method, tc.path, tc.expectedCode, rec.Code)
		}
	}
}
