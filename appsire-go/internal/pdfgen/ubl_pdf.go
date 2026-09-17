package pdfgen

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

const (
	pageWidth      = 210.0
	leftMargin     = 14.0
	rightMargin    = 14.0
	contentWidth   = pageWidth - leftMargin - rightMargin
	pageBottom     = 276.0
	defaultLineGap = 4.0
)

var documentNames = map[string]string{
	"01": "FACTURA ELECTRONICA",
	"02": "RECIBO POR HONORARIOS ELECTRONICO",
	"03": "BOLETA DE VENTA ELECTRONICA",
	"04": "LIQUIDACION DE COMPRA ELECTRONICA",
	"07": "NOTA DE CREDITO ELECTRONICA",
	"08": "NOTA DE DEBITO ELECTRONICA",
	"12": "TICKET DE MAQUINA REGISTRADORA",
	"14": "RECIBO DE SERVICIOS PUBLICOS",
	"20": "COMPROBANTE DE RETENCION ELECTRONICO",
	"30": "DOCUMENTO DEL ADQUIRENTE - TARJETAS",
	"40": "COMPROBANTE DE PERCEPCION ELECTRONICO",
	"87": "NOTA DE CREDITO ESPECIAL",
	"88": "NOTA DE DEBITO ESPECIAL",
	"91": "COMPROBANTE DE NO DOMICILIADO",
}

var unitNames = map[string]string{
	"NIU": "UNIDAD", "H87": "UNIDAD", "C62": "UNIDAD", "ZZ": "SERVICIO",
	"KGM": "KILOGRAMO", "GRM": "GRAMO", "TNE": "TONELADA", "MTR": "METRO",
	"CMT": "CENTIMETRO", "MTK": "METRO CUADRADO", "MTQ": "METRO CUBICO",
	"LTR": "LITRO", "GLL": "GALON", "BX": "CAJA", "PK": "PAQUETE",
	"BG": "BOLSA", "DZN": "DOCENA", "PR": "PAR", "SET": "JUEGO",
	"MIL": "MILLAR", "CEN": "CIENTO", "HUR": "HORA", "DAY": "DIA",
}

type document struct {
	ID                    string
	TypeCode              string
	Title                 string
	IssueDate             string
	DueDate               string
	Currency              string
	SupplierID            string
	SupplierTradeName     string
	SupplierName          string
	SupplierAddress       string
	CustomerID            string
	CustomerDocumentType  string
	CustomerName          string
	CustomerAddress       string
	PaymentMethod         string
	Notes                 string
	AmountInWords         string
	ModifiedDocument      string
	ModificationReason    string
	Payable               float64
	Taxed                 float64
	Unaffected            float64
	Exonerated            float64
	Free                  float64
	Exported              float64
	IGV                   float64
	ISC                   float64
	ICBPER                float64
	OtherTaxes            float64
	Discounts             float64
	OtherCharges          float64
	Prepaid               float64
	Rounding              float64
	SignatureDigest       string
	Lines                 []documentLine
	GlobalDiscountTaxed   float64
	GlobalDiscountUntaxed float64
	PaymentInstallments   []paymentInstallment
	PendingPayment        float64
	Detraction            *detraction
}

type documentLine struct {
	Quantity    float64
	Unit        string
	Code        string
	Description string
	UnitPrice   float64
	Amount      float64
	ICBPER      float64
}

type paymentInstallment struct {
	Name    string
	Amount  float64
	DueDate string
}

type detraction struct {
	Code       string
	Percentage float64
	Amount     float64
	Account    string
}

type value struct {
	Text string `xml:",chardata"`
}

type identifiedValue struct {
	Text     string `xml:",chardata"`
	SchemeID string `xml:"schemeID,attr"`
}

type quantityValue struct {
	Text     string `xml:",chardata"`
	UnitCode string `xml:"unitCode,attr"`
}

type noteValue struct {
	Text     string `xml:",chardata"`
	LocaleID string `xml:"languageLocaleID,attr"`
}

type rawAddress struct {
	District         string `xml:"District"`
	City             string `xml:"CityName"`
	CountrySubentity string `xml:"CountrySubentity"`
	AddressLine      struct {
		Line string `xml:"Line"`
	} `xml:"AddressLine"`
}

