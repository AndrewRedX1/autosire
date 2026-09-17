package filemanager

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"appsire-go/internal/sunat"
)

func TestSaveDownloadedFilePublishesOnlyExtractedXML(t *testing.T) {
	t.Parallel()

	manager := NewFileManager(t.TempDir())
	comp := sunat.Comprobante{
		RUC: "20123456789", Tipo: "01", Serie: "F001", Numero: "123", Libro: "2",
	}
	file := &sunat.DownloadedFile{
		FileName:        "20123456789-01-F001-123.xml",
		Content:         []byte("<Invoice/>"),
		OriginalZip:     []byte("PK original"),
		OriginalZipName: "20123456789-01-F001-123.zip",
	}

	xmlPath, err := manager.SaveDownloadedFile(comp, file, sunat.DescargaXML)
	if err != nil {
		t.Fatalf("SaveDownloadedFile() error = %v", err)
	}
	if _, err := os.Stat(xmlPath); err != nil {
		t.Fatalf("XML no guardado: %v", err)
	}
	zipPath := filepath.Join(filepath.Dir(xmlPath), file.OriginalZipName)
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Fatalf("se publicó un ZIP duplicado para XML: %v", err)
	}

	found, ok := manager.FindExistingFile(comp, sunat.DescargaXML)
	if !ok || found != xmlPath {
		t.Fatalf("FindExistingFile() = %q, %t, want %q, true", found, ok, xmlPath)
	}
}

func TestDirectoryIndexUpdatesAfterSave(t *testing.T) {
	t.Parallel()

	manager := NewFileManager(t.TempDir())
	comp := sunat.Comprobante{
		RUC: "20123456789", Tipo: "01", Serie: "F001", Numero: "000123", Libro: "2",
	}
	dir := manager.BuildDirectoryPath(comp)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.FindExistingFile(comp, sunat.DescargaXML); ok {
		t.Fatal("no debe encontrar un archivo antes de guardarlo")
	}

	wantName := "20123456789-01-F001-123.xml"
	wantPath, err := manager.SaveDownloadedFile(comp, &sunat.DownloadedFile{
		FileName: wantName,
		Content:  []byte("<Invoice/ >"),
	}, sunat.DescargaXML)
	if err != nil {
		t.Fatal(err)
	}
	found, ok := manager.FindExistingFile(comp, sunat.DescargaXML)
	if !ok || found != wantPath {
		t.Fatalf("FindExistingFile() = %q, %t, want %q, true", found, ok, wantPath)
	}
}

func TestFindExistingFileDoesNotConfuseSimilarNumbers(t *testing.T) {
	t.Parallel()

	manager := NewFileManager(t.TempDir())
	comp := sunat.Comprobante{
		RUC: "20123456789", Tipo: "01", Serie: "F001", Numero: "12", Libro: "2",
	}
	dir := manager.BuildDirectoryPath(comp)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	wrong := filepath.Join(dir, "20123456789-01-F001-123.xml")
	if err := os.WriteFile(wrong, []byte("<Invoice/>"), 0644); err != nil {
		t.Fatal(err)
	}

	if found, ok := manager.FindExistingFile(comp, sunat.DescargaXML); ok {
		t.Fatalf("FindExistingFile() encontro numero distinto: %q", found)
	}
}

func TestBuildDirectoryPathUsesBatchOwnerInsteadOfSupplier(t *testing.T) {
	t.Parallel()

	manager := NewFileManager(t.TempDir())
	first := sunat.Comprobante{
		RUC:          "20111111111",
		RazonSocial:  "PROVEEDOR UNO",
		EmpresaRUC:   "20600000001",
		EmpresaRazon: "EMPRESA ACTIVA",
		Libro:        "2",
		Periodo:      "202608",
	}
	second := first
	second.RUC = "20222222222"
	second.RazonSocial = "PROVEEDOR DOS"

	firstPath := manager.BuildDirectoryPath(first)
	secondPath := manager.BuildDirectoryPath(second)
	if firstPath != secondPath {
		t.Fatalf("proveedores del mismo lote usan carpetas distintas: %q != %q", firstPath, secondPath)
	}
	wantSuffix := filepath.Join("CPE", "20600000001 EMPRESA ACTIVA", "Compras", "202608")
	if !strings.HasSuffix(firstPath, wantSuffix) {
		t.Fatalf("ruta = %q, want suffix %q", firstPath, wantSuffix)
	}
}

