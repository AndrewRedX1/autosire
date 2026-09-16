package sunat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSelectXMLDownloadErrorPreservesMacroRetryPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		primary       error
		fallback      error
		wantStatus    int
		wantTransient bool
	}{
		{
			name:          "definitive fallback beats primary EOF",
			primary:       io.EOF,
			fallback:      &HTTPStatusError{StatusCode: http.StatusUnprocessableEntity, Body: `{"codError":"301"}`},
			wantStatus:    http.StatusUnprocessableEntity,
			wantTransient: false,
		},
		{
			name:          "primary EOF remains transient without SUNAT business code",
			primary:       io.ErrUnexpectedEOF,
			fallback:      &HTTPStatusError{StatusCode: http.StatusUnprocessableEntity, Body: `{"codError":"999"}`},
			wantTransient: true,
		},
		{
			name:          "primary transient beats definitive fallback",
			primary:       &HTTPStatusError{StatusCode: http.StatusServiceUnavailable, Body: "temporal"},
			fallback:      &HTTPStatusError{StatusCode: http.StatusNotFound, Body: "no existe"},
			wantStatus:    http.StatusServiceUnavailable,
			wantTransient: true,
		},
		{
			name:          "fallback transient has priority",
			primary:       &HTTPStatusError{StatusCode: http.StatusNotFound, Body: "no existe"},
			fallback:      &HTTPStatusError{StatusCode: http.StatusTooManyRequests, Body: "limite"},
			wantStatus:    http.StatusTooManyRequests,
			wantTransient: true,
		},
		{
			name:          "definitive fallback is retained",
			primary:       &HTTPStatusError{StatusCode: http.StatusNotFound, Body: "no existe"},
			fallback:      &HTTPStatusError{StatusCode: http.StatusForbidden, Body: "sin permiso"},
			wantStatus:    http.StatusForbidden,
			wantTransient: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := selectXMLDownloadError(tt.primary, tt.fallback)
			status, ok := HTTPStatus(err)
			if tt.wantStatus != 0 && (!ok || status != tt.wantStatus) {
				t.Fatalf("HTTPStatus() = %d, %t, want %d, true; err=%v", status, ok, tt.wantStatus, err)
			}
			if got := IsTransientDownloadError(err); got != tt.wantTransient {
				t.Fatalf("IsTransientDownloadError() = %t, want %t", got, tt.wantTransient)
			}
			var statusErr *HTTPStatusError
			if tt.wantStatus != 0 && !errors.As(err, &statusErr) {
				t.Fatalf("error %v no preserva HTTPStatusError", err)
			}
		})
	}
}

func TestDownloadXMLFallsBackWhenPrimaryBase64IsNotXML(t *testing.T) {
	t.Parallel()

	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.Contains(r.URL.Path, "/consultacpe/") {
			_, _ = w.Write([]byte(`{"nomArchivo":"respuesta.zip","valArchivo":"bm8gZXMgeG1s"}`))
			return
		}
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Invoice/>`))
	}))
	defer server.Close()

	client := NewSunatClient(staticTokenProvider{}, 0)
	client.baseURL = server.URL + "/v1/contribuyente"
	file, err := client.DownloadXML(context.Background(), Comprobante{
		RUC: "20123456789", Tipo: "01", Serie: "F001", Numero: "1", Libro: "2",
	})
	if err != nil {
		t.Fatalf("DownloadXML() error = %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("peticiones = %d, want 2", len(paths))
	}
	if file.IsZip || !IsXMLContent(file.Content) || !strings.HasSuffix(file.FileName, ".xml") {
		t.Fatalf("archivo recuperado inválido: %+v", file)
	}
}