type rawParty struct {
	CustomerAssignedAccountID string `xml:"CustomerAssignedAccountID"`
	AdditionalAccountID       string `xml:"AdditionalAccountID"`
	Party                     struct {
		Identifications []struct {
			ID identifiedValue `xml:"ID"`
		} `xml:"PartyIdentification"`
		PartyName struct {
			Name string `xml:"Name"`
		} `xml:"PartyName"`
		LegalEntity struct {
			RegistrationName    string     `xml:"RegistrationName"`
			RegistrationAddress rawAddress `xml:"RegistrationAddress"`
		} `xml:"PartyLegalEntity"`
		PostalAddress rawAddress `xml:"PostalAddress"`
	} `xml:"Party"`
}

type rawTaxTotal struct {
	Subtotals []struct {
		TaxableAmount value `xml:"TaxableAmount"`
		TaxAmount     value `xml:"TaxAmount"`
		TaxCategory   struct {
			TaxScheme struct {
				ID string `xml:"ID"`
			} `xml:"TaxScheme"`
		} `xml:"TaxCategory"`
	} `xml:"TaxSubtotal"`
}

type rawLine struct {
	InvoicedQuantity quantityValue `xml:"InvoicedQuantity"`
	CreditedQuantity quantityValue `xml:"CreditedQuantity"`
	DebitedQuantity  quantityValue `xml:"DebitedQuantity"`
	ExtensionAmount  value         `xml:"LineExtensionAmount"`
	Item             struct {
		Descriptions []string `xml:"Description"`
		SellerID     struct {
			ID string `xml:"ID"`
		} `xml:"SellersItemIdentification"`
	} `xml:"Item"`
	Price struct {
		Amount value `xml:"PriceAmount"`
	} `xml:"Price"`
	PricingReference struct {
		AlternativePrices []struct {
			Amount value `xml:"PriceAmount"`
		} `xml:"AlternativeConditionPrice"`
	} `xml:"PricingReference"`
	TaxTotals []rawTaxTotal `xml:"TaxTotal"`
}

type rawDocument struct {
	XMLName          xml.Name
	ID               string        `xml:"ID"`
	IssueDate        string        `xml:"IssueDate"`
	DueDate          string        `xml:"DueDate"`
	Currency         string        `xml:"DocumentCurrencyCode"`
	InvoiceTypeCode  string        `xml:"InvoiceTypeCode"`
	Notes            []noteValue   `xml:"Note"`
	Supplier         rawParty      `xml:"AccountingSupplierParty"`
	Customer         rawParty      `xml:"AccountingCustomerParty"`
	TaxTotals        []rawTaxTotal `xml:"TaxTotal"`
	InvoiceLines     []rawLine     `xml:"InvoiceLine"`
	CreditLines      []rawLine     `xml:"CreditNoteLine"`
	DebitLines       []rawLine     `xml:"DebitNoteLine"`
	AllowanceCharges []struct {
		ChargeIndicator string `xml:"ChargeIndicator"`
		ReasonCode      string `xml:"AllowanceChargeReasonCode"`
		Amount          value  `xml:"Amount"`
	} `xml:"AllowanceCharge"`
	MonetaryTotal struct {
		LineExtensionAmount   value `xml:"LineExtensionAmount"`
		AllowanceTotalAmount  value `xml:"AllowanceTotalAmount"`
		ChargeTotalAmount     value `xml:"ChargeTotalAmount"`
		PrepaidAmount         value `xml:"PrepaidAmount"`
		PayableRoundingAmount value `xml:"PayableRoundingAmount"`
		PayableAmount         value `xml:"PayableAmount"`
	} `xml:"LegalMonetaryTotal"`
	PaymentTerms []struct {
		ID             string `xml:"ID"`
		PaymentMeansID string `xml:"PaymentMeansID"`
		Amount         value  `xml:"Amount"`
		DueDate        string `xml:"PaymentDueDate"`
		Percent        string `xml:"PaymentPercent"`
	} `xml:"PaymentTerms"`
	PaymentMeans []struct {
		ID      string `xml:"ID"`
		Account struct {
			ID string `xml:"ID"`
		} `xml:"PayeeFinancialAccount"`
	} `xml:"PaymentMeans"`
	BillingReferences []struct {
		InvoiceReference struct {
			ID       string `xml:"ID"`
			TypeCode string `xml:"DocumentTypeCode"`
		} `xml:"InvoiceDocumentReference"`
	} `xml:"BillingReference"`
	DiscrepancyResponses []struct {
		Description string `xml:"Description"`
	} `xml:"DiscrepancyResponse"`
}

