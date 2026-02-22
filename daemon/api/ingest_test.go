package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeIngestor struct {
	dir       string
	uploadTo  string
	uploadErr error
	nowErr    error
}

func (f *fakeIngestor) Upload(ctx context.Context, filename string, r io.Reader) (string, error) {
	if f.uploadErr != nil {
		return "", f.uploadErr
	}
	_, _ = io.Copy(io.Discard, r)
	return f.uploadTo, nil
}

func (f *fakeIngestor) IngestNow(ctx context.Context) error {
	return f.nowErr
}

func (f *fakeIngestor) Directory() string {
	return f.dir
}

func buildMultipartUploadRequest(t *testing.T, target, filename, content string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader(content)); err != nil {
		t.Fatalf("copy file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Authorization", "Bearer "+testAPIToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestHandleIngestUploadServiceUnavailableWhenNotConfigured(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, buildMultipartUploadRequest(t, "/api/ingest", "note.md", "hello"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleIngestUploadSuccess(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetIngestor(&fakeIngestor{
		dir:      "/tmp/ingest",
		uploadTo: "/tmp/ingest/file.md",
	})

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, buildMultipartUploadRequest(t, "/api/ingest", "note.md", "hello"))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestHandleIngestUploadBadExtension(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetIngestor(&fakeIngestor{
		dir:       "/tmp/ingest",
		uploadErr: errors.New("unsupported file extension \".exe\""),
	})

	rec := httptest.NewRecorder()
	srv.mux.ServeHTTP(rec, buildMultipartUploadRequest(t, "/api/ingest", "bad.exe", "binary"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
