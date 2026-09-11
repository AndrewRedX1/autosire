package engine

import (
	"context"
	"errors"
	"fmt"
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
	initialWorkers = 6
	retryWorkers   = 3
	maxSweeps      = 3
	maxXMLSweeps   = 4
	sweepCooldown  = 2500 * time.Millisecond
)

type downloadClient interface {
	DownloadPDF(context.Context, sunat.Comprobante) (*sunat.DownloadedFile, error)
	DownloadXML(context.Context, sunat.Comprobante) (*sunat.DownloadedFile, error)
	DownloadXMLFallback(context.Context, sunat.Comprobante) (*sunat.DownloadedFile, error)
	DownloadCDR(context.Context, sunat.Comprobante) (*sunat.DownloadedFile, error)
	ProbeConsultacpe(context.Context, sunat.Comprobante) error
	RefreshToken(context.Context) error
}

// DownloadRequest opciones para lanzar un lote de descargas
type DownloadRequest struct {
	Comprobantes []sunat.Comprobante  `json:"comprobantes"`
	Tipos        []sunat.TipoDescarga `json:"tipos"` // PDF, XML, CDR
	Concurrency  int                  `json:"concurrency"`
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
	Status     sunat.BatchStatus
	CancelFunc context.CancelFunc
	Mu         sync.RWMutex
	Logs       []string
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
		return "", fmt.Errorf("debe seleccionar al menos un tipo de archivo (PDF, XML o CDR)")
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

	batchID := fmt.Sprintf("batch-%d", time.Now().Unix())
	totalWorkItems := expectedItemCount(req.Comprobantes, req.Tipos)

	ctx, cancel := context.WithCancel(context.Background())

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
		CancelFunc: cancel,
		Logs:       make([]string, 0, 100),
	}

	e.currentJob = active

	// Lanzar ejecución en goroutine desacoplada
	go e.runBatch(ctx, active, req, concurrency)

	return batchID, nil
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
	if tipo != sunat.DescargaPDF && tipo != sunat.DescargaXML && tipo != sunat.DescargaCDR {
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

// runBatch replica la estrategia de la macro: formatos aislados, token por
// cohorte, menor concurrencia en recuperaciones y PDF local desde el XML.
func (e *DownloadEngine) runBatch(
	ctx context.Context,
	job *ActiveBatch,
	req DownloadRequest,
	concurrency int,
) {
	startedAt := time.Now()
	workers := min(concurrency, initialWorkers)
	e.addLog(job, fmt.Sprintf("Motor por etapas iniciado con %d hilos", workers))

	selected := selectedTypes(req.Tipos)
	consultacpeAllowed := true
	// La macro XML no hace una descarga de prueba: inicia directamente sus seis
	// workers. Evitamos pedir y descartar el primer XML antes del lote.
	if len(selected) != 1 || !selected[sunat.DescargaXML] {
		consultacpeAllowed = e.preflightConsultacpe(ctx, job, req.Comprobantes)
	}
	stages := []sunat.TipoDescarga{sunat.DescargaXML, sunat.DescargaPDF, sunat.DescargaCDR}
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
		if !selected[stage] || ctx.Err() != nil {
			continue
		}

		comps := eligibleForStage(req.Comprobantes, stage)
		e.updateStageMessage(job, fmt.Sprintf("Etapa %s: preparando %d comprobantes", stage, len(comps)))
		e.addLog(job, fmt.Sprintf("Etapa %s iniciada: %d comprobantes", stage, len(comps)))

		results := e.runStage(
			ctx,
			job,
			comps,
			stage,
			workers,
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

	if status, ok := sunat.HTTPStatus(err); !ok || status != 401 {
		e.addLog(job, fmt.Sprintf("Preflight consultacpe no concluyente: %v", err))
		return true
	}

	e.addLog(job, "Preflight consultacpe recibio 401; renovando token una sola vez")
	if refreshErr := e.client.RefreshToken(ctx); refreshErr != nil {
		e.addLog(job, fmt.Sprintf("No se pudo renovar el token del lote: %v", refreshErr))
		return false
	}
	if retryErr := e.client.ProbeConsultacpe(ctx, comps[0]); retryErr == nil {
		e.addLog(job, "Preflight consultacpe autorizado despues de renovar")
		return true
	} else if status, ok := sunat.HTTPStatus(retryErr); !ok || status != 401 {
		e.addLog(job, fmt.Sprintf("Preflight consultacpe no concluyente despues de renovar: %v", retryErr))
		return true
	}

	e.addLog(job, "consultacpe no autorizado: XML usara controlcpe y se evitara repetir 401")
	return false
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
	if tipo == sunat.DescargaPDF && !consultacpeAllowed {
		attempts := make([]attemptResult, 0, len(comps))
		for _, comp := range comps {
			attempts = append(
				attempts,
				failedAttempt(comp, tipo, errors.New("consultacpe no autorizado")),
			)
		}
		return attemptItems(e.generateFailedPDFs(ctx, job, attempts, false))
	}

	attempts := e.processStage(ctx, job, comps, tipo, workers, consultacpeAllowed)
	attempts = e.retryUnauthorizedCohort(ctx, job, attempts, tipo, consultacpeAllowed)
	attempts = e.sweepTransientFailures(ctx, job, attempts, tipo, consultacpeAllowed)

	if tipo == sunat.DescargaPDF {
		attempts = e.generateFailedPDFs(ctx, job, attempts, consultacpeAllowed)
	}

	results := make([]sunat.ItemResult, 0, len(attempts))
	for _, attempt := range attempts {
		results = append(results, attempt.item)
	}
	return results
}

func attemptItems(attempts []attemptResult) []sunat.ItemResult {
	items := make([]sunat.ItemResult, 0, len(attempts))
	for _, attempt := range attempts {
		items = append(items, attempt.item)
	}
	return items
}

func (e *DownloadEngine) processStage(
	ctx context.Context,
	job *ActiveBatch,
	comps []sunat.Comprobante,
	tipo sunat.TipoDescarga,
	workers int,
	consultacpeAllowed bool,
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
				result := e.downloadSingleItem(ctx, comp, tipo, consultacpeAllowed)
				if result.err == nil {
					e.recordSuccessfulProgress(job, result.item)
				}
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
		e.addLog(job, fmt.Sprintf("Etapa %s: renovacion fallida: %v", tipo, err))
		return attempts
	}

	comps := attemptComprobantes(unauthorized)
	workers := retryWorkers
	if tipo == sunat.DescargaXML {
		workers = initialWorkers
	}
	retried := e.processStage(ctx, job, comps, tipo, workers, consultacpeAllowed)
	return replaceAttempts(attempts, retried)
}

func (e *DownloadEngine) sweepTransientFailures(
	ctx context.Context,
	job *ActiveBatch,
	attempts []attemptResult,
	tipo sunat.TipoDescarga,
	consultacpeAllowed bool,
) []attemptResult {
	sweeps := maxSweeps
	if tipo == sunat.DescargaXML {
		sweeps = maxXMLSweeps
	}
	for sweep := 1; sweep <= sweeps; sweep++ {
		pending := filterAttempts(attempts, func(attempt attemptResult) bool {
			return sunat.IsTransientDownloadError(attempt.err)
		})
		if len(pending) == 0 || ctx.Err() != nil {
			break
		}

		e.addLog(job, fmt.Sprintf("Etapa %s: barrido %d/%d para %d pendientes", tipo, sweep, sweeps, len(pending)))
		if err := waitForStage(ctx, sweepCooldown); err != nil {
			break
		}

		workers := retryWorkers
		if tipo == sunat.DescargaXML {
			workers = initialWorkers
		}
		retried := e.processStage(
			ctx,
			job,
			attemptComprobantes(pending),
			tipo,
			workers,
			consultacpeAllowed,
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
	job.Mu.Lock()
	for index, existing := range job.Status.Resultados {
		if itemResultKey(existing) == itemResultKey(result) {
			job.Status.Resultados[index] = result
			job.Mu.Unlock()
			return
		}
	}
	job.Status.Procesados++
	if result.Exito {
		job.Status.Exitosos++
	} else {
		job.Status.Errores++
	}
	job.Status.Resultados = append(job.Status.Resultados, result)
	job.Status.Porcentaje = float64(job.Status.Procesados) / float64(job.Status.TotalItems) * 100

	elapsed := time.Since(startedAt).Seconds()
	if elapsed > 0 {
		job.Status.VelocidadItemsSeg = float64(job.Status.Procesados) / elapsed
	}
	job.Status.Mensaje = fmt.Sprintf(
		"Procesados %d de %d: %d exitosos, %d fallidos",
		job.Status.Procesados,
		job.Status.TotalItems,
		job.Status.Exitosos,
		job.Status.Errores,
	)
	snapshot := cloneBatchStatus(job.Status)
	job.Mu.Unlock()
	e.broadcastStatus(snapshot)

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

// recordSuccessfulProgress hace visible cada XML apenas se guarda, sin esperar
// a que termine el lote ni a los barridos de las filas fallidas.
func (e *DownloadEngine) recordSuccessfulProgress(job *ActiveBatch, result sunat.ItemResult) {
	job.Mu.Lock()
	for _, existing := range job.Status.Resultados {
		if itemResultKey(existing) == itemResultKey(result) {
			job.Mu.Unlock()
			return
		}
	}
	job.Status.Procesados++
	job.Status.Exitosos++
	job.Status.Resultados = append(job.Status.Resultados, result)
	job.Status.Porcentaje = float64(job.Status.Procesados) / float64(job.Status.TotalItems) * 100
	snapshot := cloneBatchStatus(job.Status)
	job.Mu.Unlock()
	e.broadcastStatus(snapshot)
}

func itemResultKey(result sunat.ItemResult) string {
	return comprobanteKey(result.Comprobante) + "|" + string(result.Tipo)
}

func (e *DownloadEngine) finishBatch(ctx context.Context, job *ActiveBatch, startedAt time.Time) {
	job.Mu.Lock()
	now := time.Now()
	job.Status.FinalizadoEn = &now
	job.Status.TiempoRestanteEstimado = "0s"
	if ctx.Err() != nil {
		job.Status.Estado = "detenido"
		job.Status.Mensaje = "Descarga detenida por el usuario"
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
	finalStatus := cloneBatchStatus(job.Status)
	job.Mu.Unlock()
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

func failedAttempt(comp sunat.Comprobante, tipo sunat.TipoDescarga, err error) attemptResult {
	return attemptResult{
		item: sunat.ItemResult{
			Comprobante: comp,
			Tipo:        tipo,
			Exito:       false,
			Error:       err.Error(),
		},
		err: err,
	}
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