// FromUBL genera una representación impresa A4 desde un XML UBL 2.0/2.1.
// Incluye el resumen tributario, DigestValue y el QR reglamentario de SUNAT.
func FromUBL(xmlData []byte) ([]byte, error) {
	doc, err := parseUBL(xmlData)
	if err != nil {
		return nil, fmt.Errorf("leyendo XML UBL: %w", err)
	}

	pdfData, err := renderPDF(doc)
	if err != nil {
		return nil, fmt.Errorf("renderizando representación impresa: %w", err)
	}
	return pdfData, nil
}

func parseUBL(data []byte) (document, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		return charset.NewReaderLabel(label, input)
	}

	var raw rawDocument
	if err := decoder.Decode(&raw); err != nil {
		return document{}, err
	}

	rootName := strings.ToLower(strings.TrimSpace(raw.XMLName.Local))
	if rootName != "invoice" && rootName != "creditnote" && rootName != "debitnote" {
		return document{}, fmt.Errorf("el XML no es Invoice, CreditNote ni DebitNote")
	}
	if strings.TrimSpace(raw.ID) == "" {
		return document{}, fmt.Errorf("el XML no contiene identificador de comprobante")
	}

	typeCode := strings.TrimSpace(raw.InvoiceTypeCode)
	switch rootName {
	case "creditnote":
		typeCode = "07"
	case "debitnote":
		typeCode = "08"
	case "invoice":
		if typeCode == "" {
			typeCode = "01"
		}
	}

	doc := document{
		ID:                   clean(raw.ID),
		TypeCode:             typeCode,
		Title:                documentTitle(typeCode),
		IssueDate:            clean(raw.IssueDate),
		DueDate:              clean(raw.DueDate),
		Currency:             defaultString(clean(raw.Currency), "PEN"),
		SupplierID:           partyID(raw.Supplier),
		SupplierTradeName:    clean(raw.Supplier.Party.PartyName.Name),
		SupplierName:         clean(raw.Supplier.Party.LegalEntity.RegistrationName),
		SupplierAddress:      formatAddress(raw.Supplier.Party.LegalEntity.RegistrationAddress, raw.Supplier.Party.PostalAddress),
		CustomerID:           partyID(raw.Customer),
		CustomerDocumentType: partyDocumentType(raw.Customer),
		CustomerName:         clean(raw.Customer.Party.LegalEntity.RegistrationName),
		CustomerAddress:      formatAddress(raw.Customer.Party.LegalEntity.RegistrationAddress, raw.Customer.Party.PostalAddress),
		PaymentMethod:        "Contado",
		Payable:              number(raw.MonetaryTotal.PayableAmount.Text),
		Discounts:            number(raw.MonetaryTotal.AllowanceTotalAmount.Text),
		OtherCharges:         number(raw.MonetaryTotal.ChargeTotalAmount.Text),
		Prepaid:              number(raw.MonetaryTotal.PrepaidAmount.Text),
		Rounding:             number(raw.MonetaryTotal.PayableRoundingAmount.Text),
		SignatureDigest:      extractDigestValue(data),
	}

	parseNotes(&doc, raw.Notes)
	parseTaxTotals(&doc, raw.TaxTotals)
	parsePaymentTerms(&doc, raw)
	parseReferences(&doc, raw)
	for _, allowance := range raw.AllowanceCharges {
		if strings.EqualFold(clean(allowance.ChargeIndicator), "true") {
			continue
		}
		switch clean(allowance.ReasonCode) {
		case "02":
			doc.GlobalDiscountTaxed += number(allowance.Amount.Text)
		case "03":
			doc.GlobalDiscountUntaxed += number(allowance.Amount.Text)
		}
	}

	lines := raw.InvoiceLines
	if rootName == "creditnote" {
		lines = raw.CreditLines
	} else if rootName == "debitnote" {
		lines = raw.DebitLines
	}
	for _, line := range lines {
		doc.Lines = append(doc.Lines, parseLine(line))
	}

	return doc, nil
}

