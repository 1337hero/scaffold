package api

import (
	"encoding/json"
	"net/http"
	"time"

	"scaffold/coder"
	"scaffold/sessionbus"
)

func (s *Server) handleCoderTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.coder.ListTasks()
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleCoderTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, ok := s.coder.GetTask(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleCoderTaskKill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.coder.KillTask(id) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found or not running"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (s *Server) handleCoderDispatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Task  string `json:"task"`
		Chain string `json:"chain"`
		CWD   string `json:"cwd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Task == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task required"})
		return
	}

	msg := coder.CodeTaskMessage{
		Type:  "code_task",
		Task:  req.Task,
		Chain: req.Chain,
		CWD:   req.CWD,
	}
	data, _ := json.Marshal(msg)

	if _, err := s.sessionBus.Send(r.Context(), sessionbus.SendRequest{
		FromSessionID: "scaffold-ui",
		ToSessionID:   "scaffold-coder",
		Message:       string(data),
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "dispatched"})
}

func (s *Server) handleCoderStepEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stepNum := r.PathValue("step_num")

	_, ok := s.coder.GetTask(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}

	events, err := s.coder.ReadStepEvents(id, stepNum)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "events not found"})
		return
	}

	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleCoderStream(w http.ResponseWriter, r *http.Request) {
	// Disable write deadline for SSE — this is a long-lived connection.
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		writeInternalError(w, err)
		return
	}
	s.coder.Hub().ServeSSE(w, r)
}
