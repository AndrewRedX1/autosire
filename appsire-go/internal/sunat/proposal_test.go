package sunat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type proposalTokenProvider struct{}

func (proposalTokenProvider) GetValidToken(context.Context) (string, error) {
	return "test-token", nil
}

func (proposalTokenProvider) InvalidateToken(string) {}

func (proposalTokenProvider) ForceRefresh(context.Context) (string, error) {
	return "test-token", nil
}

func TestProposalClientDownloadRCE(t *testing.T) {
	t.Parallel()

	var ticketRequested bool
	var archiveRequested bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization inesperada: %q", r.Header.Get("Authorization"))
		}

		switch {
		case strings.Contains(r.URL.Path, "/rce/propuesta/"):
			ticketRequested = true
			fmt.Fprint(w, `{"numTicket":"12345"}`)
		case strings.HasSuffix(r.URL.Path, "/consultaestadotickets"):
			fmt.Fprint(w, `{"registros":[{"numTicket":"12345","codProceso":"10","codEstadoProceso":"06","archivoReporte":[{"nomArchivoReporte":"LE20500000001_202608_RCE.zip","codTipoAchivoReporte":"00"}]}]}`)
		case strings.HasSuffix(r.URL.Path, "/archivoreporte"):
			archiveRequested = true
			if got := r.URL.Query().Get("codLibro"); got != "080000" {
				t.Fatalf("codLibro = %q; se esperaba 080000", got)
			}
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write([]byte{'P', 'K', 3, 4, 0, 0})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewProposalClient(proposalTokenProvider{}, 5*time.Second)
	client.baseURL = server.URL

	result, err := client.Download(context.Background(), ProposalRCE, "202608", false)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if !ticketRequested || !archiveRequested {
		t.Fatalf("flujo incompleto: ticket=%v archive=%v", ticketRequested, archiveRequested)
	}
	if result.Ticket != "12345" || result.FileName != "LE20500000001_202608_RCE.zip" {
		t.Fatalf("resultado inesperado: %+v", result)
	}
	if result.Reused {
		t.Fatal("la propuesta recién generada no debe figurar como reutilizada")
	}
}

func TestProposalDefinitionRejectsInvalidPeriod(t *testing.T) {
	t.Parallel()

	invalidPeriods := []string{"", "2026", "202600", "202613", "20A601"}
	for _, period := range invalidPeriods {
		if _, err := proposalDefinitionFor(ProposalRVIE, period); err == nil {
			t.Errorf("proposalDefinitionFor(%q) no devolvió error", period)
		}
	}
}

func TestDownloadableFilePrefersDataZip(t *testing.T) {
	t.Parallel()

	files := []reportFile{
		{Name: "PCW_reporte.zip", Type: "01"},
		{Name: "propuesta_datos.zip", Typo: "00"},
	}
	got := downloadableFile(files)
	if got.Name != "propuesta_datos.zip" {
		t.Fatalf("downloadableFile() = %q", got.Name)
	}
}
