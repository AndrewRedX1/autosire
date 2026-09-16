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
	"strconv"
	"strings"
	"unicode/utf8"

	"appsire-go/internal/sunat"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const MaxPreviewRows = 1000

type ProposalItem struct {
	CompPago           string `json:"comp_pago"`
	TipoDocIdent       string `json:"tipo_doc_ident"`
	RUC                string `json:"ruc"`
	RazonSocial        string `json:"razon_social"`
	Fecha              string `json:"fecha"`
	TipoDocRef         string `json:"tipo_doc_ref"`
	SerieDocRef        string `json:"serie_doc_ref"`
	NroDocRef          string `json:"nro_doc_ref"`
	FechaRef           string `json:"fecha_ref"`
	BIGravada          string `json:"bi_gravada"`
	BIGravada10        string `json:"bi_gravada_10"`
	BIGravYNoGrav      string `json:"bi_grav_y_no_grav"`
	BINoGravada        string `json:"bi_no_gravada"`
	AdqNoGravada       string `json:"adq_no_gravada"`
	ICBPER             string `json:"icbper"`
	IGV                string `json:"igv"`
	GravYNoGravIGV     string `json:"grav_y_no_grav_igv"`
	IGV10              string `json:"igv_10"`
	ImporteTotal       string `json:"importe_total"`
	ISC                string `json:"isc"`
	NoGravIGV          string `json:"no_grav_igv"`
	OtrosConceptos     string `json:"otros_conceptos"`
	OtrosTributos      string `json:"otros_tributos"`
	ValorAdquisiciones string `json:"valor_adquisiciones"`
	Moneda             string `json:"moneda"`
	Tipo               string `json:"tipo"`
	Serie              string `json:"serie"`
	Numero             string `json:"numero"`
}

