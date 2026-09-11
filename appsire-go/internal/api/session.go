package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) HandleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	status := s.licenseMgr.GetStatus()
	active, _ := status["active"].(bool)
	if active && !s.sessionMgr.Valid(r) {
		if err := s.sessionMgr.Create(w, true); err != nil {
			respondError(w, http.StatusInternalServerError, "No se pudo crear la sesión")
			return
		}
	}
	status["authenticated"] = active
	respondJSON(w, http.StatusOK, status)
}

func (s *Server) HandleSessionLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var request struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		Key       string `json:"key"`
		ServerURL string `json:"server_url"`
		Remember  bool   `json:"remember"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, "Datos de acceso inválidos")
		return
	}

	data, err := s.licenseMgr.ActivateWithRemember(
		request.ServerURL,
		request.Username,
		request.Password,
		request.Key,
		request.Remember,
	)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Usuario, contraseña o licencia inválidos")
		return
	}
	if err := s.sessionMgr.Create(w, request.Remember); err != nil {
		respondError(w, http.StatusInternalServerError, "No se pudo crear la sesión")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"username":   data.Username,
		"plan":       data.Plan,
		"expires_at": data.ExpiresAt,
	})
}

func (s *Server) HandleSessionLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	s.sessionMgr.Delete(w, r)
	s.licenseMgr.EndSession()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}
