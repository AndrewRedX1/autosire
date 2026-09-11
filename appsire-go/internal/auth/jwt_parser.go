package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// JWTPayload extrae los campos clave que SUNAT incluye en el token
type JWTPayload struct {
	Sub      string `json:"sub"`      // Generalmente el RUC
	NumRUC   string `json:"numRUC"`   // A veces presente en userdata
	ClientID string `json:"clientId"`
	Exp      int64  `json:"exp"`      // Unix timestamp
	Nbf      int64  `json:"nbf"`
	Iat      int64  `json:"iat"`
	UserData struct {
		NumRUC     string `json:"numRUC"`
		UsuarioSOL string `json:"usuarioSOL"`
		Nombres    string `json:"nombres"`
	} `json:"userdata"`
}

// ParseJWT decodifica la sección payload de un JWT sin validar la firma criptográfica
// (la firma es emitida y validada por los servidores de SUNAT)
func ParseJWT(tokenString string) (*JWTPayload, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("formato de JWT inválido: debe tener 3 partes separadas por punto")
	}

	payloadPart := parts[1]
	// Corregir padding base64 URL si hace falta
	if l := len(payloadPart) % 4; l > 0 {
		payloadPart += strings.Repeat("=", 4-l)
	}

	decoded, err := base64.URLEncoding.DecodeString(payloadPart)
	if err != nil {
		// Intentar decoding estándar por compatibilidad
		decoded, err = base64.StdEncoding.DecodeString(payloadPart)
		if err != nil {
			return nil, errors.New("error decodificando payload base64 del JWT: " + err.Error())
		}
	}

	var payload JWTPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, errors.New("error deserializando payload JSON del JWT: " + err.Error())
	}

	return &payload, nil
}

// GetExpiresAt obtiene el time.Time de expiración del payload
func (p *JWTPayload) GetExpiresAt() time.Time {
	if p.Exp == 0 {
		return time.Now().Add(1 * time.Hour) // fallback 1 hora
	}
	return time.Unix(p.Exp, 0)
}

// GetRUC devuelve el RUC extraído de cualquier campo disponible del token
func (p *JWTPayload) GetRUC() string {
	if p.UserData.NumRUC != "" {
		return p.UserData.NumRUC
	}
	if p.NumRUC != "" {
		return p.NumRUC
	}
	if len(p.Sub) == 11 {
		return p.Sub
	}
	return ""
}
