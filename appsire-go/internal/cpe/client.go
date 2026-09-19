package cpe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	CpeScope         = "https://api.sunat.gob.pe/v1/contribuyente/contribuyentes"
	DefaultTimeout   = 20 * time.Second
	TokenEndpointFmt = "https://api-seguridad.sunat.gob.pe/v1/clientesextranet/%s/oauth2/token/"
	ServiceEndpoint  = "https://api.sunat.gob.pe/v1/contribuyente/contribuyentes/%s/validarcomprobante"
)

var (
	EstadoCpMap = map[string]string{
		"0": "NO EXISTE",
		"1": "ACEPTADO",
		"2": "ANULADO",
		"3": "AUTORIZADO",
		"4": "NO AUTORIZADO",
	}

	EstadoRucMap = map[string]string{
		"00": "ACTIVO",
		"01": "BAJA PROVISIONAL",
		"02": "BAJA PROV. POR OFICIO",
		"03": "SUSPENSION TEMPORAL",
		"10": "BAJA DEFINITIVA",
		"11": "BAJA DE OFICIO",
		"22": "INHABILITADO-VENT.UNICA",
	}

	CondDomiMap = map[string]string{
		"00": "HABIDO",
		"09": "PENDIENTE",
		"11": "POR VERIFICAR",
		"12": "NO HABIDO",
		"20": "NO HALLADO",
	}
)

// VoucherQuery representa los datos exactos requeridos por la API de SUNAT
type VoucherQuery struct {
	NumRUC       string  `json:"numRuc"`
	CodComp      string  `json:"codComp"`
	NumeroSerie  string  `json:"numeroSerie"`
	Numero       string  `json:"numero"`
	FechaEmision string  `json:"fechaEmision"`
	Monto        float64 `json:"monto,omitempty"`
}

// ValidationResult representa el resultado de consultar un comprobante
type ValidationResult struct {
	Success           bool     `json:"success"`
	EstadoComprobante string   `json:"estado_comprobante"`
	EstadoRuc         string   `json:"estado_ruc"`
	CondicionDomi     string   `json:"condicion_domicilio"`
	Observaciones     []string `json:"observaciones"`
	ObservacionTexto  string   `json:"observacion_texto"`
	ErrorCode         string   `json:"error_code,omitempty"`
	Message           string   `json:"message,omitempty"`
	RawResponse       string   `json:"-"`
}

type cachedToken struct {
	Token     string
	ExpiresAt time.Time
}

// Client gestiona tokens y consultas de validez de comprobantes electrónicos
type Client struct {
	httpClient *http.Client
	tokensMu   sync.Mutex
	tokens     map[string]cachedToken // key = clientId
	// URLs configurables para pruebas unitarias
	tokenURLTemplate   string
	serviceURLTemplate string
}

// NewClient crea una nueva instancia del cliente CPE
func NewClient() *Client {
	transport := &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   DefaultTimeout,
		},
		tokens:             make(map[string]cachedToken),
		tokenURLTemplate:   TokenEndpointFmt,
		serviceURLTemplate: ServiceEndpoint,
	}
}

// GetToken obtiene un token vigente o solicita uno nuevo a SUNAT
func (c *Client) GetToken(ctx context.Context, clientID, clientSecret string) (string, error) {
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("faltan credenciales API (Client ID / Client Secret)")
	}

	c.tokensMu.Lock()
	if cached, ok := c.tokens[clientID]; ok {
		// Margen de 2 minutos antes de expirar
		if time.Now().Before(cached.ExpiresAt.Add(-2 * time.Minute)) {
			c.tokensMu.Unlock()
			return cached.Token, nil
		}
	}
	c.tokensMu.Unlock()

	// Solicitar token a SUNAT
	token, expiresIn, err := c.requestToken(ctx, clientID, clientSecret)
	if err != nil {
		return "", err
	}

	c.tokensMu.Lock()
	c.tokens[clientID] = cachedToken{
		Token:     token,
		ExpiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	c.tokensMu.Unlock()

	return token, nil
}

