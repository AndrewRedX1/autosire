package xmlpreview

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html/charset"
	textunicode "golang.org/x/text/encoding/unicode"
)

// Preview es la representación segura y neutral de un comprobante UBL para la UI.
type Preview struct {
	DocumentType     string   `json:"document_type"`
	DocumentTypeCode string   `json:"document_type_code"`
	Number           string   `json:"number"`
	IssueDate        string   `json:"issue_date"`
	IssueTime        string   `json:"issue_time,omitempty"`
	DueDate          string   `json:"due_date,omitempty"`
	Currency         string   `json:"currency"`
	OperationType    string   `json:"operation_type,omitempty"`
	Supplier         Party    `json:"supplier"`
	Customer         Party    `json:"customer"`
	Lines            []Line   `json:"lines"`
	Totals           Totals   `json:"totals"`
	Reference        string   `json:"reference,omitempty"`
	Reason           string   `json:"reason,omitempty"`
	Notes            []string `json:"notes,omitempty"`
}

type Party struct {
	RUC     string `json:"ruc"`
	Name    string `json:"name"`
	Address string `json:"address,omitempty"`
}

type Line struct {
	Number      string `json:"number"`
	Code        string `json:"code,omitempty"`
	Description string `json:"description"`
	Quantity    string `json:"quantity"`
	UnitCode    string `json:"unit_code,omitempty"`
	UnitPrice   string `json:"unit_price"`
	TaxAmount   string `json:"tax_amount,omitempty"`
	Amount      string `json:"amount"`
}

type Totals struct {
	LineExtension string `json:"line_extension,omitempty"`
	TaxExclusive  string `json:"tax_exclusive,omitempty"`
	TaxInclusive  string `json:"tax_inclusive,omitempty"`
	Allowance     string `json:"allowance,omitempty"`
	Charge        string `json:"charge,omitempty"`
	Prepaid       string `json:"prepaid,omitempty"`
	Tax           string `json:"tax,omitempty"`
	Payable       string `json:"payable"`
}

type ublDocument struct {
	XMLName              xml.Name
	ID                   string             `xml:"ID"`
	IssueDate            string             `xml:"IssueDate"`
	IssueTime            string             `xml:"IssueTime"`
	DueDate              string             `xml:"DueDate"`
	InvoiceTypeCode      codeValue          `xml:"InvoiceTypeCode"`
	CreditNoteTypeCode   codeValue          `xml:"CreditNoteTypeCode"`
	DebitNoteTypeCode    codeValue          `xml:"DebitNoteTypeCode"`
	DocumentCurrencyCode string             `xml:"DocumentCurrencyCode"`
	Notes                []string           `xml:"Note"`
	Supplier             partyContainer     `xml:"AccountingSupplierParty"`
	Customer             partyContainer     `xml:"AccountingCustomerParty"`
	InvoiceLines         []ublLine          `xml:"InvoiceLine"`
	CreditLines          []ublLine          `xml:"CreditNoteLine"`
	DebitLines           []ublLine          `xml:"DebitNoteLine"`
	TaxTotals            []taxTotal         `xml:"TaxTotal"`
	LegalTotal           monetaryTotal      `xml:"LegalMonetaryTotal"`
	RequestedTotal       monetaryTotal      `xml:"RequestedMonetaryTotal"`
	BillingReferences    []billingReference `xml:"BillingReference"`
	Discrepancies        []discrepancy      `xml:"DiscrepancyResponse"`
}

type codeValue struct {
	Value         string `xml:",chardata"`
	OperationType string `xml:"listID,attr"`
}

type partyContainer struct {
	Party party `xml:"Party"`
}

type party struct {
	Identifications []struct {
		ID string `xml:"ID"`
	} `xml:"PartyIdentification"`
	Names []struct {
		Name string `xml:"Name"`
	} `xml:"PartyName"`
	LegalEntities []struct {
		RegistrationName string  `xml:"RegistrationName"`
		Address          address `xml:"RegistrationAddress"`
	} `xml:"PartyLegalEntity"`
	TaxSchemes []struct {
		RegistrationName string `xml:"RegistrationName"`
		CompanyID        string `xml:"CompanyID"`
	} `xml:"PartyTaxScheme"`
	PostalAddress address `xml:"PostalAddress"`
}

type address struct {
	StreetName       string `xml:"StreetName"`
	CitySubdivision  string `xml:"CitySubdivisionName"`
	CityName         string `xml:"CityName"`
	District         string `xml:"District"`
	CountrySubentity string `xml:"CountrySubentity"`
	Country          struct {
		Name string `xml:"Name"`
	} `xml:"Country"`
}

