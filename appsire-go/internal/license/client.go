package license

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	DefaultServerURL = "http://localhost:8090"
	LicenseFileName  = "license.dat"
	offlineGrace     = 24 * time.Hour
)

// LicenseData estructura guardada en disco por el cliente
type LicenseData struct {
	Token          string    `json:"token"`
	Key            string    `json:"key"`
	Username       string    `json:"username"`
	Plan           string    `json:"plan"`
	MachineID      string    `json:"machine_id"`
	ExpiresAt      time.Time `json:"expires_at"`
	ServerURL      string    `json:"server_url"`
	ActivatedAt    time.Time `json:"activated_at"`
	LastVerifiedAt time.Time `json:"last_verified_at"`
}

// LicenseManager coordina la verificación y activación de la licencia local
type LicenseManager struct {
	dataDir   string
	serverURL string
	mu        sync.RWMutex
	current   *LicenseData
	machineID string
}

// NewLicenseManager inicializa el gestor de licencias del cliente
func NewLicenseManager(dataDir string) *LicenseManager {
	if dataDir == "" {
		dataDir = "."
	}
	mgr := &LicenseManager{
		dataDir:   dataDir,
		serverURL: DefaultServerURL,
		machineID: getMachineID(),
	}
	_ = mgr.loadLocalLicense()
	return mgr
}

// GetMachineID retorna la huella única de esta PC
func (m *LicenseManager) GetMachineID() string {
	return m.machineID
}

// getMachineID genera un identificador único por hardware
func getMachineID() string {
	var rawID string

	if runtime.GOOS == "windows" {
		// Consultar UUID de BIOS/Motherboard mediante PowerShell
		cmd := exec.Command("powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_ComputerSystemProduct).UUID")
		out, err := cmd.Output()
		if err == nil && len(strings.TrimSpace(string(out))) > 5 {
			rawID = strings.TrimSpace(string(out))
		}
	}

	if rawID == "" {
		hostname, _ := os.Hostname()
		user := os.Getenv("USERNAME")
		if user == "" {
			user = os.Getenv("USER")
		}
		rawID = fmt.Sprintf("%s-%s-%s", hostname, user, runtime.GOARCH)
	}

	hash := sha256.Sum256([]byte("AUTOSIRE-HW-" + rawID))
	return hex.EncodeToString(hash[:16])
}

// loadLocalLicense lee la licencia guardada en disco si existe
func (m *LicenseManager) loadLocalLicense() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filePath := filepath.Join(m.dataDir, LicenseFileName)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var data LicenseData
	if err := json.Unmarshal(content, &data); err != nil {
		return err
	}

	if data.MachineID != m.machineID {
		return errors.New("la licencia guardada corresponde a otra máquina")
	}

	m.current = &data
	if data.ServerURL != "" {
		m.serverURL = data.ServerURL
	}
	return nil
}

// saveLocalLicense escribe la licencia en disco
func (m *LicenseManager) saveLocalLicense(data *LicenseData) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bytesData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(m.dataDir, LicenseFileName)
	m.current = data
	return os.WriteFile(filePath, bytesData, 0600)
}

// Activate conecta con el servidor central AutoSire para activar la licencia
func (m *LicenseManager) Activate(serverURL, username, password, key string) (*LicenseData, error) {
	return m.ActivateWithRemember(serverURL, username, password, key, true)
}

func (m *LicenseManager) ActivateWithRemember(
	serverURL string,
	username string,
	password string,
	key string,
	remember bool,
) (*LicenseData, error) {
	if serverURL == "" {
		serverURL = m.serverURL
	}
	serverURL = strings.TrimRight(serverURL, "/")
	if err := validateServerURL(serverURL); err != nil {
		return nil, err
	}

	payload := map[string]string{
		"username":   username,
		"password":   password,
		"key":        key,
		"machine_id": m.machineID,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	endpoint := serverURL + "/api/v1/license/activate"
	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar con el servidor de licencias en %s: %w", serverURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(bodyBytes, &errResp)
		if errResp.Error != "" {
			return nil, errors.New(errResp.Error)
		}
		return nil, fmt.Errorf("error del servidor de licencias (HTTP %d)", resp.StatusCode)
	}

	var activateResp struct {
		Success   bool      `json:"success"`
		Token     string    `json:"token"`
		Key       string    `json:"key"`
		Username  string    `json:"username"`
		Plan      string    `json:"plan"`
		ExpiresAt time.Time `json:"expires_at"`
		DaysLeft  int       `json:"days_left"`
		Message   string    `json:"message"`
	}

	if err := json.Unmarshal(bodyBytes, &activateResp); err != nil {
		return nil, fmt.Errorf("error decodificando respuesta del servidor: %w", err)
	}

	data := &LicenseData{
		Token:          activateResp.Token,
		Key:            activateResp.Key,
		Username:       activateResp.Username,
		Plan:           activateResp.Plan,
		MachineID:      m.machineID,
		ExpiresAt:      activateResp.ExpiresAt,
		ServerURL:      serverURL,
		ActivatedAt:    time.Now(),
		LastVerifiedAt: time.Now(),
	}

	if remember {
		if err := m.saveLocalLicense(data); err != nil {
			return nil, fmt.Errorf("licencia activada pero falló al guardarse en disco: %w", err)
		}
	} else {
		_ = os.Remove(filepath.Join(m.dataDir, LicenseFileName))
		m.mu.Lock()
		m.current = data
		m.serverURL = serverURL
		m.mu.Unlock()
	}

	return data, nil
}