func (c *Client) requestToken(ctx context.Context, clientID, clientSecret string) (string, int, error) {
	tokenURL := fmt.Sprintf(c.tokenURLTemplate, url.PathEscape(clientID))

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", CpeScope)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("creando petición de token: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("error de conexión al solicitar token SUNAT: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
			Message          string `json:"message"`
		}
		_ = json.Unmarshal(bodyBytes, &errResp)

		rawDesc := errResp.ErrorDescription
		if rawDesc == "" {
			rawDesc = errResp.Message
		}
		if rawDesc == "" {
			rawDesc = string(bodyBytes)
		}

		if strings.Contains(strings.ToLower(rawDesc), "cliente no autorizado") || errResp.Error == "unauthorized_client" {
			return "", 0, fmt.Errorf("Credencial no autorizada por SUNAT (cliente no autorizado). La aplicación registrada en SUNAT con este Client ID no cuenta con permisos para 'Consulta Integrada de Comprobantes' (es posible que haya sido generada únicamente para SIRE). En SUNAT SOL registre una credencial con alcance de Consulta de Comprobantes y regístrela en Empresas ▸ Editar ▸ Credenciales Específicas de Validación CPE.")
		}

		switch resp.StatusCode {
		case http.StatusBadRequest:
			return "", 0, fmt.Errorf("SUNAT 400: %s. Revise el Client ID o Client Secret de la empresa", rawDesc)
		case http.StatusUnauthorized, http.StatusForbidden:
			return "", 0, fmt.Errorf("SUNAT %d: Credenciales rechazadas (%s). Verifique que estén habilitadas para Consulta de Comprobantes en SUNAT SOL", resp.StatusCode, rawDesc)
		default:
			return "", 0, fmt.Errorf("SUNAT %d: Error al generar token (%s)", resp.StatusCode, rawDesc)
		}
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}

	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		return "", 0, fmt.Errorf("decodificando token de SUNAT: %w", err)
	}

	if parsed.AccessToken == "" {
		if parsed.ErrorDesc != "" {
			return "", 0, fmt.Errorf("SUNAT error: %s (%s)", parsed.ErrorDesc, parsed.Error)
		}
		return "", 0, fmt.Errorf("SUNAT no devolvió access_token")
	}

	expiresIn := parsed.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}

	return parsed.AccessToken, expiresIn, nil
}

