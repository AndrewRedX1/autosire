package cpe

import (
	"context"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"appsire-go/internal/sirepreview"
)

// CPEValidatedItem representa un comprobante auditado ante SUNAT
type CPEValidatedItem struct {
	Index         int     `json:"index"`
	CompPago      string  `json:"comp_pago"`
	Tipo          string  `json:"tipo"`
	Serie         string  `json:"serie"`
	Numero        string  `json:"numero"`
	Fecha         string  `json:"fecha"`
	RUCEmisor     string  `json:"ruc_emisor"`
	RazonSocial   string  `json:"razon_social"`
	Moneda        string  `json:"moneda"`
	MontoOriginal float64 `json:"monto_original"`
	EstadoCP      string  `json:"estado_cp"`
	EstadoRUC     string  `json:"estado_ruc"`
	CondDomi      string  `json:"cond_domicilio"`
	Observaciones string  `json:"observaciones"`
	EsRiesgo      bool    `json:"es_riesgo"`
	Error         string  `json:"error,omitempty"`
}

// CPEReport contiene el resumen estadístico y detalle de la auditoría CPE
type CPEReport struct {
	Book           string             `json:"book"`
	RUCConsultante string             `json:"ruc_consultante"`
	TotalAuditados int                `json:"total_auditados"`
	TotalAceptados int                `json:"total_aceptados"`
	TotalAnulados  int                `json:"total_anulados"`
	TotalNoExiste  int                `json:"total_no_existe"`
	TotalNoHabido  int                `json:"total_no_habido"`
	TotalBaja      int                `json:"total_baja"`
	TotalErrores   int                `json:"total_errores"`
	TotalConRiesgo int                `json:"total_con_riesgo"`
	Items          []CPEValidatedItem `json:"items"`
}

const MaxWorkers = 6

// ValidateBatch audita concurrentemente un conjunto de comprobantes usando hasta 6 workers
func ValidateBatch(
	ctx context.Context,
	client *Client,
	rucConsultante string,
	book string,
	items []sirepreview.ProposalItem,
	token string,
	onProgress func(item CPEValidatedItem, current, total int),
) (*CPEReport, error) {
	total := len(items)
	report := &CPEReport{
		Book:           book,
		RUCConsultante: rucConsultante,
		TotalAuditados: total,
		Items:          make([]CPEValidatedItem, total),
	}

	if total == 0 {
		return report, nil
	}

	isVentas := strings.EqualFold(strings.TrimSpace(book), "RVIE") || strings.EqualFold(strings.TrimSpace(book), "ventas")

	type job struct {
		index int
		item  sirepreview.ProposalItem
	}

	jobs := make(chan job, total)
	for i, it := range items {
		jobs <- job{index: i, item: it}
	}
	close(jobs)

	var (
		mu         sync.Mutex
		doneCount  int
		reportMu   sync.Mutex
		numWorkers = MaxWorkers
	)
	if total < numWorkers {
		numWorkers = total
	}

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for w := 0; w < numWorkers; w++ {
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					// El contexto fue cancelado por el usuario
					valItem := prepareCancelledItem(j.index, j.item, isVentas, rucConsultante)
					reportMu.Lock()
					report.Items[j.index] = valItem
					reportMu.Unlock()
					continue
				default:
				}

				valItem := validateSingle(ctx, client, rucConsultante, isVentas, j.index, j.item, token)

				reportMu.Lock()
				report.Items[j.index] = valItem
				classifyItem(report, valItem)
				reportMu.Unlock()

				mu.Lock()
				doneCount++
				currentDone := doneCount
				mu.Unlock()

				if onProgress != nil {
					onProgress(valItem, currentDone, total)
				}
			}
		}()
	}

	wg.Wait()
	return report, nil
}

