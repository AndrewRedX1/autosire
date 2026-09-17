package sunat

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func createSampleCDRZip(t *testing.T, xmlContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("R-20123456789-01-F001-1.xml")
	if err != nil {
		t.Fatalf("error creando entrada en ZIP: %v", err)
	}
	if _, err := f.Write([]byte(xmlContent)); err != nil {
		t.Fatalf("error escribiendo XML en ZIP: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("error cerrando ZIP: %v", err)
	}
	return buf.Bytes()
}

func TestDownloadCDRSuccessJsonBase64(t *testing.T) {
	t.Parallel()

	sampleXml := `<?xml version="1.0" encoding="UTF-8"?>
<ApplicationResponse xmlns="urn:oasis:names:specification:ubl:schema:xsd:ApplicationResponse-2"
                     xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
                     xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
    <cac:DocumentResponse>
        <cac:Response>
            <cbc:ResponseCode>0</cbc:ResponseCode>
            <cbc:Description>La Factura numero F001-1 ha sido aceptada</cbc:Description>
        </cac:Response>
    </cac:DocumentResponse>
</ApplicationResponse>`

	zipBytes := createSampleCDRZip(t, sampleXml)
	b64Zip := base64.StdEncoding.EncodeToString(zipBytes)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if !strings.Contains(r.URL.Path, "/consultacpe/comprobantes/20123456789-01-F001-1-2/03") {
			t.Errorf("URL inesperada: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"nomArchivo":"20123456789-01-F001-1.zip","valArchivo":"` + b64Zip + `"}`))
	}))
	defer server.Close()

	client := NewSunatClient(staticTokenProvider{}, 0)
	client.baseURL = server.URL + "/v1/contribuyente"

	file, err := client.DownloadCDR(context.Background(), Comprobante{
		RUC:    "20123456789",
		Tipo:   "01",
		Serie:  "F001",
		Numero: "1",
		Libro:  "2",
	})
	if err != nil {
		t.Fatalf("DownloadCDR() error = %v", err)
	}

	if requestCount != 1 {
		t.Fatalf("peticiones realizadas = %d, want 1", requestCount)
	}

	if !strings.HasPrefix(file.FileName, "R-") {
		t.Errorf("file.FileName no empieza con R-: %s", file.FileName)
	}
	if !file.IsZip {
		t.Errorf("file.IsZip = false, want true")
	}

	_, innerXml, extractErr := ExtractFileFromZip(file.Content, ".xml")
	if extractErr != nil {
		t.Fatalf("ExtractFileFromZip() error = %v", extractErr)
	}

	cdrInfo := InspectCDR(innerXml)
	if cdrInfo.Estado != "ACEPTADO" {
		t.Errorf("cdrInfo.Estado = %s, want ACEPTADO", cdrInfo.Estado)
	}
	if cdrInfo.ResponseCode != "0" {
		t.Errorf("cdrInfo.ResponseCode = %s, want 0", cdrInfo.ResponseCode)
	}
}

func TestDownloadCDRNotFoundDefinitive(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"codError":"404","message":"No se encontro el comprobante"}`))
	}))
	defer server.Close()

	client := NewSunatClient(staticTokenProvider{}, 0)
	client.baseURL = server.URL + "/v1/contribuyente"

	_, err := client.DownloadCDR(context.Background(), Comprobante{
		RUC:    "20123456789",
		Tipo:   "01",
		Serie:  "F001",
		Numero: "999",
		Libro:  "1",
	})
	if err == nil {
		t.Fatal("DownloadCDR() error = nil, want 404 error")
	}

	if requestCount != 1 {
		t.Fatalf("peticiones realizadas = %d, want 1 (sin reintentos en 404)", requestCount)
	}

	if !strings.Contains(err.Error(), "comprobante sin CDR en SUNAT (HTTP 404)") {
		t.Errorf("error inesperado: %v", err)
	}

	if IsTransientDownloadError(err) {
		t.Errorf("IsTransientDownloadError(err) = true, want false para 404")
	}
}

func TestDownloadCDRTransientError(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("503 Service Temporarily Unavailable"))
	}))
	defer server.Close()

	client := NewSunatClient(staticTokenProvider{}, 0)
	client.baseURL = server.URL + "/v1/contribuyente"

	_, err := client.DownloadCDR(context.Background(), Comprobante{
		RUC:    "20123456789",
		Tipo:   "01",
		Serie:  "F001",
		Numero: "1",
		Libro:  "2",
	})
	if err == nil {
		t.Fatal("DownloadCDR() error = nil, want error")
	}

	if requestCount != 1 {
		t.Fatalf("peticiones realizadas = %d, want 1 (la fase principal no reintenta en bucle)", requestCount)
	}

	if !IsTransientDownloadError(err) {
		t.Errorf("IsTransientDownloadError(err) = false, want true para 503")
	}
}

func TestNormalizeCDRFileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		candidate   string
		defaultName string
		want        string
	}{
		{
			candidate:   "20123456789-01-F001-1.zip",
			defaultName: "R-20123456789-01-F001-1.zip",
			want:        "R-20123456789-01-F001-1.zip",
		},
		{
			candidate:   "R-20123456789-01-F001-1.zip",
			defaultName: "R-20123456789-01-F001-1.zip",
			want:        "R-20123456789-01-F001-1.zip",
		},
		{
			candidate:   "20123456789-01-F001-1-CDR.zip",
			defaultName: "R-20123456789-01-F001-1.zip",
			want:        "20123456789-01-F001-1-CDR.zip",
		},
		{
			candidate:   "",
			defaultName: "R-20123456789-01-F001-1.zip",
			want:        "R-20123456789-01-F001-1.zip",
		},
		{
			candidate:   "   ",
			defaultName: "R-20123456789-01-F001-1.zip",
			want:        "R-20123456789-01-F001-1.zip",
		},
	}

	for _, tt := range tests {
		got := normalizeCDRFileName(tt.candidate, tt.defaultName)
		if got != tt.want {
			t.Errorf("normalizeCDRFileName(%q, %q) = %q, want %q", tt.candidate, tt.defaultName, got, tt.want)
		}
	}
}
