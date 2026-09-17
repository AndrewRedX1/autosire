package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"appsire-go/internal/filemanager"
	"appsire-go/internal/pdfgen"
	"appsire-go/internal/sunat"
)

const (
	initialWorkers     = 6
	descriptionWorkers = 6
	retryWorkers       = 2
	maxSweeps          = 3
	sweepCooldown      = 2500 * time.Millisecond

	defaultXMLDocumentTimeout = 20 * time.Second
	defaultXMLBatchTimeout    = 90 * time.Second
	xmlRecoveryCooldown       = 750 * time.Millisecond
	manifestCheckpointItems   = 25

	categoryDownloaded  = "descargado"
	categoryExisting    = "ya_existente"
	categoryUnavailable = "no_disponible_sunat"
	categoryUnsupported = "consulta_no_admitida"
	categoryRecoverable = "fallo_tecnico_recuperable"
)

type downloadClient interface {
	DownloadPDF(context.Context, sunat.Comprobante) (*sunat.DownloadedFile, error)
	DownloadXML(context.Context, sunat.Comprobante) (*sunat.DownloadedFile, error)
	DownloadXMLFallback(context.Context, sunat.Comprobante) (*sunat.DownloadedFile, error)
	ProbeXMLFallback(context.Context, sunat.Comprobante) error
	DownloadCDR(context.Context, sunat.Comprobante) (*sunat.DownloadedFile, error)
	DownloadDescription(context.Context, sunat.Comprobante) (sunat.DescriptionResult, error)
	ProbeConsultacpe(context.Context, sunat.Comprobante) error
	RefreshToken(context.Context) error
}

type connectionController interface {
	ResetTransport()
	CloseIdleConnections()
}

// DownloadRequest opciones para lanzar un lote de descargas
type DownloadRequest struct {
	Comprobantes        []sunat.Comprobante  `json:"comprobantes"`
	Tipos               []sunat.TipoDescarga `json:"tipos"` // PDF, XML, CDR, DESC
	Concurrency         int                  `json:"concurrency"`
	ProposalTicket      string               `json:"proposal_ticket,omitempty"`
	ProposalBook        string               `json:"proposal_book,omitempty"`
	ProposalPeriod      string               `json:"proposal_period,omitempty"`
	OwnerRUC            string               `json:"owner_ruc,omitempty"`
	BatchTimeoutSecs    int                  `json:"batch_timeout_seconds,omitempty"`
	DocumentTimeoutSecs int                  `json:"document_timeout_seconds,omitempty"`
	ContinueOnOutage    bool                 `json:"continue_on_sunat_outage,omitempty"`
}

// DownloadEngine coordina los workers y la ejecución de lotes de descarga masiva
type DownloadEngine struct {
	client      downloadClient
	fileManager *filemanager.FileManager
	mu          sync.RWMutex
	currentJob  *ActiveBatch

	// Soporte para Server-Sent Events (SSE)
	subMu       sync.RWMutex
	subscribers map[chan sunat.BatchStatus]struct{}
}

// ActiveBatch representa el estado de una ejecución activa en memoria
type ActiveBatch struct {
	Status          sunat.BatchStatus
	CancelFunc      context.CancelFunc
	Mu              sync.RWMutex
	Logs            []string
	Traffic         []string
	PendingRetries  map[string]struct{}
	DocumentTimeout time.Duration
}

// NewDownloadEngine inicializa el motor de descargas por etapas con soporte SSE.
func NewDownloadEngine(client downloadClient, fm *filemanager.FileManager) *DownloadEngine {
	return &DownloadEngine{
		client:      client,
		fileManager: fm,
		subscribers: make(map[chan sunat.BatchStatus]struct{}),
	}
}

// Subscribe registra un cliente SSE para recibir actualizaciones en tiempo real
func (e *DownloadEngine) Subscribe() chan sunat.BatchStatus {
	e.subMu.Lock()
	defer e.subMu.Unlock()
	ch := make(chan sunat.BatchStatus, 50)
	e.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe cancela la suscripción SSE
func (e *DownloadEngine) Unsubscribe(ch chan sunat.BatchStatus) {
	e.subMu.Lock()
	defer e.subMu.Unlock()
	delete(e.subscribers, ch)
	close(ch)
}

// broadcastStatus envía el estado actual a todos los clientes SSE conectados
func (e *DownloadEngine) broadcastStatus(status sunat.BatchStatus) {
	e.subMu.RLock()
	defer e.subMu.RUnlock()

	for ch := range e.subscribers {
		select {
		case ch <- status:
		default:
			// Si el cliente está saturado, no bloquear
		}
	}
}

// StartBatch inicia un nuevo lote de descargas en segundo plano
func (e *DownloadEngine) StartBatch(req DownloadRequest) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.currentJob != nil {
		e.currentJob.Mu.RLock()
		running := e.currentJob.Status.Estado == "procesando"
		e.currentJob.Mu.RUnlock()
		if running {
			return "", fmt.Errorf("ya existe un proceso de descarga en ejecución")
		}
	}

	if len(req.Comprobantes) == 0 {
		return "", fmt.Errorf("no se enviaron comprobantes para descargar")
	}
	if len(req.Tipos) == 0 {
		return "", fmt.Errorf("debe seleccionar al menos un tipo de descarga (PDF, XML, CDR o descripción)")
	}

	for i := range req.Comprobantes {
		req.Comprobantes[i] = prepareComprobante(req.Comprobantes[i])
		req.Comprobantes[i].Tipo = sunat.NormalizeTipo(req.Comprobantes[i].Tipo)
	}

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = 6
	}
	if concurrency > 15 {
		concurrency = 15
	}

	batchID := fmt.Sprintf("batch-%d", time.Now().UnixNano())
	totalWorkItems := expectedItemCount(req.Comprobantes, req.Tipos)

	ctx := context.Background()
	var cancel context.CancelFunc
	if xmlOnly(req.Tipos) {
		batchTimeout := xmlBatchTimeout(req.BatchTimeoutSecs, len(req.Comprobantes), concurrency)
		ctx, cancel = context.WithTimeout(ctx, batchTimeout)
	} else if containsOnlineFormats(req.Tipos) {
		batchTimeout := boundedBatchTimeout(req.BatchTimeoutSecs, totalWorkItems, concurrency)
		ctx, cancel = context.WithTimeout(ctx, batchTimeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	documentTimeout := boundedDuration(req.DocumentTimeoutSecs, defaultXMLDocumentTimeout, 5*time.Second, time.Minute)

	active := &ActiveBatch{
		Status: sunat.BatchStatus{
			BatchID:      batchID,
			Estado:       "procesando",
			TotalItems:   totalWorkItems,
			Procesados:   0,
			Exitosos:     0,
			Errores:      0,
			Porcentaje:   0.0,
			Mensaje:      fmt.Sprintf("Iniciando descarga de %d archivos con %d hilos paralelos...", totalWorkItems, concurrency),
			IniciadoEn:   time.Now(),
			Resultados:   make([]sunat.ItemResult, 0, totalWorkItems),
			HilosActivos: concurrency,
		},
		CancelFunc:      cancel,
		Logs:            make([]string, 0, 100),
		Traffic:         make([]string, 0, totalWorkItems),
		PendingRetries:  make(map[string]struct{}),
		DocumentTimeout: documentTimeout,
	}

	e.currentJob = active

	// Lanzar ejecución en goroutine desacoplada
	go e.runBatch(ctx, active, req, concurrency)

	return batchID, nil
}

func xmlBatchTimeout(requestedSeconds, items, workers int) time.Duration {
	if requestedSeconds > 0 {
		return boundedDuration(requestedSeconds, defaultXMLBatchTimeout, 30*time.Second, 10*time.Minute)
	}
	workers = max(1, workers)
	waves := (max(1, items) + workers - 1) / workers
	// Cinco segundos por ola admite latencia, fallback y algunos timeouts sin
	// convertir una degradación de SUNAT en una ejecución de varias horas.
	estimated := time.Duration(waves) * 5 * time.Second
	return min(max(estimated, defaultXMLBatchTimeout), 10*time.Minute)
}

func boundedBatchTimeout(requestedSeconds, totalWorkItems, workers int) time.Duration {
	if requestedSeconds > 0 {
		return boundedDuration(requestedSeconds, defaultXMLBatchTimeout, 30*time.Second, 15*time.Minute)
	}
	workers = max(1, workers)
	waves := (max(1, totalWorkItems) + workers - 1) / workers
	estimated := time.Duration(waves) * 5 * time.Second
	return min(max(estimated, defaultXMLBatchTimeout), 15*time.Minute)
}

func containsOnlineFormats(types []sunat.TipoDescarga) bool {
	for _, t := range types {
		if t == sunat.DescargaXML || t == sunat.DescargaCDR || t == sunat.DescargaPDF || t == sunat.DescargaDescripcion {
			return true
		}
	}
	return false
}

func xmlOnly(types []sunat.TipoDescarga) bool {
	return len(types) == 1 && types[0] == sunat.DescargaXML
}

func boundedDuration(seconds int, fallback, minimum, maximum time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	duration := time.Duration(seconds) * time.Second
	return min(max(duration, minimum), maximum)
}

// IsRunning informa si hay un lote que todavía puede usar el token activo.
func (e *DownloadEngine) IsRunning() bool {
	e.mu.RLock()
	job := e.currentJob
	e.mu.RUnlock()
	if job == nil {
		return false
	}
	job.Mu.RLock()
	defer job.Mu.RUnlock()
	return job.Status.Estado == "procesando"
}

// ResetSession elimina resultados y conexiones pertenecientes a la empresa
// anterior. El cambio se permite únicamente con el motor detenido.
func (e *DownloadEngine) ResetSession() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.currentJob != nil {
		e.currentJob.Mu.RLock()
		running := e.currentJob.Status.Estado == "procesando"
		e.currentJob.Mu.RUnlock()
		if running {
			return errors.New("no se puede reiniciar la sesión durante una descarga")
		}
	}
	e.currentJob = nil
	if controller, ok := e.client.(connectionController); ok {
		controller.ResetTransport()
	}
	e.broadcastStatus(sunat.BatchStatus{Estado: "inactivo", Mensaje: "Sesión de empresa reiniciada"})
	return nil
}

