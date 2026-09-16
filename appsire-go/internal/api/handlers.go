package api

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"appsire-go/internal/auth"
	"appsire-go/internal/company"
	"appsire-go/internal/engine"
	"appsire-go/internal/excel"
	"appsire-go/internal/filemanager"
	"appsire-go/internal/license"
	"appsire-go/internal/session"
	"appsire-go/internal/sirepreview"
	"appsire-go/internal/sunat"
)

// Server almacena las dependencias y servicios de la API
type Server struct {
	tokenService   *auth.TokenService
	excelReader    *excel.ExcelReader
	downloadEngine *engine.DownloadEngine
	fileManager    *filemanager.FileManager
	licenseMgr     *license.LicenseManager
	proposalClient *sunat.ProposalClient
	companyStore   *company.Store
	sessionMgr     *session.Manager
	identityMu     sync.Mutex
	proposalMu     sync.RWMutex
	proposals      map[sunat.ProposalBook]proposalBinding
}

type proposalBinding struct {
	RUC    string
	Period string
	Ticket string
}

// NewServer inicializa el servidor con sus dependencias
func NewServer(
	ts *auth.TokenService,
	er *excel.ExcelReader,
	de *engine.DownloadEngine,
	fm *filemanager.FileManager,
	lm *license.LicenseManager,
	pc *sunat.ProposalClient,
	cs *company.Store,
	sm *session.Manager,
) *Server {
	return &Server{
		tokenService:   ts,
		excelReader:    er,
		downloadEngine: de,
		fileManager:    fm,
		licenseMgr:     lm,
		proposalClient: pc,
		companyStore:   cs,
		sessionMgr:     sm,
		proposals:      make(map[sunat.ProposalBook]proposalBinding),
	}
}

func (s *Server) RequireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.sessionMgr.Valid(r) || !s.licenseMgr.HasActiveLicense() {
			respondError(w, http.StatusUnauthorized, "Debe iniciar sesión con una licencia AutoSire válida")
			return
		}
		next(w, r)
	}
}

func SameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if r.Method != http.MethodGet && r.Method != http.MethodHead && origin != "" {
			allowedHTTP := "http://" + r.Host
			allowedHTTPS := "https://" + r.Host
			if origin != allowedHTTP && origin != allowedHTTPS {
				respondError(w, http.StatusForbidden, "Origen de solicitud no permitido")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src https://fonts.gstatic.com; script-src 'self'; img-src 'self' data:; connect-src 'self'",
		)
		next.ServeHTTP(w, r)
	})
}

// JSON Helper para respuestas estandarizadas
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// HandleAuthToken solicita o actualiza las credenciales de SUNAT
func (s *Server) HandleAuthToken(w http.ResponseWriter, r *http.Request) {
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

	var creds sunat.SunatCredentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		respondError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	previousRUC := strings.TrimSpace(s.tokenService.GetCredentials().RUC)
	tokenResp, err := s.tokenService.RequestNewToken(ctx, creds)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if previousRUC != "" && previousRUC != strings.TrimSpace(creds.RUC) {
		if err := s.resetCompanySession(); err != nil {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"message":    "Autenticación exitosa con SUNAT",
		"token_type": tokenResp.TokenType,
		"expires_at": tokenResp.ExpiresAt.Format(time.RFC3339),
		"ruc":        tokenResp.RUC,
	})
}

func (s *Server) resetCompanySession() error {
	if err := s.downloadEngine.ResetSession(); err != nil {
		return err
	}
	s.proposalMu.Lock()
	s.proposals = make(map[sunat.ProposalBook]proposalBinding)
	s.proposalMu.Unlock()
	s.proposalClient.ResetTransport()
	return nil
}

// HandleAuthStatus verifica si existe un token vigente en memoria
func (s *Server) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	info := s.tokenService.GetTokenInfo()
	respondJSON(w, http.StatusOK, info)
}

