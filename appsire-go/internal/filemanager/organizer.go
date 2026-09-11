package filemanager

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"appsire-go/internal/sunat"
)

// FileManager se encarga de crear las carpetas estructuradas y almacenar los archivos descargados
type FileManager struct {
	BaseDir string
}

// NewFileManager inicializa el gestor con la ruta base indicada
func NewFileManager(baseDir string) *FileManager {
	if baseDir == "" {
		baseDir = "./downloads"
	}
	// Asegurar ruta absoluta
	if abs, err := filepath.Abs(baseDir); err == nil {
		baseDir = abs
	}
	_ = os.MkdirAll(baseDir, 0755)
	return &FileManager{
		BaseDir: baseDir,
	}
}

// sanitizeFolderName limpia caracteres no permitidos en nombres de directorios en Windows
func sanitizeFolderName(name string) string {
	name = strings.TrimSpace(name)
	invalidChars := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	for _, char := range invalidChars {
		name = strings.ReplaceAll(name, char, "")
	}
	return name
}

// BuildDirectoryPath genera la ruta estructurada:
// {BaseDir}/APP DESCARGAS/CPE/{RUC} {RazonSocial}/{Compras|Ventas}/{Periodo}
func (m *FileManager) BuildDirectoryPath(comp sunat.Comprobante) string {
	// Determinar Compras o Ventas según Libro
	tipoLibro := "Ventas"
	if comp.Libro == "2" || strings.EqualFold(comp.SheetName, "cpe") || strings.EqualFold(comp.SheetName, "rxh") {
		tipoLibro = "Compras"
	}

	razonSocialClean := sanitizeFolderName(comp.RazonSocial)
	if razonSocialClean == "" {
		razonSocialClean = "EMPRESA"
	}

	rucClean := sanitizeFolderName(comp.RUC)
	if rucClean == "" {
		rucClean = "SIN_RUC"
	}

	periodoClean := sanitizeFolderName(comp.Periodo)
	if periodoClean == "" {
		periodoClean = "VARIOS"
	}

	folderEmpresa := fmt.Sprintf("%s %s", rucClean, razonSocialClean)
	folderEmpresa = strings.TrimSpace(folderEmpresa)

	fullPath := filepath.Join(m.BaseDir, "APP DESCARGAS", "CPE", folderEmpresa, tipoLibro, periodoClean)
	return fullPath
}

// SaveDownloadedFile guarda el archivo binario en el disco y devuelve la ruta completa
func (m *FileManager) SaveDownloadedFile(comp sunat.Comprobante, file *sunat.DownloadedFile, tipoDescarga sunat.TipoDescarga) (string, error) {
	targetDir := m.BuildDirectoryPath(comp)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("error creando directorio de destino %s: %w", targetDir, err)
	}

	fileName := file.FileName
	if fileName == "" {
		ext := ".bin"
		switch tipoDescarga {
		case sunat.DescargaPDF:
			ext = ".pdf"
		case sunat.DescargaXML:
			ext = ".xml"
			if file.IsZip {
				ext = ".zip"
			}
		case sunat.DescargaCDR:
			ext = ".zip"
		}
		fileName = fmt.Sprintf("%s-%s-%s-%s%s", comp.RUC, comp.Tipo, comp.Serie, comp.Numero, ext)
	}

	filePath := filepath.Join(targetDir, fileName)

	err := os.WriteFile(filePath, file.Content, 0644)
	if err != nil {
		return "", fmt.Errorf("error escribiendo archivo en %s: %w", filePath, err)
	}
	if len(file.OriginalZip) > 0 {
		zipName := filepath.Base(file.OriginalZipName)
		if zipName == "." || zipName == "" {
			zipName = strings.TrimSuffix(fileName, filepath.Ext(fileName)) + ".zip"
		}
		zipPath := filepath.Join(targetDir, zipName)
		if err := os.WriteFile(zipPath, file.OriginalZip, 0644); err != nil {
			return "", fmt.Errorf("error conservando ZIP original en %s: %w", zipPath, err)
		}
	}

	return filePath, nil
}

// SaveSireProposal guarda la propuesta en una carpeta independiente de los CPE.
func (m *FileManager) SaveSireProposal(
	ruc string,
	period string,
	book sunat.ProposalBook,
	fileName string,
	content []byte,
) (string, error) {
	ruc = sanitizeFolderName(ruc)
	if ruc == "" {
		ruc = "SIN_RUC"
	}
	period = sanitizeFolderName(period)
	fileName = filepath.Base(fileName)
	if fileName == "." || fileName == "" {
		fileName = fmt.Sprintf("Propuesta_%s_%s.zip", book, period)
	}

	targetDir := filepath.Join(
		m.BaseDir,
		"APP DESCARGAS",
		"SIRE",
		ruc,
		string(book),
		period,
	)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("creando carpeta de propuesta SIRE: %w", err)
	}

	targetPath := filepath.Join(targetDir, fileName)
	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		return "", fmt.Errorf("guardando propuesta SIRE: %w", err)
	}
	return targetPath, nil
}

// FindExistingFile localiza una descarga valida del mismo comprobante para no
// volver a pedirla a SUNAT en ejecuciones posteriores.
func (m *FileManager) FindExistingFile(
	comp sunat.Comprobante,
	tipoDescarga sunat.TipoDescarga,
) (string, bool) {
	targetDir := m.BuildDirectoryPath(comp)
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return "", false
	}

	ruc := strings.ToLower(comp.RUC)
	serie := strings.ToLower(comp.Serie)
	numero := strings.ToLower(sunat.NormalizeNumero(comp.Numero))
	documentSuffix := "-" + serie + "-" + numero

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := strings.ToLower(entry.Name())
		baseName := strings.TrimSuffix(name, filepath.Ext(name))
		containsDocument := strings.Contains(name, ruc) && strings.HasSuffix(baseName, documentSuffix)
		if !containsDocument || !matchesDownloadType(name, tipoDescarga) {
			continue
		}

		path := filepath.Join(targetDir, entry.Name())
		info, statErr := entry.Info()
		if statErr == nil && info.Size() > 0 {
			return path, true
		}
	}

	return "", false
}

func matchesDownloadType(name string, tipoDescarga sunat.TipoDescarga) bool {
	switch tipoDescarga {
	case sunat.DescargaPDF:
		return strings.HasSuffix(name, ".pdf")
	case sunat.DescargaXML:
		return strings.HasSuffix(name, ".xml")
	case sunat.DescargaCDR:
		isCDRName := strings.HasPrefix(name, "r-") || strings.Contains(name, "cdr")
		return isCDRName && (strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".xml"))
	default:
		return false
	}
}

// OpenInExplorer abre la carpeta de descargas en el explorador de archivos del sistema operativo
func (m *FileManager) OpenInExplorer(targetPath string) error {
	if targetPath == "" {
		targetPath = m.BaseDir
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command("explorer", targetPath)
		return cmd.Start()
	}
	return nil
}

// CreateZipPackage empaqueta todos los archivos de un batch o carpeta en un único archivo ZIP para descarga web
func (m *FileManager) CreateZipPackage(folderPath string, zipWriter io.Writer) error {
	w := zip.NewWriter(zipWriter)
	defer w.Close()

	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(folderPath, path)
		if err != nil {
			return err
		}

		zipFile, err := w.Create(filepath.ToSlash(relPath))
		if err != nil {
			return err
		}

		fsFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fsFile.Close()

		_, err = io.Copy(zipFile, fsFile)
		return err
	}

	return filepath.Walk(folderPath, walker)
}
