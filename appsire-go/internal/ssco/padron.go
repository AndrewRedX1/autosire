package ssco

import (
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// SujetoSinCapacidad representa un contribuyente listado en el padrón oficial de SUNAT.
type SujetoSinCapacidad struct {
	RUC              string `json:"ruc"`
	RazonSocial      string `json:"razon_social"`
	Resolucion       string `json:"resolucion"`
	FechaFirme       string `json:"fecha_firme"`
	FechaPublicacion string `json:"fecha_publicacion"`
}

// Padron contiene los sujetos indexados en memoria para búsquedas O(1).
type Padron struct {
	PorRUC             map[string]SujetoSinCapacidad `json:"-"`
	PorNombre          map[string]SujetoSinCapacidad `json:"-"`
	Total              int                           `json:"total"`
	FechaActualizacion string                        `json:"fecha_actualizacion"`
	DesdeCache         bool                          `json:"desde_cache"`
	UltimaDescarga     time.Time                     `json:"ultima_descarga"`
}

// NewPadron inicializa un nuevo padrón vacío con sus mapas correspondientes.
func NewPadron() *Padron {
	return &Padron{
		PorRUC:    make(map[string]SujetoSinCapacidad),
		PorNombre: make(map[string]SujetoSinCapacidad),
	}
}

// SoloDigitos extrae únicamente los caracteres numéricos de una cadena.
func SoloDigitos(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// NormalizarNombre convierte a mayúsculas, elimina diacríticos/acentos mediante
// descomposición Unicode NFD y conserva exclusivamente caracteres alfanuméricos contiguos.
// Esto permite detectar homónimos y coincidencias exactas ignorando puntuación, espacios y tildes.
func NormalizarNombre(s string) string {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return ""
	}
	nfd := norm.NFD.String(s)
	var b strings.Builder
	b.Grow(len(nfd))
	for _, r := range nfd {
		if unicode.Is(unicode.Mn, r) { // Non-spacing mark (tildes, diéresis, etc.)
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