func validateSingle(
	ctx context.Context,
	client *Client,
	rucConsultante string,
	isVentas bool,
	index int,
	it sirepreview.ProposalItem,
	token string,
) CPEValidatedItem {
	rucEmisor := strings.TrimSpace(it.RUC)
	razonSocial := strings.TrimSpace(it.RazonSocial)
	if isVentas {
		// En Ventas (RVIE), el emisor del comprobante es la propia empresa consultante
		rucEmisor = rucConsultante
	}

	fechaEmision := normalizeDate(it.Fecha)
	montoOriginal := calculateOriginalAmount(it)

	valItem := CPEValidatedItem{
		Index:         index,
		CompPago:      it.CompPago,
		Tipo:          it.Tipo,
		Serie:         it.Serie,
		Numero:        it.Numero,
		Fecha:         fechaEmision,
		RUCEmisor:     rucEmisor,
		RazonSocial:   razonSocial,
		Moneda:        strings.ToUpper(strings.TrimSpace(it.Moneda)),
		MontoOriginal: montoOriginal,
	}

	if valItem.Moneda == "" {
		valItem.Moneda = "PEN"
	}

	query := VoucherQuery{
		NumRUC:       rucEmisor,
		CodComp:      it.Tipo,
		NumeroSerie:  it.Serie,
		Numero:       it.Numero,
		FechaEmision: fechaEmision,
		Monto:        montoOriginal,
	}

	res, err := client.ValidateVoucher(ctx, rucConsultante, query, token)
	if err != nil {
		valItem.EstadoCP = "ERROR"
		valItem.Error = err.Error()
		valItem.Observaciones = err.Error()
		valItem.EsRiesgo = true
		return valItem
	}

	valItem.EstadoCP = res.EstadoComprobante
	valItem.EstadoRUC = res.EstadoRuc
	valItem.CondDomi = res.CondicionDomi
	valItem.Observaciones = res.ObservacionTexto

	// Evaluar si representa un riesgo fiscal
	valItem.EsRiesgo = isTaxRisk(res.EstadoComprobante, res.EstadoRuc, res.CondicionDomi)

	return valItem
}

func isTaxRisk(estadoCP, estadoRUC, condDomi string) bool {
	cp := strings.ToUpper(strings.TrimSpace(estadoCP))
	ruc := strings.ToUpper(strings.TrimSpace(estadoRUC))
	domi := strings.ToUpper(strings.TrimSpace(condDomi))

	// 1. Estado de comprobante peligroso
	if cp == "NO EXISTE" || cp == "ANULADO" || cp == "NO AUTORIZADO" || cp == "ERROR" {
		return true
	}
	// 2. Estado de RUC peligroso
	if ruc != "" && ruc != "ACTIVO" {
		return true
	}
	// 3. Condición de domicilio peligrosa
	if domi == "NO HABIDO" || domi == "NO HALLADO" || domi == "PENDIENTE" {
		return true
	}
	return false
}

func classifyItem(report *CPEReport, it CPEValidatedItem) {
	if it.EsRiesgo {
		report.TotalConRiesgo++
	}

	switch strings.ToUpper(it.EstadoCP) {
	case "ACEPTADO", "AUTORIZADO":
		report.TotalAceptados++
	case "ANULADO":
		report.TotalAnulados++
	case "NO EXISTE", "NO AUTORIZADO":
		report.TotalNoExiste++
	case "ERROR":
		report.TotalErrores++
	}

	ruc := strings.ToUpper(it.EstadoRUC)
	if strings.Contains(ruc, "BAJA") || ruc == "SUSPENSION TEMPORAL" || ruc == "INHABILITADO-VENT.UNICA" {
		report.TotalBaja++
	}

	domi := strings.ToUpper(it.CondDomi)
	if domi == "NO HABIDO" || domi == "NO HALLADO" {
		report.TotalNoHabido++
	}
}

func prepareCancelledItem(index int, it sirepreview.ProposalItem, isVentas bool, rucConsultante string) CPEValidatedItem {
	rucEmisor := strings.TrimSpace(it.RUC)
	if isVentas {
		rucEmisor = rucConsultante
	}
	return CPEValidatedItem{
		Index:         index,
		CompPago:      it.CompPago,
		Tipo:          it.Tipo,
		Serie:         it.Serie,
		Numero:        it.Numero,
		Fecha:         normalizeDate(it.Fecha),
		RUCEmisor:     rucEmisor,
		RazonSocial:   strings.TrimSpace(it.RazonSocial),
		Moneda:        strings.ToUpper(strings.TrimSpace(it.Moneda)),
		MontoOriginal: calculateOriginalAmount(it),
		EstadoCP:      "CANCELADO",
		Observaciones: "Operación cancelada por el usuario",
	}
}

func calculateOriginalAmount(it sirepreview.ProposalItem) float64 {
	total := parseDecimal(it.ImporteTotal)
	moneda := strings.ToUpper(strings.TrimSpace(it.Moneda))
	tc := parseDecimal(it.TipoCambio)

	if moneda != "" && moneda != "PEN" && tc > 0 {
		return math.Round((total/tc)*100) / 100
	}
	return total
}

func parseDecimal(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return math.Abs(val)
}

func normalizeDate(d string) string {
	d = strings.TrimSpace(d)
	if len(d) == 10 && d[2] == '/' && d[5] == '/' {
		return d
	}
	// Si viene en formato YYYY-MM-DD
	if t, err := time.Parse("2006-01-02", d); err == nil {
		return t.Format("02/01/2006")
	}
	// Si viene en formato DD-MM-YYYY
	if t, err := time.Parse("02-01-2006", d); err == nil {
		return t.Format("02/01/2006")
	}
	return d
}
