package db

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"autosire/internal/license"
)

// Store gestiona la persistencia de licencias y usuarios en el servidor
type Store struct {
	filePath string
	mu       sync.RWMutex
	licenses map[string]*license.License // key -> License
}

// HashPassword calcula un hash SHA-256 de la contraseña con salt fijo
func HashPassword(password string) string {
	hasher := sha256.New()
	hasher.Write([]byte("AUTOSIRE-SALT-2026-" + password))
	return hex.EncodeToString(hasher.Sum(nil))
}

// NewStore inicializa el almacenamiento persistente y precarga la licencia inicial si está vacío
func NewStore(dataDir string) (*Store, error) {
	if dataDir == "" {
		dataDir = "./data"
	}
	_ = os.MkdirAll(dataDir, 0755)

	filePath := filepath.Join(dataDir, "licenses.json")
	s := &Store{
		filePath: filePath,
		licenses: make(map[string]*license.License),
	}

	if err := s.loadFromFile(); err != nil {
		// Si el archivo no existe, inicializar con la licencia de prueba del usuario
		s.seedDefaultAdminLicense()
		_ = s.saveToFile()
	} else if len(s.licenses) == 0 {
		s.seedDefaultAdminLicense()
		_ = s.saveToFile()
	}

	return s, nil
}

// seedDefaultAdminLicense crea la licencia de prueba solicitada: admin / admin por 365 días
func (s *Store) seedDefaultAdminLicense() {
	adminLicense := &license.License{
		Key:          "AUTOSIRE-TEST-ADMIN-365D",
		Username:     "admin",
		PasswordHash: HashPassword("admin"),
		Plan:         "Empresarial Full CPE (365 Días)",
		ExpiresAt:    time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:    time.Now(),
		Active:       true,
		MaxRUCs:      0, // Ilimitado
		Notes:        "Licencia inicial de prueba solicitada (admin / admin)",
	}
	s.licenses[adminLicense.Key] = adminLicense
	fmt.Println(" [AUTOSIRE] Creada licencia inicial: Usuario='admin', Vence en 365 días")
}

func (s *Store) loadFromFile() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var list []*license.License
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.licenses = make(map[string]*license.License)
	for _, l := range list {
		s.licenses[l.Key] = l
	}
	return nil
}

func (s *Store) saveToFile() error {
	s.mu.RLock()
	list := make([]*license.License, 0, len(s.licenses))
	for _, l := range s.licenses {
		list = append(list, l)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

// Authenticate verifica usuario y contraseña
func (s *Store) Authenticate(username, password string) (*license.License, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetHash := HashPassword(password)
	for _, l := range s.licenses {
		if l.Username == username && l.PasswordHash == targetHash {
			if !l.Active {
				return nil, errors.New("la licencia está desactivada o suspendida por el administrador")
			}
			if time.Now().After(l.ExpiresAt) {
				return nil, fmt.Errorf("la licencia ha caducado el %s", l.ExpiresAt.Format("02/01/2006"))
			}
			return l, nil
		}
	}

	return nil, errors.New("usuario o contraseña de licencia incorrectos")
}

// GetByKey busca una licencia por su clave alfanumérica
func (s *Store) GetByKey(key string) (*license.License, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l, ok := s.licenses[key]
	if !ok {
		return nil, errors.New("licencia no encontrada")
	}
	return l, nil
}

// Save guarda o actualiza una licencia
func (s *Store) Save(lic *license.License) error {
	s.mu.Lock()
	s.licenses[lic.Key] = lic
	s.mu.Unlock()
	return s.saveToFile()
}

// ListAll retorna todas las licencias registradas
func (s *Store) ListAll() []*license.License {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*license.License, 0, len(s.licenses))
	for _, l := range s.licenses {
		result = append(result, l)
	}
	return result
}
