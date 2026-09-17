package sunat

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// SunatCpeBaseURL base para endpoints de CPE
	SunatCpeBaseURL = "https://api-cpe.sunat.gob.pe/v1/contribuyente"

	// DefaultBrowserUserAgent emula el User-Agent del navegador para evitar que el gateway WAF de SUNAT cierre la conexión con EOF
	DefaultBrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	MaxRetriesDefault = 5
	BaseDelayMs       = 500
	MaxDelayMs        = 12000
)

// TokenProvider interfaz para obtener tokens vigentes y refrescar en caso de 401
type TokenProvider interface {
	GetValidToken(ctx context.Context) (string, error)
	InvalidateToken(token string)
	ForceRefresh(ctx context.Context) (string, error)
}

// HTTPStatusError conserva el codigo HTTP para que el motor pueda decidir si
// debe reintentar una fila, renovar un lote o detener un servicio completo.
type HTTPStatusError struct {
	StatusCode int
	Body       string
	RetryAfter time.Duration
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("SUNAT respondio HTTP %d: %s", e.StatusCode, e.Body)
}

// DownloadedFile contiene los bytes decodificados y el nombre asignado
type DownloadedFile struct {
	FileName        string
	ContentType     string
	Content         []byte
	IsZip           bool
	OriginalZip     []byte
	OriginalZipName string
	Retries         int
}

// SunatClient cliente especializado para interactuar con la API CPE de SUNAT
type SunatClient struct {
	clientMu      sync.RWMutex
	httpClient    *http.Client
	transport     *http.Transport
	timeout       time.Duration
	tokenProvider TokenProvider
	maxRetries    int
	baseURL       string
	jitterSeq     atomic.Uint64
}

// NewSunatClient mantiene hasta seis solicitudes concurrentes. Los endpoints
// CPE se usan con conexiones HTTP/1.1 aisladas, igual que ServerXMLHTTP del
// original, para que una conexión degradada no afecte comprobantes posteriores.
func NewSunatClient(tp TokenProvider, timeout time.Duration) *SunatClient {
	if timeout <= 0 {
		timeout = 35 * time.Second
	}

	client, transport := newCPEHTTPClient(timeout)
	return &SunatClient{
		httpClient:    client,
		transport:     transport,
		timeout:       timeout,
		tokenProvider: tp,
		maxRetries:    MaxRetriesDefault,
		baseURL:       SunatCpeBaseURL,
	}
}

func newCPEHTTPClient(timeout time.Duration) (*http.Client, *http.Transport) {
	dialer := &net.Dialer{
		Timeout:   8 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     false,
		DisableKeepAlives:     true,
		MaxIdleConns:          0,
		MaxIdleConnsPerHost:   0,
		MaxConnsPerHost:       6,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	return &http.Client{Transport: transport, Timeout: timeout}, transport
}

func (c *SunatClient) currentHTTPClient() *http.Client {
	c.clientMu.RLock()
	defer c.clientMu.RUnlock()
	return c.httpClient
}

// ResetTransport descarta todo estado de red compartido. Se llama al empezar
// un lote, al detectar una ola bloqueada y al cambiar de empresa.
func (c *SunatClient) ResetTransport() {
	client, transport := newCPEHTTPClient(c.timeout)
	c.clientMu.Lock()
	previous := c.transport
	c.httpClient = client
	c.transport = transport
	c.clientMu.Unlock()
	if previous != nil {
		previous.CloseIdleConnections()
	}
}

// CloseIdleConnections libera cualquier conexión remanente al cerrar el lote.
func (c *SunatClient) CloseIdleConnections() {
	c.clientMu.RLock()
	transport := c.transport
	c.clientMu.RUnlock()
	if transport != nil {
		transport.CloseIdleConnections()
	}
}

// IsTransientError determina si un código HTTP o error es recuperable mediante reintento
func IsTransientError(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout, // 408
		http.StatusTooEarly,            // 425
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// calculateBackoff calcula la espera exponencial con jitter
// min(12s, 500ms * 2^(attempt-1)) + jitter
func (c *SunatClient) calculateBackoff(attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt-1))
	delayMs := float64(BaseDelayMs) * exp
	if delayMs > float64(MaxDelayMs) {
		delayMs = float64(MaxDelayMs)
	}

	// Secuencia determinista y segura entre goroutines para repartir los reintentos.
	jitter := float64(100 + (c.jitterSeq.Add(137) % 400))
	totalMs := delayMs + jitter

	return time.Duration(totalMs) * time.Millisecond
}

