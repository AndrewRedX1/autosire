package sunat

import (
	"testing"
)

func TestInspectCDR(t *testing.T) {
	sampleCdrXml := `<?xml version="1.0" encoding="UTF-8"?>
<ar:ApplicationResponse xmlns:ar="urn:oasis:names:specification:ubl:schema:xsd:ApplicationResponse-2"
                        xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
                        xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
    <cac:DocumentResponse>
        <cac:Response>
            <cbc:ReferenceID>F001-00000123</cbc:ReferenceID>
            <cbc:ResponseCode>0</cbc:ResponseCode>
            <cbc:Description>La Factura numero F001-00000123 ha sido aceptada</cbc:Description>
        </cac:Response>
    </cac:DocumentResponse>
</ar:ApplicationResponse>`

	info := InspectCDR([]byte(sampleCdrXml))
	if info.Estado != "ACEPTADO" {
		t.Errorf("Se esperaba ACEPTADO, se obtuvo: %s", info.Estado)
	}
	if info.ResponseCode != "0" {
		t.Errorf("Se esperaba código 0, se obtuvo: %s", info.ResponseCode)
	}
	if info.Description != "La Factura numero F001-00000123 ha sido aceptada" {
		t.Errorf("Descripción inesperada: %s", info.Description)
	}
}

func TestExtractDigestValue(t *testing.T) {
	sampleXml := `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
         xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
    <ds:Signature>
        <ds:SignedInfo>
            <ds:DigestValue>n47i1gJ2N/j7L0+U2a4R9rV6w==</ds:DigestValue>
        </ds:SignedInfo>
    </ds:Signature>
</Invoice>`

	digest := ExtractDigestValue([]byte(sampleXml))
	if digest != "n47i1gJ2N/j7L0+U2a4R9rV6w==" {
		t.Errorf("DigestValue inesperado: %s", digest)
	}
}

func TestValidateContentIntegrity(t *testing.T) {
	// 1. Falso 200 con HTML
	htmlError := []byte("<!DOCTYPE html><html><body>Error 500: El servicio no se encuentra disponible</body></html>")
	err := ValidateContentIntegrity(htmlError, DescargaPDF)
	if err == nil {
		t.Errorf("Se esperaba error de integridad ante HTML de error, pero pasó")
	}

	// 2. PDF legítimo
	validPdf := []byte("%PDF-1.4\n%...\n%%EOF")
	if err := ValidateContentIntegrity(validPdf, DescargaPDF); err != nil {
		t.Errorf("Error inesperado en PDF válido: %v", err)
	}
}

func TestIsXMLContentAcceptsBOMEncodings(t *testing.T) {
	t.Parallel()

	tests := [][]byte{
		{0xEF, 0xBB, 0xBF, '<', 'I', 'n', 'v', 'o', 'i', 'c', 'e', '/', '>'},
		{0xFF, 0xFE, '<', 0, 'I', 0, 'n', 0, 'v', 0, 'o', 0, 'i', 0, 'c', 0, 'e', 0, '/', 0, '>', 0},
		{0xFE, 0xFF, 0, '<', 0, 'I', 0, 'n', 0, 'v', 0, 'o', 0, 'i', 0, 'c', 0, 'e', 0, '/', 0, '>'},
	}
	for index, data := range tests {
		if !IsXMLContent(data) {
			t.Fatalf("variante XML %d no reconocida", index)
		}
		if err := ValidateContentIntegrity(data, DescargaXML); err != nil {
			t.Fatalf("ValidateContentIntegrity(%d) error = %v", index, err)
		}
	}
}
