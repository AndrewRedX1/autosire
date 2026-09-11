package engine

import (
	"context"
	"sync"
	"testing"

	"appsire-go/internal/filemanager"
	"appsire-go/internal/sunat"
)

type recordingClient struct {
	mu           sync.Mutex
	calls        []sunat.TipoDescarga
	probeErr     error
	refreshes    int
	primaryXML   int
	fallbackXML  int
	pdfDownloads int
	cdrDownloads int
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
	_ context.Context,
	comp sunat.Comprobante,
) (*sunat.DownloadedFile, error) {
	c.record(sunat.DescargaXML)
	c.mu.Lock()
	c.primaryXML++
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

func testBatch(total int) *ActiveBatch {
	return &ActiveBatch{
		Status: sunat.BatchStatus{
			Estado:     "procesando",
			TotalItems: total,
			Resultados: []sunat.ItemResult{},
		},
		Logs: []string{},
	}
}
