package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"appsire-go/internal/engine"
	"appsire-go/internal/filemanager"
	"appsire-go/internal/sunat"
)

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

func withProposalPeriod(request engine.DownloadRequest, period string) engine.DownloadRequest {
	request.ProposalPeriod = period
	return request
}

func withProposalTicket(request engine.DownloadRequest, ticket string) engine.DownloadRequest {
	request.ProposalTicket = ticket
	return request
}
