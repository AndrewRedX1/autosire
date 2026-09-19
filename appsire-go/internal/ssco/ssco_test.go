package ssco

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"appsire-go/internal/sirepreview"
	"github.com/xuri/excelize/v2"
)

func TestNormalizarNombre(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"COMERCIALIZADORA DEL SUR S.A.C.", "COMERCIALIZADORADELSURSAC"},
		{"Árbol & Solución S.R.L.", "ARBOLYSOLUCIONSRL"}, // wait, & might not be in alnum: '&' is not letter/digit!
		{"  constructora pérez y sánchez e.i.r.l.  ", "CONSTRUCTORAPEREZYSANCHEZEIRL"},
		{"INVERSIONES 2024 S.A.", "INVERSIONES2024SA"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		got := NormalizarNombre(tt.input)
		// For '&', NormalizarNombre ignores it because it's not a letter or digit
		if tt.input == "Árbol & Solución S.R.L." {
			expectedWithoutAmp := "ARBOLSOLUCIONSRL"
			if got != expectedWithoutAmp {
				t.Errorf("NormalizarNombre(%q) = %q; esperaba %q", tt.input, got, expectedWithoutAmp)
			}
			continue
		}
		if got != tt.expected {
			t.Errorf("NormalizarNombre(%q) = %q; esperaba %q", tt.input, got, tt.expected)
		}
	}
}

func TestSoloDigitos(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"20601234567", "20601234567"},
		{" 20-60123456-7 ", "20601234567"},
		{"RUC: 20111222333", "20111222333"},
		{"abc", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := SoloDigitos(tt.input)
		if got != tt.expected {
			t.Errorf("SoloDigitos(%q) = %q; esperaba %q", tt.input, got, tt.expected)
		}
	}
}

func TestValidateItems(t *testing.T) {
	padron := NewPadron()
	padron.Total = 2
	padron.FechaActualizacion = "15/09/2026"

	// 1. Sujeto por RUC
	sujeto1 := SujetoSinCapacidad{
		RUC:              "20100000001",
		RazonSocial:      "PROVEEDOR FANTASMA SAC",
		Resolucion:       "RES-001-2024",
		FechaFirme:       "10/01/2024",
		FechaPublicacion: "15/01/2024",
	}
	padron.PorRUC[sujeto1.RUC] = sujeto1
	padron.PorNombre[NormalizarNombre(sujeto1.RazonSocial)] = sujeto1

	// 2. Sujeto por nombre
	sujeto2 := SujetoSinCapacidad{
		RUC:              "20200000002",
		RazonSocial:      "SERVICIOS FRAUDULENTOS SRL",
		Resolucion:       "RES-002-2024",
		FechaFirme:       "20/02/2024",
		FechaPublicacion: "25/02/2024",
	}
	padron.PorRUC[sujeto2.RUC] = sujeto2
	padron.PorNombre[NormalizarNombre(sujeto2.RazonSocial)] = sujeto2

	items := []sirepreview.ProposalItem{
		{
			CompPago:     "01-F001-00000100",
			Serie:        "F001",
			Numero:       "00000100",
			Fecha:        "01/09/2026",
			RUC:          "20100000001", // Coincide por RUC
			RazonSocial:  "PROVEEDOR FANTASMA SAC",
			ImporteTotal: "1180.00",
			IGV:          "180.00",
		},
		{
			CompPago:     "01-F002-00000200",
			Serie:        "F002",
			Numero:       "00000200",
			Fecha:        "02/09/2026",
			RUC:          "20999999999", // RUC distinto pero coincide nombre
			RazonSocial:  "Servicios Fraudulentos S.R.L.",
			ImporteTotal: "590.00",
			IGV:          "90.00",
		},
		{
			CompPago:     "01-F003-00000300",
			Serie:        "F003",
			Numero:       "00000300",
			Fecha:        "03/09/2026",
			RUC:          "20555555555", // Conforme
			RazonSocial:  "PROVEEDOR LIMPIO SAC",
			ImporteTotal: "2360.00",
			IGV:          "360.00",
		},
	}

	report := ValidateItems(items, padron)

	if report.TotalItems != 3 {
		t.Errorf("TotalItems = %d; esperaba 3", report.TotalItems)
	}
	if report.CountSSCO != 1 {
		t.Errorf("CountSSCO = %d; esperaba 1", report.CountSSCO)
	}
	if report.CountWarning != 1 {
		t.Errorf("CountWarning = %d; esperaba 1", report.CountWarning)
	}
	if report.CountOK != 1 {
		t.Errorf("CountOK = %d; esperaba 1", report.CountOK)
	}
	if report.TotalIGVRiesgo != 180.00 {
		t.Errorf("TotalIGVRiesgo = %f; esperaba 180.00", report.TotalIGVRiesgo)
	}

	// Verificar item 0 (Riesgo Crítico por RUC)
	item0 := report.Items[0]
	if item0.Riesgo != RiesgoCritico || item0.CoincidePor != "RUC" {
		t.Errorf("item 0 esperado CRITICAL por RUC, obtenido: %s / %s", item0.Riesgo, item0.CoincidePor)
	}
	if item0.Resolucion != "RES-001-2024" {
		t.Errorf("item 0 resolución incorrecta: %s", item0.Resolucion)
	}

	// Verificar item 1 (Alerta por Nombre)
	item1 := report.Items[1]
	if item1.Riesgo != RiesgoAlerta || item1.CoincidePor != "NOMBRE" {
		t.Errorf("item 1 esperado WARNING por NOMBRE, obtenido: %s / %s", item1.Riesgo, item1.CoincidePor)
	}

	// Verificar item 2 (Conforme)
	item2 := report.Items[2]
	if item2.Riesgo != RiesgoConforme || item2.CoincidePor != "-" {
		t.Errorf("item 2 esperado OK, obtenido: %s / %s", item2.Riesgo, item2.CoincidePor)
	}
}

