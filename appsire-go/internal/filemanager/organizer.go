package filemanager

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"appsire-go/internal/sunat"
)

// FileManager se encarga de crear las carpetas estructuradas y almacenar los archivos descargados
type FileManager struct {
	BaseDir string
	indexMu sync.RWMutex
	// manifestMu serializa los checkpoints del mismo lote; varios workers pueden
	// alcanzar umbrales consecutivos mientras el anterior todavía escribe.
	manifestMu       sync.Mutex
	manifestProgress map[string]int
	// directoryIndex replica DescargasEnCarpeta.Indexar del macro: cada carpeta
	// se lee una sola vez y cada comprobante se consulta por clave en O(1).
	directoryIndex map[string]map[string]indexedFile
	indexLoads     map[string]chan struct{}
}

type indexedFile struct {
	name     string
	size     int64
	priority int
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
		BaseDir:          baseDir,
		directoryIndex:   make(map[string]map[string]indexedFile),
		indexLoads:       make(map[string]chan struct{}),
		manifestProgress: make(map[string]int),
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

	// El macro crea una sola carpeta por empresa activa, libro y periodo. En
	// compras, comp.RUC/RazonSocial pertenecen al proveedor y no deben fragmentar
	// el lote en cientos de directorios diferentes.
	ownerRUC := comp.EmpresaRUC
	if ownerRUC == "" {
		ownerRUC = comp.RUC
	}
	ownerName := comp.EmpresaRazon
	if ownerName == "" {
		ownerName = comp.RazonSocial
	}
	razonSocialClean := sanitizeFolderName(ownerName)
	if razonSocialClean == "" {
		razonSocialClean = "EMPRESA"
	}

	rucClean := sanitizeFolderName(ownerRUC)
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

	if err := writeFileAtomic(filePath, file.Content, 0644); err != nil {
		return "", fmt.Errorf("error escribiendo archivo en %s: %w", filePath, err)
	}
	m.rememberFile(targetDir, fileName, int64(len(file.Content)))
	// Para XML se publica únicamente el comprobante extraído. Conservar además
	// el ZIP duplicaba espacio y hacía que el índice prefiriera el contenedor.
	if tipoDescarga != sunat.DescargaXML && len(file.OriginalZip) > 0 {
		zipName := filepath.Base(file.OriginalZipName)
		if zipName == "." || zipName == "" {
			zipName = strings.TrimSuffix(fileName, filepath.Ext(fileName)) + ".zip"
		}
		zipPath := filepath.Join(targetDir, zipName)
		if err := writeFileAtomic(zipPath, file.OriginalZip, 0644); err != nil {
			return "", fmt.Errorf("error conservando ZIP original en %s: %w", zipPath, err)
		}
		m.rememberFile(targetDir, zipName, int64(len(file.OriginalZip)))
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
	if err := writeFileAtomic(targetPath, content, 0644); err != nil {
		return "", fmt.Errorf("guardando propuesta SIRE: %w", err)
	}
	return targetPath, nil
}

// SaveBatchManifest persiste el resultado completo para que el resumen pueda
// reconstruirse después de cerrar la interfaz o reiniciar el proceso.
func (m *FileManager) SaveBatchManifest(status sunat.BatchStatus) (string, error) {
	m.manifestMu.Lock()
	defer m.manifestMu.Unlock()

	batchID := sanitizeFolderName(filepath.Base(status.BatchID))
	if batchID == "" || batchID == "." {
		return "", fmt.Errorf("identificador de lote inválido")
	}
	directory := filepath.Join(m.BaseDir, "APP DESCARGAS", "LOTES")
	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", fmt.Errorf("creando carpeta de manifiestos: %w", err)
	}
	path := filepath.Join(directory, batchID+".json")
	if status.Procesados < m.manifestProgress[batchID] {
		return path, nil
	}
	status.ManifestPath = path
	content, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serializando manifiesto: %w", err)
	}
	if err := writeFileAtomic(path, content, 0644); err != nil {
		return "", fmt.Errorf("guardando manifiesto: %w", err)
	}
	m.manifestProgress[batchID] = status.Procesados
	return path, nil
}

