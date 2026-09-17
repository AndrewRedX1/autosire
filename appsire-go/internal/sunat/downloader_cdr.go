package sunat

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

// DownloadCDR descarga la Constancia de Recepción (CDR) desde SUNAT (recurso 03).
// Utiliza doRequestOnce para evitar retener trabajadores con reintentos en bucle.
// Los fallos transitorios se recuperan en las pasadas de barrido del lote.
func (c *SunatClient) DownloadCDR(ctx context.Context, comp Comprobante) (*DownloadedFile, error) {
	tipo := QueryTipo(comp.Tipo, comp.Serie)
	libro := comp.Libro
	if libro == "" {
		libro = "1"
	}

	// Endpoint oficial: /consultacpe/comprobantes/{RUC}-{tipo}-{serie}-{numero}-{libro}/03
	urlStr := fmt.Sprintf("%s/consultacpe/comprobantes/%s-%s-%s-%s-%s/03",
		c.baseURL,
		comp.RUC,
		tipo,
		comp.Serie,
		comp.Numero,
		libro,
	)

	_, bodyBytes, err := c.doRequestOnce(ctx, http.MethodGet, urlStr)
	if err != nil {
		if status, ok := HTTPStatus(err); ok {
			if status == http.StatusNotFound || status == http.StatusUnprocessableEntity {
				return nil, fmt.Errorf("comprobante sin CDR en SUNAT (HTTP %d)", status)
			}
		}
		return nil, fmt.Errorf("error descargando CDR de SUNAT (%s-%s-%s): %w", comp.RUC, comp.Serie, comp.Numero, err)
	}

	defaultZipName := fmt.Sprintf("R-%s-%s-%s-%s.zip", comp.RUC, tipo, comp.Serie, comp.Numero)

	// Caso 1: Archivo binario ZIP directo
	if IsZipContent(bodyBytes) {
		return &DownloadedFile{
			FileName:    defaultZipName,
			ContentType: "application/zip",
			Content:     bodyBytes,
			IsZip:       true,
		}, nil
	}

	// Caso 2: Respuesta JSON Base64 (nomArchivo / valArchivo)
	downloaded, err := ParseJsonResponse(bodyBytes)
	if err != nil {
		// Si no es JSON pero es XML directo
		trimmed := strings.TrimSpace(string(bodyBytes))
		if strings.HasPrefix(trimmed, "<?xml") || strings.HasPrefix(trimmed, "<") {
			return &DownloadedFile{
				FileName:    fmt.Sprintf("R-%s-%s-%s-%s.xml", comp.RUC, tipo, comp.Serie, comp.Numero),
				ContentType: "application/xml",
				Content:     bodyBytes,
				IsZip:       false,
			}, nil
		}
		return nil, err
	}

	if downloaded.IsZip {
		downloaded.FileName = defaultZipName
	} else if downloaded.FileName == "" {
		downloaded.FileName = fmt.Sprintf("R-%s-%s-%s-%s.xml", comp.RUC, tipo, comp.Serie, comp.Numero)
	} else {
		downloaded.FileName = normalizeCDRFileName(downloaded.FileName, defaultZipName)
	}

	return downloaded, nil
}

func normalizeCDRFileName(candidate, defaultName string) string {
	candidate = filepath.Base(strings.TrimSpace(candidate))
	if candidate == "." || candidate == "" {
		return defaultName
	}
	lower := strings.ToLower(candidate)
	if strings.HasPrefix(lower, "r-") || strings.HasSuffix(lower, "-cdr.zip") || strings.HasSuffix(lower, "-cdr.xml") {
		return candidate
	}
	return "R-" + candidate
}
