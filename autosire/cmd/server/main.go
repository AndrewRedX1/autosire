package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"autosire/internal/db"
	"autosire/internal/license"
)

type Server struct {
	store  *db.Store
	issuer *license.Issuer
}

func main() {
	port := flag.Int("port", 8090, "Puerto del servidor de licencias AutoSire")
	dataDir := flag.String("data", "./data", "Ruta de almacenamiento persistente")
	secret := flag.String("secret", "AUTOSIRE-SECURE-VPS-KEY-2026", "Clave criptográfica del servidor")
	flag.Parse()

	// Si está en contenedor Docker leer variables de entorno
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}
	if envData := os.Getenv("DATA_DIR"); envData != "" {
		*dataDir = envData
	}
	if envSecret := os.Getenv("SECRET_KEY"); envSecret != "" {
		*secret = envSecret
	}

	store, err := db.NewStore(*dataDir)
	if err != nil {
		log.Fatalf("Error inicializando base de datos: %v", err)
	}

	issuer := license.NewIssuer(*secret)
	srv := &Server{store: store, issuer: issuer}

	mux := http.NewServeMux()

	// Endpoints para el ejecutable cliente (.exe)
	mux.HandleFunc("/api/v1/license/activate", srv.handleActivate)
	mux.HandleFunc("/api/v1/license/verify", srv.handleVerify)

	// Endpoints de administración
	mux.HandleFunc("/api/v1/admin/licenses", srv.handleListLicenses)
	mux.HandleFunc("/api/v1/admin/licenses/create", srv.handleCreateLicense)
	mux.HandleFunc("/api/v1/admin/licenses/toggle", srv.handleToggleLicense)

	// Servir panel web estático
	mux.Handle("/", http.FileServer(http.Dir("./web/static")))

	addr := fmt.Sprintf(":%d", *port)
	fmt.Println("=================================================================")
	fmt.Printf("   🛡️  AUTOSIRE - Servidor Central de Licencias y Control (VPS)\n")
	fmt.Println("=================================================================")
	fmt.Printf("   🌐 API y Panel Web disponible en: http://0.0.0.0:%d\n", *port)
	fmt.Printf("   📁 Directorio de datos:           %s\n", *dataDir)
	fmt.Println("   🔑 Licencia inicial precargada:   admin / admin (365 Días)")
	fmt.Println("=================================================================")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Error iniciando servidor AutoSire: %v", err)
	}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

