package main

import (
	"fmt"
	"log"

	"github.com/xuri/excelize/v2"
)

func main() {
	f := excelize.NewFile()

	// -------------------------------------------------------------
	// 1. Hoja "Lista_Simple" (Formato Directo y Rápido Recomendado)
	// -------------------------------------------------------------
	sheetSimple := "Lista_Simple"
	f.SetSheetName("Sheet1", sheetSimple)

	// Estilo cabecera moderna azul oscuro
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11, Family: "Segoe UI"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#1E3A8A"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	// Estilo celdas datos
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Size: 10, Family: "Segoe UI"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	headersSimple := []string{"RUC", "Tipo", "Serie", "Numero"}
	for colIdx, h := range headersSimple {
		colName, _ := excelize.ColumnNumberToName(colIdx + 1)
		cell := fmt.Sprintf("%s1", colName)
		f.SetCellValue(sheetSimple, cell, h)
		f.SetCellStyle(sheetSimple, cell, cell, headerStyle)
	}

	// Filas de ejemplo en Lista_Simple con exactamente las 4 columnas requeridas
	sampleSimple := [][]interface{}{
		{"20100070970", "01", "F001", "1045"},
		{"20100070970", "03", "B001", "5420"},
		{"10458932145", "02", "E001", "147"},
		{"10458932145", "02", "E001", "148"},
		{"20512345678", "07", "FC01", "89"},
	}

	for rIdx, row := range sampleSimple {
		rowNum := rIdx + 2
		for cIdx, val := range row {
			colName, _ := excelize.ColumnNumberToName(cIdx + 1)
			cell := fmt.Sprintf("%s%d", colName, rowNum)
			f.SetCellValue(sheetSimple, cell, val)
			f.SetCellStyle(sheetSimple, cell, cell, dataStyle)
		}
	}

	f.SetColWidth(sheetSimple, "A", "A", 18)
	f.SetColWidth(sheetSimple, "B", "B", 12)
	f.SetColWidth(sheetSimple, "C", "C", 14)
	f.SetColWidth(sheetSimple, "D", "D", 16)

	// -------------------------------------------------------------
	// 2. Hoja "cpe" (Estructura AppSireCPE / SIRE Compras Oficial)
	// -------------------------------------------------------------
	sheetCpe := "cpe"
	f.NewSheet(sheetCpe)

	metaLabelStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10, Family: "Segoe UI", Color: "#1E293B"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E2E8F0"}, Pattern: 1},
	})

	f.SetCellValue(sheetCpe, "D1", "Razón Social:")
	f.SetCellStyle(sheetCpe, "D1", "D1", metaLabelStyle)
	f.SetCellValue(sheetCpe, "E1", "MI EMPRESA DEMO S.A.C.")

	f.SetCellValue(sheetCpe, "D2", "RUC Empresa:")
	f.SetCellStyle(sheetCpe, "D2", "D2", metaLabelStyle)
	f.SetCellValue(sheetCpe, "E2", "20601234567")

	f.SetCellValue(sheetCpe, "J6", "Periodo Tributario:")
	f.SetCellStyle(sheetCpe, "J6", "J6", metaLabelStyle)
	f.SetCellValue(sheetCpe, "K6", "202401")

	// Cabeceras fila 7 en "cpe":
	// Col 15 (O): Tipo, Col 16 (P): Serie, Col 18 (R): Número, Col 21 (U): RUC Proveedor
	f.SetCellValue(sheetCpe, "O7", "Tipo CPE")
	f.SetCellValue(sheetCpe, "P7", "Serie")
	f.SetCellValue(sheetCpe, "Q7", "F. Emisión")
	f.SetCellValue(sheetCpe, "R7", "Número")
	f.SetCellValue(sheetCpe, "S7", "Monto Total")
	f.SetCellValue(sheetCpe, "U7", "RUC Proveedor")
	f.SetCellValue(sheetCpe, "V7", "Razón Social Proveedor")

	for _, c := range []string{"O7", "P7", "Q7", "R7", "S7", "U7", "V7"} {
		f.SetCellStyle(sheetCpe, c, c, headerStyle)
	}

	sampleCpe := [][]interface{}{
		{"01", "F001", "15/01/2024", "1045", 1500.0, "20100070970", "SUPERMERCADOS PERUANOS S.A."},
		{"01", "F002", "16/01/2024", "8921", 450.20, "20512345678", "DISTRIBUIDORA COMERCIAL SAC"},
	}

	for i, row := range sampleCpe {
		rNum := 8 + i
		f.SetCellValue(sheetCpe, fmt.Sprintf("O%d", rNum), row[0])
		f.SetCellValue(sheetCpe, fmt.Sprintf("P%d", rNum), row[1])
		f.SetCellValue(sheetCpe, fmt.Sprintf("Q%d", rNum), row[2])
		f.SetCellValue(sheetCpe, fmt.Sprintf("R%d", rNum), row[3])
		f.SetCellValue(sheetCpe, fmt.Sprintf("S%d", rNum), row[4])
		f.SetCellValue(sheetCpe, fmt.Sprintf("U%d", rNum), row[5])
		f.SetCellValue(sheetCpe, fmt.Sprintf("V%d", rNum), row[6])
	}

	// -------------------------------------------------------------
	// 3. Hoja "rxh" (Recibos por Honorarios)
	// -------------------------------------------------------------
	sheetRxh := "rxh"
	f.NewSheet(sheetRxh)

	f.SetCellValue(sheetRxh, "D1", "Razón Social:")
	f.SetCellStyle(sheetRxh, "D1", "D1", metaLabelStyle)
	f.SetCellValue(sheetRxh, "E1", "MI EMPRESA DEMO S.A.C.")

	f.SetCellValue(sheetRxh, "D2", "RUC Empresa:")
	f.SetCellStyle(sheetRxh, "D2", "D2", metaLabelStyle)
	f.SetCellValue(sheetRxh, "E2", "20601234567")

	f.SetCellValue(sheetRxh, "J6", "Periodo:")
	f.SetCellStyle(sheetRxh, "J6", "J6", metaLabelStyle)
	f.SetCellValue(sheetRxh, "K6", "202401")

	// Col 6 (F): Tipo, Col 7 (G): Serie, Col 8 (H): Número, Col 11 (K): RUC Emisor
	f.SetCellValue(sheetRxh, "F7", "Tipo")
	f.SetCellValue(sheetRxh, "G7", "Serie")
	f.SetCellValue(sheetRxh, "H7", "Número")
	f.SetCellValue(sheetRxh, "K7", "RUC Emisor")
	f.SetCellValue(sheetRxh, "L7", "Nombre Emisor")

	for _, c := range []string{"F7", "G7", "H7", "K7", "L7"} {
		f.SetCellStyle(sheetRxh, c, c, headerStyle)
	}

	sampleRxh := [][]interface{}{
		{"02", "E001", "147", "10458932145", "JUAN PEREZ ASESOR"},
		{"02", "E001", "148", "10458932145", "JUAN PEREZ ASESOR"},
		{"02", "E001", "149", "10458932145", "JUAN PEREZ ASESOR"},
	}

	for i, row := range sampleRxh {
		rNum := 8 + i
		f.SetCellValue(sheetRxh, fmt.Sprintf("F%d", rNum), row[0])
		f.SetCellValue(sheetRxh, fmt.Sprintf("G%d", rNum), row[1])
		f.SetCellValue(sheetRxh, fmt.Sprintf("H%d", rNum), row[2])
		f.SetCellValue(sheetRxh, fmt.Sprintf("K%d", rNum), row[3])
		f.SetCellValue(sheetRxh, fmt.Sprintf("L%d", rNum), row[4])
	}

	// Guardar archivo
	outPath := "plantilla_comprobantes.xlsx"
	if err := f.SaveAs(outPath); err != nil {
		log.Fatalf("Error guardando plantilla: %v", err)
	}
	fmt.Println("Plantilla generada exitosamente en:", outPath)
}
