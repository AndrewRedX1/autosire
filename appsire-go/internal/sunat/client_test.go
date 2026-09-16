package sunat

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type staticTokenProvider struct{}

func (staticTokenProvider) GetValidToken(context.Context) (string, error) {
	return "token", nil
}

func (staticTokenProvider) InvalidateToken(string) {}

func (staticTokenProvider) ForceRefresh(context.Context) (string, error) {
	return "token-renovado", nil
}

func TestQueryTipo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		tipo  string
		serie string
		want  string
	}{
		{name: "factura", tipo: "01", serie: "F001", want: "01"},
		{name: "nota crédito factura", tipo: "07", serie: "F001", want: "F7"},
		{name: "nota crédito portal", tipo: "07", serie: "E001", want: "F7"},
		{name: "nota crédito boleta", tipo: "07", serie: "B001", want: "B7"},
		{name: "nota débito factura", tipo: "08", serie: "F001", want: "F8"},
		{name: "nota débito boleta", tipo: "08", serie: "B001", want: "B8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := QueryTipo(tt.tipo, tt.serie); got != tt.want {
				t.Fatalf("QueryTipo(%q, %q) = %q, want %q", tt.tipo, tt.serie, got, tt.want)
			}
		})
	}
}

func TestNormalizeNumero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "separador miles", in: "10,685", want: "10685"},
		{name: "varios separadores", in: "323,992", want: "323992"},
		{name: "decimal excel", in: "123.0", want: "123"},
		{name: "ceros izquierda", in: "00000123", want: "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := NormalizeNumero(tt.in); got != tt.want {
				t.Fatalf("NormalizeNumero(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseJsonResponseAcceptsArray(t *testing.T) {
	t.Parallel()

	pdf := []byte("%PDF-1.7\ncontenido")
	body := `[{"nomArchivo":"factura.pdf","valArchivo":"` + base64.StdEncoding.EncodeToString(pdf) + `"}]`

	file, err := ParseJsonResponse([]byte(body))
	if err != nil {
		t.Fatalf("ParseJsonResponse() error = %v", err)
	}
	if string(file.Content) != string(pdf) {
		t.Fatalf("contenido = %q, want %q", file.Content, pdf)
	}
}

func TestDownloadPDFUsesPurchaseBookAndQueryTipo(t *testing.T) {
	t.Parallel()

	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		pdf := base64.StdEncoding.EncodeToString([]byte("%PDF-1.7\ncontenido"))
		_, _ = w.Write([]byte(`{"nomArchivo":"nota.pdf","valArchivo":"` + pdf + `"}`))
	}))
	defer server.Close()

	client := NewSunatClient(staticTokenProvider{}, 0)
	client.baseURL = server.URL + "/v1/contribuyente"
	client.maxRetries = 1

	_, err := client.DownloadPDF(t.Context(), Comprobante{
		RUC:    "20123456789",
		Tipo:   "07",
		Serie:  "F001",
		Numero: "10685",
		Libro:  "2",
	})
	if err != nil {
		t.Fatalf("DownloadPDF() error = %v", err)
	}

	want := "/v1/contribuyente/consultacpe/comprobantes/20123456789-F7-F001-10685-2/01"
	if requestedPath != want {
		t.Fatalf("ruta solicitada = %q, want %q", requestedPath, want)
	}
}

func TestDownloadPDFReturns401WithoutPerRequestRefresh(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":401,"message":"Unauthorized"}`))
	}))
	defer server.Close()

	client := NewSunatClient(staticTokenProvider{}, 0)
	client.baseURL = server.URL + "/v1/contribuyente"
	client.maxRetries = 5

	_, err := client.DownloadPDF(t.Context(), Comprobante{
		RUC: "20123456789", Tipo: "01", Serie: "F001", Numero: "1", Libro: "2",
	})
	if err == nil {
		t.Fatal("DownloadPDF() error = nil, want HTTP 401")
	}
	if status, ok := HTTPStatus(err); !ok || status != http.StatusUnauthorized {
		t.Fatalf("HTTPStatus() = %d, %t, want 401, true", status, ok)
	}
	if requests != 1 {
		t.Fatalf("peticiones = %d, want 1; la renovacion corresponde al lote", requests)
	}
}

