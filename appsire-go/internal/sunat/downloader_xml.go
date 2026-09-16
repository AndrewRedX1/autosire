package sunat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
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

// ProbeXMLFallback comprueba el endpoint alternativo con un comprobante real.
// El motor solo lo usa cuando consultacpe rechazo el token, para no lanzar un
// lote completo contra un recurso que tampoco esta autorizado.
func (c *SunatClient) ProbeXMLFallback(ctx context.Context, comp Comprobante) error {
	file, err := c.DownloadXMLFallback(ctx, comp)
	if err != nil {
		return err
	}
	return ValidateContentIntegrity(file.Content, DescargaXML)
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
		return nil, selectXMLDownloadError(primaryErr, fallbackErr)
	}

	file, parseErr := c.processXmlResponseBody(fallbackBytes, comp, tipoOrig)
	if parseErr != nil {
		return nil, selectXMLDownloadError(primaryErr, parseErr)
	}

	return file, nil
}

// selectXMLDownloadError conserva la precedencia del macro: el error
// transitorio del respaldo gana; si no, se conserva el transitorio primario
// para que el motor reintente la fila. Un error definitivo del fallback no
// debe ocultar un 429/5xx/timeout ocurrido en consultacpe.
func selectXMLDownloadError(primaryErr, fallbackErr error) error {
	if primaryErr == nil {
		return fmt.Errorf("descargando XML desde controlcpe: %w", fallbackErr)
	}
	// En el cliente Go consultacpe puede cerrar la respuesta con EOF para un
	// comprobante inexistente. Si controlcpe sí respondió 301/302, esa respuesta
	// de negocio es concluyente y evita reintentar el mismo documento durante
	// varios minutos. Otros 429/5xx/timeouts conservan la prioridad transitoria.
	if isDefinitiveXMLFallbackAfterEOF(primaryErr, fallbackErr) {
		return fmt.Errorf("descargando XML desde controlcpe: %w", fallbackErr)
	}
	if IsTransientDownloadError(fallbackErr) {
		return fmt.Errorf("descargando XML desde controlcpe: %w", fallbackErr)
	}
	if IsTransientDownloadError(primaryErr) {
		return fmt.Errorf("descargando XML desde consultacpe: %w", primaryErr)
	}
	return fmt.Errorf(
		"descargando XML (primario: %v, respaldo: %w)",
		primaryErr,
		fallbackErr,
	)
}

func isDefinitiveXMLFallbackAfterEOF(primaryErr, fallbackErr error) bool {
	if !errors.Is(primaryErr, io.EOF) && !errors.Is(primaryErr, io.ErrUnexpectedEOF) {
		return false
	}
	var statusErr *HTTPStatusError
	if !errors.As(fallbackErr, &statusErr) {
		return false
	}
	body := strings.ToLower(strings.ReplaceAll(statusErr.Body, " ", ""))
	return strings.Contains(body, `"coderror":"301"`) ||
		strings.Contains(body, `"coderror":"302"`)
}

// processXmlResponseBody procesa la respuesta que puede ser JSON Base64 o binario directo (ZIP/XML)
func (c *SunatClient) processXmlResponseBody(bodyBytes []byte, comp Comprobante, tipo string) (*DownloadedFile, error) {
	defaultName := fmt.Sprintf("%s-%s-%s-%s.xml", comp.RUC, tipo, comp.Serie, comp.Numero)

	// Caso A: Si es directamente un archivo ZIP en binario (magic bytes PK)
	if IsZipContent(bodyBytes) {
		xmlName, xmlContent, err := ExtractFileFromZip(bodyBytes, ".xml")
		if err != nil {
			return nil, fmt.Errorf("el ZIP de SUNAT no contiene un XML legible: %w", err)
		}
		if !IsXMLContent(xmlContent) {
			return nil, fmt.Errorf("el ZIP de SUNAT contiene un archivo que no es XML")
		}
		return &DownloadedFile{
			FileName:    xmlFileName(xmlName, defaultName),
			ContentType: "application/xml",
			Content:     xmlContent,
			IsZip:       false,
		}, nil
	}

	// Caso B: XML directo, incluido UTF-8/UTF-16 con BOM.
	if IsXMLContent(bodyBytes) {
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
		xmlName, xmlContent, err := ExtractFileFromZip(parsed.Content, ".xml")
		if err != nil {
			return nil, fmt.Errorf("el ZIP Base64 de SUNAT no contiene un XML legible: %w", err)
		}
		if !IsXMLContent(xmlContent) {
			return nil, fmt.Errorf("el ZIP Base64 de SUNAT contiene un archivo que no es XML")
		}
		parsed.FileName = xmlFileName(xmlName, defaultName)
		parsed.Content = xmlContent
		parsed.IsZip = false
	} else if !IsXMLContent(parsed.Content) {
		return nil, fmt.Errorf("SUNAT respondió contenido Base64 que no es XML ni ZIP")
	}

	parsed.FileName = xmlFileName(parsed.FileName, defaultName)
	parsed.ContentType = "application/xml"

	return parsed, nil
}

func xmlFileName(candidate, fallback string) string {
	candidate = filepath.Base(strings.TrimSpace(candidate))
	if candidate == "." || candidate == "" || !strings.EqualFold(filepath.Ext(candidate), ".xml") {
		return fallback
	}
	return candidate
}