// ValidateVoucher consulta la validez de un comprobante ante la API oficial de SUNAT
func (c *Client) ValidateVoucher(ctx context.Context, rucConsultante string, query VoucherQuery, token string) (*ValidationResult, error) {
	rucConsultante = strings.TrimSpace(rucConsultante)
	if rucConsultante == "" {
		return nil, fmt.Errorf("falta el RUC consultante de la empresa")
	}

	serviceURL := fmt.Sprintf(c.serviceURLTemplate, rucConsultante)

	// Preparar payload JSON
	reqMap := map[string]interface{}{
		"numRuc":       strings.TrimSpace(query.NumRUC),
		"codComp":      strings.TrimSpace(query.CodComp),
		"numeroSerie":  strings.TrimSpace(query.NumeroSerie),
		"numero":       strings.TrimSpace(query.Numero),
		"fechaEmision": strings.TrimSpace(query.FechaEmision),
	}
	if query.Monto > 0 {
		reqMap["monto"] = fmt.Sprintf("%.2f", query.Monto)
	}

	reqBody, err := json.Marshal(reqMap)
	if err != nil {
		return nil, fmt.Errorf("serializando petición JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serviceURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creando petición HTTP: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error de conexión con SUNAT: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("leyendo respuesta de SUNAT: %w", err)
	}

	return ParseValidationResponse(resp.StatusCode, respBytes)
}

// ParseValidationResponse interpreta la respuesta JSON y códigos de SUNAT
func ParseValidationResponse(statusCode int, respBytes []byte) (*ValidationResult, error) {
	var raw struct {
		Success   *bool           `json:"success"`
		Message   string          `json:"message"`
		ErrorCode string          `json:"errorCode"`
		Data      json.RawMessage `json:"data"`
	}

	res := &ValidationResult{
		RawResponse: string(respBytes),
	}

	if err := json.Unmarshal(respBytes, &raw); err != nil {
		res.Success = false
		res.EstadoComprobante = "ERROR"
		res.ObservacionTexto = fmt.Sprintf("HTTP %d: Respuesta no JSON de SUNAT", statusCode)
		return res, nil
	}

	res.ErrorCode = raw.ErrorCode
	res.Message = raw.Message
	if raw.Success != nil {
		res.Success = *raw.Success
	}

	// Si no hay nodo "data"
	if len(raw.Data) == 0 || string(raw.Data) == "null" {
		if statusCode == http.StatusOK {
			res.EstadoComprobante = "SIN DATOS"
		} else {
			res.EstadoComprobante = "ERROR"
		}
		res.ObservacionTexto = formatObsMessage(raw.Message, raw.ErrorCode, statusCode)
		return res, nil
	}

	var dataObj struct {
		EstadoCp      interface{} `json:"estadoCp"`
		EstadoRuc     interface{} `json:"estadoRuc"`
		CondDomiRuc   interface{} `json:"condDomiRuc"`
		Observaciones interface{} `json:"observaciones"`
	}

	if err := json.Unmarshal(raw.Data, &dataObj); err != nil {
		res.EstadoComprobante = "ERROR"
		res.ObservacionTexto = "Error al decodificar 'data' de SUNAT"
		return res, nil
	}

	codeCp := fmt.Sprintf("%v", dataObj.EstadoCp)
	codeRuc := fmt.Sprintf("%v", dataObj.EstadoRuc)
	codeCond := fmt.Sprintf("%v", dataObj.CondDomiRuc)

	res.EstadoComprobante = mapCode(EstadoCpMap, codeCp)
	res.EstadoRuc = mapCode(EstadoRucMap, codeRuc)
	res.CondicionDomi = mapCode(CondDomiMap, codeCond)

	// Extraer observaciones
	res.Observaciones = extractObservations(dataObj.Observaciones)

	// Consolidar texto de observación
	var obsParts []string
	if raw.Message != "" && !strings.EqualFold(strings.TrimSpace(raw.Message), "Operation Success") {
		obsParts = append(obsParts, raw.Message)
	}
	if len(res.Observaciones) > 0 {
		obsParts = append(obsParts, strings.Join(res.Observaciones, "; "))
	}
	if raw.ErrorCode != "" {
		obsParts = append(obsParts, "Cód: "+raw.ErrorCode)
	}

	res.ObservacionTexto = strings.Join(obsParts, " | ")
	return res, nil
}

func mapCode(m map[string]string, code string) string {
	code = strings.TrimSpace(code)
	if code == "" || code == "<nil>" {
		return ""
	}
	if val, ok := m[code]; ok {
		return val
	}
	return code
}

func extractObservations(raw interface{}) []string {
	if raw == nil {
		return nil
	}
	var res []string
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if str := strings.TrimSpace(fmt.Sprintf("%v", item)); str != "" && str != "<nil>" {
				res = append(res, str)
			}
		}
	case string:
		if s := strings.TrimSpace(v); s != "" {
			res = append(res, s)
		}
	}
	return res
}

func formatObsMessage(msg, code string, status int) string {
	var parts []string
	if msg != "" && !strings.EqualFold(strings.TrimSpace(msg), "Operation Success") {
		parts = append(parts, msg)
	}
	if code != "" {
		parts = append(parts, "Cód: "+code)
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("HTTP %d", status))
	}
	return strings.Join(parts, " | ")
}