// HandleExcelUpload procesa un archivo Excel subido por el usuario
func (s *Server) HandleExcelUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	// Límite de 32MB para archivo subido
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "Error procesando formulario: "+err.Error())
		return
	}

	file, _, err := r.FormFile("excel")
	if err != nil {
		respondError(w, http.StatusBadRequest, "No se encontró el archivo Excel en la solicitud")
		return
	}
	defer file.Close()

	targetSheet := r.FormValue("sheet")

	comprobantes, metadata, err := s.excelReader.ReadComprobantesFromStream(file, targetSheet)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Error leyendo Excel: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"total":        len(comprobantes),
		"metadata":     metadata,
		"comprobantes": comprobantes,
	})
}

// HandleDownloadTemplate sirve el archivo Excel plantilla para los usuarios
func (s *Server) HandleDownloadTemplate(w http.ResponseWriter, r *http.Request) {
	templatePath := "plantilla_comprobantes.xlsx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		templatePath = "../plantilla_comprobantes.xlsx"
	}
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		respondError(w, http.StatusNotFound, "Plantilla no encontrada")
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\"plantilla_comprobantes.xlsx\"")
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	http.ServeFile(w, r, templatePath)
}

// HandleDownloadSireProposal genera y guarda la propuesta RCE o RVIE de SUNAT.
func (s *Server) HandleDownloadSireProposal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	s.identityMu.Lock()
	defer s.identityMu.Unlock()
	if s.downloadEngine.IsRunning() {
		respondError(w, http.StatusConflict, "Espere a que termine o cancele la descarga XML antes de obtener otra propuesta")
		return
	}

	licStatus := s.licenseMgr.GetStatus()
	active, ok := licStatus["active"].(bool)
	if !ok || !active {
		respondError(w, http.StatusForbidden, "Debe activar una licencia válida para descargar propuestas.")
		return
	}

	var request struct {
		Period        string             `json:"periodo"`
		Book          sunat.ProposalBook `json:"libro"`
		ReuseExisting bool               `json:"reutilizar_existente"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()

	proposal, err := s.proposalClient.Download(
		ctx,
		request.Book,
		strings.TrimSpace(request.Period),
		request.ReuseExisting,
	)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	credentials := s.tokenService.GetCredentials()
	path, err := s.fileManager.SaveSireProposal(
		credentials.RUC,
		proposal.Period,
		proposal.Book,
		proposal.FileName,
		proposal.Content,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	preview, err := sirepreview.FromZIP(proposal.Content, proposal.Book, proposal.Period)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "La propuesta fue guardada, pero no se pudo visualizar: "+err.Error())
		return
	}
	s.proposalMu.Lock()
	s.proposals[proposal.Book] = proposalBinding{
		RUC:    credentials.RUC,
		Period: proposal.Period,
		Ticket: proposal.Ticket,
	}
	s.proposalMu.Unlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"libro":       proposal.Book,
		"periodo":     proposal.Period,
		"ticket":      proposal.Ticket,
		"file_name":   proposal.FileName,
		"path":        path,
		"reutilizado": proposal.Reused,
		"generado_en": proposal.Generated,
		"preview":     preview,
	})
}

// HandleDownloadItem descarga un formato para un único comprobante.
func (s *Server) HandleDownloadItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	if !s.licenseMgr.HasActiveLicense() {
		respondError(w, http.StatusForbidden, "Debe activar una licencia válida para descargar comprobantes.")
		return
	}

	var request struct {
		Comprobante sunat.Comprobante  `json:"comprobante"`
		Tipo        sunat.TipoDescarga `json:"tipo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}
	credentials := s.tokenService.GetCredentials()
	if request.Comprobante.RUC == "" {
		request.Comprobante.RUC = credentials.RUC
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	result, err := s.downloadEngine.DownloadItem(ctx, request.Comprobante, request.Tipo)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"result":  result,
	})
}

// HandleLicenseStatus entrega la información de la licencia activa y Machine ID
func (s *Server) HandleLicenseStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	status := s.licenseMgr.GetStatus()
	respondJSON(w, http.StatusOK, status)
}

