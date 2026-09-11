package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// License representa un registro de suscripción en el servidor central
type License struct {
	Key             string    `json:"key"`
	Username        string    `json:"username"`
	PasswordHash    string    `json:"password_hash,omitempty"`
	Plan            string    `json:"plan"`
	MachineID       string    `json:"machine_id,omitempty"`
	ExpiresAt       time.Time `json:"expires_at"`
	CreatedAt       time.Time `json:"created_at"`
	LastVerifiedAt  time.Time `json:"last_verified_at,omitempty"`
	Active          bool      `json:"active"`
	MaxRUCs         int       `json:"max_rucs"` // 0 = ilimitados
	AllowedRUCs     []string  `json:"allowed_rucs,omitempty"`
	Notes           string    `json:"notes,omitempty"`
}

// LicenseToken estructura embebida en el token firmado entregado al cliente
type LicenseToken struct {
	Key         string    `json:"key"`
	Username    string    `json:"username"`
	MachineID   string    `json:"machine_id"`
	Plan        string    `json:"plan"`
	ExpiresAt   time.Time `json:"expires_at"`
	IssuedAt    time.Time `json:"issued_at"`
	MaxRUCs     int       `json:"max_rucs"`
	AllowedRUCs []string  `json:"allowed_rucs,omitempty"`
}

// Issuer maneja la firma criptográfica y verificación de licencias
type Issuer struct {
	secretKey []byte
}

// NewIssuer crea una instancia del emisor con la clave secreta del servidor
func NewIssuer(secret string) *Issuer {
	if secret == "" {
		secret = "AUTOSIRE-SECRET-KEY-2026-SUNAT-PRO-SECURE-VPS"
	}
	return &Issuer{
		secretKey: []byte(secret),
	}
}

// GenerateSignedToken genera un token criptográficamente firmado con HMAC-SHA256
// Estructura: Base64(PayloadJSON).Base64(HMAC_Signature)
func (i *Issuer) GenerateSignedToken(tokenData LicenseToken) (string, error) {
	payloadBytes, err := json.Marshal(tokenData)
	if err != nil {
		return "", err
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)

	// Calcular HMAC-SHA256
	h := hmac.New(sha256.New, i.secretKey)
	h.Write([]byte(payloadB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	signedToken := fmt.Sprintf("%s.%s", payloadB64, sigB64)
	return signedToken, nil
}

// VerifyToken valida la firma matemática y la fecha de caducidad del token
func (i *Issuer) VerifyToken(tokenString string, expectedMachineID string) (*LicenseToken, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 2 {
		return nil, errors.New("formato de token de licencia inválido")
	}

	payloadB64, sigB64 := parts[0], parts[1]

	// Verificar firma HMAC
	h := hmac.New(sha256.New, i.secretKey)
	h.Write([]byte(payloadB64))
	expectedSig := h.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, errors.New("error decodificando firma digital")
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return nil, errors.New("la firma de la licencia es inválida o ha sido manipulada")
	}

	// Decodificar contenido
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, errors.New("error decodificando payload de licencia")
	}

	var data LicenseToken
	if err := json.Unmarshal(payloadBytes, &data); err != nil {
		return nil, errors.New("error deserializando datos de licencia")
	}

	// Validar expiración
	if time.Now().After(data.ExpiresAt) {
		return nil, fmt.Errorf("la licencia ha caducado el %s", data.ExpiresAt.Format("02/01/2006"))
	}

	// Validar Machine ID si fue especificado
	if expectedMachineID != "" && data.MachineID != "" && data.MachineID != expectedMachineID {
		return nil, fmt.Errorf("la licencia está registrada para otra máquina")
	}

	return &data, nil
}