type quantity struct {
	Value    string `xml:",chardata"`
	UnitCode string `xml:"unitCode,attr"`
}

type amount struct {
	Value string `xml:",chardata"`
}

type ublLine struct {
	ID               string     `xml:"ID"`
	InvoicedQuantity quantity   `xml:"InvoicedQuantity"`
	CreditedQuantity quantity   `xml:"CreditedQuantity"`
	DebitedQuantity  quantity   `xml:"DebitedQuantity"`
	LineExtension    amount     `xml:"LineExtensionAmount"`
	TaxTotals        []taxTotal `xml:"TaxTotal"`
	Item             item       `xml:"Item"`
	Price            struct {
		PriceAmount amount `xml:"PriceAmount"`
	} `xml:"Price"`
}

type item struct {
	Descriptions []string `xml:"Description"`
	Name         string   `xml:"Name"`
	SellerID     struct {
		ID string `xml:"ID"`
	} `xml:"SellersItemIdentification"`
}

type taxTotal struct {
	TaxAmount amount `xml:"TaxAmount"`
}

type monetaryTotal struct {
	LineExtension amount `xml:"LineExtensionAmount"`
	TaxExclusive  amount `xml:"TaxExclusiveAmount"`
	TaxInclusive  amount `xml:"TaxInclusiveAmount"`
	Allowance     amount `xml:"AllowanceTotalAmount"`
	Charge        amount `xml:"ChargeTotalAmount"`
	Prepaid       amount `xml:"PrepaidAmount"`
	Payable       amount `xml:"PayableAmount"`
}

type billingReference struct {
	InvoiceDocumentReference struct {
		ID string `xml:"ID"`
	} `xml:"InvoiceDocumentReference"`
}

type discrepancy struct {
	ReferenceID  string `xml:"ReferenceID"`
	ResponseCode string `xml:"ResponseCode"`
	Description  string `xml:"Description"`
}

