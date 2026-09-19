package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"appsire-go/internal/cpe"
	"appsire-go/internal/sirepreview"
)

type ValidateCPERequest struct {
	Book   string                     `json:"book"`   // "RCE" o "RVIE"
	Items  []sirepreview.ProposalItem `json:"items"`  // Lista de comprobantes
	Stream bool                       `json:"stream"` // Si es true, envía eventos ndjson progresivos
}

type ValidateCPEResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message,omitempty"`
	Report  *cpe.CPEReport `json:"report,omitempty"`
	Error   string         `json:"error,omitempty"`
}

func (s *Server) HandleValidateCPE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateCPERequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ValidateCPEResponse{
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
		_ = json.NewEncoder(w).Encode(ValidateCPEResponse{
			Success: true,
			Message: "No hay comprobantes para auditar",
			Report: &cpe.CPEReport{
				Book:  req.Book,
				Items: []cpe.CPEValidatedItem{},
			},
		})
		return
	}

	// 1. Obtener la empresa seleccionada para consultar sus credenciales API
	comp, err := s.companyStore.GetSelected(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ValidateCPEResponse{
			Success: false,
			Error:   "No hay una empresa activa seleccionada en el sistema. Seleccione o cree una empresa primero.",
		})
		return
	}

	// 2. Determinar credenciales API (Prioridad: específicas de CPE -> principales de SUNAT)
	clientID := strings.TrimSpace(comp.CpeClientID)
	clientSecret := strings.TrimSpace(comp.CpeClientSecret)

	if clientID == "" || clientSecret == "" {
		clientID = strings.TrimSpace(comp.ClientID)
		clientSecret = strings.TrimSpace(comp.ClientSecret)
	}

	if clientID == "" || clientSecret == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ValidateCPEResponse{
			Success: false,
			Error: fmt.Sprintf("La empresa %s (%s) no tiene credenciales de API SUNAT configuradas.\n\nRegístrelas en CONTROL ▸ Empresas (Client ID y Client Secret con permiso de Consulta Integrada).",
				comp.BusinessName, comp.RUC),
		})
		return
	}

	// 3. Obtener token OAuth2 para CPE
	token, err := s.cpeClient.GetToken(r.Context(), clientID, clientSecret)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ValidateCPEResponse{
			Success: false,
			Error:   "Error de autenticación con SUNAT: " + err.Error(),
		})
		return
	}

	// 4. Procesar auditoría (Streaming progresivo o respuesta única)
	flusher, canFlush := w.(http.Flusher)
	isStreaming := req.Stream && canFlush

	if isStreaming {
		// Deshabilitar timeout de escritura para streaming prolongado si es soportado
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})

		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		// Mutex obligatorio: múltiples workers concurrentes envían progreso
		var streamMu sync.Mutex

		onProgress := func(item cpe.CPEValidatedItem, current, total int) {
			line, err := json.Marshal(map[string]interface{}{
				"type":    "item",
				"current": current,
				"total":   total,
				"item":    item,
			})
			if err != nil {
				return
			}
			streamMu.Lock()
			defer streamMu.Unlock()
			_, _ = fmt.Fprintf(w, "%s\n", line)
			flusher.Flush()
		}

		report, err := cpe.ValidateBatch(r.Context(), s.cpeClient, comp.RUC, req.Book, req.Items, token, onProgress)
		if err != nil {
			errLine, _ := json.Marshal(map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			})
			streamMu.Lock()
			_, _ = fmt.Fprintf(w, "%s\n", errLine)
			flusher.Flush()
			streamMu.Unlock()
			return
		}

		reportLine, _ := json.Marshal(map[string]interface{}{
			"type":   "report",
			"report": report,
		})
		streamMu.Lock()
		_, _ = fmt.Fprintf(w, "%s\n", reportLine)
		flusher.Flush()
		streamMu.Unlock()
		return
	}

	// Modo síncrono estándar (Devuelve el JSON completo al finalizar)
	report, err := cpe.ValidateBatch(r.Context(), s.cpeClient, comp.RUC, req.Book, req.Items, token, nil)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ValidateCPEResponse{
			Success: false,
			Error:   "Error al procesar la validación: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ValidateCPEResponse{
		Success: true,
		Report:  report,
	})
}