func prepareComprobante(comp sunat.Comprobante) sunat.Comprobante {
	comp.Numero = sunat.NormalizeNumero(comp.Numero)
	sheet := strings.ToLower(strings.TrimSpace(comp.SheetName))

	switch {
	case strings.Contains(sheet, "rvie"):
		comp.Libro = "1"
	case strings.Contains(sheet, "cpe"), strings.Contains(sheet, "comprobante"):
		comp.Libro = "2"
	}

	return comp
}

// DownloadItem descarga un único formato para un comprobante usando las mismas
// reglas de autorización, fallback XML y generación local de PDF del lote.
func (e *DownloadEngine) DownloadItem(
	ctx context.Context,
	comp sunat.Comprobante,
	tipo sunat.TipoDescarga,
) (sunat.ItemResult, error) {
	comp = prepareComprobante(comp)
	comp.Tipo = sunat.NormalizeTipo(comp.Tipo)
	if comp.RUC == "" || comp.Tipo == "" || comp.Serie == "" || comp.Numero == "" {
		return sunat.ItemResult{}, errors.New("el comprobante no tiene RUC, tipo, serie y número completos")
	}
	if tipo != sunat.DescargaPDF && tipo != sunat.DescargaXML && tipo != sunat.DescargaCDR && tipo != sunat.DescargaDescripcion {
		return sunat.ItemResult{}, fmt.Errorf("tipo de descarga desconocido: %s", tipo)
	}
	if len(eligibleForStage([]sunat.Comprobante{comp}, tipo)) == 0 {
		return sunat.ItemResult{}, errors.New("el comprobante no admite este tipo de archivo")
	}

	consultacpeAllowed := true
	if err := e.client.ProbeConsultacpe(ctx, comp); err != nil {
		if status, ok := sunat.HTTPStatus(err); ok && status == 401 {
			if refreshErr := e.client.RefreshToken(ctx); refreshErr == nil {
				err = e.client.ProbeConsultacpe(ctx, comp)
			}
			if status, ok := sunat.HTTPStatus(err); ok && status == 401 {
				consultacpeAllowed = false
			}
		}
	}

	if tipo == sunat.DescargaCDR && !consultacpeAllowed {
		return sunat.ItemResult{}, errors.New("consultacpe rechazó el token; CDR no dispone de endpoint alternativo")
	}

	attempt := e.downloadSingleItem(ctx, comp, tipo, consultacpeAllowed)
	if attempt.err != nil && tipo == sunat.DescargaPDF {
		attempt = e.generatePDFFromXML(ctx, comp, consultacpeAllowed)
	}
	if attempt.err != nil {
		return attempt.item, attempt.err
	}
	return attempt.item, nil
}

type attemptResult struct {
	item sunat.ItemResult
	err  error
}

// ServiceAvailability resume el sondeo autenticado previo a un lote.
type ServiceAvailability struct {
	Available bool
	Detail    string
}

// CheckSunatAvailability reproduce la advertencia preventiva de la macro. Los
// 401 renuevan el token una vez y nunca se clasifican como caída del servicio.
func (e *DownloadEngine) CheckSunatAvailability(
	ctx context.Context,
	comp sunat.Comprobante,
) ServiceAvailability {
	comp = prepareComprobante(comp)
	comp.Tipo = sunat.NormalizeTipo(comp.Tipo)
	refreshed := false
	consecutiveFailures := 0

	for consecutiveFailures < 3 {
		probeCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		err := e.client.ProbeConsultacpe(probeCtx, comp)
		cancel()
		if err == nil {
			return ServiceAvailability{Available: true}
		}

		if status, ok := sunat.HTTPStatus(err); ok {
			switch status {
			case http.StatusUnauthorized:
				if !refreshed {
					refreshed = true
					if refreshErr := e.client.RefreshToken(ctx); refreshErr == nil {
						continue
					}
				}
				return ServiceAvailability{Available: true, Detail: "SUNAT rechazó la autenticación"}
			case http.StatusForbidden:
				return ServiceAvailability{Available: true, Detail: "SUNAT rechazó el permiso consultacpe"}
			case http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
				return ServiceAvailability{
					Available: false,
					Detail:    fmt.Sprintf("SUNAT respondió HTTP %d", status),
				}
			}
		}

		if !sunat.IsTransientDownloadError(err) {
			return ServiceAvailability{Available: true}
		}
		consecutiveFailures++
		if consecutiveFailures < 3 {
			if waitErr := waitForStage(ctx, 250*time.Millisecond); waitErr != nil {
				break
			}
		}
	}

	return ServiceAvailability{
		Available: false,
		Detail:    "SUNAT acumuló tres fallos consecutivos de conexión",
	}
}

