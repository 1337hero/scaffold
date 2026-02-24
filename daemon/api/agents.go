package api

import (
	"encoding/json"
	"net/http"
	"time"

	"scaffold/agents"
	"scaffold/sessionbus"
)

func (s *Server) handleAgentTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.agents.ListTasks()
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleAgentTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, ok := s.agents.GetTask(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleAgentTaskKill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.agents.KillTask(id) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found or not running"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (s *Server) handleAgentDispatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Task  string `json:"task"`
		Chain string `json:"chain"`
		CWD   string `json:"cwd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Task == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task required"})
		return
	}

	msg := agents.CodeTaskMessage{
		Type:  "code_task",
		Task:  req.Task,
		Chain: req.Chain,
		CWD:   req.CWD,
	}
	data, _ := json.Marshal(msg)

	if _, err := s.sessionBus.Send(r.Context(), sessionbus.SendRequest{
		FromSessionID: "scaffold-ui",
		ToSessionID:   "scaffold-worker",
		Message:       string(data),
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "dispatched"})
}

func (s *Server) handleAgentStepEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stepNum := r.PathValue("step_num")

	_, ok := s.agents.GetTask(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}

	events, err := s.agents.ReadStepEvents(id, stepNum)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "events not found"})
		return
	}

	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleAgentStream(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		writeInternalError(w, err)
		return
	}
	s.agents.Hub().ServeSSE(w, r)
}

func (s *Server) handleAgentChains(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.agents.ChainNames())
}
