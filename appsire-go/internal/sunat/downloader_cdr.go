package sunat

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// DownloadCDR descarga la Constancia de Recepción (CDR) desde SUNAT (recurso 03)
func (c *SunatClient) DownloadCDR(ctx context.Context, comp Comprobante) (*DownloadedFile, error) {
	tipo := QueryTipo(comp.Tipo, comp.Serie)
	libro := comp.Libro
	if libro == "" {
		libro = "1"
	}

	// Endpoint: /consultacpe/comprobantes/{RUC}-{tipo}-{serie}-{numero}-{libro}/03
	urlStr := fmt.Sprintf("%s/consultacpe/comprobantes/%s-%s-%s-%s-%s/03",
		c.baseURL,
		comp.RUC,
		tipo,
		comp.Serie,
		comp.Numero,
		libro,
	)

	_, bodyBytes, err := c.doRequestWithRetries(ctx, http.MethodGet, urlStr)
	if err != nil {
		return nil, fmt.Errorf("error descargando CDR de SUNAT (%s-%s-%s): %w", comp.RUC, comp.Serie, comp.Numero, err)
	}

	defaultZipName := fmt.Sprintf("R-%s-%s-%s-%s.zip", comp.RUC, tipo, comp.Serie, comp.Numero)

	// Caso 1: Archivo binario ZIP directo
	if IsZipContent(bodyBytes) {
		// En el caso de CDR, es muy común y recomendado conservar el archivo ZIP o extraer su XML interno
		// Guardamos el ZIP ya que SUNAT firma el ZIP con el hash del comprobante
		return &DownloadedFile{
			FileName:    defaultZipName,
			ContentType: "application/zip",
			Content:     bodyBytes,
			IsZip:       true,
		}, nil
	}

	// Caso 2: Es respuesta JSON Base64
	downloaded, err := ParseJsonResponse(bodyBytes)
	if err != nil {
		// Si no es JSON y es XML directo
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

	if downloaded.FileName == "" {
		if downloaded.IsZip {
			downloaded.FileName = defaultZipName
		} else {
			downloaded.FileName = fmt.Sprintf("R-%s-%s-%s-%s.xml", comp.RUC, tipo, comp.Serie, comp.Numero)
		}
	}

	return downloaded, nil
}
