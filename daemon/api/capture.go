package api

import (
	"encoding/json"
	"net/http"

	"scaffold/capture"
)

type captureRequest struct {
	Text string `json:"text"`
}

type captureResponse struct {
	ID           string `json:"id"`
	TriageAction string `json:"triage_action"`
	MemoryID     string `json:"memory_id,omitempty"`
	TriageStatus string `json:"triage_status"`
}

func (s *Server) handleCapture(w http.ResponseWriter, r *http.Request) {
	var req captureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text is required"})
		return
	}

	captureID, memoryID, triage, err := capture.Ingest(r.Context(), s.db, s.brain, req.Text, "web")
	if err != nil {
		writeInternalError(w, err)
		return
	}

	resp := captureResponse{ID: captureID, MemoryID: memoryID}
	if triage == nil {
		resp.TriageStatus = "degraded"
	} else {
		resp.TriageAction = triage.Action
		resp.TriageStatus = "ok"
	}

	writeJSON(w, http.StatusCreated, resp)
}
