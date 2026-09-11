package filemanager

import (
	"os"
	"path/filepath"
	"testing"

	"appsire-go/internal/sunat"
)

func TestSaveDownloadedFilePreservesXMLAndOriginalZIP(t *testing.T) {
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
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("ZIP original no conservado: %v", err)
	}

	found, ok := manager.FindExistingFile(comp, sunat.DescargaXML)
	if !ok || found != xmlPath {
		t.Fatalf("FindExistingFile() = %q, %t, want %q, true", found, ok, xmlPath)
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