func parseLine(raw rawLine) documentLine {
	quantity := raw.InvoicedQuantity
	if clean(quantity.Text) == "" {
		quantity = raw.CreditedQuantity
	}
	if clean(quantity.Text) == "" {
		quantity = raw.DebitedQuantity
	}

	unitPrice := number(raw.Price.Amount.Text)
	if unitPrice == 0 && len(raw.PricingReference.AlternativePrices) > 0 {
		unitPrice = number(raw.PricingReference.AlternativePrices[0].Amount.Text)
	}
	amount := number(raw.ExtensionAmount.Text)
	if unitPrice == 0 {
		unitPrice = amount
	}

	icbper := 0.0
	for _, total := range raw.TaxTotals {
		for _, subtotal := range total.Subtotals {
			if clean(subtotal.TaxCategory.TaxScheme.ID) == "7152" {
				icbper += number(subtotal.TaxAmount.Text)
			}
		}
	}

	descriptions := make([]string, 0, len(raw.Item.Descriptions))
	for _, description := range raw.Item.Descriptions {
		if description = clean(description); description != "" {
			descriptions = append(descriptions, description)
		}
	}
	unitCode := strings.ToUpper(clean(quantity.UnitCode))
	unit := unitNames[unitCode]
	if unit == "" {
		unit = defaultString(unitCode, "UNIDAD")
	}

	return documentLine{
		Quantity:    number(quantity.Text),
		Unit:        unit,
		Code:        defaultString(clean(raw.Item.SellerID.ID), "-"),
		Description: defaultString(strings.Join(descriptions, " "), "-"),
		UnitPrice:   unitPrice,
		Amount:      amount,
		ICBPER:      icbper,
	}
}

func parseNotes(doc *document, notes []noteValue) {
	observations := make([]string, 0, len(notes))
	for _, note := range notes {
		text := clean(note.Text)
		if text == "" {
			continue
		}
		if clean(note.LocaleID) == "1000" || strings.Contains(text, "/100") {
			if doc.AmountInWords == "" {
				doc.AmountInWords = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(text, "SON:"), "SON "))
				continue
			}
		}
		observations = append(observations, text)
	}
	doc.Notes = strings.Join(observations, " | ")
}

func parseTaxTotals(doc *document, totals []rawTaxTotal) {
	for _, total := range totals {
		for _, subtotal := range total.Subtotals {
			taxable := number(subtotal.TaxableAmount.Text)
			tax := number(subtotal.TaxAmount.Text)
			switch clean(subtotal.TaxCategory.TaxScheme.ID) {
			case "1000", "1016":
				doc.Taxed += taxable
				doc.IGV += tax
			case "9997":
				doc.Exonerated += taxable
			case "9998":
				doc.Unaffected += taxable
			case "9996":
				doc.Free += taxable
			case "9995":
				doc.Exported += taxable
			case "2000":
				doc.ISC += tax
			case "7152":
				doc.ICBPER += tax
			default:
				doc.OtherTaxes += tax
			}
		}
	}
}

func parsePaymentTerms(doc *document, raw rawDocument) {
	var detractionInfo *detraction
	for _, term := range raw.PaymentTerms {
		id := clean(term.ID)
		means := clean(term.PaymentMeansID)
		switch {
		case strings.EqualFold(id, "FormaPago") && strings.EqualFold(means, "Credito"):
			doc.PaymentMethod = "Credito"
			if amount := number(term.Amount.Text); amount > 0 {
				doc.PendingPayment = amount
			}
		case strings.EqualFold(id, "FormaPago") && strings.HasPrefix(strings.ToLower(means), "cuota"):
			doc.PaymentMethod = "Credito"
			doc.PaymentInstallments = append(doc.PaymentInstallments, paymentInstallment{
				Name: means, Amount: number(term.Amount.Text), DueDate: clean(term.DueDate),
			})
		case strings.EqualFold(id, "Detraccion"):
			detractionInfo = &detraction{
				Code: means, Percentage: number(term.Percent), Amount: number(term.Amount.Text),
			}
		}
	}
	if doc.PendingPayment == 0 {
		for _, installment := range doc.PaymentInstallments {
			doc.PendingPayment += installment.Amount
		}
	}
	if detractionInfo != nil {
		for _, means := range raw.PaymentMeans {
			if strings.EqualFold(clean(means.ID), "Detraccion") {
				detractionInfo.Account = clean(means.Account.ID)
				break
			}
		}
		doc.Detraction = detractionInfo
	}
}

