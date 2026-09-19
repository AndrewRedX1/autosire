package validation

import (
	"math"
	"testing"

	"appsire-go/internal/sirepreview"
)

func TestDetectarDescuadres(t *testing.T) {
	items := []sirepreview.ProposalItem{
		// 1. Cuadrado exacto
		{
			CompPago:     "01-F001-00000001",
			Serie:        "F001",
			Numero:       "00000001",
			BIGravada:    "1000.00",
			IGV:          "180.00",
			ImporteTotal: "1180.00",
		},
		// 2. Descuadre por defecto (-0.01)
		{
			CompPago:     "01-F001-00000002",
			Serie:        "F001",
			Numero:       "00000002",
			BIGravada:    "1000.01",
			IGV:          "180.00",
			ImporteTotal: "1180.00",
		},
		// 3. Descuadre por exceso (+0.02)
		{
			CompPago:     "01-F001-00000003",
			Serie:        "F001",
			Numero:       "00000003",
			BIGravada:    "500.00",
			IGV:          "89.98",
			ImporteTotal: "590.00",
		},
	}

	report := DetectarDescuadres(items, "RCE")

	if report.TotalItems != 3 {
		t.Errorf("TotalItems = %d; esperaba 3", report.TotalItems)
	}
	if report.TotalCuadrados != 1 {
		t.Errorf("TotalCuadrados = %d; esperaba 1", report.TotalCuadrados)
	}
	if report.TotalDescuadres != 2 {
		t.Errorf("TotalDescuadres = %d; esperaba 2", report.TotalDescuadres)
	}
	if report.TotalPorDefecto != 1 {
		t.Errorf("TotalPorDefecto = %d; esperaba 1", report.TotalPorDefecto)
	}
	if report.TotalPorExceso != 1 {
		t.Errorf("TotalPorExceso = %d; esperaba 1", report.TotalPorExceso)
	}

	// -0.01 + 0.02 = +0.01
	if report.DiferenciaNeta != 0.01 {
		t.Errorf("DiferenciaNeta = %f; esperaba 0.01", report.DiferenciaNeta)
	}

	// Comprobar item 0 descuadrado (Fila 2)
	descuadre1 := report.Items[0]
	if descuadre1.Diferencia != -0.01 {
		t.Errorf("descuadre 1 diferencia = %f; esperaba -0.01", descuadre1.Diferencia)
	}
	if descuadre1.ValorAjustado != 1000.00 {
		t.Errorf("descuadre 1 valor ajustado = %f; esperaba 1000.00", descuadre1.ValorAjustado)
	}

	// Comprobar que el item recalculado efectivamente cuadra
	nuevaSuma := SumarComponentes(descuadre1.ItemRecalculado, true)
	if math.Abs(nuevaSuma-descuadre1.ImporteTotal) > 0.0001 {
		t.Errorf("nueva suma = %f no coincide con importe total = %f", nuevaSuma, descuadre1.ImporteTotal)
	}
}

func TestAplicarAjusteEnIGV(t *testing.T) {
	it := sirepreview.ProposalItem{
		BIGravada:    "1000.00",
		IGV:          "179.99",
		ImporteTotal: "1180.00",
	}

	dif := 0.01 // ImporteTotal (1180.00) - Suma (1179.99) = +0.01
	itemAjustado, valAct, valAj := AplicarAjuste(it, "igv", dif)

	if valAct != 179.99 {
		t.Errorf("valor actual = %f; esperaba 179.99", valAct)
	}
	if valAj != 180.00 {
		t.Errorf("valor ajustado = %f; esperaba 180.00", valAj)
	}
	if itemAjustado.IGV != "180.00" {
		t.Errorf("itemAjustado.IGV = %s; esperaba 180.00", itemAjustado.IGV)
	}

	nuevaSuma := SumarComponentes(itemAjustado, true)
	if nuevaSuma != 1180.00 {
		t.Errorf("nueva suma = %f; esperaba 1180.00", nuevaSuma)
	}
}
