package api

import (
	"context"
	"net/http"
	"strings"
)

type ingestUploadResponse struct {
	Path      string `json:"path"`
	Directory string `json:"directory"`
	Queued    bool   `json:"queued"`
}

func (s *Server) handleIngestUpload(w http.ResponseWriter, r *http.Request) {
	if s.ingestor == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ingestion is not configured"})
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "multipart form is required"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	defer file.Close()

	path, err := s.ingestor.Upload(r.Context(), header.Filename, file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("process_now")), "true") {
		go func() {
			_ = s.ingestor.IngestNow(context.Background())
		}()
	}

	writeJSON(w, http.StatusCreated, ingestUploadResponse{
		Path:      path,
		Directory: s.ingestor.Directory(),
		Queued:    true,
	})
}