func parseReferences(doc *document, raw rawDocument) {
	if len(raw.BillingReferences) > 0 {
		ref := raw.BillingReferences[0].InvoiceReference
		if id := clean(ref.ID); id != "" {
			kind := "FACTURA"
			if clean(ref.TypeCode) == "03" {
				kind = "BOLETA DE VENTA"
			}
			doc.ModifiedDocument = kind + " " + id
		}
	}
	if len(raw.DiscrepancyResponses) > 0 {
		doc.ModificationReason = clean(raw.DiscrepancyResponses[0].Description)
	}
}

func renderPDF(doc document) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(leftMargin, 12, rightMargin)
	pdf.SetAutoPageBreak(false, 12)
	pdf.SetTitle(pdfText(doc.ID+" - "+doc.Title), false)
	pdf.SetAuthor("AppSireCPE", false)
	pdf.SetCreator(pdfText("AppSireCPE - representación impresa desde XML UBL"), false)
	pdf.AliasNbPages("")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(90, 90, 90)
		pdf.CellFormat(0, 4, pdfText(fmt.Sprintf("Pagina %d/{nb}", pdf.PageNo())), "", 0, "R", false, 0, "")
	})

	addPage := func() float64 {
		pdf.AddPage()
		return drawHeader(pdf, doc)
	}

	y := addPage()
	y = drawDocumentData(pdf, doc, y)
	y = drawTableHeader(pdf, y)
	for _, line := range doc.Lines {
		rowHeight := measureLineHeight(pdf, line)
		if y+rowHeight > pageBottom {
			y = addPage()
			y = drawTableHeader(pdf, y)
		}
		drawLine(pdf, line, y, rowHeight)
		y += rowHeight
	}

	finalHeight := finalBlockHeight(doc)
	if y+finalHeight > pageBottom {
		y = addPage()
	}
	drawFinalBlock(pdf, doc, y+3)

	if err := pdf.Error(); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func drawHeader(pdf *fpdf.Fpdf, doc document) float64 {
	top := 12.0
	boxX, boxW, boxH := 132.0, 64.0, 30.0
	leftW := boxX - leftMargin - 5

	pdf.SetTextColor(20, 20, 20)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetXY(leftMargin, top)
	pdf.MultiCell(leftW, 5, pdfText(defaultString(doc.SupplierName, doc.SupplierTradeName)), "", "L", false)
	y := pdf.GetY()
	if trade := clean(doc.SupplierTradeName); trade != "" && !strings.EqualFold(trade, doc.SupplierName) {
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetXY(leftMargin, y)
		pdf.MultiCell(leftW, 4, pdfText(trade), "", "L", false)
		y = pdf.GetY()
	}
	if doc.SupplierAddress != "" {
		pdf.SetFont("Helvetica", "", 7.5)
		pdf.SetXY(leftMargin, y)
		pdf.MultiCell(leftW, 3.8, pdfText(doc.SupplierAddress), "", "L", false)
	}

	pdf.SetLineWidth(0.7)
	pdf.Rect(boxX, top, boxW, boxH, "D")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetXY(boxX+2, top+2)
	pdf.CellFormat(boxW-4, 7, pdfText(doc.Title), "", 0, "C", false, 0, "")
	pdf.SetXY(boxX+2, top+11)
	pdf.CellFormat(boxW-4, 6, pdfText("RUC: "+doc.SupplierID), "", 0, "C", false, 0, "")
	pdf.SetXY(boxX+2, top+20)
	pdf.CellFormat(boxW-4, 6, pdfText(formatDocumentID(doc.ID)), "", 0, "C", false, 0, "")
	pdf.SetLineWidth(0.2)

	bottom := math.Max(pdf.GetY(), top+boxH) + 5
	pdf.Line(leftMargin, bottom, pageWidth-rightMargin, bottom)
	return bottom + 4
}

