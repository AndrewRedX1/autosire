package sunat

import (
	"errors"
	"net/http"
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
			if !ok || status != tt.wantStatus {
				t.Fatalf("HTTPStatus() = %d, %t, want %d, true; err=%v", status, ok, tt.wantStatus, err)
			}
			if got := IsTransientDownloadError(err); got != tt.wantTransient {
				t.Fatalf("IsTransientDownloadError() = %t, want %t", got, tt.wantTransient)
			}
			var statusErr *HTTPStatusError
			if !errors.As(err, &statusErr) {
				t.Fatalf("error %v no preserva HTTPStatusError", err)
			}
		})
	}
}
