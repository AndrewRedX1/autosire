package ssco

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	// DefaultPadronURL es la URL oficial donde SUNAT publica el padrón SSCO.
	DefaultPadronURL = "https://www.sunat.gob.pe/padronesnotificaciones/ssco/sujesincapacidadOperativa.xlsx"
	defaultTimeout   = 45 * time.Second
)

// Service gestiona la descarga, caché local e indexación del padrón SSCO.
type Service struct {
	padronURL  string
	cacheDir   string
	cacheFile  string
	httpClient *http.Client

	mu     sync.RWMutex
	padron *Padron
}

// NewService crea un nuevo servicio de padrón SSCO con almacenamiento local en cacheDir.
func NewService(cacheDir string) *Service {
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "autosire", "ssco")
	}
	_ = os.MkdirAll(cacheDir, 0755)

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression: false,
	}

	return &Service{
		padronURL: DefaultPadronURL,
		cacheDir:  cacheDir,
		cacheFile: filepath.Join(cacheDir, "padron_ssco.xlsx"),
		httpClient: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
		},
	}
}

// SetPadronURL permite sobreescribir la URL del padrón (útil para pruebas unitarias).
func (s *Service) SetPadronURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.padronURL = url
}

// GetPadron obtiene el padrón indexado. Si ya está en memoria y no se fuerza la actualización,
// se retorna de forma instantánea. Si forceRefresh es true, se descarga nuevamente de SUNAT.
func (s *Service) GetPadron(ctx context.Context, forceRefresh bool) (*Padron, error) {
	s.mu.RLock()
	if s.padron != nil && !forceRefresh {
		p := s.padron
		s.mu.RUnlock()
		return p, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Doble chequeo por si otra goroutine ya lo cargó mientras se esperaba el bloqueo
	if s.padron != nil && !forceRefresh {
		return s.padron, nil
	}

	var data []byte
	var desdeCache bool
	var dlErr error

	// Intentar descargar de SUNAT
	data, dlErr = s.download(ctx)
	if dlErr == nil && len(data) > 0 {
		// Guardar en caché local
		_ = os.WriteFile(s.cacheFile, data, 0644)
		desdeCache = false
	} else {
		// Fallback a caché local en disco si la descarga falló
		if cachedData, err := os.ReadFile(s.cacheFile); err == nil && len(cachedData) > 0 {
			data = cachedData
			desdeCache = true
		} else {
			if dlErr != nil {
				return nil, fmt.Errorf("no se pudo descargar el padrón SSCO de SUNAT (%v) y no existe copia local previa", dlErr)
			}
			return nil, errors.New("no se pudo obtener el padrón SSCO")
		}
	}

	// Parsear el archivo Excel
	padron, err := s.parseExcel(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("error al procesar padrón SSCO: %w", err)
	}

	padron.DesdeCache = desdeCache
	padron.UltimaDescarga = time.Now()
	s.padron = padron

	return padron, nil
}

// download descarga el archivo Excel desde SUNAT.
func (s *Service) download(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.padronURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet, application/octet-stream, */*")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("código HTTP inesperado de SUNAT: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// parseExcel lee el archivo Excel (.xlsx) y genera el padrón indexado.
func (s *Service) parseExcel(r io.ReaderAt, size int64) (*Padron, error) {
	f, err := excelize.OpenReader(bytes.NewReader(ioReaderAtToBytes(r, size)))
	if err != nil {
		return nil, fmt.Errorf("abriendo archivo Excel: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("el archivo Excel no contiene hojas de cálculo")
	}

	sheetName := sheets[0]
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("leyendo filas de %s: %w", sheetName, err)
	}

	padron := NewPadron()
	var latestPubDate time.Time

	// Fila 0 o 1 suelen ser encabezados, los datos empiezan después
	for rowIdx, row := range rows {
		if rowIdx == 0 {
			continue
		}
		if len(row) == 0 {
			continue
		}

		ruc := ""
		if len(row) > 0 {
			ruc = SoloDigitos(strings.TrimSpace(row[0]))
		}
		if ruc == "" || len(ruc) < 8 {
			// Saltar filas sin RUC válido (encabezados adicionales o notas)
			continue
		}

		razonSocial := ""
		if len(row) > 1 {
			razonSocial = strings.TrimSpace(row[1])
		}

		resolucion := ""
		if len(row) > 3 {
			resolucion = strings.TrimSpace(row[3])
		}

		fechaFirme := ""
		if len(row) > 5 {
			fechaFirme = strings.TrimSpace(row[5])
		}

		fechaPub := ""
		if len(row) > 8 {
			fechaPub = strings.TrimSpace(row[8])
			if pt, err := parseDate(fechaPub); err == nil {
				if pt.After(latestPubDate) {
					latestPubDate = pt
				}
			}
		}

		sujeto := SujetoSinCapacidad{
			RUC:              ruc,
			RazonSocial:      razonSocial,
			Resolucion:       resolucion,
			FechaFirme:       fechaFirme,
			FechaPublicacion: fechaPub,
		}

		padron.PorRUC[ruc] = sujeto

		normName := NormalizarNombre(razonSocial)
		if normName != "" {
			if _, exists := padron.PorNombre[normName]; !exists {
				padron.PorNombre[normName] = sujeto
			}
		}
	}

	padron.Total = len(padron.PorRUC)
	if padron.Total == 0 {
		return nil, errors.New("el padrón se descargó pero no contiene RUCs válidos")
	}

	if !latestPubDate.IsZero() {
		padron.FechaActualizacion = latestPubDate.Format("02/01/2006")
	} else {
		padron.FechaActualizacion = time.Now().Format("02/01/2006")
	}

	return padron, nil
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		"02/01/2006",
		"2006-01-02",
		"02-01-2006",
		"2/1/2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("formato de fecha no reconocido")
}

func ioReaderAtToBytes(r io.ReaderAt, size int64) []byte {
	buf := make([]byte, size)
	_, _ = r.ReadAt(buf, 0)
	return buf
}