// doRequestWithRetries reintenta fallos transitorios. Los 401 se devuelven al
// motor para renovar una sola vez todo el grupo afectado, igual que la macro.
func (c *SunatClient) doRequestWithRetries(ctx context.Context, method, urlStr string) (*http.Response, []byte, error) {
	var lastErr error
	var lastStatusCode int

	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		token, err := c.tokenProvider.GetValidToken(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("error obteniendo token SUNAT: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("error creando petición: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("User-Agent", DefaultBrowserUserAgent)

		resp, err := c.currentHTTPClient().Do(req)
		if err != nil {
			lastErr = err
			// Error de red/conexión: reintentar
			if err := waitForRetry(ctx, c.calculateBackoff(attempt)); err != nil {
				return nil, nil, err
			}
			continue
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if readErr != nil {
			lastErr = readErr
			if err := waitForRetry(ctx, c.calculateBackoff(attempt)); err != nil {
				return nil, nil, err
			}
			continue
		}

		lastStatusCode = resp.StatusCode

		// La renovacion se coordina a nivel de lote, no por goroutine.
		if resp.StatusCode == http.StatusUnauthorized {
			return resp, bodyBytes, &HTTPStatusError{
				StatusCode: resp.StatusCode,
				Body:       strings.TrimSpace(string(bodyBytes)),
			}
		}

		// Caso éxito (200 OK)
		if resp.StatusCode == http.StatusOK {
			sample := strings.ToLower(string(bodyBytes[:min(len(bodyBytes), 512)]))
			if strings.Contains(sample, "<!doctype html") ||
				strings.Contains(sample, "<html") ||
				strings.Contains(sample, "ha ocurrido un error") ||
				strings.Contains(sample, "servicio no disponible") {
				lastErr = errors.New("SUNAT devolvió mensaje HTML de error temporal con código 200")
				if err := waitForRetry(ctx, c.calculateBackoff(attempt)); err != nil {
					return nil, nil, err
				}
				continue
			}
			return resp, bodyBytes, nil
		}

		// Caso error no transitorio (ej: 404 No encontrado, 400 Bad Request, 403 Prohibido)
		if !IsTransientError(resp.StatusCode) {
			return resp, bodyBytes, &HTTPStatusError{
				StatusCode: resp.StatusCode,
				Body:       strings.TrimSpace(string(bodyBytes)),
			}
		}

		// Si es transitorio (429, 500, 502, 503, etc.), esperar backoff
		lastErr = &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(bodyBytes)),
		}
		if err := waitForRetry(ctx, c.calculateBackoff(attempt)); err != nil {
			return nil, nil, err
		}
	}

	return nil, nil, fmt.Errorf(
		"fallo tras %d reintentos (ultimo status %d): %w",
		c.maxRetries,
		lastStatusCode,
		lastErr,
	)
}

// doRequestOnce ejecuta una sola petición. La descarga XML controla su pasada
// de recuperación en el motor de lote, evitando multiplicar reintentos internos.
func (c *SunatClient) doRequestOnce(ctx context.Context, method, urlStr string) (*http.Response, []byte, error) {
	token, err := c.tokenProvider.GetValidToken(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("error obteniendo token SUNAT: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error creando petición: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", DefaultBrowserUserAgent)

	resp, err := c.currentHTTPClient().Do(req)
	if err != nil {
		return nil, nil, err
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return resp, nil, readErr
	}
	if resp.StatusCode != http.StatusOK {
		return resp, body, &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()),
		}
	}
	sample := strings.ToLower(string(body[:min(len(body), 512)]))
	if strings.Contains(sample, "<!doctype html") || strings.Contains(sample, "<html") ||
		strings.Contains(sample, "ha ocurrido un error") || strings.Contains(sample, "servicio no disponible") {
		return resp, body, errors.New("SUNAT devolvió mensaje HTML de error temporal con código 200")
	}
	return resp, body, nil
}

// RefreshToken fuerza una unica renovacion coordinada para un grupo de filas.
func (c *SunatClient) RefreshToken(ctx context.Context) error {
	if _, err := c.tokenProvider.ForceRefresh(ctx); err != nil {
		return fmt.Errorf("renovando token SUNAT: %w", err)
	}
	return nil
}

// ProbeConsultacpe confirma si el token tiene acceso al recurso consultacpe.
// Un 404/422 prueba que el gateway autorizo la llamada aunque el documento no exista.
func (c *SunatClient) ProbeConsultacpe(ctx context.Context, comp Comprobante) error {
	tipo := QueryTipo(comp.Tipo, comp.Serie)
	libro := comp.Libro
	if libro == "" {
		libro = "1"
	}

	urlStr := fmt.Sprintf(
		"%s/consultacpe/comprobantes/%s-%s-%s-%s-%s/02",
		c.baseURL,
		comp.RUC,
		tipo,
		comp.Serie,
		comp.Numero,
		libro,
	)

	// El preflight solo clasifica el acceso. Sus reintentos pertenecen al lote y
	// no deben retener un worker ni consumir todo el presupuesto del documento.
	_, _, err := c.doRequestOnce(ctx, http.MethodGet, urlStr)
	if err == nil {
		return nil
	}

	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) && (statusErr.StatusCode == 404 || statusErr.StatusCode == 422) {
		return nil
	}
	return fmt.Errorf("verificando permiso consultacpe: %w", err)
}

// HTTPStatus extrae el codigo HTTP preservado en la cadena de error.
func HTTPStatus(err error) (int, bool) {
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) {
		return 0, false
	}
	return statusErr.StatusCode, true
}

