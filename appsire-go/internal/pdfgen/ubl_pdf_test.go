package pdfgen

import (
	"bytes"
	"testing"
)

func TestFromUBLProducesViewablePDF(t *testing.T) {
	t.Parallel()

	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
 xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
 xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2">
 <cbc:ID>F001-123</cbc:ID><cbc:IssueDate>2026-09-09</cbc:IssueDate>
 <cbc:DocumentCurrencyCode>PEN</cbc:DocumentCurrencyCode>
 <cac:AccountingSupplierParty><cac:Party><cac:PartyLegalEntity><cbc:RegistrationName>Proveedor SAC</cbc:RegistrationName><cbc:CompanyID>20123456789</cbc:CompanyID></cac:PartyLegalEntity></cac:Party></cac:AccountingSupplierParty>
 <cac:LegalMonetaryTotal><cbc:PayableAmount>118.00</cbc:PayableAmount></cac:LegalMonetaryTotal>
 <cac:InvoiceLine><cbc:ID>1</cbc:ID><cbc:InvoicedQuantity>2</cbc:InvoicedQuantity><cbc:LineExtensionAmount>100.00</cbc:LineExtensionAmount><cac:Item><cbc:Description>Servicio de prueba</cbc:Description></cac:Item></cac:InvoiceLine>
</Invoice>`)

	pdf, err := FromUBL(xmlData)
	if err != nil {
		t.Fatalf("FromUBL() error = %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatalf("el archivo no empieza con cabecera PDF")
	}
	if !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatalf("el archivo no termina con marcador PDF")
	}
	if !bytes.Contains(pdf, []byte("F001-123")) {
		t.Fatalf("el PDF no contiene el identificador del comprobante")
	}
}
