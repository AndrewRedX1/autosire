package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"appsire-go/internal/sirepreview"
	"appsire-go/internal/validation"
)

// DetectCorrelativosRequest estructura la petición para detectar huecos correlativos.
type DetectCorrelativosRequest struct {
	Book  string                     `json:"book"`  // "RVIE"
	Items []sirepreview.ProposalItem `json:"items"` // Comprobantes de la propuesta
}

// DetectCorrelativosResponse estructura la respuesta del análisis de correlatividad.
type DetectCorrelativosResponse struct {
	Success bool                          `json:"success"`
	Message string                        `json:"message,omitempty"`
	Report  validation.CorrelativosReport `json:"report"`
	Error   string                        `json:"error,omitempty"`
}

// HandleDetectCorrelativos analiza saltos en la numeración correlativa por serie en Ventas (RVIE).
func (s *Server) HandleDetectCorrelativos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var req DetectCorrelativosRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(DetectCorrelativosResponse{
			Success: false,
			Error:   "Cuerpo de petición JSON no válido: " + err.Error(),
		})
		return
	}

	if strings.TrimSpace(req.Book) == "" {
		req.Book = "RVIE"
	}

	if len(req.Items) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(DetectCorrelativosResponse{
			Success: true,
			Message: "No hay comprobantes para verificar",
			Report: validation.CorrelativosReport{
				Book:            req.Book,
				SeriesRevisadas: []string{},
				Advertencias:    []string{},
				Faltantes:       []validation.CorrelativoFaltante{},
			},
		})
		return
	}

	report := validation.DetectarCorrelativos(req.Items)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(DetectCorrelativosResponse{
		Success: true,
		Report:  report,
	})
}