func TestFindExistingResultsRestoresProposalArtifactsAfterRestart(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	ownerRUC := "20600000001"
	savedComp := sunat.Comprobante{
		RUC:          "20123456789",
		Tipo:         "01",
		Serie:        "F001",
		Numero:       "000123",
		Libro:        "2",
		Periodo:      "202608",
		EmpresaRUC:   ownerRUC,
		EmpresaRazon: "RAZON SOCIAL ANTERIOR",
	}
	writer := NewFileManager(baseDir)
	files := []struct {
		tipo    sunat.TipoDescarga
		name    string
		content []byte
	}{
		{sunat.DescargaXML, "20123456789-01-F001-123.xml", []byte("<Invoice/>")},
		{sunat.DescargaCDR, "R-20123456789-01-F001-123.zip", []byte("PK CDR")},
		{sunat.DescargaPDF, "20123456789-01-F001-123.pdf", []byte("%PDF-1.4")},
	}
	for _, file := range files {
		if _, err := writer.SaveDownloadedFile(savedComp, &sunat.DownloadedFile{
			FileName: file.name,
			Content:  file.content,
		}, file.tipo); err != nil {
			t.Fatalf("guardando %s: %v", file.tipo, err)
		}
	}

	// Un gestor nuevo simula cerrar y volver a abrir la aplicación. La propuesta
	// no conoce la razón social usada cuando se creó la carpeta de descargas.
	reader := NewFileManager(baseDir)
	proposalComp := savedComp
	proposalComp.EmpresaRUC = ""
	proposalComp.EmpresaRazon = ""
	proposalComp.Numero = "123"
	results := reader.FindExistingResults(
		ownerRUC,
		[]sunat.Comprobante{proposalComp},
		[]sunat.TipoDescarga{sunat.DescargaXML, sunat.DescargaCDR, sunat.DescargaPDF},
	)
	if len(results) != 3 {
		t.Fatalf("FindExistingResults() devolvió %d resultados, want 3: %#v", len(results), results)
	}
	found := make(map[sunat.TipoDescarga]sunat.ItemResult, len(results))
	for _, result := range results {
		found[result.Tipo] = result
		if !result.Exito || result.Origen != "archivo existente" || result.Categoria != "ya_existente" {
			t.Fatalf("resultado existente inválido: %#v", result)
		}
		if _, err := os.Stat(result.RutaLocal); err != nil {
			t.Fatalf("ruta restaurada no existe: %v", err)
		}
	}
	for _, tipo := range []sunat.TipoDescarga{sunat.DescargaXML, sunat.DescargaCDR, sunat.DescargaPDF} {
		if _, ok := found[tipo]; !ok {
			t.Errorf("no se restauró %s", tipo)
		}
	}

	if other := reader.FindExistingResults(
		"20699999999",
		[]sunat.Comprobante{proposalComp},
		[]sunat.TipoDescarga{sunat.DescargaXML},
	); len(other) != 0 {
		t.Fatalf("se encontraron archivos de otra empresa: %#v", other)
	}
}

