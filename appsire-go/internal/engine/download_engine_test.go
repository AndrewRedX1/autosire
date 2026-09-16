package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"appsire-go/internal/filemanager"
	"appsire-go/internal/sunat"
)

type recordingClient struct {
	mu                 sync.Mutex
	calls              []sunat.TipoDescarga
	probeErr           error
	fallbackProbeErr   error
	fallbackProbes     int
	refreshes          int
	primaryXML         int
	primaryXMLFailures int
	blockXML           bool
	fallbackXML        int
	pdfDownloads       int
	cdrDownloads       int
	transportResets    int
	idleCloses         int
}

func (c *recordingClient) DownloadPDF(
	context.Context,
	sunat.Comprobante,
) (*sunat.DownloadedFile, error) {
	c.record(sunat.DescargaPDF)
	c.mu.Lock()
	c.pdfDownloads++
	c.mu.Unlock()
	return &sunat.DownloadedFile{
		FileName: "documento.pdf",
		Content:  []byte("%PDF-1.4\n%%EOF"),
	}, nil
}

func (c *recordingClient) DownloadXML(
	ctx context.Context,
	comp sunat.Comprobante,
) (*sunat.DownloadedFile, error) {
	c.record(sunat.DescargaXML)
	c.mu.Lock()
	c.primaryXML++
	if c.blockXML {
		c.mu.Unlock()
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if c.primaryXMLFailures > 0 {
		c.primaryXMLFailures--
		c.mu.Unlock()
		return nil, &sunat.HTTPStatusError{StatusCode: 503, Body: "temporal"}
	}
	c.mu.Unlock()
	return xmlFile(comp), nil
}

func (c *recordingClient) DownloadXMLFallback(
	_ context.Context,
	comp sunat.Comprobante,
) (*sunat.DownloadedFile, error) {
	c.record(sunat.DescargaXML)
	c.mu.Lock()
	c.fallbackXML++
	c.mu.Unlock()
	return xmlFile(comp), nil
}

func (c *recordingClient) ProbeXMLFallback(context.Context, sunat.Comprobante) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fallbackProbes++
	return c.fallbackProbeErr
}

func (c *recordingClient) DownloadCDR(
	context.Context,
	sunat.Comprobante,
) (*sunat.DownloadedFile, error) {
	c.record(sunat.DescargaCDR)
	c.mu.Lock()
	c.cdrDownloads++
	c.mu.Unlock()
	return &sunat.DownloadedFile{
		FileName: "R-documento.xml",
		Content:  []byte("<?xml version=\"1.0\"?><ApplicationResponse/>"),
	}, nil
}

func (c *recordingClient) ProbeConsultacpe(context.Context, sunat.Comprobante) error {
	return c.probeErr
}

func (c *recordingClient) RefreshToken(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshes++
	return nil
}

func (c *recordingClient) ResetTransport() {
	c.mu.Lock()
	c.transportResets++
	c.mu.Unlock()
}

func (c *recordingClient) CloseIdleConnections() {
	c.mu.Lock()
	c.idleCloses++
	c.mu.Unlock()
}

func (c *recordingClient) record(tipo sunat.TipoDescarga) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, tipo)
}