// handleActivate activa la licencia en una máquina específica usando usuario/contraseña o License Key
func (s *Server) handleActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var req struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		Key       string `json:"key"`
		MachineID string `json:"machine_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}

	if req.MachineID == "" {
		respondError(w, http.StatusBadRequest, "El identificador de máquina (Machine ID) es requerido")
		return
	}

	var lic *license.License
	var err error

	if req.Username != "" && req.Password != "" {
		lic, err = s.store.Authenticate(req.Username, req.Password)
	} else if req.Key != "" {
		lic, err = s.store.GetByKey(req.Key)
		if err == nil {
			if !lic.Active {
				err = fmt.Errorf("la licencia está desactivada")
			} else if time.Now().After(lic.ExpiresAt) {
				err = fmt.Errorf("la licencia ha caducado el %s", lic.ExpiresAt.Format("02/01/2006"))
			}
		}
	} else {
		respondError(w, http.StatusBadRequest, "Debe ingresar usuario y contraseña o una clave de licencia")
		return
	}

	if err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Validar o vincular Machine ID
	if lic.MachineID == "" {
		// Primera vinculación a esta máquina
		lic.MachineID = req.MachineID
		lic.LastVerifiedAt = time.Now()
		_ = s.store.Save(lic)
	} else if lic.MachineID != req.MachineID {
		respondError(w, http.StatusForbidden, "Esta licencia ya está activada en otra computadora. Comuníquese con soporte para transferirla.")
		return
	}

	// Generar token firmado
	tokenData := license.LicenseToken{
		Key:         lic.Key,
		Username:    lic.Username,
		MachineID:   lic.MachineID,
		Plan:        lic.Plan,
		ExpiresAt:   lic.ExpiresAt,
		IssuedAt:    time.Now(),
		MaxRUCs:     lic.MaxRUCs,
		AllowedRUCs: lic.AllowedRUCs,
	}

	signedToken, err := s.issuer.GenerateSignedToken(tokenData)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Error generando firma de licencia: "+err.Error())
		return
	}

	daysLeft := int(time.Until(lic.ExpiresAt).Hours() / 24)
	if daysLeft < 0 {
		daysLeft = 0
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"token":      signedToken,
		"key":        lic.Key,
		"username":   lic.Username,
		"plan":       lic.Plan,
		"expires_at": lic.ExpiresAt.Format(time.RFC3339),
		"days_left":  daysLeft,
		"message":    fmt.Sprintf("Licencia activada con éxito. Vigencia: %d días restantes.", daysLeft),
	})
}

// handleVerify valida si un token sigue siendo válido
func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var req struct {
		Token     string `json:"token"`
		MachineID string `json:"machine_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}

	tokenData, err := s.issuer.VerifyToken(req.Token, req.MachineID)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	// Verificar estado en tiempo real en la base de datos
	lic, err := s.store.GetByKey(tokenData.Key)
	if err != nil || !lic.Active {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"valid": false,
			"error": "La licencia ha sido revocada o desactivada en el servidor",
		})
		return
	}

	lic.LastVerifiedAt = time.Now()
	_ = s.store.Save(lic)

	daysLeft := int(time.Until(lic.ExpiresAt).Hours() / 24)
	if daysLeft < 0 {
		daysLeft = 0
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"valid":      true,
		"username":   lic.Username,
		"plan":       lic.Plan,
		"expires_at": lic.ExpiresAt.Format(time.RFC3339),
		"days_left":  daysLeft,
	})
}

// handleListLicenses lista todas las licencias para el panel administrativo
func (s *Server) handleListLicenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}
	licenses := s.store.ListAll()
	respondJSON(w, http.StatusOK, licenses)
}

// handleCreateLicense permite emitir una nueva licencia
func (s *Server) handleCreateLicense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Plan     string `json:"plan"`
		Days     int    `json:"days"`
		MaxRUCs  int    `json:"max_rucs"`
		Notes    string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}

	if req.Username == "" {
		respondError(w, http.StatusBadRequest, "El nombre de usuario es requerido")
		return
	}
	if req.Days <= 0 {
		req.Days = 30
	}
	if req.Plan == "" {
		req.Plan = "Plan Estándar"
	}

	randomBytes := make([]byte, 8)
	_, _ = rand.Read(randomBytes)
	key := fmt.Sprintf("AUTOSIRE-%s-%s", strings.ToUpper(req.Username), strings.ToUpper(hex.EncodeToString(randomBytes)[:8]))

	lic := &license.License{
		Key:          key,
		Username:     req.Username,
		PasswordHash: db.HashPassword(req.Password),
		Plan:         req.Plan,
		ExpiresAt:    time.Now().Add(time.Duration(req.Days) * 24 * time.Hour),
		CreatedAt:    time.Now(),
		Active:       true,
		MaxRUCs:      req.MaxRUCs,
		Notes:        req.Notes,
	}

	if err := s.store.Save(lic); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"license": lic,
	})
}

// handleToggleLicense activa o suspende una licencia
func (s *Server) handleToggleLicense(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	var req struct {
		Key    string `json:"key"`
		Active bool   `json:"active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}

	lic, err := s.store.GetByKey(req.Key)
	if err != nil {
		respondError(w, http.StatusNotFound, "Licencia no encontrada")
		return
	}

	lic.Active = req.Active
	_ = s.store.Save(lic)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"license": lic,
	})
}
