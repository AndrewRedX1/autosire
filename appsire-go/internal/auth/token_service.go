package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"appsire-go/internal/sunat"

	"golang.org/x/sync/singleflight"
)

const (
	// SunatTokenBaseURL endpoint de seguridad OAuth2 de SUNAT
	SunatTokenBaseURL = "https://api-seguridad.sunat.gob.pe/v1/clientessol/%s/oauth2/token/"
	// TokenMarginExpiry margen de 5 minutos (300 segundos) para renovar el token antes de que expire
	TokenMarginExpiry = 300 * time.Second
)

// TokenService gestiona la obtención, validación y renovación de tokens de SUNAT
type TokenService struct {
	client     *http.Client
	mu         sync.RWMutex
	cachedResp *sunat.TokenResponse
	creds      sunat.SunatCredentials
	sfGroup    singleflight.Group
}

// NewTokenService inicializa un nuevo servicio de autenticación
func NewTokenService(timeout time.Duration) *TokenService {
	if timeout <= 0 {
		timeout = 25 * time.Second
	}
	return &TokenService{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// SetCredentials asigna o actualiza las credenciales activas en memoria
func (s *TokenService) SetCredentials(creds sunat.SunatCredentials) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creds = creds
	s.cachedResp = nil // invalidar caché anterior
}

// GetCredentials retorna una copia de las credenciales actuales
func (s *TokenService) GetCredentials() sunat.SunatCredentials {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.creds
}

// RequestNewToken solicita un nuevo access_token a SUNAT usando las credenciales especificadas
func (s *TokenService) RequestNewToken(ctx context.Context, creds sunat.SunatCredentials) (*sunat.TokenResponse, error) {
	if creds.ClientID == "" || creds.ClientSecret == "" {
		return nil, errors.New("debe ingresar Client ID y Client Secret de SUNAT")
	}
	if creds.RUC == "" || creds.UsuarioSOL == "" || creds.ClaveSOL == "" {
		return nil, errors.New("debe ingresar RUC, Usuario SOL y Clave SOL")
	}

	endpoint := fmt.Sprintf(SunatTokenBaseURL, url.PathEscape(creds.ClientID))

	// Parámetros form-urlencoded
	// username = RUC + UsuarioSOL (ej: 20522128717AJOGRUNA)
	fullUsername := creds.RUC + creds.UsuarioSOL

	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("scope", "https://api-cpe.sunat.gob.pe/")
	data.Set("client_id", creds.ClientID)
	data.Set("client_secret", creds.ClientSecret)
	data.Set("username", fullUsername)
	data.Set("password", creds.ClaveSOL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error creando solicitud HTTP: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "AppSireCPE-Go/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error de conexión con SUNAT OAuth2: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error leyendo respuesta de SUNAT: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errObj struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
			Message          string `json:"message"`
		}
		_ = json.Unmarshal(bodyBytes, &errObj)

		msg := errObj.ErrorDescription
		if msg == "" {
			msg = errObj.Error
		}
		if msg == "" {
			msg = errObj.Message
		}
		if msg == "" {
			msg = string(bodyBytes)
		}
		return nil, fmt.Errorf("SUNAT rechazó la autenticación (HTTP %d): %s", resp.StatusCode, msg)
	}

	var rawResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}

	if err := json.Unmarshal(bodyBytes, &rawResp); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta de token JSON: %w", err)
	}

	if rawResp.AccessToken == "" {
		return nil, errors.New("la respuesta de SUNAT no contiene access_token")
	}

	// Decodificar JWT para obtener la fecha de expiración y RUC precisos
	var expiresAt time.Time
	var tokenRUC string

	if payload, err := ParseJWT(rawResp.AccessToken); err == nil {
		expiresAt = payload.GetExpiresAt()
		tokenRUC = payload.GetRUC()
	} else {
		// Fallback usando expires_in
		if rawResp.ExpiresIn > 0 {
			expiresAt = time.Now().Add(time.Duration(rawResp.ExpiresIn) * time.Second)
		} else {
			expiresAt = time.Now().Add(1 * time.Hour)
		}
		tokenRUC = creds.RUC
	}

	tokenResp := &sunat.TokenResponse{
		AccessToken: rawResp.AccessToken,
		TokenType:   rawResp.TokenType,
		ExpiresIn:   rawResp.ExpiresIn,
		ExpiresAt:   expiresAt,
		RUC:         tokenRUC,
	}

	// Almacenar en caché
	s.mu.Lock()
	s.cachedResp = tokenResp
	s.creds = creds
	s.mu.Unlock()

	return tokenResp, nil
}

// GetValidToken devuelve un token válido. Si está por vencer o no existe, solicita uno nuevo
func (s *TokenService) GetValidToken(ctx context.Context) (string, error) {
	s.mu.RLock()
	cached := s.cachedResp
	creds := s.creds
	s.mu.RUnlock()

	if cached != nil && time.Until(cached.ExpiresAt) > TokenMarginExpiry {
		return cached.AccessToken, nil
	}

	// Necesitamos renovar o pedir token
	if creds.ClientID == "" || creds.ClaveSOL == "" {
		return "", errors.New("no hay credenciales configuradas para renovar el token")
	}

	// Usar singleflight para que solo UNA goroutine vaya a SUNAT si 10 hilos solicitan al mismo tiempo
	v, err, _ := s.sfGroup.Do("refresh_token", func() (interface{}, error) {
		// Doble verificación dentro del lock lógico
		s.mu.RLock()
		if s.cachedResp != nil && time.Until(s.cachedResp.ExpiresAt) > TokenMarginExpiry {
			token := s.cachedResp.AccessToken
			s.mu.RUnlock()
			return token, nil
		}
		s.mu.RUnlock()

		tokenResp, reqErr := s.RequestNewToken(ctx, creds)
		if reqErr != nil {
			return "", reqErr
		}
		return tokenResp.AccessToken, nil
	})

	if err != nil {
		return "", err
	}

	return v.(string), nil
}

// InvalidateToken borra el token sólo si sigue siendo el que recibió el 401.
func (s *TokenService) InvalidateToken(rejectedToken string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cachedResp == nil || s.cachedResp.AccessToken != rejectedToken {
		return
	}
	s.cachedResp = nil
}

// ForceRefresh descarta el token actual y obtiene uno nuevo. singleflight evita
// renovaciones duplicadas si dos etapas solicitan la operacion a la vez.
func (s *TokenService) ForceRefresh(ctx context.Context) (string, error) {
	s.mu.Lock()
	s.cachedResp = nil
	s.mu.Unlock()

	s.sfGroup.Forget("refresh_token")
	return s.GetValidToken(ctx)
}

// GetTokenInfo retorna información del token actual sin revelar el token completo
func (s *TokenService) GetTokenInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cachedResp == nil {
		return map[string]interface{}{
			"active": false,
		}
	}

	remaining := time.Until(s.cachedResp.ExpiresAt)
	return map[string]interface{}{
		"active":            remaining > 0,
		"expires_at":        s.cachedResp.ExpiresAt.Format(time.RFC3339),
		"remaining_seconds": int(remaining.Seconds()),
		"ruc":               s.cachedResp.RUC,
		"token_type":        s.cachedResp.TokenType,
	}
}