// runBatch replica la estrategia de la macro: formatos aislados, token por
// cohorte, menor concurrencia en recuperaciones y PDF local desde el XML.
func (e *DownloadEngine) runBatch(
	ctx context.Context,
	job *ActiveBatch,
	req DownloadRequest,
	concurrency int,
) {
	startedAt := time.Now()
	if controller, ok := e.client.(connectionController); ok {
		controller.ResetTransport()
		defer controller.CloseIdleConnections()
	}
	workers := min(concurrency, initialWorkers)
	e.addLog(job, fmt.Sprintf("Motor por etapas iniciado con hasta %d hilos", concurrency))

	selected := selectedTypes(req.Tipos)
	consultacpeAllowed := true
	preflightTimeout := job.DocumentTimeout
	if preflightTimeout <= 0 {
		preflightTimeout = defaultXMLDocumentTimeout
	}
	preflightCtx, cancelPreflight := context.WithTimeout(ctx, preflightTimeout)
	consultacpeAllowed = e.preflightConsultacpe(preflightCtx, job, req.Comprobantes)
	if selected[sunat.DescargaXML] && !consultacpeAllowed {
		if err := e.preflightXMLFallback(preflightCtx, job, req.Comprobantes[0]); err != nil {
			cancelPreflight()
			message := globalXMLAccessMessage(err)
			for _, comp := range req.Comprobantes {
				e.recordFinalResult(job, sunat.ItemResult{
					Comprobante: comp,
					Tipo:        sunat.DescargaXML,
					Exito:       false,
					Error:       message,
				}, startedAt)
			}
			e.finishBatch(ctx, job, startedAt)
			return
		}
	}
	cancelPreflight()
	stages := []sunat.TipoDescarga{
		sunat.DescargaXML,
		sunat.DescargaPDF,
		sunat.DescargaCDR,
		sunat.DescargaDescripcion,
	}
	if selected[sunat.DescargaCDR] {
		eligibleCDR := eligibleForStage(req.Comprobantes, sunat.DescargaCDR)
		omitted := len(req.Comprobantes) - len(eligibleCDR)
		if omitted > 0 {
			e.addLog(
				job,
				fmt.Sprintf("CDR: %d comprobantes de serie E omitidos porque no tienen CDR", omitted),
			)
		}
	}

	for _, stage := range stages {
		if !selected[stage] {
			continue
		}
		if ctx.Err() != nil {
			for _, comp := range eligibleForStage(req.Comprobantes, stage) {
				e.recordFinalResult(job, failedAttempt(comp, stage, ctx.Err()).item, startedAt)
			}
			continue
		}

		comps := eligibleForStage(req.Comprobantes, stage)
		stageWorkers := workers
		if stage == sunat.DescargaDescripcion {
			stageWorkers = min(concurrency, descriptionWorkers)
		}
		e.updateStageMessage(job, fmt.Sprintf(
			"Etapa %s: consultando %d de %d",
			stage,
			min(stageWorkers, len(comps)),
			len(comps),
		))
		e.addLog(job, fmt.Sprintf("Etapa %s iniciada: %d comprobantes", stage, len(comps)))

		results := e.runStage(
			ctx,
			job,
			comps,
			stage,
			stageWorkers,
			consultacpeAllowed,
		)
		for _, result := range results {
			e.recordFinalResult(job, result, startedAt)
		}
	}

	e.finishBatch(ctx, job, startedAt)
}

func (e *DownloadEngine) preflightConsultacpe(
	ctx context.Context,
	job *ActiveBatch,
	comps []sunat.Comprobante,
) bool {
	e.updateStageMessage(job, "Verificando token y permiso consultacpe...")
	err := e.client.ProbeConsultacpe(ctx, comps[0])
	if err == nil {
		e.addLog(job, "Preflight consultacpe: autorizado")
		return true
	}

	if status, ok := sunat.HTTPStatus(err); !ok || (status != 401 && status != 403) {
		e.addLog(job, fmt.Sprintf("Preflight consultacpe no concluyente: %s", safeResultError(err)))
		return true
	}

	e.addLog(job, "Preflight consultacpe recibio 401; renovando token una sola vez")
	if status, _ := sunat.HTTPStatus(err); status == 403 {
		e.addLog(job, "Preflight consultacpe recibio 403; XML probara controlcpe")
		return false
	}
	if refreshErr := e.client.RefreshToken(ctx); refreshErr != nil {
		e.addLog(job, fmt.Sprintf("No se pudo renovar el token del lote: %s", safeResultError(refreshErr)))
		return false
	}
	if retryErr := e.client.ProbeConsultacpe(ctx, comps[0]); retryErr == nil {
		e.addLog(job, "Preflight consultacpe autorizado despues de renovar")
		return true
	} else if status, ok := sunat.HTTPStatus(retryErr); !ok || (status != 401 && status != 403) {
		e.addLog(job, fmt.Sprintf("Preflight consultacpe no concluyente despues de renovar: %s", safeResultError(retryErr)))
		return true
	}

	e.addLog(job, "consultacpe no autorizado: XML usara controlcpe y se evitara repetir 401")
	return false
}

func (e *DownloadEngine) preflightXMLFallback(
	ctx context.Context,
	job *ActiveBatch,
	comp sunat.Comprobante,
) error {
	e.updateStageMessage(job, "Verificando acceso alternativo para XML...")
	err := e.client.ProbeXMLFallback(ctx, comp)
	if err == nil {
		e.addLog(job, "Preflight controlcpe: autorizado")
		return nil
	}
	status, ok := sunat.HTTPStatus(err)
	if !ok || (status != 401 && status != 403) {
		// Un documento inexistente o mal formado no demuestra una falla global.
		e.addLog(job, fmt.Sprintf("Preflight controlcpe no concluyente: %s", safeResultError(err)))
		return nil
	}
	if status == 401 {
		e.addLog(job, "Preflight controlcpe recibio 401; renovando token una sola vez")
		if refreshErr := e.client.RefreshToken(ctx); refreshErr != nil {
			return fmt.Errorf("renovando token para controlcpe: %w", refreshErr)
		}
		if retryErr := e.client.ProbeXMLFallback(ctx, comp); retryErr == nil {
			e.addLog(job, "Preflight controlcpe autorizado despues de renovar")
			return nil
		} else {
			err = retryErr
			status, ok = sunat.HTTPStatus(err)
			if !ok || (status != 401 && status != 403) {
				e.addLog(job, fmt.Sprintf("Preflight controlcpe no concluyente despues de renovar: %s", safeResultError(err)))
				return nil
			}
		}
	}
	return err
}

func globalXMLAccessMessage(err error) string {
	status, _ := sunat.HTTPStatus(err)
	if status == 403 {
		return "SUNAT rechazo el acceso XML (HTTP 403): habilite los permisos CPE de la credencial API y genere un token nuevo"
	}
	if status == 401 {
		return "SUNAT rechazo el token XML (HTTP 401) incluso despues de renovarlo: revise el RUC y los permisos de la credencial API"
	}
	return fmt.Sprintf("no se pudo validar el acceso XML: %s", safeResultError(err))
}

