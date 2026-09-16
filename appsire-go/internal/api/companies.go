package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"appsire-go/internal/company"
)

func (s *Server) HandleCompanies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if idStr := r.URL.Query().Get("id"); idStr != "" {
			s.getCompany(w, r, idStr)
			return
		}
		s.listCompanies(w, r)
	case http.MethodPost:
		s.saveCompany(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
	}
}

func (s *Server) getCompany(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		respondError(w, http.StatusBadRequest, "Identificador de empresa inválido")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	item, err := s.companyStore.Get(ctx, id)
	if errors.Is(err, company.ErrNotFound) {
		respondError(w, http.StatusNotFound, "Empresa no encontrada")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Error obteniendo datos de empresa: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"empresa": item,
	})
}

func (s *Server) listCompanies(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	companies, err := s.companyStore.List(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "No se pudieron consultar las empresas")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"empresas": companies,
	})
}

func (s *Server) saveCompany(w http.ResponseWriter, r *http.Request) {
	var item company.Company
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&item); err != nil {
		respondError(w, http.StatusBadRequest, "Datos de empresa inválidos")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	id, err := s.companyStore.Save(ctx, item)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"message": "Empresa guardada de forma segura",
	})
}

func (s *Server) HandleCompanySelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	s.identityMu.Lock()
	defer s.identityMu.Unlock()
	if s.downloadEngine.IsRunning() {
		respondError(w, http.StatusConflict, "Espere a que termine o cancele la descarga antes de cambiar de empresa")
		return
	}

	id, err := decodeCompanyID(w, r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()

	credentials, summary, err := s.companyStore.Credentials(ctx, id)
	if errors.Is(err, company.ErrNotFound) {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "No se pudieron abrir las credenciales protegidas")
		return
	}
	previousRUC := strings.TrimSpace(s.tokenService.GetCredentials().RUC)
	token, err := s.tokenService.RequestNewToken(ctx, credentials)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.companyStore.Select(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, "SUNAT autenticó, pero no se pudo guardar la selección")
		return
	}
	if previousRUC != "" && previousRUC != strings.TrimSpace(summary.RUC) {
		if err := s.resetCompanySession(); err != nil {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"ruc":          summary.RUC,
		"razon_social": summary.BusinessName,
		"expires_at":   token.ExpiresAt.Format(time.RFC3339),
		"message":      "Empresa seleccionada y conectada con SUNAT",
	})
}

func (s *Server) HandleCompanyToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	s.identityMu.Lock()
	defer s.identityMu.Unlock()
	if s.downloadEngine.IsRunning() {
		respondError(w, http.StatusConflict, "El token del lote está en uso; espere a que termine o cancele la descarga")
		return
	}

	id, err := decodeCompanyID(w, r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()

	credentials, summary, err := s.companyStore.Credentials(ctx, id)
	if errors.Is(err, company.ErrNotFound) {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "No se pudieron obtener las credenciales: "+err.Error())
		return
	}

	previousRUC := strings.TrimSpace(s.tokenService.GetCredentials().RUC)
	token, err := s.tokenService.RequestNewToken(ctx, credentials)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Error al generar Token en SUNAT: "+err.Error())
		return
	}
	if err := s.companyStore.Select(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, "SUNAT autenticó, pero no se pudo guardar la selección")
		return
	}
	if previousRUC != "" && previousRUC != strings.TrimSpace(summary.RUC) {
		if err := s.resetCompanySession(); err != nil {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"ruc":          summary.RUC,
		"razon_social": summary.BusinessName,
		"expires_at":   token.ExpiresAt.Format("02/01/2006 15:04:05"),
		"message":      fmt.Sprintf("✔ Token generado con éxito para %s. Válido hasta: %s", summary.BusinessName, token.ExpiresAt.Format("15:04:05")),
	})
}

func (s *Server) HandleCompanyDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	id, err := decodeCompanyID(w, r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := s.companyStore.Delete(ctx, id); errors.Is(err, company.ErrNotFound) {
		respondError(w, http.StatusNotFound, err.Error())
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "No se pudo eliminar la empresa")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (s *Server) HandleCompanyDeleteMultiple(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Lista de identificadores inválida")
		return
	}
	if len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "No se seleccionaron empresas")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	if err := s.companyStore.DeleteMultiple(ctx, req.IDs); err != nil {
		respondError(w, http.StatusInternalServerError, "Error eliminando empresas: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"deleted": len(req.IDs),
	})
}

func decodeCompanyID(w http.ResponseWriter, r *http.Request) (int64, error) {
	var request struct {
		ID json.Number `json:"id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.UseNumber()
	if err := decoder.Decode(&request); err != nil {
		return 0, errors.New("identificador de empresa inválido")
	}
	id, err := strconv.ParseInt(string(request.ID), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("identificador de empresa inválido")
	}
	return id, nil
}