func drawDocumentData(pdf *fpdf.Fpdf, doc document, y float64) float64 {
	rows := [][2]string{{"Fecha de emision", formatDate(doc.IssueDate)}}
	if doc.DueDate != "" {
		rows = append(rows, [2]string{"Fecha de vencimiento", formatDate(doc.DueDate)})
	}
	rows = append(rows,
		[2]string{"Senor(es)", doc.CustomerName},
		[2]string{customerDocumentLabel(doc.CustomerDocumentType), doc.CustomerID},
		[2]string{"Direccion del cliente", defaultString(doc.CustomerAddress, "-")},
	)
	if doc.ModifiedDocument != "" {
		rows = append(rows, [2]string{"Documento que modifica", doc.ModifiedDocument})
	}
	if doc.ModificationReason != "" {
		rows = append(rows, [2]string{"Motivo o sustento", doc.ModificationReason})
	}
	rows = append(rows,
		[2]string{"Tipo de moneda", currencyName(doc.Currency)},
		[2]string{"Observacion", defaultString(doc.Notes, "-")},
	)

	startY := y
	pdf.SetFont("Helvetica", "", 7.5)
	for _, row := range rows {
		valueLines := pdf.SplitLines([]byte(pdfText(row[1])), 84)
		height := math.Max(defaultLineGap, float64(len(valueLines))*defaultLineGap)
		pdf.SetXY(leftMargin, y)
		pdf.CellFormat(39, defaultLineGap, pdfText(row[0]), "", 0, "L", false, 0, "")
		pdf.SetXY(leftMargin+40, y)
		pdf.CellFormat(3, defaultLineGap, ":", "", 0, "C", false, 0, "")
		pdf.SetXY(leftMargin+44, y)
		pdf.MultiCell(84, defaultLineGap, pdfText(row[1]), "", "L", false)
		y += height
	}
	pdf.SetFont("Helvetica", "", 7.5)
	pdf.SetXY(146, startY)
	pdf.CellFormat(24, defaultLineGap, pdfText("Forma de pago:"), "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 7.5)
	pdf.CellFormat(26, defaultLineGap, pdfText(doc.PaymentMethod), "", 0, "L", false, 0, "")
	return y + 3
}

var tableWidths = []float64{16, 22, 20, 72, 26, 26}

func drawTableHeader(pdf *fpdf.Fpdf, y float64) float64 {
	headers := []string{"Cantidad", "Unidad", "Codigo", "Descripcion", "V. unitario", "Importe"}
	x := leftMargin
	pdf.SetFillColor(235, 238, 242)
	pdf.SetFont("Helvetica", "B", 7)
	for index, header := range headers {
		pdf.SetXY(x, y)
		pdf.CellFormat(tableWidths[index], 7, pdfText(header), "1", 0, "C", true, 0, "")
		x += tableWidths[index]
	}
	return y + 7
}

func measureLineHeight(pdf *fpdf.Fpdf, line documentLine) float64 {
	pdf.SetFont("Helvetica", "", 7)
	lines := pdf.SplitLines([]byte(pdfText(line.Description)), tableWidths[3]-2)
	return math.Max(6, float64(len(lines))*3.5+1.5)
}

func drawLine(pdf *fpdf.Fpdf, line documentLine, y, height float64) {
	values := []string{
		formatQuantity(line.Quantity), line.Unit, line.Code, line.Description,
		formatDecimal(line.UnitPrice), formatDecimal(line.Amount),
	}
	alignments := []string{"C", "C", "C", "L", "R", "R"}
	x := leftMargin
	pdf.SetFont("Helvetica", "", 7)
	for index, cell := range values {
		width := tableWidths[index]
		pdf.Rect(x, y, width, height, "D")
		if index == 3 {
			pdf.SetXY(x+1, y+1)
			pdf.MultiCell(width-2, 3.5, pdfText(cell), "", "L", false)
		} else {
			pdf.SetXY(x+1, y+(height-3.5)/2)
			pdf.CellFormat(width-2, 3.5, pdfText(cell), "", 0, alignments[index], false, 0, "")
		}
		x += width
	}
}

func finalBlockHeight(doc document) float64 {
	height := 81.0
	if len(doc.PaymentInstallments) > 0 {
		height += 14 + math.Ceil(float64(len(doc.PaymentInstallments))/3)*5
	}
	if doc.Detraction != nil {
		height += 18
	}
	return height
}

func drawFinalBlock(pdf *fpdf.Fpdf, doc document, y float64) {
	leftW := 93.0
	rightX := leftMargin + leftW + 5
	rightW := contentWidth - leftW - 5

	if doc.AmountInWords != "" {
		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetXY(leftMargin, y)
		pdf.MultiCell(leftW, 3.8, pdfText("SON: "+doc.AmountInWords), "", "L", false)
	}

	totals := [][2]string{
		{"Op. gravadas", money(doc, doc.Taxed)},
		{"Op. inafectas", money(doc, doc.Unaffected)},
		{"Op. exoneradas", money(doc, doc.Exonerated)},
		{"Op. gratuitas", money(doc, doc.Free)},
		{"Op. exportacion", money(doc, doc.Exported)},
		{"Descuentos", money(doc, doc.Discounts)},
		{"ISC", money(doc, doc.ISC)},
		{"IGV", money(doc, doc.IGV)},
		{"ICBPER", money(doc, doc.ICBPER)},
		{"Otros tributos", money(doc, doc.OtherTaxes)},
		{"Otros cargos", money(doc, doc.OtherCharges)},
		{"Anticipos", money(doc, doc.Prepaid)},
		{"Redondeo", money(doc, doc.Rounding)},
		{"IMPORTE TOTAL", money(doc, doc.Payable)},
	}
	pdf.SetFont("Helvetica", "", 7)
	totalY := y
	for index, total := range totals {
		style := ""
		if index == len(totals)-1 {
			style = "B"
		}
		pdf.SetFont("Helvetica", style, 7)
		pdf.SetXY(rightX, totalY)
		pdf.CellFormat(rightW-28, 4.5, pdfText(total[0]), "1", 0, "R", false, 0, "")
		pdf.CellFormat(28, 4.5, pdfText(total[1]), "1", 0, "R", false, 0, "")
		totalY += 4.5
	}

	qrY := y + 13
	qrContent := qrContent(doc)
	if image, err := qrcode.Encode(qrContent, qrcode.Medium, 256); err == nil {
		options := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
		imageName := "sunat-qr-" + strconv.Itoa(pdf.PageNo())
		pdf.RegisterImageOptionsReader(imageName, options, bytes.NewReader(image))
		pdf.ImageOptions(imageName, leftMargin, qrY, 31, 31, false, options, 0, "")
	}
	pdf.SetFont("Helvetica", "", 6.5)
	pdf.SetXY(leftMargin+34, qrY)
	pdf.MultiCell(leftW-34, 3.5, pdfText("Codigo Hash:\n"+defaultString(doc.SignatureDigest, "No informado en el XML")), "", "L", false)

	blockY := math.Max(totalY, qrY+34) + 4
	if len(doc.PaymentInstallments) > 0 {
		blockY = drawInstallments(pdf, doc, blockY)
	}
	if doc.Detraction != nil {
		blockY = drawDetraction(pdf, doc, blockY)
	}

	pdf.SetFont("Helvetica", "I", 6.5)
	pdf.SetXY(leftMargin, blockY+3)
	pdf.MultiCell(contentWidth, 3.6, pdfText("Esta es una representacion impresa del comprobante electronico, generada por AppSireCPE desde el XML UBL. La validez tributaria corresponde al XML firmado y a su constancia de recepcion."), "1", "C", false)
}

func drawInstallments(pdf *fpdf.Fpdf, doc document, y float64) float64 {
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetXY(leftMargin, y)
	pdf.CellFormat(contentWidth, 5, pdfText(fmt.Sprintf("Informacion del credito - pendiente: %s", money(doc, doc.PendingPayment))), "1", 1, "L", false, 0, "")
	y += 5
	columnWidth := contentWidth / 3
	for index, installment := range doc.PaymentInstallments {
		column := index % 3
		row := index / 3
		x := leftMargin + float64(column)*columnWidth
		cellY := y + float64(row)*5
		text := fmt.Sprintf("%s | %s | %s", installment.Name, formatDate(installment.DueDate), money(doc, installment.Amount))
		pdf.SetFont("Helvetica", "", 6.5)
		pdf.SetXY(x, cellY)
		pdf.CellFormat(columnWidth, 5, pdfText(text), "1", 0, "C", false, 0, "")
	}
	return y + math.Ceil(float64(len(doc.PaymentInstallments))/3)*5
}

func drawDetraction(pdf *fpdf.Fpdf, doc document, y float64) float64 {
	d := doc.Detraction
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetXY(leftMargin, y+3)
	pdf.CellFormat(contentWidth, 5, pdfText("Informacion de la detraccion"), "1", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 6.5)
	pdf.SetXY(leftMargin, y+8)
	text := fmt.Sprintf("Bien/servicio: %s   Cuenta BN: %s   Porcentaje: %s%%   Monto: %s", d.Code, defaultString(d.Account, "-"), formatQuantity(d.Percentage), money(doc, d.Amount))
	pdf.CellFormat(contentWidth, 6, pdfText(text), "1", 0, "L", false, 0, "")
	return y + 14
}

func partyID(party rawParty) string {
	for _, identification := range party.Party.Identifications {
		if id := clean(identification.ID.Text); id != "" {
			return id
		}
	}
	return clean(party.CustomerAssignedAccountID)
}

func partyDocumentType(party rawParty) string {
	for _, identification := range party.Party.Identifications {
		if scheme := clean(identification.ID.SchemeID); scheme != "" {
			return scheme
		}
	}
	return defaultString(clean(party.AdditionalAccountID), "6")
}

func formatAddress(addresses ...rawAddress) string {
	for _, address := range addresses {
		line := clean(address.AddressLine.Line)
		parts := make([]string, 0, 3)
		for _, part := range []string{address.District, address.City, address.CountrySubentity} {
			if part = clean(part); part != "" {
				parts = append(parts, part)
			}
		}
		location := strings.Join(parts, " - ")
		if line != "" || location != "" {
			if location != "" && (line == "" || !strings.Contains(strings.ToUpper(line), strings.ToUpper(parts[0]))) {
				line = strings.TrimSpace(line + " " + location)
			}
			return line
		}
	}
	return ""
}

func extractDigestValue(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		return charset.NewReaderLabel(label, input)
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "DigestValue" {
			continue
		}
		var digest string
		if err := decoder.DecodeElement(&digest, &start); err == nil {
			return clean(digest)
		}
		return ""
	}
}

