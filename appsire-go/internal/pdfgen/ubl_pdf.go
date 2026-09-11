package pdfgen

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html/charset"
)

const linesPerPage = 56

type document struct {
	ID           string
	IssueDate    string
	Currency     string
	SupplierID   string
	SupplierName string
	CustomerID   string
	CustomerName string
	Payable      string
	Lines        []documentLine
}

type documentLine struct {
	ID          string
	Description string
	Quantity    string
	Amount      string
}

// FromUBL crea una representacion PDF sencilla y portable de un comprobante UBL.
func FromUBL(xmlData []byte) ([]byte, error) {
	doc, err := parseUBL(xmlData)
	if err != nil {
		return nil, fmt.Errorf("leyendo XML UBL: %w", err)
	}

	lines := documentLines(doc)
	return renderPDF(lines), nil
}

func parseUBL(data []byte) (document, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		return charset.NewReaderLabel(label, input)
	}

	var doc document
	stack := make([]string, 0, 12)
	var text strings.Builder
	var currentLine *documentLine

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return document{}, err
		}

		switch value := token.(type) {
		case xml.StartElement:
			stack = append(stack, value.Name.Local)
			text.Reset()
			if isLineElement(value.Name.Local) {
				currentLine = &documentLine{}
			}
		case xml.CharData:
			text.Write(value)
		case xml.EndElement:
			content := strings.TrimSpace(text.String())
			captureValue(&doc, currentLine, stack, value.Name.Local, content)
			if isLineElement(value.Name.Local) && currentLine != nil {
				doc.Lines = append(doc.Lines, *currentLine)
				currentLine = nil
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			text.Reset()
		}
	}

	if doc.ID == "" {
		return document{}, fmt.Errorf("el XML no contiene identificador de comprobante")
	}
	return doc, nil
}

func captureValue(
	doc *document,
	line *documentLine,
	stack []string,
	name string,
	value string,
) {
	if value == "" {
		return
	}

	path := strings.Join(stack, "/")
	if line != nil {
		switch name {
		case "ID":
			if line.ID == "" {
				line.ID = value
			}
		case "Description":
			if line.Description == "" {
				line.Description = value
			}
		case "InvoicedQuantity", "CreditedQuantity", "DebitedQuantity":
			line.Quantity = value
		case "LineExtensionAmount":
			line.Amount = value
		}
		return
	}

	switch name {
	case "ID":
		if doc.ID == "" && !strings.Contains(path, "Signature") {
			doc.ID = value
		}
	case "IssueDate":
		doc.IssueDate = value
	case "DocumentCurrencyCode":
		doc.Currency = value
	case "PayableAmount":
		doc.Payable = value
	case "RegistrationName", "Name":
		switch {
		case strings.Contains(path, "AccountingSupplierParty") && doc.SupplierName == "":
			doc.SupplierName = value
		case strings.Contains(path, "AccountingCustomerParty") && doc.CustomerName == "":
			doc.CustomerName = value
		}
	case "CompanyID":
		switch {
		case strings.Contains(path, "AccountingSupplierParty") && doc.SupplierID == "":
			doc.SupplierID = value
		case strings.Contains(path, "AccountingCustomerParty") && doc.CustomerID == "":
			doc.CustomerID = value
		}
	}
}

func isLineElement(name string) bool {
	return name == "InvoiceLine" || name == "CreditNoteLine" || name == "DebitNoteLine"
}

func documentLines(doc document) []string {
	lines := []string{
		"REPRESENTACION IMPRESA DEL COMPROBANTE ELECTRONICO",
		"",
		"Comprobante: " + doc.ID,
		"Fecha de emision: " + doc.IssueDate,
		"Emisor: " + doc.SupplierID + "  " + doc.SupplierName,
		"Cliente: " + doc.CustomerID + "  " + doc.CustomerName,
		"Moneda: " + doc.Currency,
		"Importe total: " + doc.Payable,
		"",
		"DETALLE",
	}

	for _, item := range doc.Lines {
		line := fmt.Sprintf(
			"%s  Cant: %s  Importe: %s  %s",
			item.ID,
			item.Quantity,
			item.Amount,
			item.Description,
		)
		lines = append(lines, wrapASCII(line, 94)...)
	}

	lines = append(
		lines,
		"",
		"Documento generado desde el XML UBL descargado de SUNAT.",
	)
	return lines
}

func renderPDF(lines []string) []byte {
	pages := chunkLines(lines, linesPerPage)
	objectCount := 3 + len(pages)*2
	objects := make([]string, objectCount+1)

	pageRefs := make([]string, 0, len(pages))
	for index, pageLines := range pages {
		pageObject := 4 + index*2
		contentObject := pageObject + 1
		pageRefs = append(pageRefs, fmt.Sprintf("%d 0 R", pageObject))

		stream := pageStream(pageLines)
		objects[pageObject] = fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>",
			contentObject,
		)
		objects[contentObject] = fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream)
	}

	objects[1] = "<< /Type /Catalog /Pages 2 0 R >>"
	objects[2] = fmt.Sprintf(
		"<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(pageRefs, " "),
		len(pages),
	)
	objects[3] = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>"

	var output bytes.Buffer
	output.WriteString("%PDF-1.4\n%AppSireCPE\n")
	offsets := make([]int, objectCount+1)
	for objectID := 1; objectID <= objectCount; objectID++ {
		offsets[objectID] = output.Len()
		fmt.Fprintf(&output, "%d 0 obj\n%s\nendobj\n", objectID, objects[objectID])
	}

	xref := output.Len()
	fmt.Fprintf(&output, "xref\n0 %d\n", objectCount+1)
	output.WriteString("0000000000 65535 f \n")
	for objectID := 1; objectID <= objectCount; objectID++ {
		fmt.Fprintf(&output, "%010d 00000 n \n", offsets[objectID])
	}
	fmt.Fprintf(
		&output,
		"trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		objectCount+1,
		xref,
	)
	return output.Bytes()
}

func pageStream(lines []string) string {
	var stream strings.Builder
	stream.WriteString("BT\n/F1 9 Tf\n40 800 Td\n12 TL\n")
	for _, line := range lines {
		stream.WriteString("(")
		stream.WriteString(escapePDFText(toASCII(line)))
		stream.WriteString(") Tj\nT*\n")
	}
	stream.WriteString("ET")
	return stream.String()
}

func chunkLines(lines []string, size int) [][]string {
	if len(lines) == 0 {
		return [][]string{{}}
	}

	pages := make([][]string, 0, (len(lines)+size-1)/size)
	for start := 0; start < len(lines); start += size {
		end := min(start+size, len(lines))
		pages = append(pages, lines[start:end])
	}
	return pages
}

func wrapASCII(value string, width int) []string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= width {
		return []string{value}
	}

	result := []string{}
	for len(value) > width {
		cut := strings.LastIndex(value[:width+1], " ")
		if cut <= 0 {
			cut = width
		}
		result = append(result, value[:cut])
		value = strings.TrimSpace(value[cut:])
	}
	if value != "" {
		result = append(result, value)
	}
	return result
}

func escapePDFText(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "(", "\\(")
	return strings.ReplaceAll(value, ")", "\\)")
}

func toASCII(value string) string {
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n",
		"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ñ", "N",
	)
	value = replacer.Replace(value)

	var output strings.Builder
	for _, char := range value {
		if char >= 32 && char <= 126 {
			output.WriteRune(char)
		}
	}
	return output.String()
}