func TestFindExistingResultsSkipsCDRForElectronicSeries(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	manager := NewFileManager(baseDir)
	comp := sunat.Comprobante{
		RUC: "20123456789", Tipo: "02", Serie: "E001", Numero: "7",
		Libro: "2", Periodo: "202608", EmpresaRUC: "20600000001", EmpresaRazon: "EMPRESA",
	}
	if _, err := manager.SaveDownloadedFile(comp, &sunat.DownloadedFile{
		FileName: "R-20123456789-02-E001-7.zip",
		Content:  []byte("PK CDR"),
	}, sunat.DescargaCDR); err != nil {
		t.Fatal(err)
	}

	results := NewFileManager(baseDir).FindExistingResults(
		comp.EmpresaRUC,
		[]sunat.Comprobante{comp},
		[]sunat.TipoDescarga{sunat.DescargaCDR},
	)
	if len(results) != 0 {
		t.Fatalf("una serie E no debe marcar CDR: %#v", results)
	}
}

func TestSaveDownloadedFilePublishesWithoutTemporaryFiles(t *testing.T) {
	t.Parallel()

	manager := NewFileManager(t.TempDir())
	comp := sunat.Comprobante{
		RUC: "20123456789", Tipo: "01", Serie: "F001", Numero: "9", Libro: "2",
	}
	path, err := manager.SaveDownloadedFile(comp, &sunat.DownloadedFile{
		FileName: "20123456789-01-F001-9.xml",
		Content:  []byte("<Invoice/ >"),
	}, sunat.DescargaXML)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("archivo temporal residual: %s", entry.Name())
		}
	}
}

func TestSaveBatchManifestPersistsFinalResults(t *testing.T) {
	t.Parallel()

	manager := NewFileManager(t.TempDir())
	status := sunat.BatchStatus{
		BatchID:    "batch-123",
		Estado:     "completado",
		TotalItems: 1,
		Procesados: 1,
		Exitosos:   1,
		Resultados: []sunat.ItemResult{{Exito: true, Tipo: sunat.DescargaXML}},
	}
	path, err := manager.SaveBatchManifest(status)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"batch_id": "batch-123"`) ||
		!strings.Contains(string(content), `"manifest_path":`) {
		t.Fatalf("manifiesto incompleto: %s", content)
	}
}

func TestSaveBatchManifestDoesNotRegressToOlderCheckpoint(t *testing.T) {
	t.Parallel()

	manager := NewFileManager(t.TempDir())
	newer := sunat.BatchStatus{BatchID: "batch-ordered", Procesados: 50, Exitosos: 48, Errores: 2}
	path, err := manager.SaveBatchManifest(newer)
	if err != nil {
		t.Fatal(err)
	}
	older := sunat.BatchStatus{BatchID: "batch-ordered", Procesados: 25, Exitosos: 25}
	if _, err := manager.SaveBatchManifest(older); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var persisted sunat.BatchStatus
	if err := json.Unmarshal(content, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.Procesados != 50 || persisted.Errores != 2 {
		t.Fatalf("un checkpoint antiguo reemplazó al nuevo: %#v", persisted)
	}
}

func TestCreateZipFilesIncludesOnlyRequestedBatchFiles(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	manager := NewFileManager(baseDir)
	batchDir := filepath.Join(baseDir, "APP DESCARGAS", "CPE", "empresa")
	if err := os.MkdirAll(batchDir, 0755); err != nil {
		t.Fatal(err)
	}
	included := filepath.Join(batchDir, "incluido.xml")
	excluded := filepath.Join(batchDir, "anterior.xml")
	if err := os.WriteFile(included, []byte("<Invoice/>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excluded, []byte("<OldInvoice/>"), 0644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := manager.CreateZipFiles([]string{included}, &output); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 1 || filepath.Base(archive.File[0].Name) != "incluido.xml" {
		t.Fatalf("contenido ZIP inesperado: %#v", archive.File)
	}
}

func TestCreateZipFilesRejectsPathsOutsideBaseDirectory(t *testing.T) {
	t.Parallel()

	manager := NewFileManager(t.TempDir())
	outside := filepath.Join(t.TempDir(), "externo.xml")
	if err := os.WriteFile(outside, []byte("<Invoice/>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := manager.CreateZipFiles([]string{outside}, &bytes.Buffer{}); err == nil {
		t.Fatal("se esperaba rechazo para una ruta externa")
	}
}