// FindExistingFile localiza una descarga valida del mismo comprobante para no
// volver a pedirla a SUNAT en ejecuciones posteriores.
func (m *FileManager) FindExistingFile(
	comp sunat.Comprobante,
	tipoDescarga sunat.TipoDescarga,
) (string, bool) {
	targetDir := m.BuildDirectoryPath(comp)
	if !m.ensureDirectoryIndexed(targetDir) {
		return "", false
	}

	key := downloadIndexKey(tipoDescarga, documentKey(comp.RUC, comp.Tipo, comp.Serie, comp.Numero))
	m.indexMu.RLock()
	entry, found := m.directoryIndex[targetDir][key]
	m.indexMu.RUnlock()
	if found && entry.size > 0 {
		return filepath.Join(targetDir, entry.name), true
	}

	return "", false
}

func (m *FileManager) ensureDirectoryIndexed(targetDir string) bool {
	m.indexMu.Lock()
	_, found := m.directoryIndex[targetDir]
	if found {
		m.indexMu.Unlock()
		return true
	}
	if loading, exists := m.indexLoads[targetDir]; exists {
		m.indexMu.Unlock()
		<-loading
		m.indexMu.RLock()
		_, found = m.directoryIndex[targetDir]
		m.indexMu.RUnlock()
		return found
	}
	loading := make(chan struct{})
	m.indexLoads[targetDir] = loading
	m.indexMu.Unlock()

	entries, err := os.ReadDir(targetDir)
	loaded := make(map[string]indexedFile, len(entries))
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr == nil {
				addIndexedFile(loaded, entry.Name(), info.Size())
			}
		}
	}

	m.indexMu.Lock()
	defer m.indexMu.Unlock()
	defer close(loading)
	delete(m.indexLoads, targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			m.directoryIndex[targetDir] = loaded
			return true
		}
		return false
	}

	m.directoryIndex[targetDir] = loaded
	return true
}

func (m *FileManager) rememberFile(targetDir, name string, size int64) {
	m.indexMu.RLock()
	loading := m.indexLoads[targetDir]
	m.indexMu.RUnlock()
	if loading != nil {
		<-loading
	}
	m.indexMu.Lock()
	defer m.indexMu.Unlock()
	entries, found := m.directoryIndex[targetDir]
	if !found {
		// La carpeta todavía no fue consultada. Se indexará completa cuando sea
		// necesario para no ocultar archivos preexistentes.
		return
	}
	addIndexedFile(entries, name, size)
}

func writeFileAtomic(targetPath string, content []byte, mode os.FileMode) (err error) {
	targetDir := filepath.Dir(targetPath)
	temp, err := os.CreateTemp(targetDir, "."+filepath.Base(targetPath)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("creando archivo temporal: %w", err)
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			if closeErr := temp.Close(); err == nil && closeErr != nil {
				err = fmt.Errorf("cerrando archivo temporal: %w", closeErr)
			}
		}
		if removeErr := os.Remove(tempPath); err == nil && removeErr != nil && !os.IsNotExist(removeErr) {
			err = fmt.Errorf("eliminando archivo temporal: %w", removeErr)
		}
	}()

	if err = temp.Chmod(mode); err != nil {
		return fmt.Errorf("asignando permisos al archivo temporal: %w", err)
	}
	if _, err = temp.Write(content); err != nil {
		return fmt.Errorf("escribiendo archivo temporal: %w", err)
	}
	if err = temp.Close(); err != nil {
		return fmt.Errorf("cerrando archivo temporal: %w", err)
	}
	closed = true
	if removeErr := os.Remove(targetPath); removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("reemplazando archivo existente: %w", removeErr)
	}
	if err = os.Rename(tempPath, targetPath); err != nil {
		return fmt.Errorf("publicando archivo: %w", err)
	}
	return nil
}

