package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func http200Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func http500Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
}

func http401Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
}

func TestOpenAIHealthCheckPasses(t *testing.T) {
	srv := httptest.NewServer(http200Handler())
	defer srv.Close()

	checker := &openAIHealthChecker{baseURL: srv.URL, apiKey: "test-key"}
	if err := checker.Check(context.Background()); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestOpenAIHealthCheckFailsOn500(t *testing.T) {
	srv := httptest.NewServer(http500Handler())
	defer srv.Close()

	checker := &openAIHealthChecker{baseURL: srv.URL, apiKey: "test-key"}
	err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected error containing '500', got: %v", err)
	}
}

func TestOpenAIHealthCheckFailsOnNetworkError(t *testing.T) {
	srv := httptest.NewServer(http200Handler())
	srv.Close()

	checker := &openAIHealthChecker{baseURL: srv.URL, apiKey: "test-key"}
	err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for closed server, got nil")
	}
}

func TestAnthropicHealthCheckPasses(t *testing.T) {
	srv := httptest.NewServer(http200Handler())
	defer srv.Close()

	checker := &anthropicHealthChecker{
		apiKey:  "test-key",
		baseURL: srv.URL,
	}
	if err := checker.Check(context.Background()); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestAnthropicHealthCheckFailsOn401(t *testing.T) {
	srv := httptest.NewServer(http401Handler())
	defer srv.Close()

	checker := &anthropicHealthChecker{
		apiKey:  "bad-key",
		baseURL: srv.URL,
	}
	err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected error containing '401', got: %v", err)
	}
}

func TestOpenAIHealthCheckContextCancellation(t *testing.T) {
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(blocked)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	checker := &openAIHealthChecker{baseURL: srv.URL, apiKey: "test-key"}
	err := checker.Check(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}
