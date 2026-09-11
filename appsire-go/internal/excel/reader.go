package excel

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"appsire-go/internal/sunat"

	"github.com/xuri/excelize/v2"
)

var (
	regexPeriodo = regexp.MustCompile(`(\d{6})`)
	regexRUC     = regexp.MustCompile(`(\d{11})`)
)

// ExcelReader maneja la lectura y análisis de hojas de cálculo de comprobantes
type ExcelReader struct{}

// NewExcelReader crea una instancia del lector
func NewExcelReader() *ExcelReader {
	return &ExcelReader{}
}

// ReadComprobantesFromStream lee un archivo Excel recibido por HTTP multipart
func (r *ExcelReader) ReadComprobantesFromStream(reader io.Reader, targetSheet string) ([]sunat.Comprobante, map[string]string, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("error abriendo archivo Excel: %w", err)
	}
	defer f.Close()

	return r.extractFromExcel(f, targetSheet)
}

// ReadComprobantesFromFile lee un archivo Excel local desde disco
func (r *ExcelReader) ReadComprobantesFromFile(filePath string, targetSheet string) ([]sunat.Comprobante, map[string]string, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error abriendo archivo Excel en %s: %w", filePath, err)
	}
	defer f.Close()

	return r.extractFromExcel(f, targetSheet)
}

// GetAvailableSheets retorna la lista de nombres de hojas presentes en el Excel
func (r *ExcelReader) GetAvailableSheets(reader io.Reader) ([]string, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("error abriendo Excel: %w", err)
	}
	defer f.Close()

	return f.GetSheetList(), nil
}

// extractFromExcel procesa el archivo excelize abierto
func (r *ExcelReader) extractFromExcel(f *excelize.File, targetSheet string) ([]sunat.Comprobante, map[string]string, error) {
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil, fmt.Errorf("el archivo Excel no contiene hojas")
	}

	selectedSheet := ""
	if targetSheet != "" {
		for _, s := range sheets {
			if strings.EqualFold(s, targetSheet) {
				selectedSheet = s
				break
			}
		}
	}

	// Si no se especificó o no se encontró, buscar preferentemente rvie, cpe, rxh, o la primera hoja no vacía
	if selectedSheet == "" {
		for _, s := range sheets {
			lower := strings.ToLower(s)
			if strings.Contains(lower, "cpe") || strings.Contains(lower, "rvie") || strings.Contains(lower, "rxh") {
				selectedSheet = s
				break
			}
		}
		if selectedSheet == "" {
			selectedSheet = sheets[0]
		}
	}

	mapping := GetMappingForSheet(selectedSheet)
	metadata := make(map[string]string)
	metadata["hoja"] = selectedSheet

	// Extraer metadatos de cabecera si están configurados
	if mapping.CellRUC != "" {
		if val, err := f.GetCellValue(selectedSheet, mapping.CellRUC); err == nil {
			if m := regexRUC.FindString(val); m != "" {
				metadata["ruc"] = m
			} else {
				metadata["ruc"] = CleanCellValue(val)
			}
		}
	}
	if mapping.CellRazon != "" {
		if val, err := f.GetCellValue(selectedSheet, mapping.CellRazon); err == nil {
			metadata["razon_social"] = CleanCellValue(val)
		}
	}
	if mapping.CellPeriodo != "" {
		if val, err := f.GetCellValue(selectedSheet, mapping.CellPeriodo); err == nil {
			if m := regexPeriodo.FindString(val); m != "" {
				metadata["periodo"] = m
			} else {
				metadata["periodo"] = CleanCellValue(val)
			}
		}
	}

	rows, err := f.GetRows(selectedSheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, nil, fmt.Errorf("error leyendo filas de la hoja %s: %w", selectedSheet, err)
	}

	// Intentar autodetección de columnas si la fila configurada no tiene los encabezados usuales
	colMap := r.detectColumnIndices(rows, mapping)

	var comprobantes []sunat.Comprobante

	startIdx := mapping.StartRow - 1
	if colMap.HeaderRowIdx != -1 {
		startIdx = colMap.HeaderRowIdx + 1
	} else if startIdx < 0 {
		startIdx = 0
	}

	for rIdx := startIdx; rIdx < len(rows); rIdx++ {
		row := rows[rIdx]
		if len(row) == 0 {
			continue
		}

		ruc := getCell(row, colMap.ColRUC)
		tipo := getCell(row, colMap.ColTipo)
		serie := getCell(row, colMap.ColSerie)
		numero := getCell(row, colMap.ColNumero)

		// Si el RUC no está en la fila pero está en la cabecera (caso ventas o emisor único)
		if ruc == "" && metadata["ruc"] != "" {
			ruc = metadata["ruc"]
		}

		// Validar si la fila contiene datos mínimos de un comprobante
		if serie == "" || numero == "" {
			continue
		}

		// Limpiar serie (remover espacios intermedios y convertir a mayúsculas)
		serieClean := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(serie), " ", ""))

		// Limpiar número (remover espacios, validar que sea numérico)
		numeroClean := sunat.NormalizeNumero(numero)

		tipoNorm := sunat.NormalizeTipo(tipo)
		// Si no se especificó tipo, inferir automáticamente a partir de la serie
		if tipoNorm == "" {
			if strings.HasPrefix(serieClean, "FC") || strings.HasPrefix(serieClean, "BC") {
				tipoNorm = "07" // Nota de Crédito
			} else if strings.HasPrefix(serieClean, "FD") || strings.HasPrefix(serieClean, "BD") {
				tipoNorm = "08" // Nota de Débito
			} else if strings.HasPrefix(serieClean, "F") {
				tipoNorm = "01" // Factura
			} else if strings.HasPrefix(serieClean, "B") {
				tipoNorm = "03" // Boleta
			} else if strings.HasPrefix(serieClean, "E") {
				tipoNorm = "02" // Recibo por Honorarios
			} else if strings.HasPrefix(serieClean, "T") {
				tipoNorm = "09" // Guía de Remisión
			}
		}

		id := fmt.Sprintf("%s-%s-%s-%s", ruc, tipoNorm, serieClean, numeroClean)

		fecha := getCell(row, colMap.ColFecha)
		monto := getCell(row, colMap.ColMonto)
		razon := getCell(row, colMap.ColRazon)
		if razon == "" {
			razon = metadata["razon_social"]
		}

		comp := sunat.Comprobante{
			ID:           id,
			RUC:          ruc,
			Tipo:         tipoNorm,
			Serie:        serieClean,
			Numero:       numeroClean,
			Libro:        mapping.DefaultLibro,
			Periodo:      metadata["periodo"],
			RazonSocial:  razon,
			FechaEmision: fecha,
			Monto:        monto,
			RowIndex:     rIdx + 1,
			SheetName:    selectedSheet,
		}

		comprobantes = append(comprobantes, comp)
	}

	return comprobantes, metadata, nil
}

