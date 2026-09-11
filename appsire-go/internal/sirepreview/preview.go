package sirepreview

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"appsire-go/internal/sunat"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const MaxPreviewRows = 1000

type Preview struct {
	FileName     string              `json:"file_name"`
	Headers      []string            `json:"headers"`
	Rows         [][]string          `json:"rows"`
	Comprobantes []sunat.Comprobante `json:"comprobantes"`
	TotalRows    int                 `json:"total_rows"`
	Truncated    bool                `json:"truncated"`
}

type conceptColumn struct {
	Index        int
	Label        string
	AmountColumn bool
}

func FromZIP(content []byte, book sunat.ProposalBook, period string) (Preview, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return Preview{}, fmt.Errorf("abriendo propuesta ZIP: %w", err)
	}

	candidates := make([]*zip.File, 0)
	for _, file := range reader.File {
		ext := strings.ToLower(filepath.Ext(file.Name))
		if !file.FileInfo().IsDir() && (ext == ".txt" || ext == ".csv") {
			candidates = append(candidates, file)
		}
	}
	if len(candidates) == 0 {
		return Preview{}, errors.New("la propuesta no contiene un TXT o CSV para visualizar")
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		iTXT := strings.EqualFold(filepath.Ext(candidates[i].Name), ".txt")
		jTXT := strings.EqualFold(filepath.Ext(candidates[j].Name), ".txt")
		if iTXT != jTXT {
			return iTXT
		}
		return strings.ToLower(candidates[i].Name) < strings.ToLower(candidates[j].Name)
	})

	file := candidates[0]
	stream, err := file.Open()
	if err != nil {
		return Preview{}, fmt.Errorf("abriendo datos de propuesta: %w", err)
	}
	defer stream.Close()
	raw, err := io.ReadAll(io.LimitReader(stream, 64<<20))
	if err != nil {
		return Preview{}, fmt.Errorf("leyendo datos de propuesta: %w", err)
	}
	text, err := decodeText(raw)
	if err != nil {
		return Preview{}, err
	}
	records, err := parseDelimited(text)
	if err != nil {
		return Preview{}, fmt.Errorf("interpretando datos de propuesta: %w", err)
	}
	if len(records) == 0 {
		return Preview{}, errors.New("el archivo de propuesta está vacío")
	}

	start := 0
	if looksLikeHeader(records[0]) {
		start = 1
	}

	columns, mapping, err := columnsFor(book)
	if err != nil {
		return Preview{}, err
	}
	dataRows := records[start:]
	columns = populatedColumns(columns, dataRows)
	headers := make([]string, len(columns))
	for index, column := range columns {
		headers[index] = column.Label
	}

	total := len(dataRows)
	end := min(total, MaxPreviewRows)
	rows := make([][]string, 0, end)
	comprobantes := make([]sunat.Comprobante, 0, end)
	for index, source := range dataRows[:end] {
		row := make([]string, len(columns))
		for columnIndex, column := range columns {
			row[columnIndex] = field(source, column.Index)
		}
		rows = append(rows, row)
		comprobantes = append(comprobantes, buildComprobante(source, mapping, book, period, index))
	}
	return Preview{
		FileName:     filepath.Base(file.Name),
		Headers:      headers,
		Rows:         rows,
		Comprobantes: comprobantes,
		TotalRows:    total,
		Truncated:    total > MaxPreviewRows,
	}, nil
}

type downloadMapping struct {
	RUC, Tipo, Serie, Numero, Fecha int
	Libro, Sheet                    string
}

func columnsFor(book sunat.ProposalBook) ([]conceptColumn, downloadMapping, error) {
	switch book {
	case sunat.ProposalRCE:
		return []conceptColumn{
			{Index: 4, Label: "Fecha de emisión"}, {Index: 6, Label: "Tipo"}, {Index: 7, Label: "Serie"}, {Index: 9, Label: "Número"},
			{Index: 12, Label: "RUC proveedor"}, {Index: 13, Label: "Proveedor"},
			amountColumn(14, "Base imponible gravada/exportación"), amountColumn(15, "IGV/IPM gravado/exportación"),
			amountColumn(16, "Base imponible gravada y no gravada"), amountColumn(17, "IGV/IPM gravado y no gravado"),
			amountColumn(18, "Base imponible no gravada"), amountColumn(19, "IGV/IPM no gravado"),
			amountColumn(20, "Adquisiciones no gravadas"), amountColumn(21, "ISC"), amountColumn(22, "ICBPER"),
			amountColumn(23, "Otros tributos/cargos"), amountColumn(24, "Importe total"), {Index: 25, Label: "Moneda"},
		}, downloadMapping{RUC: 12, Tipo: 6, Serie: 7, Numero: 9, Fecha: 4, Libro: "2", Sheet: "cpe"}, nil
	case sunat.ProposalRVIE:
		return []conceptColumn{
			{Index: 4, Label: "Fecha de emisión"}, {Index: 6, Label: "Tipo"}, {Index: 7, Label: "Serie"}, {Index: 8, Label: "Número"},
			{Index: 11, Label: "Documento cliente"}, {Index: 12, Label: "Cliente"},
			amountColumn(13, "Valor facturado exportación"), amountColumn(14, "Base imponible gravada"),
			amountColumn(15, "Descuento base imponible"), amountColumn(16, "IGV/IPM"), amountColumn(17, "Descuento IGV/IPM"),
			amountColumn(18, "Operación exonerada"), amountColumn(19, "Operación inafecta"), amountColumn(20, "ISC"),
			amountColumn(21, "Base imponible con IVAP"), amountColumn(22, "IVAP"), amountColumn(23, "ICBPER"),
			amountColumn(24, "Otros tributos/cargos"), amountColumn(25, "Importe total"), {Index: 26, Label: "Moneda"},
		}, downloadMapping{RUC: 0, Tipo: 6, Serie: 7, Numero: 8, Fecha: 4, Libro: "1", Sheet: "rvie"}, nil
	default:
		return nil, downloadMapping{}, errors.New("tipo de propuesta inválido para visualizar")
	}
}

