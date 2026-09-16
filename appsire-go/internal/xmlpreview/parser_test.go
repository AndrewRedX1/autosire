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