// HandleLicenseActivate recibe credenciales o clave para activar la licencia contra el servidor AutoSire
func (s *Server) HandleLicenseActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var req struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		Key       string `json:"key"`
		ServerURL string `json:"server_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}

	data, err := s.licenseMgr.Activate(req.ServerURL, req.Username, req.Password, req.Key)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Activación fallida: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "¡Licencia activada con éxito!",
		"username": data.Username,
		"plan":     data.Plan,
		"expires":  data.ExpiresAt.Format("02/01/2006"),
	})
}

// HandleStartDownload inicia el proceso de descarga masiva (protegido por licencia)
func (s *Server) HandleStartDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	s.identityMu.Lock()
	defer s.identityMu.Unlock()

	// Comprobación de licencia activa
	licStatus := s.licenseMgr.GetStatus()
	active, ok := licStatus["active"].(bool)
	if !ok || !active {
		respondError(w, http.StatusForbidden, "Debe activar una licencia válida para descargar comprobantes.")
		return
	}

	var req engine.DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}

	cachedCreds := s.tokenService.GetCredentials()
	if err := s.validateProposalBinding(req, cachedCreds.RUC); err != nil {
		respondError(w, http.StatusConflict, err.Error())
		return
	}
	for i := range req.Comprobantes {
		// La carpeta pertenece siempre a la empresa activa; el RUC del
		// comprobante se conserva como emisor para construir la URL SUNAT.
		req.Comprobantes[i].EmpresaRUC = cachedCreds.RUC
		if (req.Comprobantes[i].RUC == "" || req.Comprobantes[i].Libro == "1") && cachedCreds.RUC != "" {
			req.Comprobantes[i].RUC = cachedCreds.RUC
			req.Comprobantes[i].ID = fmt.Sprintf("%s-%s-%s-%s",
				req.Comprobantes[i].RUC,
				req.Comprobantes[i].Tipo,
				req.Comprobantes[i].Serie,
				req.Comprobantes[i].Numero)
		}
	}

	batchID, err := s.downloadEngine.StartBatch(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"batch_id": batchID,
		"message":  "Descarga masiva iniciada con éxito",
	})
}

func (s *Server) validateProposalBinding(req engine.DownloadRequest, currentRUC string) error {
	if strings.TrimSpace(req.ProposalBook) == "" {
		return nil
	}
	book := sunat.ProposalBook(strings.ToUpper(strings.TrimSpace(req.ProposalBook)))
	s.proposalMu.RLock()
	binding, found := s.proposals[book]
	s.proposalMu.RUnlock()
	if !found || binding.Ticket != strings.TrimSpace(req.ProposalTicket) ||
		binding.Period != strings.TrimSpace(req.ProposalPeriod) ||
		binding.RUC != strings.TrimSpace(currentRUC) || binding.RUC != strings.TrimSpace(req.OwnerRUC) {
		return fmt.Errorf("la propuesta ya no coincide con la empresa o período activos; vuelva a obtenerla antes de descargar XML")
	}
	return nil
}

// HandleDownloadStatus retorna el progreso en tiempo real (modo polling fallback)
func (s *Server) HandleDownloadStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	status, logs := s.downloadEngine.GetCurrentStatus()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": status,
		"logs":   logs,
	})
}

// HandleDownloadEvents transmite actualizaciones en tiempo real vía Server-Sent Events (SSE)
func (s *Server) HandleDownloadEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming no soportado", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := s.downloadEngine.Subscribe()
	defer s.downloadEngine.Unsubscribe(ch)

	// Enviar estado inicial inmediato
	initStatus, logs := s.downloadEngine.GetCurrentStatus()
	initData := map[string]interface{}{
		"status": initStatus,
		"logs":   logs,
	}
	if b, err := json.Marshal(initData); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case status, ok := <-ch:
			if !ok {
				return
			}
			_, currentLogs := s.downloadEngine.GetCurrentStatus()
			data := map[string]interface{}{
				"status": status,
				"logs":   currentLogs,
			}
			if b, err := json.Marshal(data); err == nil {
				fmt.Fprintf(w, "data: %s\n\n", b)
				flusher.Flush()
			}
		}
	}
}

// HandleCancelDownload cancela la descarga activa
func (s *Server) HandleCancelDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	err := s.downloadEngine.CancelCurrentBatch()
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Descarga cancelada",
	})
}

// HandleOpenFolder abre la carpeta de descargas en el Explorador de Windows
func (s *Server) HandleOpenFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	err := s.fileManager.OpenInExplorer("")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "No se pudo abrir la carpeta: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Carpeta abierta en el Explorador de Windows",
		"path":    s.fileManager.BaseDir,
	})
}

// HandleDownloadZip entrega solo los archivos exitosos del lote visible.
func (s *Server) HandleDownloadZip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	status, _ := s.downloadEngine.GetCurrentStatus()
	if status.BatchID == "" {
		respondError(w, http.StatusNotFound, "No existe un lote para descargar")
		return
	}
	if status.Estado == "procesando" {
		respondError(w, http.StatusConflict, "Espere a que termine o cancele el lote antes de generar el ZIP")
		return
	}
	paths := make([]string, 0, status.Exitosos+1)
	for _, result := range status.Resultados {
		if result.Exito && strings.TrimSpace(result.RutaLocal) != "" {
			paths = append(paths, result.RutaLocal)
		}
	}
	if strings.TrimSpace(status.ManifestPath) != "" {
		paths = append(paths, status.ManifestPath)
	}
	if len(paths) == 0 {
		respondError(w, http.StatusNotFound, "El lote no contiene archivos para entregar")
		return
	}

	zipFileName := fmt.Sprintf("CPE_%s.zip", status.BatchID)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFileName))

	err := s.fileManager.CreateZipFiles(paths, w)
	if err != nil {
		http.Error(w, "Error generando ZIP: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// HandleViewFile muestra un resultado descargado directamente en el navegador.
func (s *Server) HandleViewFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	requestedPath, err := filepath.Abs(r.URL.Query().Get("path"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Ruta de archivo inválida")
		return
	}
	basePath, err := filepath.Abs(s.fileManager.BaseDir)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "No se pudo resolver la carpeta de descargas")
		return
	}
	relativePath, err := filepath.Rel(basePath, requestedPath)
	outsideBase := err != nil ||
		relativePath == ".." ||
		strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
	if outsideBase {
		respondError(w, http.StatusBadRequest, "El archivo no pertenece a las descargas")
		return
	}
	if _, err := os.Stat(requestedPath); err != nil {
		respondError(w, http.StatusNotFound, "Archivo no encontrado")
		return
	}

	disposition := "inline"
	if r.URL.Query().Get("download") == "1" {
		disposition = "attachment"
	}
	if disposition == "inline" && strings.EqualFold(filepath.Ext(requestedPath), ".zip") {
		if serveZIPPreview(w, requestedPath) {
			return
		}
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q", disposition, filepath.Base(requestedPath)))
	http.ServeFile(w, r, requestedPath)
}

func serveZIPPreview(w http.ResponseWriter, path string) bool {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer archive.Close()

	for _, wanted := range []string{".pdf", ".xml", ".txt", ".csv"} {
		for _, file := range archive.File {
			if file.FileInfo().IsDir() || !strings.EqualFold(filepath.Ext(file.Name), wanted) {
				continue
			}
			stream, err := file.Open()
			if err != nil {
				continue
			}
			content, readErr := io.ReadAll(io.LimitReader(stream, 64<<20))
			stream.Close()
			if readErr != nil {
				continue
			}
			contentType := mime.TypeByExtension(wanted)
			if contentType == "" {
				contentType = "text/plain; charset=utf-8"
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filepath.Base(file.Name)))
			_, _ = w.Write(content)
			return true
		}
	}
	return false
}