func amountColumn(index int, label string) conceptColumn {
	return conceptColumn{Index: index, Label: label, AmountColumn: true}
}

func populatedColumns(columns []conceptColumn, rows [][]string) []conceptColumn {
	result := make([]conceptColumn, 0, len(columns))
	for _, column := range columns {
		for _, row := range rows {
			value := field(row, column.Index)
			if value != "" && (!column.AmountColumn || !isZeroAmount(value)) {
				result = append(result, column)
				break
			}
		}
	}
	return result
}

func isZeroAmount(value string) bool {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "0.,+- ")
	return value == ""
}

func buildComprobante(row []string, mapping downloadMapping, book sunat.ProposalBook, period string, index int) sunat.Comprobante {
	ruc := field(row, mapping.RUC)
	tipo := sunat.NormalizeTipo(field(row, mapping.Tipo))
	serie := strings.ToUpper(strings.ReplaceAll(field(row, mapping.Serie), " ", ""))
	numero := sunat.NormalizeNumero(field(row, mapping.Numero))
	return sunat.Comprobante{
		ID:           fmt.Sprintf("%s-%s-%s-%s-%d", ruc, tipo, serie, numero, index),
		RUC:          ruc,
		Tipo:         tipo,
		Serie:        serie,
		Numero:       numero,
		Libro:        mapping.Libro,
		Periodo:      period,
		FechaEmision: field(row, mapping.Fecha),
		SheetName:    mapping.Sheet,
	}
}

func field(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func decodeText(raw []byte) (string, error) {
	switch {
	case bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}):
		return string(raw[3:]), nil
	case bytes.HasPrefix(raw, []byte{0xff, 0xfe}):
		decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(raw[2:]), unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()))
		return string(decoded), err
	case bytes.HasPrefix(raw, []byte{0xfe, 0xff}):
		decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(raw[2:]), unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder()))
		return string(decoded), err
	case utf8.Valid(raw):
		return string(raw), nil
	default:
		decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(raw), charmap.Windows1252.NewDecoder()))
		return string(decoded), err
	}
}

func parseDelimited(content string) ([][]string, error) {
	separator := detectSeparator(content)
	reader := csv.NewReader(strings.NewReader(content))
	reader.Comma = separator
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = false

	rows := make([][]string, 0)
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		empty := true
		for index := range row {
			row[index] = strings.TrimSpace(strings.TrimSuffix(row[index], "\r"))
			if row[index] != "" {
				empty = false
			}
		}
		if !empty {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func detectSeparator(content string) rune {
	candidates := []rune{'|', ';', ',', '\t'}
	counts := make(map[rune]int, len(candidates))
	inQuotes := false
	lines := 0
	for _, char := range content {
		if lines >= 5 {
			break
		}
		if char == '"' {
			inQuotes = !inQuotes
			continue
		}
		if inQuotes {
			continue
		}
		if char == '\n' {
			lines++
			continue
		}
		for _, candidate := range candidates {
			if char == candidate {
				counts[candidate]++
			}
		}
	}
	selected := '|'
	best := 0
	for _, candidate := range candidates {
		if counts[candidate] > best {
			selected = candidate
			best = counts[candidate]
		}
	}
	return selected
}

func looksLikeHeader(row []string) bool {
	if len(row) == 0 {
		return false
	}
	first := strings.TrimSpace(strings.Trim(row[0], "\""))
	if first == "" {
		return true
	}
	for _, char := range first {
		if char < '0' || char > '9' {
			return true
		}
	}
	return false
}
