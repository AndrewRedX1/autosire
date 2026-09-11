package sunat

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// NormalizeTipo normaliza el tipo de comprobante a 2 dígitos (ej: "1" -> "01", "F7" -> "07", "B7" -> "07")
func NormalizeTipo(tipo string) string {
	tipo = strings.TrimSpace(tipo)
	if strings.EqualFold(tipo, "F7") || strings.EqualFold(tipo, "B7") {
		return "07"
	}
	if strings.EqualFold(tipo, "F8") || strings.EqualFold(tipo, "B8") {
		return "08"
	}
	if len(tipo) == 1 {
		return "0" + tipo
	}
	return tipo
}

// DownloadPDF descarga la representación impresa (PDF) de un comprobante desde SUNAT
func (c *SunatClient) DownloadPDF(ctx context.Context, comp Comprobante) (*DownloadedFile, error) {
	tipo := QueryTipo(comp.Tipo, comp.Serie)
	libro := comp.Libro
	if libro == "" {
		libro = "1" // Default Ventas
	}

	// Endpoint: /consultacpe/comprobantes/{RUC}-{tipo}-{serie}-{numero}-{libro}/01
	urlStr := fmt.Sprintf("%s/consultacpe/comprobantes/%s-%s-%s-%s-%s/01",
		c.baseURL,
		comp.RUC,
		tipo,
		comp.Serie,
		comp.Numero,
		libro,
	)

	_, bodyBytes, err := c.doRequestWithRetries(ctx, http.MethodGet, urlStr)
	if err != nil {
		return nil, fmt.Errorf("error descargando PDF de SUNAT (%s-%s-%s): %w", comp.RUC, comp.Serie, comp.Numero, err)
	}

	// SUNAT devuelve un JSON {"nomArchivo": "...", "valArchivo": "base64..."}
	// o a veces directamente el stream binario del PDF
	if len(bodyBytes) >= 4 && string(bodyBytes[0:4]) == "%PDF" {
		fileName := fmt.Sprintf("%s-%s-%s-%s.pdf", comp.RUC, tipo, comp.Serie, comp.Numero)
		return &DownloadedFile{
			FileName:    fileName,
			ContentType: "application/pdf",
			Content:     bodyBytes,
			IsZip:       false,
		}, nil
	}

	// Si es JSON
	downloaded, err := ParseJsonResponse(bodyBytes)
	if err != nil {
		return nil, err
	}

	// Si dentro vino un ZIP, extraer el PDF
	if downloaded.IsZip {
		innerName, innerContent, err := ExtractFileFromZip(downloaded.Content, ".pdf")
		if err == nil {
			downloaded.FileName = innerName
			downloaded.Content = innerContent
			downloaded.IsZip = false
		}
	}

	if downloaded.FileName == "" {
		downloaded.FileName = fmt.Sprintf("%s-%s-%s-%s.pdf", comp.RUC, tipo, comp.Serie, comp.Numero)
	}
	downloaded.ContentType = "application/pdf"

	return downloaded, nil
}
