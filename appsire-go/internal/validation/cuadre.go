package validation

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"appsire-go/internal/sirepreview"
)

const (
	// ToleranciaCuadre es la diferencia mínima permitida para considerar un comprobante cuadrado.
	ToleranciaCuadre = 0.005
)

// ColumnaAjusteOpcion describe una columna contable disponible para recibir el ajuste de cuadre.
type ColumnaAjusteOpcion struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

// CuadreItem representa un comprobante con descuadre numérico entre sus componentes y el total.
type CuadreItem struct {
	Index           int                      `json:"index"`
	CompPago        string                   `json:"comp_pago"`
	Tipo            string                   `json:"tipo"`
	Serie           string                   `json:"serie"`
	Numero          string                   `json:"numero"`
	Fecha           string                   `json:"fecha"`
	RUC             string                   `json:"ruc"`
	RazonSocial     string                   `json:"razon_social"`
	SumaComponentes float64                  `json:"suma_componentes"`
	ImporteTotal    float64                  `json:"importe_total"`
	Diferencia      float64                  `json:"diferencia"`
	ColumnaAjuste   string                   `json:"columna_ajuste"`
	ValorActual     float64                  `json:"valor_actual"`
	ValorAjustado   float64                  `json:"valor_ajustado"`
	ItemRecalculado sirepreview.ProposalItem `json:"item_recalculado"`
}

// CuadreReport consolida el resultado de la auditoría de cuadre de importes.
type CuadreReport struct {
	Book                 string                `json:"book"`
	TotalItems           int                   `json:"total_items"`
	TotalDescuadres      int                   `json:"total_descuadres"`
	TotalCuadrados       int                   `json:"total_cuadrados"`
	TotalPorExceso       int                   `json:"total_por_exceso"` // Diferencia > 0 (Total > Suma)
	TotalPorDefecto      int                   `json:"total_por_defecto"` // Diferencia < 0 (Total < Suma)
	DiferenciaNeta       float64               `json:"diferencia_neta"`
	ColumnasDisponibles  []ColumnaAjusteOpcion `json:"columnas_disponibles"`
	ColumnaDefault       string                `json:"columna_default"`
	Items                []CuadreItem          `json:"items"`
}

// ObtenerColumnasAjuste retorna las opciones de columnas a ajustar según el libro tributario.
func ObtenerColumnasAjuste(book string) []ColumnaAjusteOpcion {
	if strings.EqualFold(book, "RVIE") {
		return []ColumnaAjusteOpcion{
			{ID: "bi_gravada", Nombre: "Base Imponible Gravada"},
			{ID: "igv", Nombre: "IGV"},
			{ID: "adq_no_gravada", Nombre: "Operación Exonerada"},
			{ID: "bi_no_gravada", Nombre: "Operación Inafecta"},
			{ID: "otros_conceptos", Nombre: "Otros Tributos y Cargos"},
		}
	}
	// RCE (Compras)
	return []ColumnaAjusteOpcion{
		{ID: "bi_gravada", Nombre: "Base Imponible Gravada (Recomendado)"},
		{ID: "igv", Nombre: "IGV (Operaciones Gravadas)"},
		{ID: "adq_no_gravada", Nombre: "Adquisiciones No Gravadas"},
		{ID: "bi_grav_y_no_grav", Nombre: "Base Gravada y No Gravada"},
		{ID: "otros_conceptos", Nombre: "Otros Tributos y Cargos"},
	}
}

// SumarComponentes calcula la suma contable de los conceptos que integran el comprobante.
func SumarComponentes(it sirepreview.ProposalItem, isRce bool) float64 {
	if isRce {
		return parseMontoFloat(it.BIGravada) +
			parseMontoFloat(it.BIGravada10) +
			parseMontoFloat(it.BIGravYNoGrav) +
			parseMontoFloat(it.BINoGravada) +
			parseMontoFloat(it.AdqNoGravada) +
			parseMontoFloat(it.ValorAdquisiciones) +
			parseMontoFloat(it.IGV) +
			parseMontoFloat(it.IGV10) +
			parseMontoFloat(it.GravYNoGravIGV) +
			parseMontoFloat(it.NoGravIGV) +
			parseMontoFloat(it.ISC) +
			parseMontoFloat(it.ICBPER) +
			parseMontoFloat(it.OtrosConceptos) +
			parseMontoFloat(it.OtrosTributos)
	}

	// RVIE (Ventas)
	return parseMontoFloat(it.BIGravada) +
		parseMontoFloat(it.BIGravada10) +
		parseMontoFloat(it.AdqNoGravada) + // Exonerada
		parseMontoFloat(it.BINoGravada) +  // Inafecta
		parseMontoFloat(it.IGV) +
		parseMontoFloat(it.IGV10) +
		parseMontoFloat(it.ISC) +
		parseMontoFloat(it.ICBPER) +
		parseMontoFloat(it.OtrosConceptos) +
		parseMontoFloat(it.OtrosTributos)
}

