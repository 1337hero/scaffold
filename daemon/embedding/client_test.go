package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockOllama(t *testing.T, dims int, count int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/embed":
			var req embedRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			n := count
			if n == 0 {
				switch v := req.Input.(type) {
				case string:
					n = 1
				case []interface{}:
					n = len(v)
				default:
					n = 1
				}
			}

			embeddings := make([][]float32, n)
			for i := range embeddings {
				vec := make([]float32, dims)
				for j := range vec {
					vec[j] = float32(i)*0.1 + float32(j)*0.01
				}
				embeddings[i] = vec
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(embedResponse{Embeddings: embeddings})

		case "/api/tags":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"models":[]}`))

		default:
			http.NotFound(w, r)
		}
	}))
}

func TestEmbed(t *testing.T) {
	srv := mockOllama(t, 384, 0)
	defer srv.Close()

	client := NewOllamaClient(srv.URL, "all-minilm", 384)
	vec, err := client.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	if len(vec) != 384 {
		t.Fatalf("expected 384 dimensions, got %d", len(vec))
	}
	if vec[0] != 0.0 {
		t.Errorf("expected vec[0]=0.0, got %f", vec[0])
	}
}

func TestEmbedServerDown(t *testing.T) {
	client := NewOllamaClient("http://127.0.0.1:1", "all-minilm", 384)
	_, err := client.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error when server is down")
	}
}

func TestEmbedBatch(t *testing.T) {
	srv := mockOllama(t, 384, 0)
	defer srv.Close()

	client := NewOllamaClient(srv.URL, "all-minilm", 384)
	texts := []string{"one", "two", "three"}
	vecs, err := client.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch failed: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("expected 3 embeddings, got %d", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 384 {
			t.Errorf("embedding[%d]: expected 384 dims, got %d", i, len(v))
		}
	}
}

func TestAvailableTrue(t *testing.T) {
	srv := mockOllama(t, 384, 0)
	defer srv.Close()

	client := NewOllamaClient(srv.URL, "all-minilm", 384)
	if !client.Available(context.Background()) {
		t.Error("expected Available=true with running server")
	}
}

func TestAvailableFalse(t *testing.T) {
	client := NewOllamaClient("http://127.0.0.1:1", "all-minilm", 384)
	if client.Available(context.Background()) {
		t.Error("expected Available=false with no server")
	}
}

func TestEmbedDimensionMismatch(t *testing.T) {
	srv := mockOllama(t, 128, 0)
	defer srv.Close()

	client := NewOllamaClient(srv.URL, "all-minilm", 384)
	_, err := client.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected dimension mismatch error")
	}
}

func TestEmbedBatchDimensionMismatch(t *testing.T) {
	srv := mockOllama(t, 128, 0)
	defer srv.Close()

	client := NewOllamaClient(srv.URL, "all-minilm", 384)
	_, err := client.EmbedBatch(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("expected dimension mismatch error")
	}
}