// RetryAfter extrae la espera solicitada por SUNAT de un error HTTP.
func RetryAfter(err error) time.Duration {
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.RetryAfter < 0 {
		return 0
	}
	return statusErr.RetryAfter
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	date, err := http.ParseTime(value)
	if err != nil || !date.After(now) {
		return 0
	}
	return date.Sub(now)
}

// IsTransientDownloadError indica si conviene incluir una fila en los barridos.
func IsTransientDownloadError(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(strings.ToLower(err.Error()), "sin cdr en sunat") {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if status, ok := HTTPStatus(err); ok {
		return IsTransientError(status)
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "timeout") ||
		strings.Contains(message, "unexpected eof") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "server closed") ||
		strings.Contains(message, "connection") ||
		strings.Contains(message, "conex") ||
		strings.Contains(message, "temporal") ||
		strings.Contains(message, "unavailable") ||
		strings.Contains(message, "gateway") ||
		strings.Contains(message, "no se pudo leer") ||
		strings.Contains(message, "archivo vac") ||
		strings.Contains(message, "respuesta vac") ||
		strings.Contains(message, "base64") ||
		strings.Contains(message, "zip") ||
		strings.Contains(message, "json")
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// IsZipContent verifica si un slice de bytes comienza con la firma mágica de ZIP (PK\x03\x04 o PK\x05\x06)
func IsZipContent(data []byte) bool {
	return len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4B
}

// ExtractFileFromZip descomprime y extrae el primer archivo que coincida con la extensión deseada (.xml o .pdf)
func ExtractFileFromZip(zipData []byte, targetExt string) (string, []byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", nil, fmt.Errorf("error leyendo archivo ZIP: %w", err)
	}

	targetExt = strings.ToLower(targetExt)
	for _, f := range reader.File {
		if strings.HasSuffix(strings.ToLower(f.Name), targetExt) {
			rc, err := f.Open()
			if err != nil {
				return "", nil, fmt.Errorf("error abriendo archivo %s dentro del zip: %w", f.Name, err)
			}
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", nil, fmt.Errorf("error leyendo contenido de %s: %w", f.Name, err)
			}
			return f.Name, content, nil
		}
	}

	// Si no encontró por extensión, devolver el primer archivo disponible
	if len(reader.File) > 0 {
		f := reader.File[0]
		rc, err := f.Open()
		if err == nil {
			content, _ := io.ReadAll(rc)
			rc.Close()
			return f.Name, content, nil
		}
	}

	return "", nil, errors.New("no se encontró ningún archivo válido dentro del ZIP")
}

// ParseJsonResponse decodifica el objeto JSON nomArchivo/valArchivo que devuelve SUNAT
func ParseJsonResponse(bodyBytes []byte) (*DownloadedFile, error) {
	var cpeResp ApiResponseCpe
	if err := json.Unmarshal(bodyBytes, &cpeResp); err != nil {
		var responses []ApiResponseCpe
		if arrayErr := json.Unmarshal(bodyBytes, &responses); arrayErr != nil || len(responses) == 0 {
			return nil, fmt.Errorf("la respuesta no es un JSON válido: %w", err)
		}
		cpeResp = responses[0]
	}

	if cpeResp.ValArchivo == "" {
		if cpeResp.MsgRespuesta != "" {
			return nil, fmt.Errorf("SUNAT reportó mensaje: %s", cpeResp.MsgRespuesta)
		}
		return nil, errors.New("la respuesta de SUNAT no contiene valArchivo")
	}

	decoded, err := base64.StdEncoding.DecodeString(cpeResp.ValArchivo)
	if err != nil {
		return nil, fmt.Errorf("error decodificando base64 de valArchivo: %w", err)
	}

	return &DownloadedFile{
		FileName: cpeResp.NomArchivo,
		Content:  decoded,
		IsZip:    IsZipContent(decoded),
	}, nil
}

// QueryTipo reproduce el código especial que consultacpe exige para notas.
// El fallback controlcpe continúa usando el código original 07/08.
func QueryTipo(tipo, serie string) string {
	tipo = NormalizeTipo(tipo)
	serie = strings.ToUpper(strings.TrimSpace(serie))

	switch tipo {
	case "07":
		if isFacturaSerie(serie) {
			return "F7"
		}
		return "B7"
	case "08":
		if isFacturaSerie(serie) {
			return "F8"
		}
		return "B8"
	default:
		return tipo
	}
}

func isFacturaSerie(serie string) bool {
	if strings.HasPrefix(serie, "F") {
		return true
	}
	return strings.HasPrefix(serie, "E") && !strings.HasPrefix(serie, "EB")
}

// NormalizeNumero elimina el formato visual de miles que Excel aplica a
// correlativos enteros. SUNAT espera el valor sin separadores.
func NormalizeNumero(numero string) string {
	numero = strings.ReplaceAll(strings.TrimSpace(numero), " ", "")
	numero = strings.ReplaceAll(numero, ",", "")

	if value, err := strconv.ParseInt(strings.TrimSuffix(numero, ".0"), 10, 64); err == nil {
		return strconv.FormatInt(value, 10)
	}
	return numero
}