func (e *DownloadEngine) runStage(
	ctx context.Context,
	job *ActiveBatch,
	comps []sunat.Comprobante,
	tipo sunat.TipoDescarga,
	workers int,
	consultacpeAllowed bool,
) []sunat.ItemResult {
	if tipo == sunat.DescargaCDR && !consultacpeAllowed {
		return blockedResults(
			comps,
			tipo,
			"consultacpe rechazo el token; CDR no dispone de endpoint alternativo",
		)
	}
	if tipo == sunat.DescargaPDF && !consultacpeAllowed {
		attempts := make([]attemptResult, 0, len(comps))
		for _, comp := range comps {
			attempts = append(
				attempts,
				failedAttempt(comp, tipo, errors.New("consultacpe no autorizado")),
			)
		}
		attempts = e.generateFailedPDFs(ctx, job, attempts, consultacpeAllowed)
		results := make([]sunat.ItemResult, 0, len(attempts))
		for _, attempt := range attempts {
			results = append(results, attempt.item)
		}
		return results
	}

	var attempts []attemptResult
	if tipo == sunat.DescargaXML {
		initialCtx, cancelInitial := xmlInitialContext(ctx)
		attempts = e.processXMLWaves(initialCtx, job, comps, workers, consultacpeAllowed, 0)
		cancelInitial()
	} else {
		attempts = e.processStage(ctx, job, comps, tipo, workers, consultacpeAllowed, 0)
	}
	attempts = e.retryUnauthorizedCohort(ctx, job, attempts, tipo, consultacpeAllowed)
	if tipo == sunat.DescargaXML {
		attempts = e.retryXMLTransient(ctx, job, attempts, consultacpeAllowed)
	} else {
		attempts = e.sweepTransientFailures(ctx, job, attempts, tipo, consultacpeAllowed)
	}

	if tipo == sunat.DescargaPDF {
		attempts = e.generateFailedPDFs(ctx, job, attempts, consultacpeAllowed)
	}

	results := make([]sunat.ItemResult, 0, len(attempts))
	for _, attempt := range attempts {
		results = append(results, attempt.item)
	}
	return results
}

func xmlInitialContext(ctx context.Context) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return context.WithCancel(ctx)
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, remaining*7/10)
}

func (e *DownloadEngine) processXMLWaves(
	ctx context.Context,
	job *ActiveBatch,
	comps []sunat.Comprobante,
	workers int,
	consultacpeAllowed bool,
	retries int,
) []attemptResult {
	workers = max(1, workers)
	attempts := make([]attemptResult, 0, len(comps))
	for start := 0; start < len(comps); start += workers {
		end := min(start+workers, len(comps))
		wave := e.processStage(
			ctx,
			job,
			comps[start:end],
			sunat.DescargaXML,
			workers,
			consultacpeAllowed,
			retries,
		)
		attempts = append(attempts, wave...)
		if ctx.Err() != nil {
			if end < len(comps) {
				attempts = append(attempts, e.processStage(
					ctx,
					job,
					comps[end:],
					sunat.DescargaXML,
					workers,
					consultacpeAllowed,
					retries,
				)...)
			}
			break
		}
		transient := transientCount(wave)
		stallThreshold := max(1, (len(wave)+1)/2)
		if transient >= stallThreshold && end < len(comps) {
			e.addLog(job, fmt.Sprintf(
				"XML: ola degradada (%d/%d); renovando conexiones antes de continuar",
				transient,
				len(wave),
			))
			if controller, ok := e.client.(connectionController); ok {
				controller.ResetTransport()
			}
			if err := waitForStage(ctx, xmlRecoveryCooldown); err != nil {
				continue
			}
		}
	}
	return attempts
}

func (e *DownloadEngine) processStage(
	ctx context.Context,
	job *ActiveBatch,
	comps []sunat.Comprobante,
	tipo sunat.TipoDescarga,
	workers int,
	consultacpeAllowed bool,
	retries int,
) []attemptResult {
	work := make(chan sunat.Comprobante, len(comps))
	results := make(chan attemptResult, len(comps))
	for _, comp := range comps {
		work <- comp
	}
	close(work)

	var group sync.WaitGroup
	for range max(1, workers) {
		group.Add(1)
		go func() {
			defer group.Done()
			for comp := range work {
				if ctx.Err() != nil {
					result := failedAttempt(comp, tipo, ctx.Err())
					result.item.Reintentos = retries
					e.recordStageProgress(job, result.item, false)
					e.logTrafficAttempt(job, tipo, comp, retries+1, 0, result)
					results <- result
					continue
				}
				itemCtx := ctx
				cancel := func() {}
				if stageUsesDocumentTimeout(tipo) {
					documentTimeout := job.DocumentTimeout
					if documentTimeout <= 0 {
						documentTimeout = defaultXMLDocumentTimeout
					}
					itemCtx, cancel = context.WithTimeout(ctx, documentTimeout)
				}
				started := time.Now()
				result := e.downloadSingleItem(itemCtx, comp, tipo, consultacpeAllowed)
				duration := time.Since(started)
				cancel()
				result.item.Reintentos = retries
				pending := result.err != nil && isRecoverableAttempt(result.err)
				e.recordStageProgress(job, result.item, pending)
				e.logTrafficAttempt(job, tipo, comp, retries+1, duration, result)
				results <- result
			}
		}()
	}
	go func() {
		group.Wait()
		close(results)
	}()

	collected := make([]attemptResult, 0, len(comps))
	completed := 0
	for result := range results {
		completed++
		collected = append(collected, result)
		e.updateStageMessage(
			job,
			fmt.Sprintf("Etapa %s: %d de %d", tipo, completed, len(comps)),
		)
	}
	return collected
}

// retryXMLTransient recupera pendientes en hasta tres barridos con menor
// concurrencia. Las esperas suceden fuera del pool para no ocupar workers.
func (e *DownloadEngine) retryXMLTransient(
	ctx context.Context,
	job *ActiveBatch,
	attempts []attemptResult,
	consultacpeAllowed bool,
) []attemptResult {
	for sweep := 1; sweep <= maxSweeps; sweep++ {
		pending := filterAttempts(attempts, func(attempt attemptResult) bool {
			return sunat.IsTransientDownloadError(attempt.err)
		})
		if len(pending) == 0 || ctx.Err() != nil {
			break
		}

		e.updateRecoveryMessage(job, fmt.Sprintf(
			"XML: recuperación %d/%d de %d pendientes",
			sweep,
			maxSweeps,
			len(pending),
		))
		recoveryDelay := xmlRecoveryCooldown * time.Duration(1<<(sweep-1))
		for _, attempt := range pending {
			recoveryDelay = max(recoveryDelay, sunat.RetryAfter(attempt.err))
		}
		if deadline, ok := ctx.Deadline(); ok && time.Now().Add(recoveryDelay).After(deadline) {
			e.updateStageMessage(job, "XML: la espera indicada por SUNAT excede el presupuesto del lote")
			break
		}
		if err := waitForStage(ctx, recoveryDelay); err != nil {
			break
		}
		if controller, ok := e.client.(connectionController); ok {
			controller.ResetTransport()
		}
		retried := e.processXMLWaves(
			ctx,
			job,
			attemptComprobantes(pending),
			retryWorkers,
			consultacpeAllowed,
			sweep,
		)
		attempts = replaceAttempts(attempts, retried)
	}
	return attempts
}

func (e *DownloadEngine) updateRecoveryMessage(job *ActiveBatch, message string) {
	job.Mu.Lock()
	if job.Status.Porcentaje >= 100 {
		job.Status.Porcentaje = 99
	}
	job.Status.Mensaje = message
	snapshot := cloneBatchStatus(job.Status)
	job.Mu.Unlock()
	e.broadcastStatus(snapshot)
}