// DetectarDescuadres analiza todos los ítems de la propuesta y detecta diferencias con el Importe Total.
func DetectarDescuadres(items []sirepreview.ProposalItem, book string) CuadreReport {
	isRce := !strings.EqualFold(book, "RVIE")
	bookName := "RCE"
	if !isRce {
		bookName = "RVIE"
	}

	cols := ObtenerColumnasAjuste(bookName)
	defaultCol := "bi_gravada"

	report := CuadreReport{
		Book:                bookName,
		TotalItems:          len(items),
		ColumnasDisponibles: cols,
		ColumnaDefault:      defaultCol,
		Items:               make([]CuadreItem, 0),
	}

	var difNeta float64

	for idx, it := range items {
		impTotal := parseMontoFloat(it.ImporteTotal)
		sumaComp := SumarComponentes(it, isRce)

		// Si ambos son cero, no hay datos financieros
		if impTotal == 0 && sumaComp == 0 {
			report.TotalCuadrados++
			continue
		}

		dif := math.Round((impTotal-sumaComp)*100) / 100

		if math.Abs(dif) < ToleranciaCuadre {
			report.TotalCuadrados++
			continue
		}

		report.TotalDescuadres++
		difNeta += dif

		if dif > 0 {
			report.TotalPorExceso++
		} else {
			report.TotalPorDefecto++
		}

		// Calcular el ajuste con la columna por defecto
		itemAjustado, valActual, valAjustado := AplicarAjuste(it, defaultCol, dif)

		report.Items = append(report.Items, CuadreItem{
			Index:           idx,
			CompPago:        it.CompPago,
			Tipo:            it.Tipo,
			Serie:           it.Serie,
			Numero:          it.Numero,
			Fecha:           it.Fecha,
			RUC:             it.RUC,
			RazonSocial:     it.RazonSocial,
			SumaComponentes: math.Round(sumaComp*100) / 100,
			ImporteTotal:    math.Round(impTotal*100) / 100,
			Diferencia:      dif,
			ColumnaAjuste:   defaultCol,
			ValorActual:     valActual,
			ValorAjustado:   valAjustado,
			ItemRecalculado: itemAjustado,
		})
	}

	report.DiferenciaNeta = math.Round(difNeta*100) / 100
	return report
}

// AplicarAjuste ajusta el valor de la columna seleccionada sumándole la diferencia,
// de modo que la nueva suma de componentes cuadre exactamente con el Importe Total.
func AplicarAjuste(it sirepreview.ProposalItem, colID string, diferencia float64) (sirepreview.ProposalItem, float64, float64) {
	cloned := it
	var valActual float64

	switch colID {
	case "bi_gravada":
		valActual = parseMontoFloat(cloned.BIGravada)
		valAjustado := math.Round((valActual+diferencia)*100) / 100
		cloned.BIGravada = fmt.Sprintf("%.2f", valAjustado)
		return cloned, valActual, valAjustado

	case "igv":
		valActual = parseMontoFloat(cloned.IGV)
		valAjustado := math.Round((valActual+diferencia)*100) / 100
		cloned.IGV = fmt.Sprintf("%.2f", valAjustado)
		return cloned, valActual, valAjustado

	case "adq_no_gravada":
		valActual = parseMontoFloat(cloned.AdqNoGravada)
		valAjustado := math.Round((valActual+diferencia)*100) / 100
		cloned.AdqNoGravada = fmt.Sprintf("%.2f", valAjustado)
		return cloned, valActual, valAjustado

	case "bi_grav_y_no_grav":
		valActual = parseMontoFloat(cloned.BIGravYNoGrav)
		valAjustado := math.Round((valActual+diferencia)*100) / 100
		cloned.BIGravYNoGrav = fmt.Sprintf("%.2f", valAjustado)
		return cloned, valActual, valAjustado

	case "bi_no_gravada":
		valActual = parseMontoFloat(cloned.BINoGravada)
		valAjustado := math.Round((valActual+diferencia)*100) / 100
		cloned.BINoGravada = fmt.Sprintf("%.2f", valAjustado)
		return cloned, valActual, valAjustado

	case "otros_conceptos":
		valActual = parseMontoFloat(cloned.OtrosConceptos)
		valAjustado := math.Round((valActual+diferencia)*100) / 100
		cloned.OtrosConceptos = fmt.Sprintf("%.2f", valAjustado)
		return cloned, valActual, valAjustado

	default:
		// Fallback a bi_gravada
		valActual = parseMontoFloat(cloned.BIGravada)
		valAjustado := math.Round((valActual+diferencia)*100) / 100
		cloned.BIGravada = fmt.Sprintf("%.2f", valAjustado)
		return cloned, valActual, valAjustado
	}
}

func parseMontoFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	if s == "" || s == "-" {
		return 0.0
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return val
}
