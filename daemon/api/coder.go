package api

import (
	"net/http"
	"time"
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

func (s *Server) handleCoderStream(w http.ResponseWriter, r *http.Request) {
	// Disable write deadline for SSE — this is a long-lived connection.
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		writeInternalError(w, err)
		return
	}
	s.coder.Hub().ServeSSE(w, r)
}
