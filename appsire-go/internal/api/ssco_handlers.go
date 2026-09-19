package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"appsire-go/internal/sirepreview"
	"appsire-go/internal/ssco"
)

type ValidateSSCORequest struct {
	Book         string                     `json:"book"`          // "RCE"
	Items        []sirepreview.ProposalItem `json:"items"`         // Lista de comprobantes
	ForceRefresh bool                       `json:"force_refresh"` // Forzar re-descarga desde SUNAT
}

type ValidateSSCOResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message,omitempty"`
	Report  ssco.SSCOReport `json:"report"`
	Error   string          `json:"error,omitempty"`
}

// HandleValidateSSCO audita la lista de comprobantes de compras contra el padrón oficial de SSCO.
func (s *Server) HandleValidateSSCO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateSSCORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ValidateSSCOResponse{
			Success: false,
			Error:   "Cuerpo de petición JSON no válido: " + err.Error(),
		})
		return
	}

	if strings.TrimSpace(req.Book) == "" {
		req.Book = "RCE"
	}

	if s.sscoService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(ValidateSSCOResponse{
			Success: false,
			Error:   "El servicio de validación SSCO no está inicializado en el servidor.",
		})
		return
	}

	if len(req.Items) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ValidateSSCOResponse{
			Success: true,
			Message: "No hay comprobantes para auditar",
			Report: ssco.SSCOReport{
				Book:  req.Book,
				Items: []ssco.SSCOValidatedItem{},
			},
		})
		return
	}

	// Obtener el padrón (en memoria, descargado de SUNAT o desde caché local)
	padron, err := s.sscoService.GetPadron(r.Context(), req.ForceRefresh)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(ValidateSSCOResponse{
			Success: false,
			Error:   "No se pudo cargar el padrón de SUNAT: " + err.Error(),
		})
		return
	}

	// Ejecutar la auditoría
	report := ssco.ValidateItems(req.Items, padron)
	report.Book = req.Book

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ValidateSSCOResponse{
		Success: true,
		Report:  report,
	})
}

// HandleRefreshSSCO fuerza la re-descarga del padrón oficial desde el portal de SUNAT.
func (s *Server) HandleRefreshSSCO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	if s.sscoService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "El servicio SSCO no está disponible",
		})
		return
	}

	padron, err := s.sscoService.GetPadron(r.Context(), true)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Error al actualizar padrón SSCO: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":             true,
		"message":             "Padrón SSCO actualizado correctamente",
		"total_sujetos":       padron.Total,
		"fecha_actualizacion": padron.FechaActualizacion,
		"desde_cache":         padron.DesdeCache,
	})
}
