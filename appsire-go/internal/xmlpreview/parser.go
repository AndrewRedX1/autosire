package xmlpreview

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"golang.org/x/net/html/charset"
	textunicode "golang.org/x/text/encoding/unicode"
)

// Preview es la representación segura y neutral de un comprobante UBL para la UI.
type Preview struct {
	Kind             string            `json:"kind"`
	Family           string            `json:"family,omitempty"`
	DocumentType     string            `json:"document_type"`
	DocumentTypeCode string            `json:"document_type_code"`
	Number           string            `json:"number"`
	IssueDate        string            `json:"issue_date"`
	IssueTime        string            `json:"issue_time,omitempty"`
	DueDate          string            `json:"due_date,omitempty"`
	Currency         string            `json:"currency"`
	OperationType    string            `json:"operation_type,omitempty"`
	Supplier         Party             `json:"supplier"`
	Customer         Party             `json:"customer"`
	Lines            []Line            `json:"lines"`
	Totals           Totals            `json:"totals"`
	Financial        *FinancialSummary `json:"financial,omitempty"`
	Reference        string            `json:"reference,omitempty"`
	ReferenceType    string            `json:"reference_type,omitempty"`
	ReferenceDate    string            `json:"reference_date,omitempty"`
	ReasonCode       string            `json:"reason_code,omitempty"`
	Reason           string            `json:"reason,omitempty"`
	Taxes            []TaxBreakdown    `json:"taxes,omitempty"`
	Payment          *PaymentSummary   `json:"payment,omitempty"`
	RelatedDocuments []RelatedDocument `json:"related_documents,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
	Notes            []string          `json:"notes,omitempty"`
	ResponseCode     string            `json:"response_code,omitempty"`
	ResponseStatus   string            `json:"response_status,omitempty"`
	ResponseMessage  string            `json:"response_message,omitempty"`
}

type Party struct {
	RUC     string `json:"ruc"`
	Name    string `json:"name"`
	Address string `json:"address,omitempty"`
}

type Line struct {
	Number           string         `json:"number"`
	Code             string         `json:"code,omitempty"`
	Description      string         `json:"description"`
	Quantity         string         `json:"quantity"`
	UnitCode         string         `json:"unit_code,omitempty"`
	UnitPrice        string         `json:"unit_price"`
	TaxAmount        string         `json:"tax_amount,omitempty"`
	Amount           string         `json:"amount"`
	AdjustmentAmount string         `json:"adjustment_amount,omitempty"`
	AlternativePrice string         `json:"alternative_price,omitempty"`
	Properties       []ItemProperty `json:"properties,omitempty"`
}

type ItemProperty struct {
	Code  string `json:"code,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value"`
}

type TaxBreakdown struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	TaxableAmount string `json:"taxable_amount,omitempty"`
	TaxAmount     string `json:"tax_amount,omitempty"`
}

type PaymentSummary struct {
	Mode              string        `json:"mode,omitempty"`
	OutstandingAmount string        `json:"outstanding_amount,omitempty"`
	Installments      []Installment `json:"installments,omitempty"`
	DetractionCode    string        `json:"detraction_code,omitempty"`
	DetractionPercent string        `json:"detraction_percent,omitempty"`
	DetractionAmount  string        `json:"detraction_amount,omitempty"`
	DetractionAccount string        `json:"detraction_account,omitempty"`
}

type Installment struct {
	Number  string `json:"number"`
	Amount  string `json:"amount"`
	DueDate string `json:"due_date,omitempty"`
}

type RelatedDocument struct {
	Number           string `json:"number"`
	DocumentTypeCode string `json:"document_type_code,omitempty"`
	DocumentType     string `json:"document_type,omitempty"`
	IssueDate        string `json:"issue_date,omitempty"`
	InvoiceAmount    string `json:"invoice_amount,omitempty"`
	PaymentID        string `json:"payment_id,omitempty"`
	PaidAmount       string `json:"paid_amount,omitempty"`
	Rate             string `json:"rate,omitempty"`
	AdjustmentAmount string `json:"adjustment_amount,omitempty"`
	NetAmount        string `json:"net_amount,omitempty"`
	AdjustmentDate   string `json:"adjustment_date,omitempty"`
	Currency         string `json:"currency,omitempty"`
}

// FinancialSummary concilia los documentos autorizados 30 y 42, cuyos
// totales representan una liquidación y no el total de una factura común.
type FinancialSummary struct {
	GrossSettlement string `json:"gross_settlement"`
	RegistryBase    string `json:"registry_base"`
	Tax             string `json:"tax"`
	RegistryTotal   string `json:"registry_total"`
	NetSettlement   string `json:"net_settlement"`
	Reconciles      bool   `json:"reconciles"`
}

type Totals struct {
	LineExtension string `json:"line_extension,omitempty"`
	TaxExclusive  string `json:"tax_exclusive,omitempty"`
	TaxInclusive  string `json:"tax_inclusive,omitempty"`
	Allowance     string `json:"allowance,omitempty"`
	Charge        string `json:"charge,omitempty"`
	Prepaid       string `json:"prepaid,omitempty"`
	Rounding      string `json:"rounding,omitempty"`
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
	Sender               cdrParty           `xml:"SenderParty"`
	Receiver             cdrParty           `xml:"ReceiverParty"`
	DocumentResponses    []documentResponse `xml:"DocumentResponse"`
	PaymentTerms         []paymentTerm      `xml:"PaymentTerms"`
	PaymentMeans         []paymentMeans     `xml:"PaymentMeans"`
}

type cdrParty struct {
	Identifications []struct {
		ID string `xml:"ID"`
	} `xml:"PartyIdentification"`
	Names []struct {
		Name string `xml:"Name"`
	} `xml:"PartyName"`
	LegalEntities []struct {
		RegistrationName string `xml:"RegistrationName"`
	} `xml:"PartyLegalEntity"`
	TaxSchemes []struct {
		RegistrationName string `xml:"RegistrationName"`
		CompanyID        string `xml:"CompanyID"`
	} `xml:"PartyTaxScheme"`
	Party party `xml:"Party"`
}

type documentResponse struct {
	Response struct {
		ReferenceID  string   `xml:"ReferenceID"`
		ResponseCode string   `xml:"ResponseCode"`
		Description  string   `xml:"Description"`
		Notes        []string `xml:"Note"`
	} `xml:"Response"`
	DocumentReference struct {
		ID               string `xml:"ID"`
		DocumentTypeCode string `xml:"DocumentTypeCode"`
	} `xml:"DocumentReference"`
	Recipient cdrParty `xml:"RecipientParty"`
}

type codeValue struct {
	Value         string `xml:",chardata"`
	OperationType string `xml:"listID,attr"`
	SchemeID      string `xml:"schemeID,attr"`
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
	AddressLines []struct {
		Line string `xml:"Line"`
	} `xml:"AddressLine"`
}

type quantity struct {
	Value    string `xml:",chardata"`
	UnitCode string `xml:"unitCode,attr"`
}

type amount struct {
	Value      string `xml:",chardata"`
	CurrencyID string `xml:"currencyID,attr"`
}

type ublLine struct {
	ID               string            `xml:"ID"`
	InvoicedQuantity quantity          `xml:"InvoicedQuantity"`
	CreditedQuantity quantity          `xml:"CreditedQuantity"`
	DebitedQuantity  quantity          `xml:"DebitedQuantity"`
	LineExtension    amount            `xml:"LineExtensionAmount"`
	TaxTotals        []taxTotal        `xml:"TaxTotal"`
	Item             item              `xml:"Item"`
	SubInvoiceLines  []subInvoiceLine  `xml:"SubInvoiceLine"`
	AllowanceCharges []allowanceCharge `xml:"AllowanceCharge"`
	Price            struct {
		PriceAmount amount `xml:"PriceAmount"`
	} `xml:"Price"`
	PricingReference struct {
		AlternativePrices []struct {
			PriceAmount   amount `xml:"PriceAmount"`
			PriceTypeCode string `xml:"PriceTypeCode"`
		} `xml:"AlternativeConditionPrice"`
	} `xml:"PricingReference"`
	ItemPriceExtension struct {
		Amount amount `xml:"Amount"`
	} `xml:"ItemPriceExtension"`
}

type subInvoiceLine struct {
	Item item `xml:"Item"`
}

type allowanceCharge struct {
	ID              string `xml:"ID"`
	Reason          string `xml:"AllowanceChargeReason"`
	ChargeIndicator string `xml:"ChargeIndicator"`
	Amount          amount `xml:"Amount"`
}

type item struct {
	Descriptions []string `xml:"Description"`
	Name         string   `xml:"Name"`
	SellerID     struct {
		ID string `xml:"ID"`
	} `xml:"SellersItemIdentification"`
	AdditionalProperties []struct {
		NameCode string `xml:"NameCode"`
		Name     string `xml:"Name"`
		Value    string `xml:"Value"`
	} `xml:"AdditionalItemProperty"`
}

type taxTotal struct {
	TaxAmount amount        `xml:"TaxAmount"`
	Subtotals []taxSubtotal `xml:"TaxSubtotal"`
}

type taxSubtotal struct {
	TaxableAmount amount `xml:"TaxableAmount"`
	TaxAmount     amount `xml:"TaxAmount"`
	TaxCategory   struct {
		TaxScheme struct {
			ID   string `xml:"ID"`
			Name string `xml:"Name"`
		} `xml:"TaxScheme"`
	} `xml:"TaxCategory"`
}

type monetaryTotal struct {
	LineExtension amount `xml:"LineExtensionAmount"`
	TaxExclusive  amount `xml:"TaxExclusiveAmount"`
	TaxInclusive  amount `xml:"TaxInclusiveAmount"`
	Allowance     amount `xml:"AllowanceTotalAmount"`
	Charge        amount `xml:"ChargeTotalAmount"`
	Prepaid       amount `xml:"PrepaidAmount"`
	Rounding      amount `xml:"PayableRoundingAmount"`
	Payable       amount `xml:"PayableAmount"`
}

type billingReference struct {
	InvoiceDocumentReference struct {
		ID               string `xml:"ID"`
		IssueDate        string `xml:"IssueDate"`
		DocumentTypeCode string `xml:"DocumentTypeCode"`
	} `xml:"InvoiceDocumentReference"`
	DocumentReference struct {
		ID               string `xml:"ID"`
		IssueDate        string `xml:"IssueDate"`
		DocumentTypeCode string `xml:"DocumentTypeCode"`
	} `xml:"DocumentReference"`
}

type discrepancy struct {
	ReferenceID  string `xml:"ReferenceID"`
	ResponseCode string `xml:"ResponseCode"`
	Description  string `xml:"Description"`
}

type paymentTerm struct {
	ID             string `xml:"ID"`
	PaymentMeansID string `xml:"PaymentMeansID"`
	Amount         amount `xml:"Amount"`
	PaymentPercent string `xml:"PaymentPercent"`
	PaymentDueDate string `xml:"PaymentDueDate"`
}

type paymentMeans struct {
	ID               string `xml:"ID"`
	PaymentMeansCode string `xml:"PaymentMeansCode"`
	PayeeAccount     struct {
		ID string `xml:"ID"`
	} `xml:"PayeeFinancialAccount"`
}

type withholdingDocument struct {
	XMLName              xml.Name
	ID                   string                 `xml:"ID"`
	IssueDate            string                 `xml:"IssueDate"`
	Agent                party                  `xml:"AgentParty"`
	Receiver             party                  `xml:"ReceiverParty"`
	RetentionPercent     string                 `xml:"SUNATRetentionPercent"`
	PerceptionPercent    string                 `xml:"SUNATPerceptionPercent"`
	TotalPaid            amount                 `xml:"SUNATTotalPaid"`
	TotalRetention       amount                 `xml:"SUNATTotalRetention"`
	TotalCashed          amount                 `xml:"SUNATTotalCashed"`
	TotalPerception      amount                 `xml:"SUNATTotalPerception"`
	RetentionReferences  []withholdingReference `xml:"SUNATRetentionDocumentReference"`
	PerceptionReferences []withholdingReference `xml:"SUNATPerceptionDocumentReference"`
}

type withholdingReference struct {
	ID                 codeValue `xml:"ID"`
	IssueDate          string    `xml:"IssueDate"`
	TotalInvoiceAmount amount    `xml:"TotalInvoiceAmount"`
	Payment            struct {
		ID         string `xml:"ID"`
		PaidAmount amount `xml:"PaidAmount"`
	} `xml:"Payment"`
	RetentionInformation struct {
		Amount amount `xml:"SUNATRetentionAmount"`
		Net    amount `xml:"SUNATNetTotalPaid"`
		Date   string `xml:"SUNATRetentionDate"`
	} `xml:"SUNATRetentionInformation"`
	PerceptionInformation struct {
		Amount amount `xml:"SUNATPerceptionAmount"`
		Net    amount `xml:"SUNATNetTotalCashed"`
		Date   string `xml:"SUNATPerceptionDate"`
	} `xml:"SUNATPerceptionInformation"`
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
	if document.XMLName.Local == "ApplicationResponse" {
		return applicationResponsePreview(document), nil
	}
	if document.XMLName.Local == "Retention" || document.XMLName.Local == "Perception" {
		return withholdingPreview(data, document.XMLName.Local)
	}

	typeCode := documentTypeCode(document.XMLName.Local, document.InvoiceTypeCode.Value)
	operationType := strings.TrimSpace(document.InvoiceTypeCode.OperationType)
	lines := firstLines(document)
	resultLines := make([]Line, 0, len(lines))
	for _, source := range lines {
		qty := firstQuantity(source.InvoicedQuantity, source.CreditedQuantity, source.DebitedQuantity)
		descriptions := cleanStrings(source.Item.Descriptions)
		for _, subLine := range source.SubInvoiceLines {
			descriptions = appendUniqueStrings(descriptions, subLine.Item.Descriptions...)
			if subLine.Item.Name != "" {
				descriptions = appendUniqueStrings(descriptions, subLine.Item.Name)
			}
		}
		for _, allowance := range source.AllowanceCharges {
			descriptions = appendUniqueStrings(descriptions, allowance.ID, allowance.Reason)
		}
		description := strings.Join(descriptions, " · ")
		if description == "" {
			description = strings.TrimSpace(source.Item.Name)
		}
		alternativePrice := firstAlternativePrice(source)
		unitPrice := strings.TrimSpace(source.Price.PriceAmount.Value)
		if typeCode == "30" || typeCode == "42" {
			unitPrice = firstNonEmpty(source.ItemPriceExtension.Amount.Value, unitPrice, alternativePrice, source.LineExtension.Value)
		} else {
			unitPrice = firstNonEmpty(unitPrice, alternativePrice, source.ItemPriceExtension.Amount.Value, source.LineExtension.Value)
		}
		resultLines = append(resultLines, Line{
			Number:           strings.TrimSpace(source.ID),
			Code:             strings.TrimSpace(source.Item.SellerID.ID),
			Description:      description,
			Quantity:         strings.TrimSpace(qty.Value),
			UnitCode:         strings.TrimSpace(qty.UnitCode),
			UnitPrice:        unitPrice,
			TaxAmount:        totalTaxAmount(source.TaxTotals),
			Amount:           strings.TrimSpace(source.LineExtension.Value),
			AdjustmentAmount: financialLineAdjustment(source, unitPrice),
			AlternativePrice: alternativePrice,
			Properties:       itemProperties(source.Item),
		})
	}

	total := document.LegalTotal
	if total.Payable.Value == "" {
		total = document.RequestedTotal
	}
	currency := strings.TrimSpace(document.DocumentCurrencyCode)
	if currency == "" {
		currency = firstNonEmpty(
			total.Payable.CurrencyID,
			total.LineExtension.CurrencyID,
			firstLineCurrency(lines),
		)
	}
	reference := ""
	referenceType := ""
	referenceDate := ""
	if len(document.BillingReferences) > 0 {
		invoiceReference := document.BillingReferences[0].InvoiceDocumentReference
		genericReference := document.BillingReferences[0].DocumentReference
		reference = firstNonEmpty(invoiceReference.ID, genericReference.ID)
		referenceType = firstNonEmpty(invoiceReference.DocumentTypeCode, genericReference.DocumentTypeCode)
		referenceDate = firstNonEmpty(invoiceReference.IssueDate, genericReference.IssueDate)
	}
	reason := ""
	reasonCode := ""
	if len(document.Discrepancies) > 0 {
		if reference == "" {
			reference = strings.TrimSpace(document.Discrepancies[0].ReferenceID)
		}
		reason = strings.TrimSpace(document.Discrepancies[0].Description)
		reasonCode = strings.TrimSpace(document.Discrepancies[0].ResponseCode)
	}
	reason = noteReason(document.XMLName.Local, reasonCode, reason)
	taxes := taxBreakdown(document.TaxTotals)
	warnings := monetaryWarnings(typeCode, total, document.TaxTotals)

	return Preview{
		Kind:             "document",
		Family:           documentFamily(document.XMLName.Local, typeCode),
		DocumentType:     documentType(document.XMLName.Local, typeCode),
		DocumentTypeCode: typeCode,
		Number:           strings.TrimSpace(document.ID),
		IssueDate:        strings.TrimSpace(document.IssueDate),
		IssueTime:        strings.TrimSpace(document.IssueTime),
		DueDate:          strings.TrimSpace(document.DueDate),
		Currency:         currency,
		OperationType:    strings.TrimSpace(operationType),
		Supplier:         previewParty(document.Supplier.Party),
		Customer:         previewParty(document.Customer.Party),
		Lines:            resultLines,
		Financial:        financialSummary(typeCode, total, resultLines),
		Taxes:            taxes,
		Payment:          paymentSummary(document.PaymentTerms, document.PaymentMeans),
		Warnings:         warnings,
		Totals: Totals{
			LineExtension: strings.TrimSpace(total.LineExtension.Value),
			TaxExclusive:  strings.TrimSpace(total.TaxExclusive.Value),
			TaxInclusive:  strings.TrimSpace(total.TaxInclusive.Value),
			Allowance:     strings.TrimSpace(total.Allowance.Value),
			Charge:        strings.TrimSpace(total.Charge.Value),
			Prepaid:       strings.TrimSpace(total.Prepaid.Value),
			Rounding:      strings.TrimSpace(total.Rounding.Value),
			Tax:           totalTaxAmount(document.TaxTotals),
			Payable:       strings.TrimSpace(total.Payable.Value),
		},
		Reference:     reference,
		ReferenceType: referenceType,
		ReferenceDate: referenceDate,
		ReasonCode:    reasonCode,
		Reason:        reason,
		Notes:         cleanStrings(document.Notes),
	}, nil
}

func applicationResponsePreview(document ublDocument) Preview {
	response := documentResponse{}
	if len(document.DocumentResponses) > 0 {
		response = document.DocumentResponses[0]
	}
	reference := strings.TrimSpace(response.DocumentReference.ID)
	if reference == "" {
		reference = strings.TrimSpace(response.Response.ReferenceID)
	}
	receiver := previewCDRParty(document.Receiver)
	if receiver.RUC == "" && receiver.Name == "" {
		receiver = previewCDRParty(response.Recipient)
	}
	code := strings.TrimSpace(response.Response.ResponseCode)
	message := strings.TrimSpace(response.Response.Description)

	notes := cleanStrings(append(document.Notes, response.Response.Notes...))
	return Preview{
		Kind:             "cdr",
		Family:           "cdr",
		DocumentType:     "Constancia de recepción SUNAT",
		DocumentTypeCode: strings.TrimSpace(response.DocumentReference.DocumentTypeCode),
		Number:           strings.TrimSpace(document.ID),
		IssueDate:        strings.TrimSpace(document.IssueDate),
		IssueTime:        strings.TrimSpace(document.IssueTime),
		Supplier:         previewCDRParty(document.Sender),
		Customer:         receiver,
		Reference:        reference,
		Reason:           message,
		Notes:            notes,
		ResponseCode:     code,
		ResponseStatus:   cdrResponseStatus(code, notes),
		ResponseMessage:  message,
	}
}

func previewCDRParty(source cdrParty) Party {
	if source.Party.TaxSchemes != nil || source.Party.Identifications != nil ||
		source.Party.LegalEntities != nil || source.Party.Names != nil {
		return previewParty(source.Party)
	}
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
	return result
}

func cdrResponseStatus(code string, notes []string) string {
	code = strings.TrimSpace(code)
	switch {
	case code == "0":
		if len(notes) > 0 {
			return "ACEPTADO CON OBSERVACIONES"
		}
		return "ACEPTADO"
	case strings.HasPrefix(code, "2"), strings.HasPrefix(code, "3"):
		return "RECHAZADO"
	case strings.HasPrefix(code, "4"):
		return "OBSERVADO"
	case code != "":
		return "PROCESADO"
	default:
		return "SIN ESTADO"
	}
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

// NormalizeSource devuelve el XML en UTF-8 para mostrarlo de forma legible.
func NormalizeSource(data []byte) (string, error) {
	normalized, err := normalizeEncoding(data)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
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
		firstNonEmpty(addr.StreetName, firstAddressLine(addr)), addr.CitySubdivision, addr.District, addr.CityName, addr.CountrySubentity, addr.Country.Name,
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

func totalTaxAmount(totals []taxTotal) string {
	total := 0.0
	count := 0
	for _, item := range totals {
		value, ok := parseAmount(item.TaxAmount.Value)
		if ok {
			total += value
			count++
		}
	}
	if count == 0 {
		return ""
	}
	return formatAmount(total)
}

func taxBreakdown(totals []taxTotal) []TaxBreakdown {
	type aggregate struct {
		name    string
		taxable float64
		tax     float64
	}
	order := make([]string, 0)
	values := make(map[string]aggregate)
	for _, total := range totals {
		for _, subtotal := range total.Subtotals {
			code := strings.TrimSpace(subtotal.TaxCategory.TaxScheme.ID)
			if code == "" {
				continue
			}
			value := values[code]
			if _, exists := values[code]; !exists {
				order = append(order, code)
			}
			value.name = taxName(code, subtotal.TaxCategory.TaxScheme.Name)
			taxable, _ := parseAmount(subtotal.TaxableAmount.Value)
			tax, _ := parseAmount(subtotal.TaxAmount.Value)
			value.taxable += taxable
			value.tax += tax
			values[code] = value
		}
	}
	result := make([]TaxBreakdown, 0, len(order))
	for _, code := range order {
		value := values[code]
		result = append(result, TaxBreakdown{
			Code:          code,
			Name:          value.name,
			TaxableAmount: formatAmount(value.taxable),
			TaxAmount:     formatAmount(value.tax),
		})
	}
	return result
}

func taxName(code, fallback string) string {
	switch strings.TrimSpace(code) {
	case "1000", "1016":
		return "IGV"
	case "2000":
		return "ISC"
	case "7152":
		return "ICBPER"
	case "9995":
		return "Operación de exportación"
	case "9996":
		return "Operación gratuita"
	case "9997":
		return "Operación exonerada"
	case "9998":
		return "Operación inafecta"
	}
	return firstNonEmpty(fallback, "Otro tributo")
}

func documentTypeCode(root, invoiceCode string) string {
	switch root {
	case "CreditNote":
		return "07"
	case "DebitNote":
		return "08"
	case "Retention":
		return "20"
	case "Perception":
		return "40"
	default:
		return normalizeDocumentCode(invoiceCode)
	}
}

func normalizeDocumentCode(code string) string {
	code = strings.TrimSpace(code)
	if len(code) == 1 && code[0] >= '0' && code[0] <= '9' {
		return "0" + code
	}
	return code
}

func documentFamily(root, code string) string {
	switch root {
	case "CreditNote", "DebitNote":
		return "note"
	}
	if code == "30" || code == "42" {
		return "financial"
	}
	return "invoice"
}

func noteReason(root, code, declared string) string {
	if strings.TrimSpace(declared) != "" {
		return strings.TrimSpace(declared)
	}
	code = normalizeDocumentCode(code)
	if root == "CreditNote" {
		reasons := map[string]string{
			"01": "Anulación de la operación",
			"02": "Anulación por error en el RUC",
			"03": "Corrección por error en la descripción",
			"04": "Descuento global",
			"05": "Descuento por ítem",
			"06": "Devolución total",
			"07": "Devolución por ítem",
			"08": "Bonificación",
			"09": "Disminución en el valor",
			"10": "Otros conceptos",
			"11": "Ajustes de operaciones de exportación",
		}
		return reasons[code]
	}
	if root == "DebitNote" {
		reasons := map[string]string{
			"01": "Intereses por mora",
			"02": "Aumento en el valor",
			"03": "Penalidades u otros conceptos",
			"10": "Ajustes de operaciones de exportación",
		}
		return reasons[code]
	}
	return ""
}

func documentType(root, code string) string {
	switch strings.TrimSpace(code) {
	case "01":
		return "Factura electrónica"
	case "02":
		return "Recibo por honorarios"
	case "03":
		return "Boleta de venta electrónica"
	case "04":
		return "Liquidación de compra"
	case "06":
		return "Carta de porte aéreo"
	case "07":
		return "Nota de crédito electrónica"
	case "08":
		return "Nota de débito electrónica"
	case "09":
		return "Guía de remisión remitente"
	case "12":
		return "Ticket de máquina registradora"
	case "13":
		return "Documento de entidad financiera o de seguros"
	case "14":
		return "Recibo de servicios públicos"
	case "15":
		return "Boleto de transporte urbano o ferroviario"
	case "16":
		return "Boleto de viaje interprovincial"
	case "18":
		return "Documento emitido por AFP"
	case "20":
		return "Comprobante de retención"
	case "21":
		return "Conocimiento de embarque"
	case "23":
		return "Póliza de adjudicación"
	case "24":
		return "Certificado de pago de regalías"
	case "30":
		return "Documento de adquirente del sistema de pago"
	case "31":
		return "Guía de remisión transportista"
	case "42":
		return "Documento de empresa del sistema financiero"
	case "40":
		return "Comprobante de percepción"
	case "41":
		return "Comprobante de percepción de venta interna"
	case "34":
		return "Documento del operador"
	case "37":
		return "Documento de revisión técnica"
	case "43":
		return "Boleto de transporte aéreo no regular"
	case "45":
		return "Documento de entidad educativa o cultural"
	case "50":
		return "Declaración aduanera o liquidación de cobranza"
	case "52":
		return "Despacho simplificado"
	case "53":
		return "Declaración de mensajería o courier"
	case "56":
		return "Comprobante de pago SEAE"
	case "91":
		return "Comprobante de no domiciliado"
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

func firstAlternativePrice(line ublLine) string {
	for _, candidate := range line.PricingReference.AlternativePrices {
		if value := strings.TrimSpace(candidate.PriceAmount.Value); value != "" {
			return value
		}
	}
	return ""
}

func itemProperties(source item) []ItemProperty {
	result := make([]ItemProperty, 0, len(source.AdditionalProperties))
	for _, property := range source.AdditionalProperties {
		value := strings.TrimSpace(property.Value)
		if value == "" {
			continue
		}
		result = append(result, ItemProperty{
			Code:  strings.TrimSpace(property.NameCode),
			Name:  strings.TrimSpace(property.Name),
			Value: value,
		})
	}
	return result
}

func firstAddressLine(source address) string {
	for _, addressLine := range source.AddressLines {
		if value := strings.TrimSpace(addressLine.Line); value != "" {
			return value
		}
	}
	return ""
}

func paymentSummary(terms []paymentTerm, means []paymentMeans) *PaymentSummary {
	result := &PaymentSummary{}
	for _, term := range terms {
		id := strings.TrimSpace(term.ID)
		method := strings.TrimSpace(term.PaymentMeansID)
		switch {
		case strings.EqualFold(id, "Detraccion"):
			result.DetractionCode = method
			result.DetractionPercent = strings.TrimSpace(term.PaymentPercent)
			result.DetractionAmount = strings.TrimSpace(term.Amount.Value)
		case strings.HasPrefix(strings.ToLower(method), "cuota"):
			result.Installments = append(result.Installments, Installment{
				Number:  method,
				Amount:  strings.TrimSpace(term.Amount.Value),
				DueDate: strings.TrimSpace(term.PaymentDueDate),
			})
		case strings.EqualFold(id, "FormaPago"):
			if strings.EqualFold(method, "Credito") {
				result.Mode = "Crédito"
				result.OutstandingAmount = strings.TrimSpace(term.Amount.Value)
			} else if strings.EqualFold(method, "Contado") {
				result.Mode = "Contado"
			}
		}
	}
	for _, item := range means {
		if strings.EqualFold(strings.TrimSpace(item.ID), "Detraccion") {
			if result.DetractionCode == "" {
				result.DetractionCode = strings.TrimSpace(item.PaymentMeansCode)
			}
			result.DetractionAccount = strings.TrimSpace(item.PayeeAccount.ID)
		}
	}
	if result.Mode == "" && len(result.Installments) > 0 {
		result.Mode = "Crédito"
	}
	if result.Mode == "" && result.DetractionCode == "" && len(result.Installments) == 0 {
		return nil
	}
	return result
}

func monetaryWarnings(typeCode string, total monetaryTotal, taxes []taxTotal) []string {
	if typeCode == "30" || typeCode == "42" {
		return nil
	}
	warnings := make([]string, 0, 2)
	line, lineOK := parseAmount(total.LineExtension.Value)
	tax, taxOK := parseAmount(totalTaxAmount(taxes))
	allowance, _ := parseAmount(total.Allowance.Value)
	charge, _ := parseAmount(total.Charge.Value)
	taxInclusive, inclusiveOK := parseAmount(total.TaxInclusive.Value)
	if lineOK && taxOK && inclusiveOK {
		expected := line - allowance + charge + tax
		if math.Abs(expected-taxInclusive) > 0.011 {
			warnings = append(warnings, fmt.Sprintf(
				"El precio de venta declarado (%s) no concilia con valor de venta, descuentos, cargos e impuestos (%s).",
				formatAmount(taxInclusive), formatAmount(expected),
			))
		}
	}
	payable, payableOK := parseAmount(total.Payable.Value)
	prepaid, _ := parseAmount(total.Prepaid.Value)
	rounding, _ := parseAmount(total.Rounding.Value)
	base := taxInclusive
	baseOK := inclusiveOK
	if !baseOK && lineOK && taxOK {
		base = line - allowance + charge + tax
		baseOK = true
	}
	if baseOK && payableOK {
		expected := base - prepaid + rounding
		if math.Abs(expected-payable) > 0.011 {
			warnings = append(warnings, fmt.Sprintf(
				"El monto pagadero declarado (%s) difiere del precio de venta menos anticipos más redondeo (%s).",
				formatAmount(payable), formatAmount(expected),
			))
		}
	}
	return warnings
}

func withholdingPreview(data []byte, root string) (Preview, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel
	var document withholdingDocument
	if err := decoder.Decode(&document); err != nil {
		return Preview{}, fmt.Errorf("interpretando comprobante de retención o percepción: %w", err)
	}
	isRetention := root == "Retention"
	typeCode := "40"
	typeName := "Comprobante de percepción"
	rate := strings.TrimSpace(document.PerceptionPercent)
	references := document.PerceptionReferences
	totalOperation := document.TotalCashed
	totalAdjustment := document.TotalPerception
	if isRetention {
		typeCode = "20"
		typeName = "Comprobante de retención"
		rate = strings.TrimSpace(document.RetentionPercent)
		references = document.RetentionReferences
		totalOperation = document.TotalPaid
		totalAdjustment = document.TotalRetention
	}

	related := make([]RelatedDocument, 0, len(references))
	currency := firstNonEmpty(totalOperation.CurrencyID, totalAdjustment.CurrencyID)
	for _, reference := range references {
		adjustment := reference.PerceptionInformation.Amount
		net := reference.PerceptionInformation.Net
		date := reference.PerceptionInformation.Date
		if isRetention {
			adjustment = reference.RetentionInformation.Amount
			net = reference.RetentionInformation.Net
			date = reference.RetentionInformation.Date
		}
		documentCode := normalizeDocumentCode(reference.ID.SchemeID)
		related = append(related, RelatedDocument{
			Number:           strings.TrimSpace(reference.ID.Value),
			DocumentTypeCode: documentCode,
			DocumentType:     documentType("Invoice", documentCode),
			IssueDate:        strings.TrimSpace(reference.IssueDate),
			InvoiceAmount:    strings.TrimSpace(reference.TotalInvoiceAmount.Value),
			PaymentID:        strings.TrimSpace(reference.Payment.ID),
			PaidAmount:       strings.TrimSpace(reference.Payment.PaidAmount.Value),
			Rate:             rate,
			AdjustmentAmount: strings.TrimSpace(adjustment.Value),
			NetAmount:        strings.TrimSpace(net.Value),
			AdjustmentDate:   strings.TrimSpace(date),
			Currency: firstNonEmpty(
				reference.TotalInvoiceAmount.CurrencyID,
				reference.Payment.PaidAmount.CurrencyID,
				adjustment.CurrencyID,
				net.CurrencyID,
			),
		})
		if currency == "" {
			currency = related[len(related)-1].Currency
		}
	}

	return Preview{
		Kind:             "withholding",
		Family:           strings.ToLower(root),
		DocumentType:     typeName,
		DocumentTypeCode: typeCode,
		Number:           strings.TrimSpace(document.ID),
		IssueDate:        strings.TrimSpace(document.IssueDate),
		Currency:         currency,
		Supplier:         previewParty(document.Agent),
		Customer:         previewParty(document.Receiver),
		RelatedDocuments: related,
		Totals: Totals{
			LineExtension: strings.TrimSpace(totalOperation.Value),
			Tax:           strings.TrimSpace(totalAdjustment.Value),
			Payable:       strings.TrimSpace(totalOperation.Value),
		},
	}, nil
}

func financialLineAdjustment(line ublLine, componentValue string) string {
	if len(line.AllowanceCharges) == 0 {
		return ""
	}
	component, componentOK := parseAmount(componentValue)
	initial, initialOK := parseAmount(line.LineExtension.Value)
	tax, _ := parseAmount(totalTaxAmount(line.TaxTotals))
	if componentOK && initialOK {
		return formatAmount(component - initial - tax)
	}
	return lineAdjustmentAmount(line.AllowanceCharges)
}

func lineAdjustmentAmount(adjustments []allowanceCharge) string {
	if len(adjustments) == 0 {
		return ""
	}
	total := 0.0
	valid := false
	for _, adjustment := range adjustments {
		value, ok := parseAmount(adjustment.Amount.Value)
		if !ok {
			continue
		}
		valid = true
		if strings.EqualFold(strings.TrimSpace(adjustment.ChargeIndicator), "false") {
			total -= value
		} else {
			total += value
		}
	}
	if !valid {
		return ""
	}
	return formatAmount(total)
}

func financialSummary(typeCode string, total monetaryTotal, lines []Line) *FinancialSummary {
	typeCode = strings.TrimSpace(typeCode)
	if typeCode != "30" && typeCode != "42" {
		return nil
	}

	gross, grossOK := parseAmount(total.LineExtension.Value)
	net, netOK := parseAmount(total.Payable.Value)
	registryTotal := 0.0
	tax := 0.0
	componentCount := 0
	for _, line := range lines {
		lineTax, _ := parseAmount(line.TaxAmount)
		tax += lineTax

		component, ok := parseAmount(line.UnitPrice)
		if !ok {
			component, ok = parseAmount(line.Amount)
			adjustment, _ := parseAmount(line.AdjustmentAmount)
			component += adjustment + lineTax
		}
		if ok {
			registryTotal += component
			componentCount++
		}
	}

	result := &FinancialSummary{
		GrossSettlement: strings.TrimSpace(total.LineExtension.Value),
		NetSettlement:   strings.TrimSpace(total.Payable.Value),
	}
	if componentCount > 0 {
		result.RegistryBase = formatAmount(registryTotal - tax)
		result.Tax = formatAmount(tax)
		result.RegistryTotal = formatAmount(registryTotal)
	}
	if grossOK && netOK && componentCount > 0 {
		result.Reconciles = math.Abs((gross-net)-registryTotal) < 0.011
	}
	return result
}

func parseAmount(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil
}

func formatAmount(value float64) string {
	if math.Abs(value) < 0.0000001 {
		value = 0
	}
	return fmt.Sprintf("%.2f", value)
}

func firstLineCurrency(lines []ublLine) string {
	for _, line := range lines {
		if currency := firstNonEmpty(
			line.LineExtension.CurrencyID,
			line.Price.PriceAmount.CurrencyID,
			line.ItemPriceExtension.Amount.CurrencyID,
		); currency != "" {
			return currency
		}
	}
	return ""
}

func appendUniqueStrings(values []string, candidates ...string) []string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		found := false
		for _, value := range values {
			if strings.EqualFold(value, candidate) {
				found = true
				break
			}
		}
		if !found {
			values = append(values, candidate)
		}
	}
	return values
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
