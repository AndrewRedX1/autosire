package xmlpreview

import (
	"strings"
	"testing"

	textunicode "golang.org/x/text/encoding/unicode"
)

func TestParseInvoiceUBL(t *testing.T) {
	t.Parallel()
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
 xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
 xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
 <cbc:ID>F001-123</cbc:ID><cbc:IssueDate>2026-09-15</cbc:IssueDate>
 <cbc:InvoiceTypeCode listID="0101">01</cbc:InvoiceTypeCode><cbc:DocumentCurrencyCode>PEN</cbc:DocumentCurrencyCode>
 <cac:AccountingSupplierParty><cac:Party><cac:PartyTaxScheme><cbc:RegistrationName>EMISOR SAC</cbc:RegistrationName><cbc:CompanyID>20111111111</cbc:CompanyID></cac:PartyTaxScheme></cac:Party></cac:AccountingSupplierParty>
 <cac:AccountingCustomerParty><cac:Party><cac:PartyTaxScheme><cbc:RegistrationName>CLIENTE SAC</cbc:RegistrationName><cbc:CompanyID>20222222222</cbc:CompanyID></cac:PartyTaxScheme></cac:Party></cac:AccountingCustomerParty>
 <cac:InvoiceLine><cbc:ID>1</cbc:ID><cbc:InvoicedQuantity unitCode="NIU">2</cbc:InvoicedQuantity><cbc:LineExtensionAmount>100.00</cbc:LineExtensionAmount><cac:Item><cbc:Description>Servicio contable</cbc:Description></cac:Item><cac:Price><cbc:PriceAmount>50.00</cbc:PriceAmount></cac:Price></cac:InvoiceLine>
 <cac:TaxTotal><cbc:TaxAmount>18.00</cbc:TaxAmount></cac:TaxTotal>
 <cac:LegalMonetaryTotal><cbc:TaxExclusiveAmount>100.00</cbc:TaxExclusiveAmount><cbc:TaxInclusiveAmount>118.00</cbc:TaxInclusiveAmount><cbc:PayableAmount>118.00</cbc:PayableAmount></cac:LegalMonetaryTotal>
</Invoice>`

	preview, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatal(err)
	}
	if preview.DocumentType != "Factura electrónica" || preview.Number != "F001-123" {
		t.Fatalf("cabecera inesperada: %+v", preview)
	}
	if preview.Supplier.RUC != "20111111111" || preview.Customer.Name != "CLIENTE SAC" {
		t.Fatalf("partes inesperadas: %+v / %+v", preview.Supplier, preview.Customer)
	}
	if len(preview.Lines) != 1 || preview.Lines[0].Description != "Servicio contable" {
		t.Fatalf("detalle inesperado: %+v", preview.Lines)
	}
	if preview.Totals.Payable != "118.00" || preview.Totals.Tax != "18.00" {
		t.Fatalf("totales inesperados: %+v", preview.Totals)
	}
}

func TestParseReaderRejectsOversizedXML(t *testing.T) {
	t.Parallel()
	_, err := ParseReader(strings.NewReader("<Invoice><ID>1</ID></Invoice>"), 8)
	if err == nil {
		t.Fatal("ParseReader() error = nil, want límite")
	}
}

func TestParseAcceptsUTF16XML(t *testing.T) {
	t.Parallel()
	source := []byte(`<?xml version="1.0" encoding="UTF-16"?><Invoice><ID>F001-9</ID><InvoiceTypeCode>01</InvoiceTypeCode></Invoice>`)
	encoded, err := textunicode.UTF16(textunicode.LittleEndian, textunicode.UseBOM).NewEncoder().Bytes(source)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Number != "F001-9" {
		t.Fatalf("numero = %q, want F001-9", preview.Number)
	}
}

func TestParseApplicationResponseCDR(t *testing.T) {
	t.Parallel()
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<ApplicationResponse xmlns="urn:oasis:names:specification:ubl:schema:xsd:ApplicationResponse-2"
 xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
 xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
 <cbc:ID>R-20600000001-01-F001-123</cbc:ID>
 <cbc:IssueDate>2026-09-16</cbc:IssueDate><cbc:IssueTime>10:20:30</cbc:IssueTime>
 <cac:SenderParty><cac:PartyIdentification><cbc:ID>20131312955</cbc:ID></cac:PartyIdentification><cac:PartyLegalEntity><cbc:RegistrationName>SUNAT</cbc:RegistrationName></cac:PartyLegalEntity></cac:SenderParty>
 <cac:ReceiverParty><cac:PartyIdentification><cbc:ID>20600000001</cbc:ID></cac:PartyIdentification></cac:ReceiverParty>
 <cac:DocumentResponse>
  <cac:Response><cbc:ReferenceID>F001-123</cbc:ReferenceID><cbc:ResponseCode>0</cbc:ResponseCode><cbc:Description>La factura ha sido aceptada</cbc:Description></cac:Response>
  <cac:DocumentReference><cbc:ID>F001-123</cbc:ID><cbc:DocumentTypeCode>01</cbc:DocumentTypeCode></cac:DocumentReference>
 </cac:DocumentResponse>
</ApplicationResponse>`

	preview, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatal(err)
	}
	if preview.Kind != "cdr" || preview.ResponseStatus != "ACEPTADO" || preview.ResponseCode != "0" {
		t.Fatalf("estado CDR inesperado: %+v", preview)
	}
	if preview.Reference != "F001-123" || preview.Supplier.RUC != "20131312955" || preview.Customer.RUC != "20600000001" {
		t.Fatalf("datos CDR inesperados: %+v", preview)
	}
}

func TestParseFinancialDocument42(t *testing.T) {
	t.Parallel()
	xmlData := []byte(`<?xml version="1.0"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
 xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
 xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
 <cbc:ID>F004-02139635</cbc:ID><cbc:InvoiceTypeCode listID="02">42</cbc:InvoiceTypeCode>
 <cac:LegalMonetaryTotal><cbc:LineExtensionAmount currencyID="USD">25.00</cbc:LineExtensionAmount><cbc:PayableAmount currencyID="USD">20.86</cbc:PayableAmount></cac:LegalMonetaryTotal>
 <cac:InvoiceLine><cbc:ID>2</cbc:ID><cbc:LineExtensionAmount currencyID="USD">0</cbc:LineExtensionAmount><cac:SubInvoiceLine><cac:Item><cbc:Description>BANCO PICHINCHA</cbc:Description></cac:Item></cac:SubInvoiceLine></cac:InvoiceLine>
 <cac:InvoiceLine><cbc:ID>1</cbc:ID><cbc:LineExtensionAmount currencyID="USD">1.75</cbc:LineExtensionAmount><cac:AllowanceCharge><cbc:ID>MANEJO DE CUENTA</cbc:ID><cbc:Amount currencyID="USD">2.07</cbc:Amount></cac:AllowanceCharge><cac:ItemPriceExtension><cbc:Amount currencyID="USD">4.14</cbc:Amount></cac:ItemPriceExtension></cac:InvoiceLine>
</Invoice>`)

	preview, err := Parse(xmlData)
	if err != nil {
		t.Fatal(err)
	}
	if preview.DocumentTypeCode != "42" || preview.Currency != "USD" || preview.Totals.Payable != "20.86" {
		t.Fatalf("cabecera tipo 42 inesperada: %+v", preview)
	}
	if len(preview.Lines) != 2 || preview.Lines[0].Description != "BANCO PICHINCHA" || preview.Lines[1].Description != "MANEJO DE CUENTA" {
		t.Fatalf("líneas tipo 42 inesperadas: %+v", preview.Lines)
	}
}
