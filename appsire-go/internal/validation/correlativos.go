package validation

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"appsire-go/internal/sirepreview"
)

const (
	// MaxFaltantesPorSerie es el límite máximo de faltantes analizados por serie para evitar desbordes ante anomalías.
	MaxFaltantesPorSerie = 5000
)

// CorrelativoFaltante representa un número omitido dentro de una serie emitida de ventas.
type CorrelativoFaltante struct {
	Tipo            string                   `json:"tipo"`
	Serie           string                   `json:"serie"`
	Numero          string                   `json:"numero"`
	NumeroInt       int64                    `json:"numero_int"`
	FechaReferencia string                   `json:"fecha_referencia"`
	ItemPropuesto   sirepreview.ProposalItem `json:"item_propuesto"`
}

// CorrelativosReport consolida el resultado del escaneo de correlativos para RVIE.
type CorrelativosReport struct {
	Book               string                `json:"book"`
	TotalDocumentos    int                   `json:"total_documentos"`
	TotalSeries        int                   `json:"total_series"`
	TotalFaltantes     int                   `json:"total_faltantes"`
	SeriesConFaltantes int                   `json:"series_con_faltantes"`
	SeriesRevisadas    []string              `json:"series_revisadas"`
	Advertencias       []string              `json:"advertencias"`
	Faltantes          []CorrelativoFaltante `json:"faltantes"`
}

type itemRef struct {
	numero int64
	fecha  string
	padLen int
	item   sirepreview.ProposalItem
}

type serieGrupo struct {
	tipoDoc   string
	serie     string
	presentes map[int64]bool
	items     []itemRef
	minNum    int64
	maxNum    int64
	maxPad    int
}