type Preview struct {
	FileName     string              `json:"file_name"`
	Headers      []string            `json:"headers"`
	Rows         [][]string          `json:"rows"`
	Comprobantes []sunat.Comprobante `json:"comprobantes"`
	Items        []ProposalItem      `json:"items"`
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
	scores := make(map[*zip.File]float64, len(candidates))
	for _, candidate := range candidates {
		scores[candidate] = proposalCandidateScore(candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if scores[candidates[i]] != scores[candidates[j]] {
			return scores[candidates[i]] > scores[candidates[j]]
		}
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
	items := make([]ProposalItem, 0, end)
	for index, source := range dataRows[:end] {
		row := make([]string, len(columns))
		for columnIndex, column := range columns {
			row[columnIndex] = field(source, column.Index)
		}
		rows = append(rows, row)
		comprobantes = append(comprobantes, buildComprobante(source, mapping, book, period, index))
		items = append(items, buildProposalItem(source, book))
	}
	return Preview{
		FileName:     filepath.Base(file.Name),
		Headers:      headers,
		Rows:         rows,
		Comprobantes: comprobantes,
		Items:        items,
		TotalRows:    total,
		Truncated:    total > MaxPreviewRows,
	}, nil
}

// proposalCandidateScore conserva la heurística del original: favorece el
// archivo con más campos por fila y descarta reportes auxiliares aunque su
// nombre lo coloque primero alfabéticamente.
func proposalCandidateScore(file *zip.File) float64 {
	stream, err := file.Open()
	if err != nil {
		return -2000
	}
	raw, readErr := io.ReadAll(io.LimitReader(stream, 256<<10))
	closeErr := stream.Close()
	if readErr != nil || closeErr != nil {
		return -2000
	}
	text, err := decodeText(raw)
	if err != nil {
		return -2000
	}

	upperName := strings.ToUpper(filepath.Base(file.Name))
	report := strings.Contains(upperName, "REPORTE") ||
		strings.Contains(upperName, "INCONSIST") ||
		strings.Contains(upperName, "PARAMETR") ||
		strings.Contains(upperName, "OBSERV") ||
		contentLooksLikeReport(text)

	separator := detectSeparator(text)
	lines := strings.Split(text, "\n")
	totalFields := 0
	used := 0
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		totalFields += strings.Count(line, string(separator)) + 1
		used++
		if used == 20 {
			break
		}
	}
	if used == 0 {
		return -2000
	}
	score := float64(totalFields) / float64(used)
	if report {
		score -= 1000
	}
	return score
}

func contentLooksLikeReport(text string) bool {
	lines := strings.Split(text, "\n")
	for index := 0; index < min(len(lines), 12); index++ {
		upper := strings.ToUpper(lines[index])
		if strings.Contains(upper, "INCONSISTENCIA") ||
			strings.Contains(upper, "REPORTE DE") ||
			strings.Contains(upper, "NÚMERO DE RUC") ||
			strings.Contains(upper, "NUMERO DE RUC") ||
			strings.Contains(upper, "DATOS DEL CONTRIBUYENTE") ||
			strings.Contains(upper, "SEMAFORO") {
			return true
		}
	}
	return false
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
	if book == sunat.ProposalRVIE {
		if emisorRuc := field(row, 0); emisorRuc != "" && len(emisorRuc) == 11 {
			ruc = emisorRuc
		}
	}
	tipo := sunat.NormalizeTipo(field(row, mapping.Tipo))
	serie := strings.ToUpper(strings.ReplaceAll(field(row, mapping.Serie), " ", ""))
	numero := sunat.NormalizeNumero(field(row, mapping.Numero))
	razonSocial := ""
	monto := ""
	moneda := ""
	if book == sunat.ProposalRCE {
		razonSocial = field(row, 13)
		monto = formatMoney(field(row, 24))
		moneda = field(row, 25)
	} else if book == sunat.ProposalRVIE {
		razonSocial = field(row, 12)
		monto = formatMoney(field(row, 25))
		moneda = field(row, 26)
	}
	if moneda == "" {
		moneda = "PEN"
	}
	return sunat.Comprobante{
		ID:           fmt.Sprintf("%s-%s-%s-%s-%d", ruc, tipo, serie, numero, index),
		RUC:          ruc,
		Tipo:         tipo,
		Serie:        serie,
		Numero:       numero,
		Libro:        mapping.Libro,
		Periodo:      period,
		RazonSocial:  razonSocial,
		FechaEmision: field(row, mapping.Fecha),
		Monto:        monto,
		Moneda:       moneda,
		SheetName:    mapping.Sheet,
	}
}

func buildProposalItem(row []string, book sunat.ProposalBook) ProposalItem {
	if book == sunat.ProposalRCE {
		tipo := sunat.NormalizeTipo(field(row, 6))
		serie := strings.ToUpper(strings.TrimSpace(field(row, 7)))
		numero := sunat.NormalizeNumero(field(row, 9))
		compPago := fmt.Sprintf("%s-%s", serie, numero)
		if tipo != "" {
			compPago = fmt.Sprintf("%s %s-%s", tipo, serie, numero)
		}
		return ProposalItem{
			CompPago:           strings.TrimSpace(compPago),
			Tipo:               tipo,
			Serie:              serie,
			Numero:             numero,
			TipoDocIdent:       field(row, 11),
			RUC:                field(row, 12),
			RazonSocial:        field(row, 13),
			Fecha:              field(row, 4),
			TipoDocRef:         field(row, 28),
			SerieDocRef:        field(row, 29),
			NroDocRef:          field(row, 31),
			FechaRef:           field(row, 27),
			BIGravada:          formatMoney(field(row, 14)),
			BIGravada10:        "0.00",
			BIGravYNoGrav:      formatMoney(field(row, 16)),
			BINoGravada:        formatMoney(field(row, 18)),
			AdqNoGravada:       formatMoney(field(row, 20)),
			ICBPER:             formatMoney(field(row, 22)),
			IGV:                formatMoney(field(row, 15)),
			GravYNoGravIGV:     formatMoney(field(row, 17)),
			IGV10:              "0.00",
			ImporteTotal:       formatMoney(field(row, 24)),
			ISC:                formatMoney(field(row, 21)),
			NoGravIGV:          formatMoney(field(row, 19)),
			OtrosConceptos:     formatMoney(field(row, 23)),
			OtrosTributos:      "0.00",
			ValorAdquisiciones: formatMoney(field(row, 35)),
			Moneda:             field(row, 25),
		}
	} else {
		// RVIE (Ventas)
		tipo := sunat.NormalizeTipo(field(row, 6))
		serie := strings.ToUpper(strings.TrimSpace(field(row, 7)))
		numero := sunat.NormalizeNumero(field(row, 8))
		compPago := fmt.Sprintf("%s-%s", serie, numero)
		if tipo != "" {
			compPago = fmt.Sprintf("%s %s-%s", tipo, serie, numero)
		}
		return ProposalItem{
			CompPago:           strings.TrimSpace(compPago),
			Tipo:               tipo,
			Serie:              serie,
			Numero:             numero,
			TipoDocIdent:       field(row, 10),
			RUC:                field(row, 11),
			RazonSocial:        field(row, 12),
			Fecha:              field(row, 4),
			TipoDocRef:         field(row, 29),
			SerieDocRef:        field(row, 30),
			NroDocRef:          field(row, 31),
			FechaRef:           field(row, 28),
			BIGravada:          formatMoney(field(row, 14)),
			BIGravada10:        "0.00",
			BIGravYNoGrav:      "0.00",
			BINoGravada:        formatMoney(field(row, 19)),
			AdqNoGravada:       formatMoney(field(row, 18)),
			ICBPER:             formatMoney(field(row, 23)),
			IGV:                formatMoney(field(row, 16)),
			GravYNoGravIGV:     "0.00",
			IGV10:              "0.00",
			ImporteTotal:       formatMoney(field(row, 25)),
			ISC:                formatMoney(field(row, 20)),
			NoGravIGV:          "0.00",
			OtrosConceptos:     formatMoney(field(row, 24)),
			OtrosTributos:      formatMoney(field(row, 22)),
			ValorAdquisiciones: formatMoney(field(row, 13)),
			Moneda:             field(row, 26),
		}
	}
}

func formatMoney(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return "0.00"
	}
	clean := strings.ReplaceAll(val, ",", "")
	f, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return val
	}
	return fmt.Sprintf("%.2f", f)
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
