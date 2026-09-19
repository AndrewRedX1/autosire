package validation

import (
	"testing"

	"appsire-go/internal/exchangerate"
	"appsire-go/internal/sirepreview"
)

func TestValidateTC(t *testing.T) {
	rates := map[string]exchangerate.Rate{
		"2024-09-10": {
			Fecha:  "2024-09-10",
			Compra: 3.802,
			Venta:  3.816,
		},
		"2024-08-15": {
			Fecha:  "2024-08-15",
			Compra: 3.730,
			Venta:  3.740,
		},
	}

	items := []sirepreview.ProposalItem{
		// 1. Factura en PEN -> Debe ser omitida
		{
			CompPago:     "01 F001-100",
			Moneda:       "PEN",
			Fecha:        "10/09/2024",
			ImporteTotal: "1000.00",
		},
		// 2. Factura en USD con TC coincidente (3.816)
		{
			CompPago:     "01 F001-101",
			Tipo:         "01",
			Moneda:       "USD",
			Fecha:        "10/09/2024",
			TipoCambio:   "3.816",
			BIGravada:    "100.00",
			IGV:          "18.00",
			ImporteTotal: "118.00",
		},
		// 3. Factura en USD con TC diferente (3.750 != 3.816)
		{
			CompPago:     "01 F001-102",
			Tipo:         "01",
			Moneda:       "USD",
			Fecha:        "10/09/2024",
			TipoCambio:   "3.750",
			BIGravada:    "375.00",
			IGV:          "67.50",
			ImporteTotal: "442.50",
		},
		// 4. Factura en USD con TC Vacío
		{
			CompPago:     "01 F001-103",
			Tipo:         "01",
			Moneda:       "USD",
			Fecha:        "10/09/2024",
			TipoCambio:   "0.000",
			BIGravada:    "100.00",
			IGV:          "18.00",
			ImporteTotal: "118.00",
		},
		// 5. Nota de Crédito (Tipo 07) emitida en Sep pero con FechaRef en Ago (15/08/2024 -> Venta 3.740)
		{
			CompPago:     "07 FC01-50",
			Tipo:         "07",
			Moneda:       "USD",
			Fecha:        "10/09/2024",
			FechaRef:     "15/08/2024",
			TipoCambio:   "3.816", // Trae el de emisión, pero debe usar el de referencia 3.740
			BIGravada:    "-100.00",
			IGV:          "-18.00",
			ImporteTotal: "-118.00",
		},
	}

	report := ValidateTC(items, "RCE", "Venta", rates)

	if report.TotalUSD != 4 {
		t.Fatalf("esperaba 4 items USD, obtuvo %d", report.TotalUSD)
	}
	if report.TotalOK != 1 {
		t.Errorf("esperaba 1 OK, obtuvo %d", report.TotalOK)
	}
	if report.TotalDiferente != 2 { // F001-102 y FC01-50
		t.Errorf("esperaba 2 Diferente, obtuvo %d", report.TotalDiferente)
	}
	if report.TotalVacio != 1 {
		t.Errorf("esperaba 1 Vacío, obtuvo %d", report.TotalVacio)
	}

	// Verificar recálculo de F001-102 (375 / 3.75 * 3.816 = 381.60)
	it102 := report.Items[1] // index 1 en items evaluados (F001-102)
	if it102.Estado != "DIFERENTE" {
		t.Errorf("F001-102 debió ser DIFERENTE, fue %s", it102.Estado)
	}
	if it102.ItemRecalculado.BIGravada != "381.60" {
		t.Errorf("F001-102 BIGravada recalculada esperada 381.60, fue %s", it102.ItemRecalculado.BIGravada)
	}
	if it102.ItemRecalculado.TipoCambio != "3.816" {
		t.Errorf("F001-102 TipoCambio esperado 3.816, fue %s", it102.ItemRecalculado.TipoCambio)
	}

	// Verificar que la NC utilizó la fecha de referencia (15/08/2024) y la tasa 3.740
	itNC := report.Items[3]
	if !itNC.EsNotaCredito {
		t.Errorf("itNC debió detectarse como Nota de Crédito")
	}
	if itNC.FechaUsada != "15/08/2024" {
		t.Errorf("itNC debió usar fecha de referencia 15/08/2024, usó %s", itNC.FechaUsada)
	}
	if itNC.TCSunat != 3.740 {
		t.Errorf("itNC debió usar TC SUNAT 3.740, usó %.4f", itNC.TCSunat)
	}
}
