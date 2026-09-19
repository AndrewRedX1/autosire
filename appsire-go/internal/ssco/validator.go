package ssco

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"appsire-go/internal/sirepreview"
)

// Constantes de clasificación de riesgo
const (
	RiesgoCritico   = "CRITICAL" // SSCO confirmado por RUC (pérdida de crédito fiscal)
	RiesgoAlerta    = "WARNING"  // Coincidencia por nombre (requiere revisión)
	RiesgoConforme  = "OK"       // No figura en el padrón
)

// SSCOValidatedItem representa el resultado de auditoría para un comprobante individual.
type SSCOValidatedItem struct {
	Index            int     `json:"index"`
	CompPago         string  `json:"comp_pago"`
	Tipo             string  `json:"tipo"`
	Serie            string  `json:"serie"`
	Numero           string  `json:"numero"`
	Fecha            string  `json:"fecha"`
	RUC              string  `json:"ruc"`
	RazonSocial      string  `json:"razon_social"`
	ImporteTotal     float64 `json:"importe_total"`
	IGV              float64 `json:"igv"`
	Estado           string  `json:"estado"`
	CoincidePor      string  `json:"coincide_por"`
	Resolucion       string  `json:"resolucion"`
	FechaFirme       string  `json:"fecha_firme"`
	FechaPublicacion string  `json:"fecha_publicacion"`
	Detalle          string  `json:"detalle"`
	Riesgo           string  `json:"riesgo"`
}

// SSCOReport consolida el resumen cuantitativo y las filas auditadas de la propuesta.
type SSCOReport struct {
	Book            string              `json:"book"`
	TotalItems      int                 `json:"total_items"`
	CountSSCO       int                 `json:"count_ssco"`
	CountWarning    int                 `json:"count_warning"`
	CountOK         int                 `json:"count_ok"`
	TotalIGVRiesgo  float64             `json:"total_igv_riesgo"`
	PadronTotal     int                 `json:"padron_total"`
	PadronFecha     string              `json:"padron_fecha"`
	DesdeCache      bool                `json:"desde_cache"`
	Items           []SSCOValidatedItem `json:"items"`
}

// ValidateItems ejecuta la auditoría tributaria cruzando cada comprobante de compras
// contra el padrón de Sujetos Sin Capacidad Operativa indexado en memoria.
func ValidateItems(items []sirepreview.ProposalItem, padron *Padron) SSCOReport {
	report := SSCOReport{
		Book:        "RCE",
		TotalItems:  len(items),
		PadronTotal: padron.Total,
		PadronFecha: padron.FechaActualizacion,
		DesdeCache:  padron.DesdeCache,
		Items:       make([]SSCOValidatedItem, 0, len(items)),
	}

	if padron == nil || len(items) == 0 {
		return report
	}

	var totalIGVRiesgo float64

	for idx, it := range items {
		cleanRUC := SoloDigitos(it.RUC)
		cleanName := strings.TrimSpace(it.RazonSocial)
		impTotal := parseMonto(it.ImporteTotal)
		igvTotal := parseMonto(it.IGV)

		valItem := SSCOValidatedItem{
			Index:        idx,
			CompPago:     it.CompPago,
			Tipo:         it.Tipo,
			Serie:        it.Serie,
			Numero:       it.Numero,
			Fecha:        it.Fecha,
			RUC:          cleanRUC,
			RazonSocial:  cleanName,
			ImporteTotal: impTotal,
			IGV:          igvTotal,
		}

		// Si no hay RUC ni Razón Social, se considera sin datos suficientes
		if cleanRUC == "" && cleanName == "" {
			valItem.Estado = "NO REGISTRADO"
			valItem.CoincidePor = "-"
			valItem.Riesgo = RiesgoConforme
			report.CountOK++
			report.Items = append(report.Items, valItem)
			continue
		}

		// 1. Criterio Primario: Coincidencia directa por RUC
		if sujeto, ok := padron.PorRUC[cleanRUC]; ok && cleanRUC != "" {
			valItem.Estado = "SIN CAPACIDAD OPERATIVA"
			valItem.CoincidePor = "RUC"
			valItem.Riesgo = RiesgoCritico
			valItem.Resolucion = sujeto.Resolucion
			valItem.FechaFirme = sujeto.FechaFirme
			valItem.FechaPublicacion = sujeto.FechaPublicacion
			valItem.Detalle = buildDetalle(sujeto)

			report.CountSSCO++
			totalIGVRiesgo += igvTotal
			report.Items = append(report.Items, valItem)
			continue
		}

		// 2. Criterio Secundario: Coincidencia por Razón Social normalizada
		normName := NormalizarNombre(cleanName)
		if sujeto, ok := padron.PorNombre[normName]; ok && normName != "" {
			valItem.Estado = "REVISAR (coincide nombre)"
			valItem.CoincidePor = "NOMBRE"
			valItem.Riesgo = RiesgoAlerta
			valItem.Resolucion = sujeto.Resolucion
			valItem.FechaFirme = sujeto.FechaFirme
			valItem.FechaPublicacion = sujeto.FechaPublicacion
			valItem.Detalle = fmt.Sprintf("RUC en padrón: %s | %s", sujeto.RUC, buildDetalle(sujeto))

			report.CountWarning++
			report.Items = append(report.Items, valItem)
			continue
		}

		// 3. Conforme: No figura en el padrón
		valItem.Estado = "NO REGISTRADO"
		valItem.CoincidePor = "-"
		valItem.Riesgo = RiesgoConforme
		valItem.Detalle = ""
		report.CountOK++
		report.Items = append(report.Items, valItem)
	}

	report.TotalIGVRiesgo = math.Round(totalIGVRiesgo*100) / 100
	return report
}

func buildDetalle(s SujetoSinCapacidad) string {
	parts := make([]string, 0, 3)
	if s.Resolucion != "" {
		parts = append(parts, s.Resolucion)
	}
	if s.FechaFirme != "" {
		parts = append(parts, "firme "+s.FechaFirme)
	}
	if s.FechaPublicacion != "" {
		parts = append(parts, "pub. "+s.FechaPublicacion)
	}
	return strings.Join(parts, " | ")
}

func parseMonto(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return val
}
