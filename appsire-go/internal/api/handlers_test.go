package api

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"appsire-go/internal/engine"
	"appsire-go/internal/filemanager"
	"appsire-go/internal/ssco"
	"appsire-go/internal/sunat"
)

func TestSecurityHeadersAllowSameOriginFileViewer(t *testing.T) {
	t.Parallel()

	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := response.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Fatalf("X-Frame-Options = %q, want SAMEORIGIN", got)
	}
	if policy := response.Header().Get("Content-Security-Policy"); !strings.Contains(policy, "frame-src 'self'") {
		t.Fatalf("Content-Security-Policy no permite el visor del mismo origen: %q", policy)
	}
}

func TestHandleViewFileServesPDFInline(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	pdfPath := filepath.Join(baseDir, "representacion impresa.pdf")
	pdfData := []byte("%PDF-1.4\n%%EOF\n")
	if err := os.WriteFile(pdfPath, pdfData, 0o600); err != nil {
		t.Fatal(err)
	}

	server := &Server{fileManager: filemanager.NewFileManager(baseDir)}
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/files/view?path="+url.QueryEscape(pdfPath),
		nil,
	)
	response := httptest.NewRecorder()
	server.HandleViewFile(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", got)
	}
	if disposition := response.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, "inline;") {
		t.Fatalf("Content-Disposition = %q, want inline", disposition)
	}
	if !bytes.Equal(response.Body.Bytes(), pdfData) {
		t.Fatalf("contenido PDF servido fue alterado")
	}
}

func TestValidateProposalBindingRejectsChangedIdentity(t *testing.T) {
	t.Parallel()

	server := &Server{proposals: map[sunat.ProposalBook]proposalBinding{
		sunat.ProposalRCE: {RUC: "20600000001", Period: "202608", Ticket: "ticket-1"},
	}}
	base := engine.DownloadRequest{
		ProposalBook:   "RCE",
		ProposalPeriod: "202608",
		ProposalTicket: "ticket-1",
		OwnerRUC:       "20600000001",
	}
	tests := []struct {
		name       string
		request    engine.DownloadRequest
		currentRUC string
		wantError  bool
	}{
		{name: "same proposal", request: base, currentRUC: "20600000001"},
		{name: "company changed", request: base, currentRUC: "20999999999", wantError: true},
		{name: "period changed", request: withProposalPeriod(base, "202609"), currentRUC: "20600000001", wantError: true},
		{name: "ticket changed", request: withProposalTicket(base, "ticket-2"), currentRUC: "20600000001", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.validateProposalBinding(tt.request, tt.currentRUC)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateProposalBinding() error = %v, wantError %t", err, tt.wantError)
			}
		})
	}
}