func xmlFile(comp sunat.Comprobante) *sunat.DownloadedFile {
	return &sunat.DownloadedFile{
		FileName: comp.RUC + "-" + comp.Tipo + "-" + comp.Serie + "-" + comp.Numero + ".xml",
		Content: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
 xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
 <cbc:ID>` + comp.Serie + `-` + comp.Numero + `</cbc:ID>
</Invoice>`),
	}
}

func TestPrepareComprobanteNormalizesUploadedPurchase(t *testing.T) {
	t.Parallel()

	got := prepareComprobante(sunat.Comprobante{
		Numero:    "10,685",
		Libro:     "1",
		SheetName: "Comprobantes",
	})

	if got.Numero != "10685" {
		t.Fatalf("Numero = %q, want %q", got.Numero, "10685")
	}
	if got.Libro != "2" {
		t.Fatalf("Libro = %q, want %q", got.Libro, "2")
	}
}

func TestRunBatchProcessesFormatsInStages(t *testing.T) {
	t.Parallel()

	client := &recordingClient{}
	manager := filemanager.NewFileManager(t.TempDir())
	downloadEngine := NewDownloadEngine(client, manager)
	comps := []sunat.Comprobante{
		{RUC: "20111111111", Tipo: "01", Serie: "F001", Numero: "1", Libro: "2"},
		{RUC: "20222222222", Tipo: "01", Serie: "F001", Numero: "2", Libro: "2"},
	}
	job := testBatch(6)

	downloadEngine.runBatch(t.Context(), job, DownloadRequest{
		Comprobantes: comps,
		Tipos: []sunat.TipoDescarga{
			sunat.DescargaPDF,
			sunat.DescargaCDR,
			sunat.DescargaXML,
		},
		Concurrency: 6,
	}, 6)

	client.mu.Lock()
	calls := append([]sunat.TipoDescarga{}, client.calls...)
	client.mu.Unlock()
	want := []sunat.TipoDescarga{
		sunat.DescargaXML,
		sunat.DescargaXML,
		sunat.DescargaPDF,
		sunat.DescargaPDF,
		sunat.DescargaCDR,
		sunat.DescargaCDR,
	}
	if len(calls) != len(want) {
		t.Fatalf("llamadas = %v, want %v", calls, want)
	}
	for index := range want {
		if calls[index] != want[index] {
			t.Fatalf("llamadas = %v, want orden por etapas %v", calls, want)
		}
	}
}

func TestRunBatchAvoidsUnauthorizedConsultacpeFanout(t *testing.T) {
	t.Parallel()

	client := &recordingClient{probeErr: &sunat.HTTPStatusError{
		StatusCode: 401,
		Body:       `{"status":401,"message":"Unauthorized"}`,
	}}
	downloadEngine := NewDownloadEngine(client, filemanager.NewFileManager(t.TempDir()))
	comp := sunat.Comprobante{
		RUC: "20111111111", Tipo: "01", Serie: "F001", Numero: "10", Libro: "2",
	}
	job := testBatch(3)

	downloadEngine.runBatch(t.Context(), job, DownloadRequest{
		Comprobantes: []sunat.Comprobante{comp},
		Tipos: []sunat.TipoDescarga{
			sunat.DescargaXML,
			sunat.DescargaPDF,
			sunat.DescargaCDR,
		},
	}, 6)

	client.mu.Lock()
	refreshes := client.refreshes
	primaryXML := client.primaryXML
	fallbackXML := client.fallbackXML
	pdfDownloads := client.pdfDownloads
	cdrDownloads := client.cdrDownloads
	client.mu.Unlock()

	if refreshes != 1 {
		t.Fatalf("renovaciones = %d, want 1 por lote", refreshes)
	}
	if primaryXML != 0 || fallbackXML != 1 {
		t.Fatalf("XML primario/respaldo = %d/%d, want 0/1", primaryXML, fallbackXML)
	}
	if pdfDownloads != 0 || cdrDownloads != 0 {
		t.Fatalf("consultas PDF/CDR = %d/%d, want 0/0", pdfDownloads, cdrDownloads)
	}

	job.Mu.RLock()
	defer job.Mu.RUnlock()
	if job.Status.Exitosos != 2 || job.Status.Errores != 1 {
		t.Fatalf(
			"resultado exitosos/errores = %d/%d, want 2/1",
			job.Status.Exitosos,
			job.Status.Errores,
		)
	}
	if job.Status.Resultados[1].Origen != "generado desde XML" {
		t.Fatalf("origen PDF = %q, want generado desde XML", job.Status.Resultados[1].Origen)
	}
}

func TestRunBatchStopsXMLFanoutWhenBothResourcesRejectAccess(t *testing.T) {
	t.Parallel()

	client := &recordingClient{
		probeErr:         &sunat.HTTPStatusError{StatusCode: 403, Body: "Forbidden"},
		fallbackProbeErr: &sunat.HTTPStatusError{StatusCode: 403, Body: "Forbidden"},
	}
	downloadEngine := NewDownloadEngine(client, filemanager.NewFileManager(t.TempDir()))
	comps := []sunat.Comprobante{
		{RUC: "20111111111", Tipo: "01", Serie: "F001", Numero: "10", Libro: "2"},
		{RUC: "20222222222", Tipo: "01", Serie: "F001", Numero: "11", Libro: "2"},
	}
	job := testBatch(len(comps))

	downloadEngine.runBatch(t.Context(), job, DownloadRequest{
		Comprobantes: comps,
		Tipos:        []sunat.TipoDescarga{sunat.DescargaXML},
	}, 6)

	client.mu.Lock()
	primaryXML := client.primaryXML
	fallbackXML := client.fallbackXML
	fallbackProbes := client.fallbackProbes
	client.mu.Unlock()
	if primaryXML != 0 || fallbackXML != 0 || fallbackProbes != 1 {
		t.Fatalf("descargas primarias/respaldo/probes = %d/%d/%d, want 0/0/1", primaryXML, fallbackXML, fallbackProbes)
	}
	job.Mu.RLock()
	defer job.Mu.RUnlock()
	if job.Status.Exitosos != 0 || job.Status.Errores != len(comps) {
		t.Fatalf("resultado exitosos/errores = %d/%d, want 0/%d", job.Status.Exitosos, job.Status.Errores, len(comps))
	}
	if !strings.Contains(job.Status.Resultados[0].Error, "HTTP 403") {
		t.Fatalf("error global = %q, want explicación HTTP 403", job.Status.Resultados[0].Error)
	}
}

func TestRunBatchRetriesXMLInRecoveryPass(t *testing.T) {
	t.Parallel()

	client := &recordingClient{primaryXMLFailures: 1}
	downloadEngine := NewDownloadEngine(client, filemanager.NewFileManager(t.TempDir()))
	comp := sunat.Comprobante{
		RUC: "20111111111", Tipo: "01", Serie: "F001", Numero: "10", Libro: "2",
	}
	job := testBatch(1)

	downloadEngine.runBatch(t.Context(), job, DownloadRequest{
		Comprobantes: []sunat.Comprobante{comp},
		Tipos:        []sunat.TipoDescarga{sunat.DescargaXML},
	}, 6)

	client.mu.Lock()
	requests := client.primaryXML
	client.mu.Unlock()
	if requests != 2 {
		t.Fatalf("intentos XML = %d, want 2", requests)
	}
	job.Mu.RLock()
	defer job.Mu.RUnlock()
	if job.Status.Exitosos != 1 || job.Status.Errores != 0 {
		t.Fatalf("resultado exitosos/errores = %d/%d, want 1/0", job.Status.Exitosos, job.Status.Errores)
	}
	if got := job.Status.Resultados[0].Reintentos; got != 1 {
		t.Fatalf("reintentos registrados = %d, want 1", got)
	}
}

func TestRunBatchBoundsEachXMLAndCompletesAllResults(t *testing.T) {
	t.Parallel()

	client := &recordingClient{blockXML: true}
	downloadEngine := NewDownloadEngine(client, filemanager.NewFileManager(t.TempDir()))
	comps := make([]sunat.Comprobante, 7)
	for index := range comps {
		comps[index] = sunat.Comprobante{
			RUC: "20111111111", Tipo: "01", Serie: "F001", Numero: fmt.Sprint(index + 1), Libro: "2",
		}
	}
	job := testBatch(len(comps))
	job.DocumentTimeout = 20 * time.Millisecond
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	started := time.Now()
	downloadEngine.runBatch(ctx, job, DownloadRequest{
		Comprobantes: comps,
		Tipos:        []sunat.TipoDescarga{sunat.DescargaXML},
	}, 6)
	elapsed := time.Since(started)

	if elapsed > 250*time.Millisecond {
		t.Fatalf("lote bloqueado durante %s; se esperaba cierre acotado", elapsed)
	}
	job.Mu.RLock()
	defer job.Mu.RUnlock()
	if job.Status.Procesados != len(comps) || len(job.Status.Resultados) != len(comps) {
		t.Fatalf("procesados/resultados = %d/%d, want %d/%d", job.Status.Procesados, len(job.Status.Resultados), len(comps), len(comps))
	}
	if job.Status.Exitosos != 0 || job.Status.Errores != len(comps) {
		t.Fatalf("exitosos/errores = %d/%d, want 0/%d", job.Status.Exitosos, job.Status.Errores, len(comps))
	}
}

func TestXMLWaveRenewsConnectionsAfterConcentratedTransientFailures(t *testing.T) {
	t.Parallel()

	client := &recordingClient{primaryXMLFailures: 6}
	downloadEngine := NewDownloadEngine(client, filemanager.NewFileManager(t.TempDir()))
	comps := make([]sunat.Comprobante, 7)
	for index := range comps {
		comps[index] = sunat.Comprobante{
			RUC: "20111111111", Tipo: "01", Serie: "F001", Numero: fmt.Sprint(index + 1), Libro: "2",
		}
	}
	job := testBatch(len(comps))
	downloadEngine.runBatch(t.Context(), job, DownloadRequest{
		Comprobantes: comps,
		Tipos:        []sunat.TipoDescarga{sunat.DescargaXML},
	}, 6)

	client.mu.Lock()
	resets := client.transportResets
	closes := client.idleCloses
	client.mu.Unlock()
	if resets < 2 {
		t.Fatalf("reinicios de transporte = %d, want al menos inicio + ola degradada", resets)
	}
	if closes != 1 {
		t.Fatalf("cierres de conexiones = %d, want 1", closes)
	}
}

func TestResetSessionClearsPreviousCompanyBatch(t *testing.T) {
	t.Parallel()

	client := &recordingClient{}
	downloadEngine := NewDownloadEngine(client, filemanager.NewFileManager(t.TempDir()))
	downloadEngine.currentJob = testBatch(1)
	downloadEngine.currentJob.Status.Estado = "completado"
	if err := downloadEngine.ResetSession(); err != nil {
		t.Fatal(err)
	}
	status, _ := downloadEngine.GetCurrentStatus()
	if status.Estado != "inactivo" {
		t.Fatalf("estado posterior al reinicio = %q, want inactivo", status.Estado)
	}
	client.mu.Lock()
	resets := client.transportResets
	client.mu.Unlock()
	if resets != 1 {
		t.Fatalf("reinicios de transporte = %d, want 1", resets)
	}
}

func TestRunBatchAccountsForQueuedXMLWhenBatchDeadlineExpires(t *testing.T) {
	t.Parallel()

	client := &recordingClient{blockXML: true}
	downloadEngine := NewDownloadEngine(client, filemanager.NewFileManager(t.TempDir()))
	comps := make([]sunat.Comprobante, 12)
	for index := range comps {
		comps[index] = sunat.Comprobante{
			RUC: "20111111111", Tipo: "01", Serie: "F001", Numero: fmt.Sprint(index + 1), Libro: "2",
		}
	}
	job := testBatch(len(comps))
	job.DocumentTimeout = 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Millisecond)
	defer cancel()

	downloadEngine.runBatch(ctx, job, DownloadRequest{
		Comprobantes: comps,
		Tipos:        []sunat.TipoDescarga{sunat.DescargaXML},
	}, 6)

	job.Mu.RLock()
	defer job.Mu.RUnlock()
	if job.Status.Procesados != len(comps) || job.Status.Errores != len(comps) {
		t.Fatalf("procesados/errores = %d/%d, want %d/%d", job.Status.Procesados, job.Status.Errores, len(comps), len(comps))
	}
	if job.Status.Porcentaje != 100 {
		t.Fatalf("porcentaje = %.1f, want 100", job.Status.Porcentaje)
	}
}

func TestExpectedItemCountOmitsPortalCDR(t *testing.T) {
	t.Parallel()

	comps := []sunat.Comprobante{
		{Serie: "E001"},
		{Serie: "F001"},
	}
	types := []sunat.TipoDescarga{sunat.DescargaXML, sunat.DescargaCDR}

	if got := expectedItemCount(comps, types); got != 3 {
		t.Fatalf("expectedItemCount() = %d, want 3", got)
	}
}

func TestClassifyResultNormalizesFinalSummaryCategories(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result sunat.ItemResult
		want   string
	}{
		{name: "downloaded", result: sunat.ItemResult{Exito: true, Origen: "descargado de SUNAT"}, want: categoryDownloaded},
		{name: "existing", result: sunat.ItemResult{Exito: true, Origen: "archivo existente"}, want: categoryExisting},
		{name: "missing 301", result: sunat.ItemResult{Error: `SUNAT HTTP 422: {"codError":"301","desError":"No se encontro el xml"}`}, want: categoryUnavailable},
		{name: "invalid query 302", result: sunat.ItemResult{Error: `{"codError":"302","desError":"Consulta Invalida"}`}, want: categoryUnsupported},
		{name: "recoverable response", result: sunat.ItemResult{Error: "el archivo no contiene estructura válida de XML ni ZIP"}, want: categoryRecoverable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyResult(tt.result); got != tt.want {
				t.Fatalf("classifyResult() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCloneBatchStatusRebuildsCategorySummary(t *testing.T) {
	t.Parallel()

	status := sunat.BatchStatus{Resultados: []sunat.ItemResult{
		{Exito: true, Origen: "descargado de SUNAT"},
		{Exito: true, Origen: "archivo existente"},
		{Error: `{"codError":"301"}`},
		{Error: `{"codError":"302"}`},
		{Error: "EOF"},
	}}
	clone := cloneBatchStatus(status)
	for _, category := range []string{
		categoryDownloaded,
		categoryExisting,
		categoryUnavailable,
		categoryUnsupported,
		categoryRecoverable,
	} {
		if clone.ResumenCategorias[category] != 1 {
			t.Fatalf("resumen[%q] = %d, want 1", category, clone.ResumenCategorias[category])
		}
	}
}

func TestXMLBatchTimeoutScalesWithBatchSize(t *testing.T) {
	t.Parallel()

	if got := xmlBatchTimeout(0, 431, 6); got != 6*time.Minute {
		t.Fatalf("xmlBatchTimeout(431, 6) = %s, want 6m", got)
	}
	if got := xmlBatchTimeout(0, 20, 6); got != defaultXMLBatchTimeout {
		t.Fatalf("xmlBatchTimeout pequeño = %s, want %s", got, defaultXMLBatchTimeout)
	}
	if got := xmlBatchTimeout(120, 431, 6); got != 2*time.Minute {
		t.Fatalf("xmlBatchTimeout explícito = %s, want 2m", got)
	}
}

func testBatch(total int) *ActiveBatch {
	return &ActiveBatch{
		Status: sunat.BatchStatus{
			Estado:     "procesando",
			TotalItems: total,
			Resultados: []sunat.ItemResult{},
		},
		Logs:            []string{},
		DocumentTimeout: defaultXMLDocumentTimeout,
	}
}