func (e *DownloadEngine) retryUnauthorizedCohort(
	ctx context.Context,
	job *ActiveBatch,
	attempts []attemptResult,
	tipo sunat.TipoDescarga,
	consultacpeAllowed bool,
) []attemptResult {
	unauthorized := filterAttempts(attempts, func(attempt attemptResult) bool {
		status, ok := sunat.HTTPStatus(attempt.err)
		return ok && status == 401
	})
	if len(unauthorized) == 0 {
		return attempts
	}

	e.addLog(job, fmt.Sprintf("Etapa %s: renovando token para %d filas con 401", tipo, len(unauthorized)))
	if err := e.client.RefreshToken(ctx); err != nil {
		e.addLog(job, fmt.Sprintf("Etapa %s: renovacion fallida: %s", tipo, safeResultError(err)))
		return attempts
	}

	comps := attemptComprobantes(unauthorized)
	workers := retryWorkers
	if tipo == sunat.DescargaXML {
		workers = initialWorkers
	}
	retried := e.processStage(ctx, job, comps, tipo, workers, consultacpeAllowed, 1)
	return replaceAttempts(attempts, retried)
}

func (e *DownloadEngine) sweepTransientFailures(
	ctx context.Context,
	job *ActiveBatch,
	attempts []attemptResult,
	tipo sunat.TipoDescarga,
	consultacpeAllowed bool,
) []attemptResult {
	stageSweeps := maxSweeps
	if tipo == sunat.DescargaDescripcion {
		stageSweeps = 2
	}
	for sweep := 1; sweep <= stageSweeps; sweep++ {
		pending := filterAttempts(attempts, func(attempt attemptResult) bool {
			return sunat.IsTransientDownloadError(attempt.err)
		})
		if len(pending) == 0 || ctx.Err() != nil {
			break
		}

		e.addLog(job, fmt.Sprintf("Etapa %s: barrido %d/%d para %d pendientes", tipo, sweep, stageSweeps, len(pending)))
		if err := waitForStage(ctx, sweepCooldown); err != nil {
			break
		}

		retried := e.processStage(
			ctx,
			job,
			attemptComprobantes(pending),
			tipo,
			retryWorkers,
			consultacpeAllowed,
			sweep,
		)
		updated := replaceAttempts(attempts, retried)
		if transientCount(updated) >= len(pending) {
			attempts = updated
			break
		}
		attempts = updated
	}
	return attempts
}

func (e *DownloadEngine) generateFailedPDFs(
	ctx context.Context,
	job *ActiveBatch,
	attempts []attemptResult,
	consultacpeAllowed bool,
) []attemptResult {
	failed := filterAttempts(attempts, func(attempt attemptResult) bool {
		return attempt.err != nil
	})
	if len(failed) == 0 {
		return attempts
	}

	e.addLog(job, fmt.Sprintf("PDF: generando desde XML %d representaciones faltantes", len(failed)))
	generated := make([]attemptResult, 0, len(failed))
	for _, failedAttempt := range failed {
		if ctx.Err() != nil {
			break
		}
		generated = append(
			generated,
			e.generatePDFFromXML(ctx, failedAttempt.item.Comprobante, consultacpeAllowed),
		)
	}
	return replaceAttempts(attempts, generated)
}

func (e *DownloadEngine) generatePDFFromXML(
	ctx context.Context,
	comp sunat.Comprobante,
	consultacpeAllowed bool,
) attemptResult {
	xmlPath, exists := e.fileManager.FindExistingFile(comp, sunat.DescargaXML)
	if !exists {
		xmlAttempt := e.downloadSingleItem(ctx, comp, sunat.DescargaXML, consultacpeAllowed)
		if xmlAttempt.err != nil {
			return failedAttempt(
				comp,
				sunat.DescargaPDF,
				fmt.Errorf("obteniendo XML para generar PDF: %w", xmlAttempt.err),
			)
		}
		xmlPath = xmlAttempt.item.RutaLocal
	}

	xmlData, err := os.ReadFile(xmlPath)
	if err != nil {
		return failedAttempt(comp, sunat.DescargaPDF, fmt.Errorf("leyendo XML local: %w", err))
	}
	if sunat.IsZipContent(xmlData) {
		_, extractedXML, extractErr := sunat.ExtractFileFromZip(xmlData, ".xml")
		if extractErr != nil {
			return failedAttempt(
				comp,
				sunat.DescargaPDF,
				fmt.Errorf("extrayendo XML local del ZIP: %w", extractErr),
			)
		}
		xmlData = extractedXML
	}
	pdfData, err := pdfgen.FromUBL(xmlData)
	if err != nil {
		return failedAttempt(comp, sunat.DescargaPDF, fmt.Errorf("generando PDF desde XML: %w", err))
	}

	file := &sunat.DownloadedFile{
		FileName: fmt.Sprintf(
			"%s-%s-%s-%s.pdf",
			comp.RUC,
			sunat.NormalizeTipo(comp.Tipo),
			comp.Serie,
			comp.Numero,
		),
		ContentType: "application/pdf",
		Content:     pdfData,
	}
	path, err := e.fileManager.SaveDownloadedFile(comp, file, sunat.DescargaPDF)
	if err != nil {
		return failedAttempt(comp, sunat.DescargaPDF, fmt.Errorf("guardando PDF generado: %w", err))
	}

	return attemptResult{item: sunat.ItemResult{
		Comprobante: comp,
		Tipo:        sunat.DescargaPDF,
		Exito:       true,
		NomArchivo:  file.FileName,
		RutaLocal:   path,
		TamanoBytes: int64(len(pdfData)),
		Origen:      "generado desde XML",
	}}
}

func (e *DownloadEngine) recordFinalResult(
	job *ActiveBatch,
	result sunat.ItemResult,
	startedAt time.Time,
) {
	result.Categoria = classifyResult(result)
	job.Mu.Lock()
	replaced := false
	for index, existing := range job.Status.Resultados {
		if itemResultKey(existing) == itemResultKey(result) {
			if existing.Exito != result.Exito {
				if result.Exito {
					job.Status.Exitosos++
					job.Status.Errores--
				} else {
					job.Status.Exitosos--
					job.Status.Errores++
				}
			}
			job.Status.Resultados[index] = result
			replaced = true
			break
		}
	}
	if !replaced {
		job.Status.Procesados++
		if result.Exito {
			job.Status.Exitosos++
		} else {
			job.Status.Errores++
		}
		job.Status.Resultados = append(job.Status.Resultados, result)
	}
	delete(job.PendingRetries, itemResultKey(result))
	refreshFailureBreakdownLocked(job)
	refreshProgressLocked(job, startedAt)
	job.Status.Mensaje = progressMessage(job.Status)
	snapshot := cloneBatchStatus(job.Status)
	job.Mu.Unlock()
	e.broadcastStatus(snapshot)

	if replaced {
		return
	}

	if result.Exito {
		e.addLog(job, fmt.Sprintf("[OK] %s: %s (%s)", result.Tipo, result.NomArchivo, result.Origen))
		return
	}
	e.addLog(job, fmt.Sprintf(
		"[ERROR] %s (%s-%s-%s): %s",
		result.Tipo,
		result.Comprobante.RUC,
		result.Comprobante.Serie,
		result.Comprobante.Numero,
		result.Error,
	))
}

func (e *DownloadEngine) updateStageMessage(job *ActiveBatch, message string) {
	job.Mu.Lock()
	job.Status.Mensaje = message
	snapshot := cloneBatchStatus(job.Status)
	job.Mu.Unlock()
	e.broadcastStatus(snapshot)
}