type detectedColumns struct {
	ColRUC       int
	ColTipo      int
	ColSerie     int
	ColNumero    int
	ColFecha     int
	ColMonto     int
	ColRazon     int
	HeaderRowIdx int // -1 si no se detectó por encabezados
}

// detectColumnIndices detecta índices de columna (0-indexed) o usa el mapping establecido
func (r *ExcelReader) detectColumnIndices(rows [][]string, defaultMap SheetMapping) detectedColumns {
	res := detectedColumns{
		ColRUC:       defaultMap.ColRUC - 1,
		ColTipo:      defaultMap.ColTipo - 1,
		ColSerie:     defaultMap.ColSerie - 1,
		ColNumero:    defaultMap.ColNumero - 1,
		ColFecha:     defaultMap.ColFecha - 1,
		ColMonto:     defaultMap.ColMonto - 1,
		ColRazon:     defaultMap.ColRazon - 1,
		HeaderRowIdx: -1,
	}

	// Buscar en las primeras 12 filas si hay una fila de encabezados explícitos
	for rIdx := 0; rIdx < len(rows) && rIdx < 12; rIdx++ {
		row := rows[rIdx]
		matchedCount := 0
		var detectedRUC, detectedTipo, detectedSerie, detectedNum int = -1, -1, -1, -1
		var detectedFecha, detectedMonto, detectedRazon int = -1, -1, -1

		for cIdx, cell := range row {
			header := strings.ToLower(strings.TrimSpace(cell))
			switch {
			case strings.Contains(header, "ruc") || strings.Contains(header, "nro. doc") || strings.Contains(header, "doc emisor"):
				detectedRUC = cIdx
				matchedCount++
			case (strings.Contains(header, "tipo") && (strings.Contains(header, "doc") || strings.Contains(header, "comp") || strings.Contains(header, "cpe"))) || header == "tipo":
				detectedTipo = cIdx
				matchedCount++
			case strings.Contains(header, "serie"):
				detectedSerie = cIdx
				matchedCount++
			case strings.Contains(header, "número") || strings.Contains(header, "numero") || strings.Contains(header, "correlativo") || header == "num":
				detectedNum = cIdx
				matchedCount++
			case strings.Contains(header, "fecha"):
				detectedFecha = cIdx
			case strings.Contains(header, "monto") || strings.Contains(header, "total") || strings.Contains(header, "importe"):
				detectedMonto = cIdx
			case strings.Contains(header, "razon") || strings.Contains(header, "razón") || strings.Contains(header, "proveedor") || strings.Contains(header, "cliente"):
				detectedRazon = cIdx
			}
		}

		// Si encontramos serie y número en la misma fila de encabezados
		if detectedSerie != -1 && detectedNum != -1 {
			res.HeaderRowIdx = rIdx
			res.ColSerie = detectedSerie
			res.ColNumero = detectedNum
			if detectedRUC != -1 {
				res.ColRUC = detectedRUC
			}
			if detectedTipo != -1 {
				res.ColTipo = detectedTipo
			}
			if detectedFecha != -1 {
				res.ColFecha = detectedFecha
			}
			if detectedMonto != -1 {
				res.ColMonto = detectedMonto
			}
			if detectedRazon != -1 {
				res.ColRazon = detectedRazon
			}
			break
		}
	}

	return res
}

func getCell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return CleanCellValue(row[idx])
}
