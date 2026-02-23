package api

import "net/http"

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	dash, err := s.db.DashboardData()
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dash)
}
