package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"appsire-go/internal/sirepreview"
	"appsire-go/internal/validation"
)

type DetectCuadreRequest struct {
	Book  string                     `json:"book"`  // "RCE" o "RVIE"
	Items []sirepreview.ProposalItem `json:"items"` // Comprobantes de la propuesta
}

type DetectCuadreResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message,omitempty"`
	Report  validation.CuadreReport `json:"report"`
	Error   string                  `json:"error,omitempty"`
}

// HandleDetectCuadre analiza la consistencia aritmética de los comprobantes contra su Importe Total.
func (s *Server) HandleDetectCuadre(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var req DetectCuadreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(DetectCuadreResponse{
			Success: false,
			Error:   "Cuerpo de petición JSON no válido: " + err.Error(),
		})
		return
	}

	if strings.TrimSpace(req.Book) == "" {
		req.Book = "RCE"
	}

	if len(req.Items) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(DetectCuadreResponse{
			Success: true,
			Message: "No hay comprobantes para verificar",
			Report: validation.CuadreReport{
				Book:                req.Book,
				ColumnasDisponibles: validation.ObtenerColumnasAjuste(req.Book),
				ColumnaDefault:      "bi_gravada",
				Items:               []validation.CuadreItem{},
			},
		})
		return
	}

	report := validation.DetectarDescuadres(req.Items, req.Book)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(DetectCuadreResponse{
		Success: true,
		Report:  report,
	})
}