func qrContent(doc document) string {
	parts := strings.SplitN(doc.ID, "-", 2)
	series, number := doc.ID, ""
	if len(parts) == 2 {
		series, number = parts[0], parts[1]
	}
	return strings.Join([]string{
		doc.SupplierID, doc.TypeCode, series, number,
		fmt.Sprintf("%.2f", doc.IGV), fmt.Sprintf("%.2f", doc.Payable),
		doc.IssueDate, doc.CustomerDocumentType, doc.CustomerID, "",
	}, "|")
}

func documentTitle(typeCode string) string {
	if title := documentNames[typeCode]; title != "" {
		return title
	}
	return "COMPROBANTE ELECTRONICO (CODIGO " + typeCode + ")"
}

func customerDocumentLabel(code string) string {
	switch code {
	case "1":
		return "DNI"
	case "4":
		return "Carnet de extranjeria"
	case "7":
		return "Pasaporte"
	case "A":
		return "Cedula diplomatica"
	case "0":
		return "Documento de identidad"
	default:
		return "RUC"
	}
}

func currencyName(currency string) string {
	switch strings.ToUpper(clean(currency)) {
	case "USD":
		return "DOLAR AMERICANO"
	case "EUR":
		return "EURO"
	default:
		return "SOL"
	}
}

