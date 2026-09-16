package sirepreview

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"appsire-go/internal/sunat"
)

func TestFromZIPParsesPipeDelimitedProposal(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, err := writer.Create("LE_TEST_RCE.txt")
	if err != nil {
		t.Fatal(err)
	}
	first := make([]string, 41)
	first[0] = "20600000001"
	first[4], first[6], first[7], first[9] = "01/08/2026", "01", "F001", "1045"
	first[12], first[13], first[24], first[25] = "20100000001", "PROVEEDOR UNO", "118.00", "PEN"
	first[14], first[15] = "100.00", "18.00"
	first[21] = "0.00"
	second := make([]string, 41)
	second[0] = "20600000001"
	second[4], second[6], second[7], second[9] = "02/08/2026", "03", "B001", "99"
	second[12], second[13], second[24], second[25] = "20100000002", "PROVEEDOR DOS", "59.00", "PEN"
	_, _ = file.Write([]byte(strings.Join(first, "|") + "\n" + strings.Join(second, "|") + "\n"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	preview, err := FromZIP(archive.Bytes(), sunat.ProposalRCE, "202608")
	if err != nil {
		t.Fatal(err)
	}
	if preview.TotalRows != 2 || len(preview.Rows) != 2 {
		t.Fatalf("filas = %d/%d; se esperaba 2/2", preview.TotalRows, len(preview.Rows))
	}
	if preview.Headers[1] != "Tipo" || preview.Rows[0][2] != "F001" {
		t.Fatalf("vista inesperada: %#v", preview)
	}
	if !contains(preview.Headers, "Base imponible gravada/exportación") || !contains(preview.Headers, "IGV/IPM gravado/exportación") {
		t.Fatalf("faltan conceptos con valores: %#v", preview.Headers)
	}
	if contains(preview.Headers, "ISC") {
		t.Fatalf("una columna vacía no debe mostrarse: %#v", preview.Headers)
	}
	if preview.Comprobantes[0].RUC != "20100000001" || preview.Comprobantes[0].Numero != "1045" {
		t.Fatalf("comprobante inesperado: %#v", preview.Comprobantes[0])
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func TestFromZIPDropsConceptColumnsWithoutValues(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, _ := writer.Create("datos.csv")
	row := make([]string, 40)
	row[0], row[4], row[6], row[7], row[8] = "20600000001", "03/08/2026", "01", "F001", "7"
	_, _ = file.Write([]byte(strings.Join(row, ";") + "\n"))
	_ = writer.Close()

	preview, err := FromZIP(archive.Bytes(), sunat.ProposalRVIE, "202608")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Headers) != 4 || preview.TotalRows != 1 {
		t.Fatalf("vista inesperada: %#v", preview)
	}
	if preview.Comprobantes[0].RUC != "20600000001" {
		t.Fatalf("RUC RVIE inesperado: %#v", preview.Comprobantes[0])
	}
}

func TestFromZIPPrefersDataOverAuxiliaryReport(t *testing.T) {
	t.Parallel()

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	report, err := writer.Create("A_REPORTE.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := report.Write([]byte("REPORTE DE INCONSISTENCIAS|202608\n")); err != nil {
		t.Fatal(err)
	}
	data, err := writer.Create("Z_DATOS.csv")
	if err != nil {
		t.Fatal(err)
	}
	row := make([]string, 41)
	row[0] = "20600000001"
	row[4], row[6], row[7], row[9] = "01/08/2026", "01", "F001", "45"
	row[12], row[13] = "20100000001", "PROVEEDOR"
	if _, err := data.Write([]byte(strings.Join(row, "|") + "\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	preview, err := FromZIP(archive.Bytes(), sunat.ProposalRCE, "202608")
	if err != nil {
		t.Fatal(err)
	}
	if preview.FileName != "Z_DATOS.csv" {
		t.Fatalf("archivo seleccionado = %q, want Z_DATOS.csv", preview.FileName)
	}
	if len(preview.Comprobantes) != 1 || preview.Comprobantes[0].Numero != "45" {
		t.Fatalf("comprobantes inesperados: %#v", preview.Comprobantes)
	}
}
