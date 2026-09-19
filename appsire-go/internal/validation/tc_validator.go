package validation

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"appsire-go/internal/exchangerate"
	"appsire-go/internal/sirepreview"
)

type ValidatedItem struct {
	Index              int                    `json:"index"`
	CompPago           string                 `json:"comp_pago"`
	RUC                string                 `json:"ruc"`
	RazonSocial        string                 `json:"razon_social"`
	Tipo               string                 `json:"tipo"`
	Serie              string                 `json:"serie"`
	Numero             string                 `json:"numero"`
	FechaUsada         string                 `json:"fecha_usada"`
	FechaOrigen        string                 `json:"fecha_origen"` // "emision" o "referencia"
	EsNotaCredito      bool                   `json:"es_nota_credito"`
	Moneda             string                 `json:"moneda"`
	TCArchivo          float64                `json:"tc_archivo"`
	TCSunat            float64                `json:"tc_sunat"`
	Diferencia         float64                `json:"diferencia"`
	Estado             string                 `json:"estado"` // "OK", "DIFERENTE", "VACIO", "SIN_TC", "ERROR_FECHA"
	Observacion        string                 `json:"observacion"`
	ImporteOriginal    float64                `json:"importe_original"`
	ImporteRecalculado float64                `json:"importe_recalculado"`
	ItemRecalculado    sirepreview.ProposalItem `json:"item_recalculado"`
}

type TCValidationReport struct {
	Book           string          `json:"book"`
	TCType         string          `json:"tc_type"` // "Venta" o "Compra"
	TotalUSD       int             `json:"total_usd"`
	TotalOK        int             `json:"total_ok"`
	TotalDiferente int             `json:"total_diferente"`
	TotalVacio     int             `json:"total_vacio"`
	TotalSinTC     int             `json:"total_sin_tc"`
	TotalError     int             `json:"total_error"`
	Items          []ValidatedItem `json:"items"`
}

func ExtractMonthsFromItems(items []sirepreview.ProposalItem) []exchangerate.YearMonth {
	seen := make(map[string]exchangerate.YearMonth)
	for _, it := range items {
		moneda := strings.ToUpper(strings.TrimSpace(it.Moneda))
		if moneda == "PEN" || moneda == "" {
			continue
		}

		fechaStr := it.Fecha
		if isNotaCredito(it) && strings.TrimSpace(it.FechaRef) != "" {
			fechaStr = it.FechaRef
		}

		t, ok := parseDate(fechaStr)
		if !ok {
			// Intento de fallback a fecha de emisión
			t, ok = parseDate(it.Fecha)
			if !ok {
				continue
			}
		}

		key := fmt.Sprintf("%04d-%02d", t.Year(), t.Month())
		seen[key] = exchangerate.YearMonth{Year: t.Year(), Month: int(t.Month())}
	}

	result := make([]exchangerate.YearMonth, 0, len(seen))
	for _, ym := range seen {
		result = append(result, ym)
	}
	return result
}

func ValidateTC(items []sirepreview.ProposalItem, book string, tcType string, rates map[string]exchangerate.Rate) TCValidationReport {
	if tcType == "" {
		if strings.EqualFold(book, "RVIE") {
			tcType = "Venta"
		} else {
			tcType = "Venta" // Según numeral 17 Art 5 del reglamento de la Ley del IGV
		}
	}

	report := TCValidationReport{
		Book:   book,
		TCType: tcType,
		Items:  make([]ValidatedItem, 0),
	}

	for idx, it := range items {
		moneda := strings.ToUpper(strings.TrimSpace(it.Moneda))
		if moneda == "PEN" || moneda == "" {
			continue
		}
		report.TotalUSD++

		isNC := isNotaCredito(it)
		fechaUsada := it.Fecha
		fechaOrigen := "emision"
		if isNC && strings.TrimSpace(it.FechaRef) != "" {
			fechaUsada = it.FechaRef
			fechaOrigen = "referencia"
		}

		parsedDate, ok := parseDate(fechaUsada)
		if !ok && isNC && strings.TrimSpace(it.Fecha) != "" {
			// Fallback para NC sin fecha ref válida
			fechaUsada = it.Fecha
			fechaOrigen = "emision (fallback)"
			parsedDate, ok = parseDate(fechaUsada)
		}

		tcArchivo := parseDecimal(it.TipoCambio)
		importeOriginal := parseDecimal(it.ImporteTotal)

		valItem := ValidatedItem{
			Index:           idx,
			CompPago:        it.CompPago,
			RUC:             it.RUC,
			RazonSocial:     it.RazonSocial,
			Tipo:            it.Tipo,
			Serie:           it.Serie,
			Numero:          it.Numero,
			FechaUsada:      fechaUsada,
			FechaOrigen:     fechaOrigen,
			EsNotaCredito:   isNC,
			Moneda:          moneda,
			TCArchivo:       tcArchivo,
			ImporteOriginal: importeOriginal,
		}

		if !ok {
			valItem.Estado = "ERROR_FECHA"
			valItem.Observacion = fmt.Sprintf("Fecha no válida (%s)", fechaUsada)
			valItem.ItemRecalculado = it
			report.TotalError++
			report.Items = append(report.Items, valItem)
			continue
		}

		// Buscar en rates
		rate, found := findRate(parsedDate, rates)
		if !found {
			valItem.Estado = "SIN_TC"
			valItem.Observacion = fmt.Sprintf("SUNAT no publicó cotización oficial para %02d/%02d/%04d", parsedDate.Day(), parsedDate.Month(), parsedDate.Year())
			valItem.ItemRecalculado = it
			report.TotalSinTC++
			report.Items = append(report.Items, valItem)
			continue
		}

		tcSunat := rate.Venta
		if strings.EqualFold(tcType, "Compra") {
			tcSunat = rate.Compra
		}
		valItem.TCSunat = tcSunat
		valItem.Diferencia = round(tcArchivo-tcSunat, 4)

		ncTag := ""
		if isNC {
			ncTag = " [NC fecha ref]"
		}

		if tcArchivo <= 0 {
			valItem.Estado = "VACIO"
			valItem.Observacion = fmt.Sprintf("Sin T/C en propuesta. SUNAT %s: %.4f%s", tcType, tcSunat, ncTag)
			report.TotalVacio++
		} else if math.Abs(tcArchivo-tcSunat) > 0.0001 {
			valItem.Estado = "DIFERENTE"
			valItem.Observacion = fmt.Sprintf("Propuesta %.4f ≠ SUNAT %s %.4f%s", tcArchivo, tcType, tcSunat, ncTag)
			report.TotalDiferente++
		} else {
			valItem.Estado = "OK"
			valItem.Observacion = fmt.Sprintf("Coincide con SUNAT %s %.4f%s", tcType, tcSunat, ncTag)
			report.TotalOK++
		}

		// Recalcular montos
		recalcItem := recalculateItem(it, tcArchivo, tcSunat)
		valItem.ImporteRecalculado = parseDecimal(recalcItem.ImporteTotal)
		valItem.ItemRecalculado = recalcItem

		report.Items = append(report.Items, valItem)
	}

	return report
}