func currencySymbol(currency string) string {
	switch strings.ToUpper(clean(currency)) {
	case "USD":
		return "US$"
	case "EUR":
		return "EUR"
	default:
		return "S/"
	}
}

func money(doc document, amount float64) string {
	return currencySymbol(doc.Currency) + " " + formatDecimal(amount)
}

func formatDocumentID(id string) string {
	parts := strings.SplitN(clean(id), "-", 2)
	if len(parts) != 2 {
		return clean(id)
	}
	number := strings.TrimLeft(parts[1], "0")
	if number == "" {
		number = "0"
	}
	return parts[0] + " - " + number
}

func formatDate(value string) string {
	value = clean(value)
	if len(value) >= 10 && value[4] == '-' && value[7] == '-' {
		return value[8:10] + "/" + value[5:7] + "/" + value[:4]
	}
	return value
}

func formatDecimal(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

func formatQuantity(value float64) string {
	formatted := strconv.FormatFloat(value, 'f', 6, 64)
	formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	if formatted == "" {
		return "0"
	}
	return formatted
}

func number(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func clean(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func defaultString(value, fallback string) string {
	if clean(value) == "" {
		return fallback
	}
	return value
}

func pdfText(value string) string {
	value = strings.NewReplacer(
		"–", "-", "—", "-", "‑", "-", "“", `"`, "”", `"`, "’", "'",
	).Replace(value)
	encoded, _, err := transform.String(
		encoding.ReplaceUnsupported(charmap.Windows1252.NewEncoder()),
		value,
	)
	if err != nil {
		return value
	}
	return encoded
}
