package excel

import (
	"os"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestSyntheticCpeSheet(t *testing.T) {
	tempFile := "test_cpe_sample.xlsx"
	defer os.Remove(tempFile)

	f := excelize.NewFile()
	sheetName := "cpe"
	f.NewSheet(sheetName)
	f.DeleteSheet("Sheet1")

	// Metadatos en E1, E2, K6
	f.SetCellValue(sheetName, "E1", "EMPRESA DE PRUEBA SAC")
	f.SetCellValue(sheetName, "E2", "20123456789")
	f.SetCellValue(sheetName, "K6", "202401")

	// Fila 8: Datos según mapeo cpe (RUC col 21, Tipo col 15, Serie col 16, Num col 18)
	// Col 15 = O, Col 16 = P, Col 18 = R, Col 21 = U
	f.SetCellValue(sheetName, "O8", "01")          // Tipo Factura
	f.SetCellValue(sheetName, "P8", "F001")        // Serie
	f.SetCellValue(sheetName, "R8", "00000123")    // Num
	f.SetCellValue(sheetName, "U8", "20555666777") // RUC Emisor

	// Fila 9: Segunda fila
	f.SetCellValue(sheetName, "O9", "07")          // NC
	f.SetCellValue(sheetName, "P9", "FC01")        // Serie
	f.SetCellValue(sheetName, "R9", "45")          // Num
	f.SetCellValue(sheetName, "U9", "20555666777") // RUC Emisor

	// Fila 10: Excel muestra 10,685, pero SUNAT requiere 10685.
	f.SetCellValue(sheetName, "O10", "01")
	f.SetCellValue(sheetName, "P10", "F002")
	f.SetCellValue(sheetName, "R10", 10685)
	f.SetCellValue(sheetName, "U10", "20555666777")
	numberStyle, err := f.NewStyle(&excelize.Style{NumFmt: 3})
	if err != nil {
		t.Fatalf("Error creando estilo numérico: %v", err)
	}
	if err := f.SetCellStyle(sheetName, "R10", "R10", numberStyle); err != nil {
		t.Fatalf("Error aplicando estilo numérico: %v", err)
	}

	if err := f.SaveAs(tempFile); err != nil {
		t.Fatalf("Error guardando excel: %v", err)
	}

	reader := NewExcelReader()
	comps, meta, err := reader.ReadComprobantesFromFile(tempFile, "cpe")
	if err != nil {
		t.Fatalf("Error leyendo comprobantes: %v", err)
	}

	if len(comps) != 3 {
		t.Fatalf("Se esperaban 3 comprobantes, se encontraron %d", len(comps))
	}

	if comps[0].Serie != "F001" || comps[0].Numero != "123" {
		t.Errorf("Comprobante 1 inesperado: %+v", comps[0])
	}
	if comps[1].Serie != "FC01" || comps[1].Numero != "45" {
		t.Errorf("Comprobante 2 inesperado: %+v", comps[1])
	}
	if comps[2].Numero != "10685" {
		t.Errorf("Comprobante con separador de miles inesperado: %+v", comps[2])
	}

	if meta["ruc"] != "20123456789" || meta["periodo"] != "202401" {
		t.Errorf("Metadatos inesperados: %+v", meta)
	}
}
