package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"appsire-go/internal/exchangerate"
	"appsire-go/internal/sirepreview"
	"appsire-go/internal/validation"
)

type ValidateTCRequest struct {
	Book   string                   `json:"book"`    // "RCE" o "RVIE"
	TCType string                   `json:"tc_type"` // "Venta" o "Compra"
	Items  []sirepreview.ProposalItem `json:"items"`
}

type ValidateTCResponse struct {
	Success bool                          `json:"success"`
	Message string                        `json:"message,omitempty"`
	Report  validation.TCValidationReport `json:"report"`
}

func (s *Server) HandleValidateTC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateTCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Cuerpo de petición JSON no válido: " + err.Error(),
		})
		return
	}

	if strings.TrimSpace(req.Book) == "" {
		req.Book = "RCE"
	}
	if strings.TrimSpace(req.TCType) == "" {
		req.TCType = "Venta"
	}

	if len(req.Items) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ValidateTCResponse{
			Success: true,
			Message: "No hay comprobantes para analizar",
			Report: validation.TCValidationReport{
				Book:   req.Book,
				TCType: req.TCType,
				Items:  []validation.ValidatedItem{},
			},
		})
		return
	}

	// 1. Extraer los meses únicos que se necesitan
	months := validation.ExtractMonthsFromItems(req.Items)

	// 2. Si hay servicio de tipo de cambio, asegurar que los meses estén descargados en SQLite
	rates := make(map[string]exchangerate.Rate)
	if s.exchangeRateService != nil && len(months) > 0 {
		// Intentar asegurar los meses
		_ = s.exchangeRateService.EnsureMonths(r.Context(), months)

		// Leer las tasas de SQLite
		fetchedRates, err := s.exchangeRateService.GetRatesForMonths(r.Context(), months)
		if err == nil {
			rates = fetchedRates
		}
	}

	// 3. Ejecutar la validación y recálculo tributario
	report := validation.ValidateTC(req.Items, req.Book, req.TCType, rates)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ValidateTCResponse{
		Success: true,
		Report:  report,
	})
}