func addIndexedFile(index map[string]indexedFile, name string, size int64) {
	baseName := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	docKey, ok := documentKeyFromFileName(baseName)
	if !ok {
		return
	}
	ext := strings.ToLower(filepath.Ext(name))
	isCDR := isCDRFileName(baseName)
	var tipo sunat.TipoDescarga
	priority := 0
	switch {
	case isCDR && (ext == ".zip" || ext == ".xml"):
		tipo = sunat.DescargaCDR
	case !isCDR && ext == ".pdf":
		tipo = sunat.DescargaPDF
	case !isCDR && (ext == ".zip" || ext == ".xml"):
		tipo = sunat.DescargaXML
		// La aplicación publica XML extraído; un ZIP previo queda como respaldo.
		if ext == ".zip" {
			priority = 1
		}
	default:
		return
	}

	key := downloadIndexKey(tipo, docKey)
	if current, found := index[key]; !found || priority < current.priority {
		index[key] = indexedFile{name: name, size: size, priority: priority}
	}
}

func documentKeyFromFileName(baseName string) (string, bool) {
	parts := strings.Split(baseName, "-")
	for i := 0; i+3 < len(parts); i++ {
		if isIdentityDocument(parts[i]) {
			return documentKey(parts[i], parts[i+1], parts[i+2], parts[i+3]), true
		}
	}
	return "", false

}

func isIdentityDocument(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 8 && len(value) != 11 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func isCDRFileName(baseName string) bool {
	baseName = strings.TrimSpace(baseName)
	return strings.HasPrefix(strings.ToLower(baseName), "r-") ||
		strings.HasSuffix(strings.ToLower(baseName), "-cdr")
}

func documentKey(ruc, tipo, serie, numero string) string {
	tipo = sunat.NormalizeTipo(tipo)
	return strings.ToLower(strings.Join([]string{
		strings.TrimSpace(ruc),
		strings.TrimSpace(tipo),
		strings.TrimSpace(serie),
		sunat.NormalizeNumero(numero),
	}, "|"))
}

func downloadIndexKey(tipo sunat.TipoDescarga, docKey string) string {
	return strings.ToLower(string(tipo)) + "|" + docKey
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

// CreateZipPackage empaqueta una carpeta completa. Se conserva para exportaciones
// administrativas; la entrega de un lote usa CreateZipFiles.
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

// CreateZipFiles empaqueta únicamente resultados del lote indicado y rechaza
// rutas externas al directorio administrado por la aplicación.
func (m *FileManager) CreateZipFiles(filePaths []string, destination io.Writer) error {
	basePath, err := filepath.Abs(m.BaseDir)
	if err != nil {
		return fmt.Errorf("resolviendo carpeta de descargas: %w", err)
	}
	type packageFile struct {
		path string
		name string
	}
	files := make([]packageFile, 0, len(filePaths))
	seen := make(map[string]struct{}, len(filePaths))
	for _, requestedPath := range filePaths {
		path, err := filepath.Abs(strings.TrimSpace(requestedPath))
		if err != nil {
			return fmt.Errorf("resolviendo archivo del lote: %w", err)
		}
		relativePath, err := filepath.Rel(basePath, path)
		if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
			return fmt.Errorf("el archivo no pertenece a la carpeta de descargas: %s", requestedPath)
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("abriendo archivo del lote %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("la ruta del lote no es un archivo regular: %s", path)
		}
		key := strings.ToLower(path)
		if _, found := seen[key]; found {
			continue
		}
		seen[key] = struct{}{}
		files = append(files, packageFile{path: path, name: filepath.ToSlash(relativePath)})
	}
	if len(files) == 0 {
		return errors.New("el lote no contiene archivos descargados")
	}

	archive := zip.NewWriter(destination)
	for _, item := range files {
		entry, err := archive.Create(item.name)
		if err != nil {
			_ = archive.Close()
			return fmt.Errorf("creando entrada ZIP %s: %w", item.name, err)
		}
		source, err := os.Open(item.path)
		if err != nil {
			_ = archive.Close()
			return fmt.Errorf("abriendo %s: %w", item.path, err)
		}
		_, copyErr := io.Copy(entry, source)
		closeErr := source.Close()
		if copyErr != nil {
			_ = archive.Close()
			return fmt.Errorf("copiando %s al ZIP: %w", item.path, copyErr)
		}
		if closeErr != nil {
			_ = archive.Close()
			return fmt.Errorf("cerrando %s: %w", item.path, closeErr)
		}
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("cerrando ZIP del lote: %w", err)
	}
	return nil
}
