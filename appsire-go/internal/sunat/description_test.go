package sunat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractUBLDescriptionIncludesItemsAndVehiclePlate(t *testing.T) {
	t.Parallel()
	xmlData := []byte(`<?xml version="1.0"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
 xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
 xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
 <cac:InvoiceLine><cac:Item><cbc:Description>Servicio mensual</cbc:Description></cac:Item></cac:InvoiceLine>
 <cac:InvoiceLine><cac:Item><cbc:Description>Transporte</cbc:Description><cac:AdditionalItemProperty><cbc:NameCode>7000</cbc:NameCode><cbc:Value>ABC-123</cbc:Value></cac:AdditionalItemProperty></cac:Item></cac:InvoiceLine>
</Invoice>`)

	got, err := ExtractUBLDescription(xmlData)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "Servicio mensual; Transporte" || got.VehiclePlate != "ABC-123" {
		t.Fatalf("resultado = %#v", got)
	}
}

func TestExtractUBLDescriptionSupportsFinancialDocument42(t *testing.T) {
	t.Parallel()
	xmlData := []byte(`<?xml version="1.0"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
 xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
 xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
 <cac:InvoiceLine><cac:SubInvoiceLine><cac:Item><cbc:Description>BANCO PICHINCHA</cbc:Description></cac:Item></cac:SubInvoiceLine></cac:InvoiceLine>
 <cac:InvoiceLine><cac:AllowanceCharge><cbc:ID>MANEJO DE CUENTA</cbc:ID><cbc:Amount currencyID="USD">2.07</cbc:Amount></cac:AllowanceCharge></cac:InvoiceLine>
</Invoice>`)

	got, err := ExtractUBLDescription(xmlData)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "BANCO PICHINCHA; MANEJO DE CUENTA" {
		t.Fatalf("descripción = %q", got.Description)
	}
}

func TestDownloadDescriptionUsesBookSuffixAndParsesAllItems(t *testing.T) {
	t.Parallel()
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"comprobantes":[{"placaVehicular":"XYZ-987","informacionItems":[{"desItem":"Item uno"},{"desItem":"Item dos"}]}]}`))
	}))
	defer server.Close()

	client := NewSunatClient(staticTokenProvider{}, 0)
	client.baseURL = server.URL
	result, err := client.DownloadDescription(context.Background(), Comprobante{
		RUC: "20111111111", Tipo: "01", Serie: "F001", Numero: "25", Libro: "2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(requestedPath, "/20111111111-01-F001-25-2") {
		t.Fatalf("ruta = %q", requestedPath)
	}
	if result.Description != "Item uno; Item dos" || result.VehiclePlate != "XYZ-987" {
		t.Fatalf("resultado = %#v", result)
	}
}

func TestDownloadDescriptionIncludesConceptAndObservation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"comprobantes":[{"desConcepto":"Honorarios profesionales","desObservacion":"Pago por transferencia"}]}`))
	}))
	defer server.Close()

	client := NewSunatClient(staticTokenProvider{}, time.Second)
	client.baseURL = server.URL
	result, err := client.DownloadDescription(context.Background(), Comprobante{
		RUC: "20111111111", Tipo: "02", Serie: "E001", Numero: "10", Libro: "2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Description != "Honorarios profesionales; Observación: Pago por transferencia" {
		t.Fatalf("descripción = %q", result.Description)
	}
}