func (m *LicenseManager) HasActiveLicense() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current != nil && time.Now().Before(m.current.ExpiresAt)
}

func (m *LicenseManager) EndSession() {
	m.mu.Lock()
	m.current = nil
	m.mu.Unlock()
}

// GetStatus retorna el estado actual de la licencia sincronizado con el VPS
func (m *LicenseManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	cur := m.current
	m.mu.RUnlock()

	if cur == nil {
		return map[string]interface{}{
			"active":     false,
			"machine_id": m.machineID,
			"server_url": m.serverURL,
			"message":    "Sin licencia activada",
		}
	}

	daysLeft := int(time.Until(cur.ExpiresAt).Hours() / 24)
	isExpired := time.Now().After(cur.ExpiresAt)

	if isExpired {
		return map[string]interface{}{
			"active":     false,
			"username":   cur.Username,
			"plan":       cur.Plan,
			"machine_id": m.machineID,
			"is_expired": true,
			"message":    "Licencia caducada",
		}
	}

	// Consultar validación en tiempo real al servidor central (VPS)
	onlineValid, verifyErr := m.VerifyOnline()
	if !onlineValid {
		errMsg := "Licencia suspendida o revocada en el servidor central"
		if verifyErr != nil {
			errMsg = verifyErr.Error()
		}
		return map[string]interface{}{
			"active":       false,
			"username":     cur.Username,
			"plan":         cur.Plan,
			"machine_id":   m.machineID,
			"expires_at":   cur.ExpiresAt.Format(time.RFC3339),
			"days_left":    daysLeft,
			"server_url":   cur.ServerURL,
			"activated_at": cur.ActivatedAt.Format(time.RFC3339),
			"is_expired":   false,
			"is_suspended": true,
			"message":      errMsg,
		}
	}

	return map[string]interface{}{
		"active":       true,
		"username":     cur.Username,
		"plan":         cur.Plan,
		"key":          cur.Key,
		"machine_id":   m.machineID,
		"expires_at":   cur.ExpiresAt.Format(time.RFC3339),
		"days_left":    daysLeft,
		"server_url":   cur.ServerURL,
		"activated_at": cur.ActivatedAt.Format(time.RFC3339),
		"is_expired":   false,
		"is_suspended": false,
		"message":      "Licencia activa",
	}
}

// VerifyOnline verifica en segundo plano con el servidor si la licencia sigue activa
func (m *LicenseManager) VerifyOnline() (bool, error) {
	m.mu.RLock()
	cur := m.current
	m.mu.RUnlock()

	if cur == nil {
		return false, errors.New("no hay licencia local")
	}

	if time.Now().After(cur.ExpiresAt) {
		return false, errors.New("la licencia local ha caducado")
	}

	payload := map[string]string{
		"token":      cur.Token,
		"machine_id": m.machineID,
	}
	jsonBytes, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Post(cur.ServerURL+"/api/v1/license/verify", "application/json", bytes.NewReader(jsonBytes))
	if err != nil {
		lastVerified := cur.LastVerifiedAt
		if lastVerified.IsZero() {
			lastVerified = cur.ActivatedAt
		}
		if !lastVerified.IsZero() && time.Since(lastVerified) <= offlineGrace {
			return true, nil
		}
		return false, errors.New("no se pudo verificar la licencia con AutoSire")
	}
	defer resp.Body.Close()

	var result struct {
		Valid bool   `json:"valid"`
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if !result.Valid {
		return false, errors.New(result.Error)
	}

	m.mu.Lock()
	if m.current != nil && m.current.Token == cur.Token {
		m.current.LastVerifiedAt = time.Now()
		cur = m.current
	}
	m.mu.Unlock()
	_ = m.saveLocalLicense(cur)

	return true, nil
}

func validateServerURL(serverURL string) error {
	parsed, err := url.Parse(serverURL)
	if err != nil || parsed.Hostname() == "" {
		return errors.New("la dirección del servidor AutoSire no es válida")
	}
	isLocal := parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1"
	if parsed.Scheme != "https" && !(isLocal && parsed.Scheme == "http") {
		return errors.New("el servidor AutoSire debe usar HTTPS; HTTP solo se permite en localhost")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("la dirección del servidor AutoSire contiene datos no permitidos")
	}
	return nil
}