// recordStageProgress publica exitos y errores apenas termina cada comprobante.
// Si una renovacion posterior recupera un 401, reemplaza el resultado y ajusta
// los contadores sin duplicar el trabajo procesado.
func (e *DownloadEngine) recordStageProgress(
	job *ActiveBatch,
	result sunat.ItemResult,
	pendingRetry bool,
) {
	result.Categoria = classifyResult(result)
	job.Mu.Lock()
	replaced := false
	for index, existing := range job.Status.Resultados {
		if itemResultKey(existing) == itemResultKey(result) {
			if existing.Exito != result.Exito {
				if result.Exito {
					job.Status.Exitosos++
					job.Status.Errores--
				} else {
					job.Status.Exitosos--
					job.Status.Errores++
				}
			}
			job.Status.Resultados[index] = result
			replaced = true
			break
		}
	}
	if !replaced {
		job.Status.Procesados++
		if result.Exito {
			job.Status.Exitosos++
		} else {
			job.Status.Errores++
		}
		job.Status.Resultados = append(job.Status.Resultados, result)
	}
	key := itemResultKey(result)
	if pendingRetry {
		if job.PendingRetries == nil {
			job.PendingRetries = make(map[string]struct{})
		}
		job.PendingRetries[key] = struct{}{}
	} else {
		delete(job.PendingRetries, key)
	}
	refreshFailureBreakdownLocked(job)
	refreshProgressLocked(job, job.Status.IniciadoEn)
	job.Status.Mensaje = progressMessage(job.Status)
	snapshot := cloneBatchStatus(job.Status)
	job.Mu.Unlock()
	e.broadcastStatus(snapshot)
	if !replaced && snapshot.Procesados%manifestCheckpointItems == 0 {
		e.saveManifestCheckpoint(job, snapshot)
	}
	if result.Exito {
		e.addLog(job, fmt.Sprintf("[OK] %s: %s (%s)", result.Tipo, result.NomArchivo, result.Origen))
		return
	}
	e.addLog(job, fmt.Sprintf(
		"[ERROR] %s (%s-%s-%s): %s",
		result.Tipo,
		result.Comprobante.RUC,
		result.Comprobante.Serie,
		result.Comprobante.Numero,
		result.Error,
	))
}

func refreshFailureBreakdownLocked(job *ActiveBatch) {
	job.Status.PendientesReintento = len(job.PendingRetries)
	job.Status.FallidosDefinitivos = max(0, job.Status.Errores-job.Status.PendientesReintento)
}

func refreshProgressLocked(job *ActiveBatch, startedAt time.Time) {
	if job.Status.TotalItems > 0 {
		job.Status.Porcentaje = float64(job.Status.Procesados) / float64(job.Status.TotalItems) * 100
		if job.Status.PendientesReintento > 0 && job.Status.Porcentaje >= 100 {
			job.Status.Porcentaje = 99
		}
	}
	elapsed := time.Since(startedAt).Seconds()
	if elapsed > 0 {
		job.Status.VelocidadItemsSeg = float64(job.Status.Procesados) / elapsed
	}
}

func progressMessage(status sunat.BatchStatus) string {
	return fmt.Sprintf(
		"Procesados %d de %d: %d obtenidos, %d pendientes, %d fallidos definitivos",
		status.Procesados,
		status.TotalItems,
		status.Exitosos,
		status.PendientesReintento,
		status.FallidosDefinitivos,
	)
}

func (e *DownloadEngine) saveManifestCheckpoint(job *ActiveBatch, snapshot sunat.BatchStatus) {
	manifestPath, err := e.fileManager.SaveBatchManifest(snapshot)
	if err != nil {
		e.addLog(job, fmt.Sprintf("[ERROR] No se pudo guardar el checkpoint del lote: %v", err))
		return
	}
	job.Mu.Lock()
	job.Status.ManifestPath = manifestPath
	job.Mu.Unlock()
}

func itemResultKey(result sunat.ItemResult) string {
	return comprobanteKey(result.Comprobante) + "|" + string(result.Tipo)
}

func classifyResult(result sunat.ItemResult) string {
	if result.Exito {
		if strings.EqualFold(strings.TrimSpace(result.Origen), "archivo existente") {
			return categoryExisting
		}
		return categoryDownloaded
	}
	errorText := strings.ToLower(strings.TrimSpace(result.Error))
	switch {
	case strings.Contains(errorText, `"coderror":"301"`),
		strings.Contains(errorText, "no se encontro el xml"),
		strings.Contains(errorText, "no se encontró el xml"),
		strings.Contains(errorText, "sin cdr en sunat"),
		strings.Contains(errorText, "http 404"):
		return categoryUnavailable
	case strings.Contains(errorText, `"coderror":"302"`),
		strings.Contains(errorText, "consulta invalida"),
		strings.Contains(errorText, "consulta inválida"),
		strings.Contains(errorText, "http 422"),
		strings.Contains(errorText, "http 400"),
		strings.Contains(errorText, "http 403"):
		return categoryUnsupported
	default:
		return categoryRecoverable
	}
}

func (e *DownloadEngine) finishBatch(ctx context.Context, job *ActiveBatch, startedAt time.Time) {
	job.Mu.Lock()
	now := time.Now()
	for key := range job.PendingRetries {
		delete(job.PendingRetries, key)
	}
	refreshFailureBreakdownLocked(job)
	job.Status.FinalizadoEn = &now
	job.Status.TiempoRestanteEstimado = "0s"
	job.Status.HilosActivos = 0
	if job.Status.TotalItems > 0 {
		job.Status.Porcentaje = float64(job.Status.Procesados) / float64(job.Status.TotalItems) * 100
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		job.Status.Estado = "detenido"
		job.Status.Mensaje = fmt.Sprintf(
			"Descarga detenida: %d exitosos, %d fallidos de %d procesados",
			job.Status.Exitosos,
			job.Status.Errores,
			job.Status.Procesados,
		)
	} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		job.Status.Estado = "completado"
		job.Status.Mensaje = fmt.Sprintf(
			"Lote cerrado por presupuesto en %s: %d exitosos, %d pendientes o fallidos",
			time.Since(startedAt).Round(time.Second),
			job.Status.Exitosos,
			job.Status.Errores,
		)
	} else {
		job.Status.Estado = "completado"
		job.Status.Mensaje = fmt.Sprintf(
			"Lote completado en %s: %d exitosos, %d fallidos (Promedio: %.1f docs/seg)",
			time.Since(startedAt).Round(time.Second),
			job.Status.Exitosos,
			job.Status.Errores,
			job.Status.VelocidadItemsSeg,
		)
	}
	traffic := append([]string{}, job.Traffic...)
	finalStatus := cloneBatchStatus(job.Status)
	job.Mu.Unlock()
	trafficPath, trafficErr := e.fileManager.SaveBatchTrafficLog(finalStatus.BatchID, traffic)
	if trafficErr != nil {
		e.addLog(job, fmt.Sprintf("[ERROR] No se pudo guardar la traza del lote: %v", trafficErr))
	} else {
		job.Mu.Lock()
		job.Status.TrafficLogPath = trafficPath
		finalStatus = cloneBatchStatus(job.Status)
		job.Mu.Unlock()
	}
	manifestPath, manifestErr := e.fileManager.SaveBatchManifest(finalStatus)
	if manifestErr != nil {
		e.addLog(job, fmt.Sprintf("[ERROR] No se pudo guardar el manifiesto del lote: %v", manifestErr))
	} else {
		job.Mu.Lock()
		job.Status.ManifestPath = manifestPath
		finalStatus = cloneBatchStatus(job.Status)
		job.Mu.Unlock()
	}
	e.addLog(job, finalStatus.Mensaje)
	e.broadcastStatus(finalStatus)
}

