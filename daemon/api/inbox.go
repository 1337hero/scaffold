package api

import "net/http"

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	captures, err := s.db.ListRecent(50)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, captures)
}

func (s *Server) handleMemories(w http.ResponseWriter, r *http.Request) {
	memories, err := s.db.ListByImportance(50)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, memories)
}