func TestProbeConsultacpeAcceptsDocumentNotFoundAsAuthorized(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"cod":"422","msg":"no encontrado"}`))
	}))
	defer server.Close()

	client := NewSunatClient(staticTokenProvider{}, 0)
	client.baseURL = server.URL + "/v1/contribuyente"
	client.maxRetries = 1

	err := client.ProbeConsultacpe(t.Context(), Comprobante{
		RUC: "20123456789", Tipo: "01", Serie: "F001", Numero: "1", Libro: "2",
	})
	if err != nil {
		t.Fatalf("ProbeConsultacpe() error = %v, want nil", err)
	}
}

func TestProbeConsultacpeDoesNotRetryTransientResponse(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("temporal"))
	}))
	defer server.Close()

	client := NewSunatClient(staticTokenProvider{}, 0)
	client.baseURL = server.URL + "/v1/contribuyente"
	client.maxRetries = 5
	err := client.ProbeConsultacpe(t.Context(), Comprobante{
		RUC: "20123456789", Tipo: "01", Serie: "F001", Numero: "1", Libro: "2",
	})
	if err == nil {
		t.Fatal("ProbeConsultacpe() error = nil, want HTTP 503")
	}
	if requests != 1 {
		t.Fatalf("peticiones de preflight = %d, want 1", requests)
	}
}

func TestDownloadXMLControlsRetriesAtBatchLevel(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if strings.Contains(r.URL.Path, "/consultacpe/") {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("temporal"))
			return
		}
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Invoice/>`))
	}))
	defer server.Close()

	client := NewSunatClient(staticTokenProvider{}, 0)
	client.baseURL = server.URL + "/v1/contribuyente"
	client.maxRetries = 5
	_, err := client.DownloadXML(t.Context(), Comprobante{
		RUC: "20123456789", Tipo: "01", Serie: "F001", Numero: "1", Libro: "2",
	})
	if err != nil {
		t.Fatalf("DownloadXML() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("peticiones = %d, want 2 (primaria + respaldo sin reintento interno)", requests)
	}
}

func TestCPETransportUsesIsolatedHTTP1ConnectionsAndCanReset(t *testing.T) {
	t.Parallel()

	client := NewSunatClient(staticTokenProvider{}, time.Second)
	previous := client.transport
	if !previous.DisableKeepAlives {
		t.Fatal("DisableKeepAlives = false, want true")
	}
	if previous.ForceAttemptHTTP2 {
		t.Fatal("ForceAttemptHTTP2 = true, want false")
	}
	client.ResetTransport()
	if client.transport == previous {
		t.Fatal("ResetTransport conservó el transporte anterior")
	}
}

func TestHTTPStatusFindsWrappedStatus(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("descargando: %w", &HTTPStatusError{StatusCode: 503, Body: "unavailable"})
	status, ok := HTTPStatus(err)
	if !ok || status != 503 {
		t.Fatalf("HTTPStatus() = %d, %t, want 503, true", status, ok)
	}
	if !errors.As(err, new(*HTTPStatusError)) {
		t.Fatal("la cadena de error no conserva HTTPStatusError")
	}
}

func TestParseRetryAfterSupportsSecondsAndHTTPDate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "seconds", value: "7", want: 7 * time.Second},
		{name: "HTTP date", value: now.Add(9 * time.Second).Format(http.TimeFormat), want: 9 * time.Second},
		{name: "expired date", value: now.Add(-time.Second).Format(http.TimeFormat), want: 0},
		{name: "invalid", value: "después", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseRetryAfter(tt.value, now); got != tt.want {
				t.Fatalf("parseRetryAfter(%q) = %s, want %s", tt.value, got, tt.want)
			}
		})
	}
}