func waitForStage(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// downloadSingleItem ejecuta la descarga y enriquece con metadatos tributarios (CDR, Digest)
func (e *DownloadEngine) downloadSingleItem(
	ctx context.Context,
	comp sunat.Comprobante,
	tipo sunat.TipoDescarga,
	consultacpeAllowed bool,
) attemptResult {
	if tipo == sunat.DescargaDescripcion {
		return e.downloadDescription(ctx, comp, consultacpeAllowed)
	}
	if existingPath, ok := e.fileManager.FindExistingFile(comp, tipo); ok {
		info, err := os.Stat(existingPath)
		if err == nil {
			return attemptResult{item: sunat.ItemResult{
				Comprobante: comp,
				Tipo:        tipo,
				Exito:       true,
				NomArchivo:  filepath.Base(existingPath),
				RutaLocal:   existingPath,
				TamanoBytes: info.Size(),
				Origen:      "archivo existente",
			}}
		}
	}

	var file *sunat.DownloadedFile
	var err error

	switch tipo {
	case sunat.DescargaPDF:
		file, err = e.client.DownloadPDF(ctx, comp)
	case sunat.DescargaXML:
		if consultacpeAllowed {
			file, err = e.client.DownloadXML(ctx, comp)
		} else {
			file, err = e.client.DownloadXMLFallback(ctx, comp)
		}
	case sunat.DescargaCDR:
		file, err = e.client.DownloadCDR(ctx, comp)
	default:
		err = fmt.Errorf("tipo de descarga desconocido: %s", tipo)
	}

	if err != nil {
		return failedAttempt(comp, tipo, err)
	}

	// Validación estricta de Magic Bytes / Detección de Falsos 200
	if integrityErr := sunat.ValidateContentIntegrity(file.Content, tipo); integrityErr != nil {
		return failedAttempt(comp, tipo, integrityErr)
	}

	// Enriquecimiento de datos tributarios al vuelo (Zero-Disk)
	var estadoCDR, codigoCDR, mensajeCDR, digestVal string

	if tipo == sunat.DescargaXML && !file.IsZip {
		digestVal = sunat.ExtractDigestValue(file.Content)
	} else if tipo == sunat.DescargaCDR {
		var cdrXmlBytes []byte
		if file.IsZip {
			_, innerXml, extractErr := sunat.ExtractFileFromZip(file.Content, ".xml")
			if extractErr == nil {
				cdrXmlBytes = innerXml
			}
		} else {
			cdrXmlBytes = file.Content
		}

		if len(cdrXmlBytes) > 0 {
			cdrInfo := sunat.InspectCDR(cdrXmlBytes)
			estadoCDR = cdrInfo.Estado
			codigoCDR = cdrInfo.ResponseCode
			mensajeCDR = cdrInfo.Description
		}
	}

	// Guardar en disco organizado
	savedPath, saveErr := e.fileManager.SaveDownloadedFile(comp, file, tipo)
	if saveErr != nil {
		return failedAttempt(comp, tipo, fmt.Errorf("guardando archivo: %w", saveErr))
	}

	return attemptResult{item: sunat.ItemResult{
		Comprobante: comp,
		Tipo:        tipo,
		Exito:       true,
		NomArchivo:  file.FileName,
		RutaLocal:   savedPath,
		TamanoBytes: int64(len(file.Content)),
		EstadoCDR:   estadoCDR,
		CodigoCDR:   codigoCDR,
		MensajeCDR:  mensajeCDR,
		DigestValue: digestVal,
		Origen:      "descargado de SUNAT",
	}}
}

func (e *DownloadEngine) downloadDescription(
	ctx context.Context,
	comp sunat.Comprobante,
	consultacpeAllowed bool,
) attemptResult {
	var xmlErr error
	if xmlPath, ok := e.fileManager.FindExistingFile(comp, sunat.DescargaXML); ok {
		if xmlData, err := os.ReadFile(xmlPath); err == nil {
			if sunat.IsZipContent(xmlData) {
				if _, extractedXML, extractErr := sunat.ExtractFileFromZip(xmlData, ".xml"); extractErr == nil {
					xmlData = extractedXML
				} else {
					xmlErr = fmt.Errorf("leyendo ZIP XML local: %w", extractErr)
				}
			}
			if extracted, extractErr := sunat.ExtractUBLDescription(xmlData); extractErr == nil {
				return descriptionAttempt(
					comp,
					extracted,
					"extraído del XML local",
					filepath.Base(xmlPath),
					xmlPath,
				)
			} else {
				xmlErr = fmt.Errorf("interpretando XML local: %w", extractErr)
			}
		} else {
			xmlErr = fmt.Errorf("leyendo XML local: %w", err)
		}
	}

	var xmlFile *sunat.DownloadedFile
	var err error
	if consultacpeAllowed {
		xmlFile, err = e.client.DownloadXML(ctx, comp)
	} else {
		xmlFile, err = e.client.DownloadXMLFallback(ctx, comp)
	}
	if err == nil {
		if integrityErr := sunat.ValidateContentIntegrity(xmlFile.Content, sunat.DescargaXML); integrityErr != nil {
			xmlErr = integrityErr
		} else if savedPath, saveErr := e.fileManager.SaveDownloadedFile(comp, xmlFile, sunat.DescargaXML); saveErr != nil {
			xmlErr = fmt.Errorf("guardando XML para descripción: %w", saveErr)
		} else if extracted, extractErr := sunat.ExtractUBLDescription(xmlFile.Content); extractErr != nil {
			xmlErr = fmt.Errorf("interpretando XML descargado: %w", extractErr)
		} else {
			return descriptionAttempt(
				comp,
				extracted,
				"extraído del XML descargado",
				xmlFile.FileName,
				savedPath,
			)
		}
	} else {
		xmlErr = err
	}

	if !consultacpeAllowed {
		return failedAttempt(
			comp,
			sunat.DescargaDescripcion,
			fmt.Errorf("no se pudo obtener el XML para extraer la descripción: %w", xmlErr),
		)
	}

	direct, err := e.client.DownloadDescription(ctx, comp)
	if err != nil {
		if xmlErr != nil && isRecoverableAttempt(xmlErr) && !isRecoverableAttempt(err) {
			return failedAttempt(
				comp,
				sunat.DescargaDescripcion,
				fmt.Errorf("obteniendo XML para descripción: %w", xmlErr),
			)
		}
		return failedAttempt(comp, sunat.DescargaDescripcion, err)
	}
	return descriptionAttempt(comp, direct, "consulta JSON de respaldo", "", "")
}

func descriptionAttempt(
	comp sunat.Comprobante,
	result sunat.DescriptionResult,
	origin string,
	xmlName string,
	xmlPath string,
) attemptResult {
	return attemptResult{item: sunat.ItemResult{
		Comprobante: comp,
		Tipo:        sunat.DescargaDescripcion,
		Exito:       true,
		Descripcion: result.Description,
		Placa:       result.VehiclePlate,
		Origen:      origin,
		XMLNombre:   xmlName,
		XMLRuta:     xmlPath,
	}}
}

func failedAttempt(comp sunat.Comprobante, tipo sunat.TipoDescarga, err error) attemptResult {
	return attemptResult{
		item: sunat.ItemResult{
			Comprobante: comp,
			Tipo:        tipo,
			Exito:       false,
			Error:       safeResultError(err),
		},
		err: err,
	}
}

func safeResultError(err error) string {
	message := strings.TrimSpace(err.Error())
	lower := strings.ToLower(message)
	if strings.Contains(lower, "valarchivo") ||
		strings.Contains(lower, "access_token") ||
		strings.Contains(lower, "authorization") ||
		strings.Contains(lower, "bearer ") ||
		strings.Contains(lower, "client_secret") ||
		strings.Contains(lower, "clavesol") ||
		containsLongEncodedValue(message) {
		if status, ok := sunat.HTTPStatus(err); ok {
			return fmt.Sprintf("SUNAT respondió HTTP %d (detalle omitido por seguridad)", status)
		}
		return "detalle técnico omitido por seguridad"
	}
	const maxErrorLength = 800
	if len(message) > maxErrorLength {
		return message[:maxErrorLength] + "..."
	}
	return message
}

func containsLongEncodedValue(value string) bool {
	run := 0
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '+' || char == '/' || char == '=' {
			run++
			if run >= 80 {
				return true
			}
			continue
		}
		run = 0
	}
	return false
}