func Parse(data []byte) (Preview, error) {
	data, err := normalizeEncoding(data)
	if err != nil {
		return Preview{}, err
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel
	var document ublDocument
	if err := decoder.Decode(&document); err != nil {
		return Preview{}, fmt.Errorf("el comprobante no contiene XML UBL válido: %w", err)
	}
	if document.ID == "" {
		return Preview{}, fmt.Errorf("el XML no contiene el número del comprobante")
	}

	typeCode := firstNonEmpty(document.InvoiceTypeCode.Value, document.CreditNoteTypeCode.Value, document.DebitNoteTypeCode.Value)
	operationType := firstNonEmpty(document.InvoiceTypeCode.OperationType, document.CreditNoteTypeCode.OperationType, document.DebitNoteTypeCode.OperationType)
	lines := firstLines(document)
	resultLines := make([]Line, 0, len(lines))
	for _, source := range lines {
		qty := firstQuantity(source.InvoicedQuantity, source.CreditedQuantity, source.DebitedQuantity)
		description := strings.Join(cleanStrings(source.Item.Descriptions), " · ")
		if description == "" {
			description = strings.TrimSpace(source.Item.Name)
		}
		resultLines = append(resultLines, Line{
			Number:      strings.TrimSpace(source.ID),
			Code:        strings.TrimSpace(source.Item.SellerID.ID),
			Description: description,
			Quantity:    strings.TrimSpace(qty.Value),
			UnitCode:    strings.TrimSpace(qty.UnitCode),
			UnitPrice:   strings.TrimSpace(source.Price.PriceAmount.Value),
			TaxAmount:   firstTaxAmount(source.TaxTotals),
			Amount:      strings.TrimSpace(source.LineExtension.Value),
		})
	}

	total := document.LegalTotal
	if total.Payable.Value == "" {
		total = document.RequestedTotal
	}
	reference := ""
	if len(document.BillingReferences) > 0 {
		reference = strings.TrimSpace(document.BillingReferences[0].InvoiceDocumentReference.ID)
	}
	reason := ""
	if len(document.Discrepancies) > 0 {
		if reference == "" {
			reference = strings.TrimSpace(document.Discrepancies[0].ReferenceID)
		}
		reason = strings.TrimSpace(document.Discrepancies[0].Description)
	}

	return Preview{
		DocumentType:     documentType(document.XMLName.Local, typeCode),
		DocumentTypeCode: typeCode,
		Number:           strings.TrimSpace(document.ID),
		IssueDate:        strings.TrimSpace(document.IssueDate),
		IssueTime:        strings.TrimSpace(document.IssueTime),
		DueDate:          strings.TrimSpace(document.DueDate),
		Currency:         strings.TrimSpace(document.DocumentCurrencyCode),
		OperationType:    strings.TrimSpace(operationType),
		Supplier:         previewParty(document.Supplier.Party),
		Customer:         previewParty(document.Customer.Party),
		Lines:            resultLines,
		Totals: Totals{
			LineExtension: strings.TrimSpace(total.LineExtension.Value),
			TaxExclusive:  strings.TrimSpace(total.TaxExclusive.Value),
			TaxInclusive:  strings.TrimSpace(total.TaxInclusive.Value),
			Allowance:     strings.TrimSpace(total.Allowance.Value),
			Charge:        strings.TrimSpace(total.Charge.Value),
			Prepaid:       strings.TrimSpace(total.Prepaid.Value),
			Tax:           firstTaxAmount(document.TaxTotals),
			Payable:       strings.TrimSpace(total.Payable.Value),
		},
		Reference: reference,
		Reason:    reason,
		Notes:     cleanStrings(document.Notes),
	}, nil
}

func normalizeEncoding(data []byte) ([]byte, error) {
	if len(data) >= 2 && ((data[0] == 0xFF && data[1] == 0xFE) || (data[0] == 0xFE && data[1] == 0xFF)) {
		decoded, err := textunicode.UTF16(textunicode.LittleEndian, textunicode.ExpectBOM).NewDecoder().Bytes(data)
		if err != nil {
			return nil, fmt.Errorf("decodificando XML UTF-16: %w", err)
		}
		for _, declaration := range []string{`encoding="UTF-16"`, `encoding="utf-16"`, `encoding='UTF-16'`, `encoding='utf-16'`} {
			decoded = bytes.ReplaceAll(decoded, []byte(declaration), []byte(`encoding="UTF-8"`))
		}
		return decoded, nil
	}
	return data, nil
}

func previewParty(source party) Party {
	result := Party{}
	if len(source.TaxSchemes) > 0 {
		result.RUC = strings.TrimSpace(source.TaxSchemes[0].CompanyID)
		result.Name = strings.TrimSpace(source.TaxSchemes[0].RegistrationName)
	}
	if result.RUC == "" && len(source.Identifications) > 0 {
		result.RUC = strings.TrimSpace(source.Identifications[0].ID)
	}
	if result.Name == "" && len(source.LegalEntities) > 0 {
		result.Name = strings.TrimSpace(source.LegalEntities[0].RegistrationName)
	}
	if result.Name == "" && len(source.Names) > 0 {
		result.Name = strings.TrimSpace(source.Names[0].Name)
	}
	addr := source.PostalAddress
	if addr.StreetName == "" && len(source.LegalEntities) > 0 {
		addr = source.LegalEntities[0].Address
	}
	result.Address = strings.Join(cleanStrings([]string{
		addr.StreetName, addr.CitySubdivision, addr.District, addr.CityName, addr.CountrySubentity, addr.Country.Name,
	}), ", ")
	return result
}

func firstLines(document ublDocument) []ublLine {
	if len(document.InvoiceLines) > 0 {
		return document.InvoiceLines
	}
	if len(document.CreditLines) > 0 {
		return document.CreditLines
	}
	return document.DebitLines
}

func firstQuantity(values ...quantity) quantity {
	for _, value := range values {
		if strings.TrimSpace(value.Value) != "" {
			return value
		}
	}
	return quantity{}
}

func firstTaxAmount(totals []taxTotal) string {
	for _, total := range totals {
		if value := strings.TrimSpace(total.TaxAmount.Value); value != "" {
			return value
		}
	}
	return ""
}

func documentType(root, code string) string {
	switch strings.TrimSpace(code) {
	case "01":
		return "Factura electrónica"
	case "03":
		return "Boleta de venta electrónica"
	case "07":
		return "Nota de crédito electrónica"
	case "08":
		return "Nota de débito electrónica"
	}
	switch root {
	case "CreditNote":
		return "Nota de crédito electrónica"
	case "DebitNote":
		return "Nota de débito electrónica"
	default:
		return "Comprobante electrónico"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func cleanStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func ParseReader(reader io.Reader, limit int64) (Preview, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return Preview{}, fmt.Errorf("leyendo XML: %w", err)
	}
	if int64(len(data)) > limit {
		return Preview{}, fmt.Errorf("el XML supera el límite de %d bytes", limit)
	}
	return Parse(data)
}
