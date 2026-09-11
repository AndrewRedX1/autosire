package sunat

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// DownloadXML descarga el archivo XML (o ZIP que contiene XML) de SUNAT
// Intenta primero el endpoint de consulta CPE (02) y, si falla o no se encuentra, usa el endpoint de controlcpe/consultaxml como fallback
func (c *SunatClient) DownloadXML(ctx context.Context, comp Comprobante) (*DownloadedFile, error) {
	tipoOrig := NormalizeTipo(comp.Tipo)
	tipoConsulta := QueryTipo(comp.Tipo, comp.Serie)
	libro := comp.Libro
	if libro == "" {
		libro = "1"
	}

	// 1. Intento primario: /consultacpe/comprobantes/{RUC}-{tipo}-{serie}-{numero}-{libro}/02
	primaryURL := fmt.Sprintf("%s/consultacpe/comprobantes/%s-%s-%s-%s-%s/02",
		c.baseURL,
		comp.RUC,
		tipoConsulta,
		comp.Serie,
		comp.Numero,
		libro,
	)

	_, bodyBytes, err := c.doRequestOnce(ctx, http.MethodGet, primaryURL)
	if err == nil && len(bodyBytes) > 0 {
		file, parseErr := c.processXmlResponseBody(bodyBytes, comp, tipoOrig)
		if parseErr == nil && file != nil {
			return file, nil
		}
		err = parseErr
	}

	return c.downloadXMLFallback(ctx, comp, tipoOrig, err)
}

// DownloadXMLFallback evita consultacpe cuando el preflight ya demostro que
// ese recurso no esta autorizado para las credenciales actuales.
func (c *SunatClient) DownloadXMLFallback(ctx context.Context, comp Comprobante) (*DownloadedFile, error) {
	return c.downloadXMLFallback(ctx, comp, NormalizeTipo(comp.Tipo), nil)
}

func (c *SunatClient) downloadXMLFallback(
	ctx context.Context,
	comp Comprobante,
	tipoOrig string,
	primaryErr error,
) (*DownloadedFile, error) {
	fallbackURL := fmt.Sprintf("%s/controlcpe/consultaxml/%s-%s-%s-%s",
		c.baseURL,
		comp.RUC,
		tipoOrig,
		comp.Serie,
		comp.Numero,
	)

	_, fallbackBytes, fallbackErr := c.doRequestOnce(ctx, http.MethodGet, fallbackURL)
	if fallbackErr != nil {
		if primaryErr != nil {
			return nil, fmt.Errorf(
				"descargando XML (primario: %v, respaldo: %w)",
				primaryErr,
				fallbackErr,
			)
		}
		return nil, fmt.Errorf("descargando XML desde controlcpe: %w", fallbackErr)
	}

	file, parseErr := c.processXmlResponseBody(fallbackBytes, comp, tipoOrig)
	if parseErr != nil {
		return nil, fmt.Errorf("error procesando respuesta de XML de SUNAT: %w", parseErr)
	}

	return file, nil
}

// processXmlResponseBody procesa la respuesta que puede ser JSON Base64 o binario directo (ZIP/XML)
func (c *SunatClient) processXmlResponseBody(bodyBytes []byte, comp Comprobante, tipo string) (*DownloadedFile, error) {
	defaultName := fmt.Sprintf("%s-%s-%s-%s.xml", comp.RUC, tipo, comp.Serie, comp.Numero)

	// Caso A: Si es directamente un archivo ZIP en binario (magic bytes PK)
	if IsZipContent(bodyBytes) {
		// Extraer el archivo XML de adentro
		xmlName, xmlContent, err := ExtractFileFromZip(bodyBytes, ".xml")
		if err == nil {
			return &DownloadedFile{
				FileName:        xmlName,
				ContentType:     "application/xml",
				Content:         xmlContent,
				IsZip:           false,
				OriginalZip:     bodyBytes,
				OriginalZipName: strings.TrimSuffix(defaultName, ".xml") + ".zip",
			}, nil
		}
		// Si no se pudo extraer o es un ZIP opaco, devolver el zip
		return &DownloadedFile{
			FileName:    strings.TrimSuffix(defaultName, ".xml") + ".zip",
			ContentType: "application/zip",
			Content:     bodyBytes,
			IsZip:       true,
		}, nil
	}

	// Caso B: Si es texto XML directo (comienza con <?xml o <)
	trimmed := strings.TrimSpace(string(bodyBytes))
	if strings.HasPrefix(trimmed, "<?xml") || strings.HasPrefix(trimmed, "<") {
		return &DownloadedFile{
			FileName:    defaultName,
			ContentType: "application/xml",
			Content:     bodyBytes,
			IsZip:       false,
		}, nil
	}

	// Caso C: Es un JSON con {"nomArchivo": "...", "valArchivo": "..."}
	parsed, err := ParseJsonResponse(bodyBytes)
	if err != nil {
		return nil, err
	}

	// Si dentro del Base64 vino un ZIP, extraer el XML
	if parsed.IsZip {
		originalZip := parsed.Content
		originalName := parsed.FileName
		xmlName, xmlContent, err := ExtractFileFromZip(parsed.Content, ".xml")
		if err == nil {
			parsed.FileName = xmlName
			parsed.Content = xmlContent
			parsed.IsZip = false
			parsed.OriginalZip = originalZip
			if strings.HasSuffix(strings.ToLower(originalName), ".zip") {
				parsed.OriginalZipName = originalName
			} else {
				parsed.OriginalZipName = strings.TrimSuffix(defaultName, ".xml") + ".zip"
			}
		}
	}

	if parsed.FileName == "" {
		parsed.FileName = defaultName
	}
	parsed.ContentType = "application/xml"

	return parsed, nil
}