func blockedResults(
	comps []sunat.Comprobante,
	tipo sunat.TipoDescarga,
	message string,
) []sunat.ItemResult {
	results := make([]sunat.ItemResult, 0, len(comps))
	for _, comp := range comps {
		results = append(results, sunat.ItemResult{
			Comprobante: comp,
			Tipo:        tipo,
			Exito:       false,
			Error:       message,
		})
	}
	return results
}

func selectedTypes(types []sunat.TipoDescarga) map[sunat.TipoDescarga]bool {
	selected := make(map[sunat.TipoDescarga]bool, len(types))
	for _, tipo := range types {
		selected[tipo] = true
	}
	return selected
}

func eligibleForStage(
	comps []sunat.Comprobante,
	tipo sunat.TipoDescarga,
) []sunat.Comprobante {
	eligible := make([]sunat.Comprobante, 0, len(comps))
	for _, comp := range comps {
		if tipo == sunat.DescargaCDR && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(comp.Serie)), "E") {
			continue
		}
		eligible = append(eligible, comp)
	}
	return eligible
}

func expectedItemCount(comps []sunat.Comprobante, types []sunat.TipoDescarga) int {
	total := 0
	for _, tipo := range types {
		total += len(eligibleForStage(comps, tipo))
	}
	return total
}

func filterAttempts(
	attempts []attemptResult,
	keep func(attemptResult) bool,
) []attemptResult {
	filtered := make([]attemptResult, 0, len(attempts))
	for _, attempt := range attempts {
		if keep(attempt) {
			filtered = append(filtered, attempt)
		}
	}
	return filtered
}

func attemptComprobantes(attempts []attemptResult) []sunat.Comprobante {
	comps := make([]sunat.Comprobante, 0, len(attempts))
	for _, attempt := range attempts {
		comps = append(comps, attempt.item.Comprobante)
	}
	return comps
}

func replaceAttempts(current, replacements []attemptResult) []attemptResult {
	byKey := make(map[string]attemptResult, len(replacements))
	for _, replacement := range replacements {
		byKey[comprobanteKey(replacement.item.Comprobante)] = replacement
	}
	for index, attempt := range current {
		if replacement, ok := byKey[comprobanteKey(attempt.item.Comprobante)]; ok {
			current[index] = replacement
		}
	}
	return current
}

func transientCount(attempts []attemptResult) int {
	return len(filterAttempts(attempts, func(attempt attemptResult) bool {
		return sunat.IsTransientDownloadError(attempt.err)
	}))
}

func comprobanteKey(comp sunat.Comprobante) string {
	return strings.Join(
		[]string{comp.RUC, comp.Tipo, comp.Serie, comp.Numero, fmt.Sprint(comp.RowIndex)},
		"|",
	)
}

// CancelCurrentBatch detiene el lote activo
func (e *DownloadEngine) CancelCurrentBatch() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.currentJob == nil {
		return fmt.Errorf("no hay lote activo")
	}

	e.currentJob.CancelFunc()
	return nil
}

// GetCurrentStatus obtiene el estado actual
func (e *DownloadEngine) GetCurrentStatus() (sunat.BatchStatus, []string) {
	e.mu.RLock()
	job := e.currentJob
	e.mu.RUnlock()

	if job == nil {
		return sunat.BatchStatus{
			Estado:  "inactivo",
			Mensaje: "Sin descargas recientes",
		}, nil
	}

	job.Mu.RLock()
	defer job.Mu.RUnlock()

	logsCopy := make([]string, len(job.Logs))
	copy(logsCopy, job.Logs)

	return cloneBatchStatus(job.Status), logsCopy
}

func cloneBatchStatus(status sunat.BatchStatus) sunat.BatchStatus {
	clone := status
	clone.Resultados = append([]sunat.ItemResult{}, status.Resultados...)
	clone.ResumenCategorias = make(map[string]int, 5)
	for index := range clone.Resultados {
		clone.Resultados[index].Categoria = classifyResult(clone.Resultados[index])
		clone.ResumenCategorias[clone.Resultados[index].Categoria]++
	}
	return clone
}

func (e *DownloadEngine) addLog(job *ActiveBatch, msg string) {
	job.Mu.Lock()
	defer job.Mu.Unlock()
	timestamp := time.Now().Format("15:04:05")
	job.Logs = append(job.Logs, fmt.Sprintf("[%s] %s", timestamp, msg))
	if len(job.Logs) > 500 {
		job.Logs = job.Logs[len(job.Logs)-500:]
	}
}

func (e *DownloadEngine) logTrafficAttempt(
	job *ActiveBatch,
	tipo sunat.TipoDescarga,
	comp sunat.Comprobante,
	attempt int,
	duration time.Duration,
	result attemptResult,
) {
	status := "OK"
	classification := "completado"
	if result.err == nil {
		switch {
		case strings.EqualFold(result.item.Origen, "archivo existente"):
			status = "local"
			classification = "reutilizado"
		case tipo == sunat.DescargaCDR && strings.EqualFold(filepath.Ext(result.item.NomArchivo), ".zip"):
			classification = "ZIP válido"
		case tipo == sunat.DescargaCDR:
			classification = "CDR XML válido"
		}
	} else if code, ok := sunat.HTTPStatus(result.err); ok {
		status = fmt.Sprintf("HTTP %d", code)
		if isRecoverableAttempt(result.err) {
			classification = "transitorio"
		} else {
			classification = "definitivo"
		}
	} else if errors.Is(result.err, context.DeadlineExceeded) {
		status = "timeout"
		classification = "pendiente"
	} else if errors.Is(result.err, context.Canceled) {
		status = "cancelado"
		classification = "definitivo"
	} else if sunat.IsTransientDownloadError(result.err) {
		status = "conexión"
		classification = "transitorio"
	} else {
		status = "error"
		classification = "definitivo"
	}

	line := fmt.Sprintf(
		"%s %s-%s | intento %d | %s | %s | %s",
		tipo,
		strings.ToUpper(strings.TrimSpace(comp.Serie)),
		sunat.NormalizeNumero(comp.Numero),
		attempt,
		status,
		formatAttemptDuration(duration),
		classification,
	)
	timestamped := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), line)
	job.Mu.Lock()
	job.Traffic = append(job.Traffic, timestamped)
	job.Logs = append(job.Logs, timestamped)
	if len(job.Logs) > 500 {
		job.Logs = job.Logs[len(job.Logs)-500:]
	}
	job.Mu.Unlock()
}

func stageUsesDocumentTimeout(tipo sunat.TipoDescarga) bool {
	return tipo == sunat.DescargaXML || tipo == sunat.DescargaCDR || tipo == sunat.DescargaDescripcion
}

func isRecoverableAttempt(err error) bool {
	if status, ok := sunat.HTTPStatus(err); ok && status == http.StatusUnauthorized {
		return true
	}
	return sunat.IsTransientDownloadError(err)
}

func formatAttemptDuration(duration time.Duration) string {
	if duration >= time.Second {
		return fmt.Sprintf("%.1f s", duration.Seconds())
	}
	return fmt.Sprintf("%d ms", duration.Milliseconds())
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if m > 0 {
		return fmt.Sprintf("%dm %02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