func TestHandleXMLPreviewParsesFileInsideDownloads(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	xmlPath := filepath.Join(baseDir, "factura.xml")
	xmlData := `<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2" xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"><cbc:ID>F001-1</cbc:ID><cbc:InvoiceTypeCode>01</cbc:InvoiceTypeCode><cbc:DocumentCurrencyCode>PEN</cbc:DocumentCurrencyCode></Invoice>`
	if err := os.WriteFile(xmlPath, []byte(xmlData), 0600); err != nil {
		t.Fatal(err)
	}
	server := &Server{fileManager: filemanager.NewFileManager(baseDir)}
	req := httptest.NewRequest(http.MethodGet, "/api/files/xml-preview?path="+url.QueryEscape(xmlPath), nil)
	response := httptest.NewRecorder()

	server.HandleXMLPreview(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"number":"F001-1"`) {
		t.Fatalf("respuesta inesperada: %s", response.Body.String())
	}
}

func TestHandleXMLPreviewRejectsOutsidePath(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	server := &Server{fileManager: filemanager.NewFileManager(baseDir)}
	outside := filepath.Join(filepath.Dir(baseDir), "outside.xml")
	req := httptest.NewRequest(http.MethodGet, "/api/files/xml-preview?path="+url.QueryEscape(outside), nil)
	response := httptest.NewRecorder()

	server.HandleXMLPreview(response, req)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestHandleXMLPreviewReadsCDRFromZIP(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	zipPath := filepath.Join(baseDir, "R-20600000001-01-F001-123.zip")
	archiveFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(archiveFile)
	xmlFile, err := archive.Create("R-20600000001-01-F001-123.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = xmlFile.Write([]byte(`<ApplicationResponse><ID>R-20600000001-01-F001-123</ID><DocumentResponse><Response><ResponseCode>0</ResponseCode><Description>Aceptado</Description></Response><DocumentReference><ID>F001-123</ID><DocumentTypeCode>01</DocumentTypeCode></DocumentReference></DocumentResponse></ApplicationResponse>`))
	if err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archiveFile.Close(); err != nil {
		t.Fatal(err)
	}

	server := &Server{fileManager: filemanager.NewFileManager(baseDir)}
	req := httptest.NewRequest(http.MethodGet, "/api/files/xml-preview?path="+url.QueryEscape(zipPath), nil)
	response := httptest.NewRecorder()
	server.HandleXMLPreview(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"kind":"cdr"`) || !strings.Contains(response.Body.String(), `"raw_xml"`) {
		t.Fatalf("respuesta CDR inesperada: %s", response.Body.String())
	}
}

func withProposalPeriod(request engine.DownloadRequest, period string) engine.DownloadRequest {
	request.ProposalPeriod = period
	return request
}

func withProposalTicket(request engine.DownloadRequest, ticket string) engine.DownloadRequest {
	request.ProposalTicket = ticket
	return request
}

func TestArchiveHandlers(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	// Período 1: Compras 202602 (XML + CDR)
	compDir1 := filepath.Join(baseDir, "APP DESCARGAS", "CPE", "20490304101 HOTELES CBC S.A.C", "Compras", "202602")
	if err := os.MkdirAll(compDir1, 0755); err != nil {
		t.Fatal(err)
	}
	xmlPath1 := filepath.Join(compDir1, "10238508903-01-FF02-61781.xml")
	if err := os.WriteFile(xmlPath1, []byte("<xml>test</xml>"), 0644); err != nil {
		t.Fatal(err)
	}
	cdrPath1 := filepath.Join(compDir1, "R-10238508903-01-FF02-61781.xml")
	if err := os.WriteFile(cdrPath1, []byte("<cdr>ok</cdr>"), 0644); err != nil {
		t.Fatal(err)
	}

	// Período 2: Compras 202606 (PDF)
	compDir2 := filepath.Join(baseDir, "APP DESCARGAS", "CPE", "20490304101 HOTELES CBC S.A.C", "Compras", "202606")
	if err := os.MkdirAll(compDir2, 0755); err != nil {
		t.Fatal(err)
	}
	pdfPath2 := filepath.Join(compDir2, "10238508903-01-FF02-61781.pdf")
	if err := os.WriteFile(pdfPath2, []byte("%PDF-1.4\n%%EOF"), 0644); err != nil {
		t.Fatal(err)
	}

	server := &Server{fileManager: filemanager.NewFileManager(baseDir)}

	// Test HandleArchiveTree
	reqTree := httptest.NewRequest(http.MethodGet, "/api/archive/tree", nil)
	wTree := httptest.NewRecorder()
	server.HandleArchiveTree(wTree, reqTree)

	if wTree.Code != http.StatusOK {
		t.Fatalf("tree status = %d, body = %s", wTree.Code, wTree.Body.String())
	}
	treeBody := wTree.Body.String()
	if !strings.Contains(treeBody, "20490304101") || !strings.Contains(treeBody, "Compras") {
		t.Fatalf("tree body missing expected data: %s", treeBody)
	}
	if !strings.Contains(treeBody, "periods") || !strings.Contains(treeBody, "Feb-2026") || !strings.Contains(treeBody, "Jun-2026") {
		t.Fatalf("tree body missing period items or labels: %s", treeBody)
	}

	// Test HandleArchiveFiles consolidates into vouchers with XML and CDR
	reqFiles := httptest.NewRequest(http.MethodGet, "/api/archive/files?company=20490304101&book=Compras&period=202602", nil)
	wFiles := httptest.NewRecorder()
	server.HandleArchiveFiles(wFiles, reqFiles)

	if wFiles.Code != http.StatusOK {
		t.Fatalf("files status = %d, body = %s", wFiles.Code, wFiles.Body.String())
	}
	filesBody := wFiles.Body.String()
	if !strings.Contains(filesBody, "FF02-61781") || !strings.Contains(filesBody, "Factura") {
		t.Fatalf("files body missing expected voucher data: %s", filesBody)
	}
	if !strings.Contains(filesBody, `"has_xml":true`) || !strings.Contains(filesBody, `"has_cdr":true`) {
		t.Fatalf("files body missing consolidated voucher flags: %s", filesBody)
	}
}

