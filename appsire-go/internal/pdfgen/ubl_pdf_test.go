package pdfgen

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseUBLExtractsSUNATRepresentationData(t *testing.T) {
	t.Parallel()

	doc, err := parseUBL(testInvoiceXML())
	if err != nil {
		t.Fatalf("parseUBL() error = %v", err)
	}

	if doc.ID != "F001-123" || doc.TypeCode != "01" {
		t.Fatalf("identidad = %q/%q", doc.ID, doc.TypeCode)
	}
	if doc.SupplierID != "20123456789" || doc.CustomerID != "20987654321" {
		t.Fatalf("RUC emisor/cliente = %q/%q", doc.SupplierID, doc.CustomerID)
	}
	if doc.Taxed != 100 || doc.IGV != 18 || doc.Payable != 118 {
		t.Fatalf("totales gravado/IGV/importe = %.2f/%.2f/%.2f", doc.Taxed, doc.IGV, doc.Payable)
	}
	if doc.SignatureDigest != "ABCDEF0123456789=" {
		t.Fatalf("DigestValue = %q", doc.SignatureDigest)
	}
	if len(doc.Lines) != 1 {
		t.Fatalf("líneas = %d, want 1", len(doc.Lines))
	}
	line := doc.Lines[0]
	if line.Unit != "UNIDAD" || line.Code != "SERV-01" || line.Amount != 100 {
		t.Fatalf("línea = %#v", line)
	}
	wantQR := "20123456789|01|F001|123|18.00|118.00|2026-09-09|6|20987654321|"
	if got := qrContent(doc); got != wantQR {
		t.Fatalf("QR = %q, want %q", got, wantQR)
	}
}

func TestFromUBLProducesViewablePDFWithQR(t *testing.T) {
	t.Parallel()

	pdf, err := FromUBL(testInvoiceXML())
	if err != nil {
		t.Fatalf("FromUBL() error = %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("el archivo no empieza con cabecera PDF")
	}
	if !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatalf("el archivo no termina con marcador PDF")
	}
	if !bytes.Contains(pdf, []byte("F001-123")) {
		t.Fatalf("los metadatos PDF no contienen el identificador del comprobante")
	}
	if !bytes.Contains(pdf, []byte("/Subtype /Image")) {
		t.Fatalf("el PDF no contiene la imagen QR")
	}
}

func TestParseUBLRejectsUnsupportedRoot(t *testing.T) {
	t.Parallel()

	_, err := parseUBL([]byte(`<ApplicationResponse><ID>R-1</ID></ApplicationResponse>`))
	if err == nil || !strings.Contains(err.Error(), "Invoice") {
		t.Fatalf("error = %v, want tipo UBL no soportado", err)
	}
}

func testInvoiceXML() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
 xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
 xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
 xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
 <cbc:ID>F001-123</cbc:ID>
 <cbc:IssueDate>2026-09-09</cbc:IssueDate>
 <cbc:DueDate>2026-10-09</cbc:DueDate>
 <cbc:InvoiceTypeCode>01</cbc:InvoiceTypeCode>
 <cbc:DocumentCurrencyCode>PEN</cbc:DocumentCurrencyCode>
 <cbc:Note languageLocaleID="1000">SON: CIENTO DIECIOCHO CON 00/100 SOLES</cbc:Note>
 <cac:AccountingSupplierParty><cac:Party>
  <cac:PartyIdentification><cbc:ID schemeID="6">20123456789</cbc:ID></cac:PartyIdentification>
  <cac:PartyName><cbc:Name>Proveedor Perú</cbc:Name></cac:PartyName>
  <cac:PartyLegalEntity><cbc:RegistrationName>Proveedor Perú S.A.C.</cbc:RegistrationName>
   <cac:RegistrationAddress><cbc:District>Miraflores</cbc:District><cbc:CityName>Lima</cbc:CityName><cbc:CountrySubentity>Lima</cbc:CountrySubentity><cac:AddressLine><cbc:Line>Av. Prueba 123</cbc:Line></cac:AddressLine></cac:RegistrationAddress>
  </cac:PartyLegalEntity>
 </cac:Party></cac:AccountingSupplierParty>
 <cac:AccountingCustomerParty><cac:Party>
  <cac:PartyIdentification><cbc:ID schemeID="6">20987654321</cbc:ID></cac:PartyIdentification>
  <cac:PartyLegalEntity><cbc:RegistrationName>Cliente S.A.</cbc:RegistrationName></cac:PartyLegalEntity>
 </cac:Party></cac:AccountingCustomerParty>
 <cac:TaxTotal><cbc:TaxAmount currencyID="PEN">18.00</cbc:TaxAmount><cac:TaxSubtotal>
  <cbc:TaxableAmount currencyID="PEN">100.00</cbc:TaxableAmount><cbc:TaxAmount currencyID="PEN">18.00</cbc:TaxAmount>
  <cac:TaxCategory><cac:TaxScheme><cbc:ID>1000</cbc:ID></cac:TaxScheme></cac:TaxCategory>
 </cac:TaxSubtotal></cac:TaxTotal>
 <cac:LegalMonetaryTotal><cbc:LineExtensionAmount>100.00</cbc:LineExtensionAmount><cbc:PayableAmount>118.00</cbc:PayableAmount></cac:LegalMonetaryTotal>
 <cac:InvoiceLine><cbc:ID>1</cbc:ID><cbc:InvoicedQuantity unitCode="NIU">2</cbc:InvoicedQuantity><cbc:LineExtensionAmount>100.00</cbc:LineExtensionAmount>
  <cac:Item><cbc:Description>Servicio de consultoría</cbc:Description><cac:SellersItemIdentification><cbc:ID>SERV-01</cbc:ID></cac:SellersItemIdentification></cac:Item>
  <cac:Price><cbc:PriceAmount>50.00</cbc:PriceAmount></cac:Price>
 </cac:InvoiceLine>
 <ds:Signature><ds:SignedInfo><ds:Reference><ds:DigestValue>ABCDEF0123456789=</ds:DigestValue></ds:Reference></ds:SignedInfo></ds:Signature>
</Invoice>`)
}
