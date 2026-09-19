package validation

import (
	"testing"

	"appsire-go/internal/sirepreview"
)

func TestDetectarCorrelativosSinHuecos(t *testing.T) {
	items := []sirepreview.ProposalItem{
		{Tipo: "01", Serie: "F001", Numero: "00000001", Fecha: "01/09/2026"},
		{Tipo: "01", Serie: "F001", Numero: "00000002", Fecha: "02/09/2026"},
		{Tipo: "01", Serie: "F001", Numero: "00000003", Fecha: "03/09/2026"},
	}

	report := DetectarCorrelativos(items)
	if report.TotalFaltantes != 0 {
		t.Fatalf("Esperaba 0 faltantes, obtuvo %d", report.TotalFaltantes)
	}
	if report.TotalSeries != 1 {
		t.Errorf("Esperaba 1 serie, obtuvo %d", report.TotalSeries)
	}
	if report.SeriesConFaltantes != 0 {
		t.Errorf("Esperaba 0 series con faltantes, obtuvo %d", report.SeriesConFaltantes)
	}
}

func TestDetectarCorrelativosConHuecos(t *testing.T) {
	items := []sirepreview.ProposalItem{
		{Tipo: "01", Serie: "F001", Numero: "00000001", Fecha: "01/09/2026", BIGravada: "100.00", ImporteTotal: "118.00"},
		{Tipo: "01", Serie: "F001", Numero: "00000003", Fecha: "05/09/2026", BIGravada: "200.00", ImporteTotal: "236.00"},
		{Tipo: "01", Serie: "F001", Numero: "00000005", Fecha: "10/09/2026", BIGravada: "50.00", ImporteTotal: "59.00"},
		// Otra serie
		{Tipo: "03", Serie: "B001", Numero: "10", Fecha: "02/09/2026"},
		{Tipo: "03", Serie: "B001", Numero: "12", Fecha: "04/09/2026"},
	}

	report := DetectarCorrelativos(items)
	if report.TotalSeries != 2 {
		t.Errorf("Esperaba 2 series, obtuvo %d", report.TotalSeries)
	}
	if report.SeriesConFaltantes != 2 {
		t.Errorf("Esperaba 2 series con faltantes, obtuvo %d", report.SeriesConFaltantes)
	}
	// F001 falta 2 y 4; B001 falta 11. Total faltantes = 3
	if report.TotalFaltantes != 3 {
		t.Fatalf("Esperaba 3 faltantes, obtuvo %d", report.TotalFaltantes)
	}

	// Verificar F001-2
	f1 := report.Faltantes[0]
	if f1.Serie != "F001" || f1.NumeroInt != 2 {
		t.Errorf("Faltante 0 incorrecto: %+v", f1)
	}
	if f1.ItemPropuesto.RazonSocial != "ANULADO" {
		t.Errorf("Esperaba Razón Social ANULADO, obtuvo %s", f1.ItemPropuesto.RazonSocial)
	}
	if f1.ItemPropuesto.ImporteTotal != "0.00" {
		t.Errorf("Esperaba ImporteTotal 0.00, obtuvo %s", f1.ItemPropuesto.ImporteTotal)
	}
	if f1.ItemPropuesto.BIGravada != "0.00" {
		t.Errorf("Esperaba BIGravada 0.00, obtuvo %s", f1.ItemPropuesto.BIGravada)
	}
	if f1.ItemPropuesto.TipoDocIdent != "0" {
		t.Errorf("Esperaba TipoDocIdent 0, obtuvo %s", f1.ItemPropuesto.TipoDocIdent)
	}
	if f1.ItemPropuesto.RUC != "0001" {
		t.Errorf("Esperaba RUC 0001, obtuvo %s", f1.ItemPropuesto.RUC)
	}
	// Fecha de referencia debe ser la más cercana (1 o 3 tienen distancia 1)
	if f1.FechaReferencia != "01/09/2026" && f1.FechaReferencia != "05/09/2026" {
		t.Errorf("Fecha de referencia inesperada: %s", f1.FechaReferencia)
	}

	// Verificar B001-11
	f3 := report.Faltantes[2]
	if f3.Serie != "B001" || f3.NumeroInt != 11 {
		t.Errorf("Faltante 2 incorrecto: %+v", f3)
	}
}

func TestDetectarCorrelativosSaltoExcesivo(t *testing.T) {
	items := []sirepreview.ProposalItem{
		{Tipo: "01", Serie: "F001", Numero: "1", Fecha: "01/09/2026"},
		{Tipo: "01", Serie: "F001", Numero: "10000", Fecha: "15/09/2026"},
	}

	report := DetectarCorrelativos(items)
	if len(report.Advertencias) == 0 {
		t.Fatal("Esperaba advertencia por salto mayor a 5000")
	}
	if report.TotalFaltantes != 0 {
		t.Errorf("Serie omitida no debió generar faltantes, generó %d", report.TotalFaltantes)
	}
}