// DetectarCorrelativos analiza la numeración correlativa por Tipo + Serie en la propuesta de Ventas (RVIE).
func DetectarCorrelativos(items []sirepreview.ProposalItem) CorrelativosReport {
	report := CorrelativosReport{
		Book:            "RVIE",
		TotalDocumentos: len(items),
		SeriesRevisadas: make([]string, 0),
		Advertencias:    make([]string, 0),
		Faltantes:       make([]CorrelativoFaltante, 0),
	}

	if len(items) == 0 {
		return report
	}

	grupos := make(map[string]*serieGrupo)

	for _, it := range items {
		tipo := strings.TrimSpace(it.Tipo)
		serie := strings.ToUpper(strings.TrimSpace(it.Serie))
		numStr := strings.TrimSpace(it.Numero)

		if tipo == "" && strings.Contains(it.CompPago, "-") {
			parts := strings.SplitN(it.CompPago, "-", 2)
			if serie == "" {
				serie = strings.ToUpper(strings.TrimSpace(parts[0]))
			}
			if numStr == "" {
				numStr = strings.TrimSpace(parts[1])
			}
		}

		if tipo == "" {
			tipo = "01" // Tipo Factura por defecto si no está especificado
		}

		numInt, padLen, ok := parseNumero(numStr)
		if !ok || serie == "" {
			continue
		}

		key := fmt.Sprintf("%s||%s", tipo, serie)
		g, exists := grupos[key]
		if !exists {
			g = &serieGrupo{
				tipoDoc:   tipo,
				serie:     serie,
				presentes: make(map[int64]bool),
				items:     make([]itemRef, 0),
				minNum:    math.MaxInt64,
				maxNum:    math.MinInt64,
				maxPad:    padLen,
			}
			grupos[key] = g
		}

		g.presentes[numInt] = true
		if padLen > g.maxPad {
			g.maxPad = padLen
		}
		if numInt < g.minNum {
			g.minNum = numInt
		}
		if numInt > g.maxNum {
			g.maxNum = numInt
		}

		g.items = append(g.items, itemRef{
			numero: numInt,
			fecha:  strings.TrimSpace(it.Fecha),
			padLen: padLen,
			item:   it,
		})
	}

	// Ordenar claves de grupos para resultado determinista
	keys := make([]string, 0, len(grupos))
	for k := range grupos {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	report.TotalSeries = len(keys)
	seriesConHuecos := 0

	for _, k := range keys {
		g := grupos[k]
		report.SeriesRevisadas = append(report.SeriesRevisadas, fmt.Sprintf("%s - %s", g.tipoDoc, g.serie))

		if len(g.presentes) == 0 || g.minNum > g.maxNum {
			continue
		}

		teoricos := g.maxNum - g.minNum + 1
		faltantesCount := teoricos - int64(len(g.presentes))

		if faltantesCount <= 0 {
			continue
		}

		if faltantesCount > MaxFaltantesPorSerie {
			report.Advertencias = append(report.Advertencias, fmt.Sprintf(
				"Tipo %s serie %s: %d faltantes entre el n° %d y %d. Se omitió la serie para evitar inconsistencias por reinicio o error de datos.",
				g.tipoDoc, g.serie, faltantesCount, g.minNum, g.maxNum,
			))
			continue
		}

		seriesConHuecos++

		padLen := g.maxPad
		if padLen < 8 {
			padLen = 8 // SUNAT suele manejar correlativos a 8 dígitos
		}

		for n := g.minNum; n <= g.maxNum; n++ {
			if !g.presentes[n] {
				ref := g.buscarMasCercano(n)
				fechaRef := ""
				var refItem *sirepreview.ProposalItem
				if ref != nil {
					fechaRef = ref.fecha
					refItem = &ref.item
				}

				itemAnulado := ConstruirItemAnulado(g.tipoDoc, g.serie, n, padLen, refItem)

				report.Faltantes = append(report.Faltantes, CorrelativoFaltante{
					Tipo:            g.tipoDoc,
					Serie:           g.serie,
					Numero:          itemAnulado.Numero,
					NumeroInt:       n,
					FechaReferencia: fechaRef,
					ItemPropuesto:   itemAnulado,
				})
			}
		}
	}

	report.SeriesConFaltantes = seriesConHuecos
	report.TotalFaltantes = len(report.Faltantes)
	return report
}

// buscarMasCercano encuentra el comprobante presente con el número más próximo al faltante.
func (g *serieGrupo) buscarMasCercano(objetivo int64) *itemRef {
	var masCercano *itemRef
	var distMin int64 = math.MaxInt64

	for i := range g.items {
		it := &g.items[i]
		dist := it.numero - objetivo
		if dist < 0 {
			dist = -dist
		}

		if dist < distMin || (dist == distMin && masCercano != nil && it.numero < masCercano.numero) {
			distMin = dist
			masCercano = it
		}
	}

	return masCercano
}

// ConstruirItemAnulado genera la estructura de un comprobante de venta anulado con importes en 0.
func ConstruirItemAnulado(tipo, serie string, numero int64, padLen int, refItem *sirepreview.ProposalItem) sirepreview.ProposalItem {
	numStr := strconv.FormatInt(numero, 10)
	if padLen > 0 && len(numStr) < padLen {
		numStr = strings.Repeat("0", padLen-len(numStr)) + numStr
	}

	compPago := fmt.Sprintf("%s-%s", serie, numStr)
	fecha := ""
	moneda := "PEN"
	tc := "1.000"

	if refItem != nil {
		if strings.TrimSpace(refItem.Fecha) != "" {
			fecha = refItem.Fecha
		}
		if strings.TrimSpace(refItem.Moneda) != "" {
			moneda = refItem.Moneda
		}
		if strings.TrimSpace(refItem.TipoCambio) != "" && refItem.TipoCambio != "0" && refItem.TipoCambio != "0.00" {
			tc = refItem.TipoCambio
		}
	}

	return sirepreview.ProposalItem{
		CompPago:           compPago,
		Tipo:               tipo,
		Serie:              serie,
		Numero:             numStr,
		Fecha:              fecha,
		TipoDocIdent:       "0", // Sin documento / Otros
		RUC:                "0001",
		RazonSocial:        "ANULADO",
		BIGravada:          "0.00",
		BIGravada10:        "0.00",
		BIGravYNoGrav:      "0.00",
		BINoGravada:        "0.00",
		AdqNoGravada:       "0.00",
		ICBPER:             "0.00",
		IGV:                "0.00",
		GravYNoGravIGV:     "0.00",
		IGV10:              "0.00",
		ImporteTotal:       "0.00",
		ISC:                "0.00",
		NoGravIGV:          "0.00",
		OtrosConceptos:     "0.00",
		OtrosTributos:      "0.00",
		ValorAdquisiciones: "0.00",
		Moneda:             moneda,
		TipoCambio:         tc,
	}
}

func parseNumero(s string) (int64, int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	s = strings.TrimLeft(s, "-")
	padLen := len(s)

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return val, padLen, true
}