func isNotaCredito(it sirepreview.ProposalItem) bool {
	tipo := strings.TrimSpace(it.Tipo)
	if tipo == "07" || tipo == "7" {
		return true
	}
	tipoRef := strings.TrimSpace(it.TipoDocRef)
	if tipoRef != "" && (tipo == "07" || strings.HasPrefix(it.CompPago, "07") || strings.HasPrefix(it.CompPago, "FC") || strings.HasPrefix(it.CompPago, "BC")) {
		return true
	}
	return false
}

func findRate(t time.Time, rates map[string]exchangerate.Rate) (exchangerate.Rate, bool) {
	ymd := t.Format("2006-01-02")
	if r, ok := rates[ymd]; ok && (r.Compra > 0 || r.Venta > 0) {
		return r, true
	}
	dmy := fmt.Sprintf("%02d/%02d/%04d", t.Day(), t.Month(), t.Year())
	if r, ok := rates[dmy]; ok && (r.Compra > 0 || r.Venta > 0) {
		return r, true
	}
	return exchangerate.Rate{}, false
}

func parseDate(str string) (time.Time, bool) {
	str = strings.TrimSpace(str)
	if str == "" {
		return time.Time{}, false
	}
	formats := []string{
		"02/01/2006",
		"2006-01-02",
		"02-01-2006",
		"2/1/2006",
		"2006/01/02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, str); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseDecimal(s string) float64 {
	clean := strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if clean == "" {
		return 0
	}
	f, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0
	}
	return f
}

func round(val float64, decimals int) float64 {
	factor := math.Pow10(decimals)
	return math.Round(val*factor) / factor
}

func recalculateItem(it sirepreview.ProposalItem, tcArchivo, tcSunat float64) sirepreview.ProposalItem {
	recalc := it

	recalcAmount := func(originalStr string) string {
		orig := parseDecimal(originalStr)
		if orig == 0 {
			return "0.00"
		}
		var nuevo float64
		if tcArchivo <= 0 {
			nuevo = orig * tcSunat
		} else {
			nuevo = (orig / tcArchivo) * tcSunat
		}
		return fmt.Sprintf("%.2f", round(nuevo, 2))
	}

	recalc.BIGravada = recalcAmount(it.BIGravada)
	recalc.BIGravada10 = recalcAmount(it.BIGravada10)
	recalc.BIGravYNoGrav = recalcAmount(it.BIGravYNoGrav)
	recalc.BINoGravada = recalcAmount(it.BINoGravada)
	recalc.AdqNoGravada = recalcAmount(it.AdqNoGravada)
	recalc.ICBPER = recalcAmount(it.ICBPER)
	recalc.IGV = recalcAmount(it.IGV)
	recalc.GravYNoGravIGV = recalcAmount(it.GravYNoGravIGV)
	recalc.IGV10 = recalcAmount(it.IGV10)
	recalc.ImporteTotal = recalcAmount(it.ImporteTotal)
	recalc.ISC = recalcAmount(it.ISC)
	recalc.NoGravIGV = recalcAmount(it.NoGravIGV)
	recalc.OtrosConceptos = recalcAmount(it.OtrosConceptos)
	recalc.OtrosTributos = recalcAmount(it.OtrosTributos)
	recalc.ValorAdquisiciones = recalcAmount(it.ValorAdquisiciones)

	recalc.TipoCambio = fmt.Sprintf("%.3f", tcSunat)
	return recalc
}
