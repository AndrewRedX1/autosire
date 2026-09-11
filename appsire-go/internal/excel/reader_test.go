package excel

import (
	"os"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestReadComprobantesFromFile(t *testing.T) {
	excelPath := "../../../AppSireCPE.xlsx"
	if _, err := os.Stat(excelPath); os.IsNotExist(err) {
		t.Skip("Archivo AppSireCPE.xlsx no disponible para test")
	}

	reader := NewExcelReader()

	// Probar leyendo la hoja rxh
	comps, meta, err := reader.ReadComprobantesFromFile(excelPath, "rxh")
	if err != nil {
		t.Fatalf("Error leyendo rxh: %v", err)
	}

	t.Logf("Metadatos leídos: %+v", meta)
	t.Logf("Comprobantes encontrados: %d", len(comps))

	for i, c := range comps {
		t.Logf("[%d] ID=%s RUC=%s Tipo=%s Serie=%s Num=%s", i+1, c.ID, c.RUC, c.Tipo, c.Serie, c.Numero)
	}
}

func TestFlatListDetection(t *testing.T) {
	f := excelize.NewFile()
	sheet := "Sheet1"

	// Fila 1: Encabezados
	f.SetCellValue(sheet, "A1", "RUC")
	f.SetCellValue(sheet, "B1", "Tipo")
	f.SetCellValue(sheet, "C1", "Serie")
	f.SetCellValue(sheet, "D1", "Numero")
	f.SetCellValue(sheet, "E1", "Razon Social")

	// Fila 2
	f.SetCellValue(sheet, "A2", "20100000001")
	f.SetCellValue(sheet, "B2", "01")
	f.SetCellValue(sheet, "C2", "F001")
	f.SetCellValue(sheet, "D2", "1500")
	f.SetCellValue(sheet, "E2", "EMPRESA DEMO SAC")

	// Fila 3: Solo Serie y Numero con espacio en la serie
	f.SetCellValue(sheet, "A3", "")
	f.SetCellValue(sheet, "B3", "")
	f.SetCellValue(sheet, "C3", "E 001")
	f.SetCellValue(sheet, "D3", "00000147")
	f.SetCellValue(sheet, "E3", "")

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("Error escribiendo excel: %v", err)
	}

	reader := NewExcelReader()
	comps, _, err := reader.ReadComprobantesFromStream(buf, sheet)
	if err != nil {
		t.Fatalf("Error leyendo comprobantes: %v", err)
	}

	if len(comps) != 2 {
		t.Fatalf("Se esperaban 2 comprobantes, se obtuvieron %d", len(comps))
	}

	// Comprobante 1
	if comps[0].Serie != "F001" || comps[0].Numero != "1500" || comps[0].Tipo != "01" || comps[0].RUC != "20100000001" {
		t.Errorf("Comprobante 1 inesperado: %+v", comps[0])
	}

	// Comprobante 2 (Autodetección de RxH E001 -> tipo 02, y normalización de correlativo)
	if comps[1].Serie != "E001" || comps[1].Numero != "147" || comps[1].Tipo != "02" {
		t.Errorf("Comprobante 2 inesperado (falló inferencia de tipo o limpieza de espacios): %+v", comps[1])
	}
}

func TestReadGeneratedTemplate(t *testing.T) {
	templatePath := "../../plantilla_comprobantes.xlsx"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skip("Plantilla no existe en ruta relativa esperada")
	}

	reader := NewExcelReader()

	// 1. Probar Lista_Simple
	compsSimple, _, err := reader.ReadComprobantesFromFile(templatePath, "Lista_Simple")
	if err != nil {
		t.Fatalf("Error leyendo Lista_Simple: %v", err)
	}
	if len(compsSimple) != 5 {
		t.Errorf("Esperaba 5 comprobantes en Lista_Simple, obtuvo %d", len(compsSimple))
	}

	// 2. Probar cpe
	compsCpe, metaCpe, err := reader.ReadComprobantesFromFile(templatePath, "cpe")
	if err != nil {
		t.Fatalf("Error leyendo cpe: %v", err)
	}
	if len(compsCpe) != 2 {
		t.Errorf("Esperaba 2 comprobantes en cpe, obtuvo %d", len(compsCpe))
	}
	if metaCpe["ruc"] != "20601234567" {
		t.Errorf("RUC empresa en cpe incorrecto: %s", metaCpe["ruc"])
	}

	// 3. Probar rxh
	compsRxh, _, err := reader.ReadComprobantesFromFile(templatePath, "rxh")
	if err != nil {
		t.Fatalf("Error leyendo rxh: %v", err)
	}
	if len(compsRxh) != 3 {
		t.Errorf("Esperaba 3 comprobantes en rxh, obtuvo %d", len(compsRxh))
	}
}

func TestFourColumnsOnly(t *testing.T) {
	f := excelize.NewFile()
	sheet := "Sheet1"

	// Solo y exactamente 4 columnas
	f.SetCellValue(sheet, "A1", "RUC")
	f.SetCellValue(sheet, "B1", "Tipo")
	f.SetCellValue(sheet, "C1", "Serie")
	f.SetCellValue(sheet, "D1", "Numero")

	f.SetCellValue(sheet, "A2", "20100070970")
	f.SetCellValue(sheet, "B2", "01")
	f.SetCellValue(sheet, "C2", "F001")
	f.SetCellValue(sheet, "D2", "1045")

	f.SetCellValue(sheet, "A3", "10458932145")
	f.SetCellValue(sheet, "B3", "02")
	f.SetCellValue(sheet, "C3", "E001")
	f.SetCellValue(sheet, "D3", "147")

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("Error escribiendo archivo excel: %v", err)
	}

	reader := NewExcelReader()
	comps, _, err := reader.ReadComprobantesFromStream(buf, sheet)
	if err != nil {
		t.Fatalf("Error leyendo comprobantes con solo 4 columnas: %v", err)
	}

	if len(comps) != 2 {
		t.Fatalf("Se esperaban 2 comprobantes, se obtuvieron %d", len(comps))
	}

	if comps[0].ID != "20100070970-01-F001-1045" {
		t.Errorf("ID esperado 20100070970-01-F001-1045, se obtuvo %s", comps[0].ID)
	}
	if comps[1].ID != "10458932145-02-E001-147" {
		t.Errorf("ID esperado 10458932145-02-E001-147, se obtuvo %s", comps[1].ID)
	}
}