func TestHandleValidateTC(t *testing.T) {
	t.Parallel()

	server := &Server{}
	body := bytes.NewBufferString(`{
		"book": "RCE",
		"tc_type": "Venta",
		"items": [
			{
				"comp_pago": "01 F001-100",
				"moneda": "USD",
				"fecha": "10/09/2024",
				"tipo_cambio": "3.800",
				"importe_total": "100.00"
			},
			{
				"comp_pago": "01 F001-101",
				"moneda": "PEN",
				"fecha": "10/09/2024",
				"importe_total": "500.00"
			}
		]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/sire/validate-tc", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.HandleValidateTC(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, `"success":true`) || !strings.Contains(resp, `"total_usd":1`) {
		t.Fatalf("response missing expected validation report: %s", resp)
	}
}

func TestHandleValidateCPE_EmptyItems(t *testing.T) {
	t.Parallel()

	server := &Server{}
	body := strings.NewReader(`{
		"book": "RCE",
		"items": []
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/sire/validate-cpe", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.HandleValidateCPE(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, `"success":true`) {
		t.Fatalf("response missing success: %s", resp)
	}
}

func TestHandleValidateSSCO_EmptyItems(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	svc := ssco.NewService(tmpDir)
	server := &Server{sscoService: svc}

	body := strings.NewReader(`{
		"book": "RCE",
		"items": []
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/sire/validate-ssco", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.HandleValidateSSCO(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, `"success":true`) {
		t.Fatalf("response missing success: %s", resp)
	}
}

func TestHandleDetectCuadre(t *testing.T) {
	t.Parallel()

	server := &Server{}
	body := strings.NewReader(`{
		"book": "RCE",
		"items": [
			{
				"comp_pago": "01-F001-00000001",
				"bi_gravada": "1000.01",
				"igv": "180.00",
				"importe_total": "1180.00"
			}
		]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/sire/cuadre/detect", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.HandleDetectCuadre(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, `"success":true`) || !strings.Contains(resp, `"total_descuadres":1`) {
		t.Fatalf("response missing expected descuadre report: %s", resp)
	}
}

func TestHandleDetectCorrelativos(t *testing.T) {
	t.Parallel()

	server := &Server{}
	body := strings.NewReader(`{
		"book": "RVIE",
		"items": [
			{
				"tipo": "01",
				"serie": "F001",
				"numero": "00000001",
				"fecha": "01/09/2026"
			},
			{
				"tipo": "01",
				"serie": "F001",
				"numero": "00000003",
				"fecha": "05/09/2026"
			}
		]
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/sire/correlativos/detect", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.HandleDetectCorrelativos(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, `"success":true`) || !strings.Contains(resp, `"total_faltantes":1`) {
		t.Fatalf("response missing expected correlativos report: %s", resp)
	}
}


