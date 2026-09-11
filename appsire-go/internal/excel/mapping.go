package excel

import (
	"strings"
)

// SheetMapping define qué columnas contienen los datos de los comprobantes
type SheetMapping struct {
	SheetName    string
	StartRow     int // 1-indexed
	ColRUC       int // 1-indexed
	ColTipo      int
	ColSerie     int
	ColNumero    int
	ColFecha     int
	ColMonto     int
	ColRazon     int
	CellRUC      string // ej: "E2"
	CellRazon    string // ej: "E1"
	CellPeriodo  string // ej: "K6"
	DefaultLibro string // "1" Ventas, "2" Compras
}

// GetMappingForSheet devuelve la configuración correspondiente al tipo de reporte
func GetMappingForSheet(sheetName string) SheetMapping {
	nameLower := strings.ToLower(strings.TrimSpace(sheetName))

	switch {
	case strings.Contains(nameLower, "comprobante"):
		// Lista plana de comprobantes de compras. La detección de encabezados
		// resolverá las columnas reales; el dato crítico es libro=2.
		return SheetMapping{
			SheetName:    sheetName,
			StartRow:     2,
			ColRUC:       1,
			ColTipo:      2,
			ColSerie:     3,
			ColNumero:    4,
			DefaultLibro: "2",
		}
	case strings.Contains(nameLower, "rvie"):
		// Registro de Ventas SIRE
		return SheetMapping{
			SheetName:    sheetName,
			StartRow:     8,
			ColRUC:       9,
			ColTipo:      15,
			ColSerie:     16,
			ColNumero:    17,
			ColFecha:     10,
			ColMonto:     24,
			CellRUC:      "E2",
			CellRazon:    "E1",
			CellPeriodo:  "K6",
			DefaultLibro: "1", // Ventas
		}

	case strings.Contains(nameLower, "cpe"):
		// Comprobantes Compras / General
		return SheetMapping{
			SheetName:    sheetName,
			StartRow:     8,
			ColRUC:       21,
			ColTipo:      15,
			ColSerie:     16,
			ColNumero:    18,
			ColFecha:     11,
			ColMonto:     27,
			CellRUC:      "E2",
			CellRazon:    "E1",
			CellPeriodo:  "K6",
			DefaultLibro: "2", // Compras
		}

	case strings.Contains(nameLower, "rxh"):
		// Recibos por honorarios
		return SheetMapping{
			SheetName:    sheetName,
			StartRow:     8,
			ColRUC:       11,
			ColTipo:      6,
			ColSerie:     7,
			ColNumero:    8,
			ColFecha:     5,
			ColMonto:     18,
			CellPeriodo:  "A2",
			DefaultLibro: "2",
		}

	case strings.Contains(nameLower, "liq"):
		// Liquidación de compra
		return SheetMapping{
			SheetName:    sheetName,
			StartRow:     8,
			ColRUC:       5,
			ColTipo:      6,
			ColSerie:     7,
			ColNumero:    8,
			CellRUC:      "E2",
			CellRazon:    "E1",
			CellPeriodo:  "K6",
			DefaultLibro: "2",
		}

	default:
		// Mapeo por defecto (similar a CPE)
		return SheetMapping{
			SheetName:    sheetName,
			StartRow:     2, // En hojas planas fila 1 encabezados, fila 2 datos
			ColRUC:       1,
			ColTipo:      2,
			ColSerie:     3,
			ColNumero:    4,
			DefaultLibro: "1",
		}
	}
}

// CleanCellValue limpia espacios y caracteres invisibles comunes de celdas Excel
func CleanCellValue(val string) string {
	val = strings.TrimSpace(val)
	val = strings.ReplaceAll(val, "\u00a0", "") // Non-breaking space
	return val
}