func TestServiceDownloadAndParse(t *testing.T) {
	// Crear un archivo Excel en memoria con formato SSCO
	f := excelize.NewFile()
	sheet := "Sheet1"
	_ = f.SetCellValue(sheet, "A1", "RUC")
	_ = f.SetCellValue(sheet, "B1", "RAZON SOCIAL")
	_ = f.SetCellValue(sheet, "D1", "RESOLUCION")
	_ = f.SetCellValue(sheet, "F1", "FECHA FIRME")
	_ = f.SetCellValue(sheet, "I1", "FECHA PUBLICACION")

	_ = f.SetCellValue(sheet, "A2", "20123456789")
	_ = f.SetCellValue(sheet, "B2", "EMPRESA SIN CAPACIDAD SAC")
	_ = f.SetCellValue(sheet, "D2", "RS-100-2025")
	_ = f.SetCellValue(sheet, "F2", "12/03/2025")
	_ = f.SetCellValue(sheet, "I2", "15/03/2025")

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("error creando excel de prueba: %v", err)
	}

	// Servidor HTTP mock
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		_, _ = w.Write(buf.Bytes())
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	svc := NewService(tempDir)
	svc.SetPadronURL(ts.URL)

	padron, err := svc.GetPadron(context.Background(), true)
	if err != nil {
		t.Fatalf("GetPadron falló: %v", err)
	}

	if padron.Total != 1 {
		t.Fatalf("Total esperado 1, obtenido: %d", padron.Total)
	}
	if padron.FechaActualizacion != "15/03/2025" {
		t.Errorf("FechaActualizacion esperada 15/03/2025, obtenida: %s", padron.FechaActualizacion)
	}
	if padron.DesdeCache {
		t.Errorf("esperaba DesdeCache = false")
	}

	sujeto, ok := padron.PorRUC["20123456789"]
	if !ok {
		t.Fatalf("RUC 20123456789 no encontrado en padrón")
	}
	if sujeto.Resolucion != "RS-100-2025" {
		t.Errorf("Resolución esperada RS-100-2025, obtenida: %s", sujeto.Resolucion)
	}

	// Verificar que el archivo de caché se guardó en disco
	cachedPath := filepath.Join(tempDir, "padron_ssco.xlsx")
	if _, err := os.Stat(cachedPath); err != nil {
		t.Errorf("archivo de caché no existe en %s: %v", cachedPath, err)
	}

	// Prueba de fallback a caché cuando SUNAT no responde
	svc2 := NewService(tempDir)
	svc2.SetPadronURL("http://127.0.0.1:54321/no-existe") // URL rota
	padronCached, err := svc2.GetPadron(context.Background(), true)
	if err != nil {
		t.Fatalf("Fallback a caché falló: %v", err)
	}
	if !padronCached.DesdeCache {
		t.Errorf("esperaba DesdeCache = true en fallback")
	}
	if padronCached.Total != 1 {
		t.Errorf("Total en caché esperado 1, obtenido %d", padronCached.Total)
	}
}
