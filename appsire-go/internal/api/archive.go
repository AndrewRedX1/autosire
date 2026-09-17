package api

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ArchiveTreeResponse representa la jerarquía general de empresas y carpetas
// ArchiveTreeResponse representa la jerarquía general de empresas y carpetas
type ArchiveTreeResponse struct {
	Success    bool                `json:"success"`
	BaseDir    string              `json:"base_dir"`
	Companies  []ArchiveCompany    `json:"companies"`
	Periods    []ArchivePeriodItem `json:"periods"`
	TotalFiles int                 `json:"total_files"`
	TotalBytes int64               `json:"total_bytes"`
}

// ArchivePeriodItem representa una tarjeta de período descargado
type ArchivePeriodItem struct {
	CompanyFolder string `json:"company_folder"`
	CompanyRUC    string `json:"company_ruc"`
	CompanyName   string `json:"company_name"`
	Book          string `json:"book"`
	Period        string `json:"period"`
	PeriodLabel   string `json:"period_label"`
	XMLCount      int    `json:"xml_count"`
	CDRCount      int    `json:"cdr_count"`
	PDFCount      int    `json:"pdf_count"`
	TotalFiles    int    `json:"total_files"`
	TotalBytes    int64  `json:"total_bytes"`
	Path          string `json:"path"`
}

// ArchiveCompany representa una empresa encontrada en el archivador
type ArchiveCompany struct {
	Folder     string        `json:"folder"`
	RUC        string        `json:"ruc"`
	Name       string        `json:"name"`
	Books      []ArchiveBook `json:"books"`
	TotalFiles int           `json:"total_files"`
	TotalBytes int64         `json:"total_bytes"`
}

// ArchiveBook representa un libro (Compras / Ventas)
type ArchiveBook struct {
	Name       string          `json:"name"`
	Periods    []ArchivePeriod `json:"periods"`
	TotalFiles int             `json:"total_files"`
	TotalBytes int64           `json:"total_bytes"`
}

// ArchivePeriod representa un período tributario con métricas de archivos
type ArchivePeriod struct {
	Period     string `json:"period"`
	TotalFiles int    `json:"total_files"`
	XMLCount   int    `json:"xml_count"`
	CDRCount   int    `json:"cdr_count"`
	PDFCount   int    `json:"pdf_count"`
	TotalBytes int64  `json:"total_bytes"`
	Path       string `json:"path"`
}

// ArchiveVoucherItem consolida las descargas (XML, CDR, PDF) de un mismo comprobante
type ArchiveVoucherItem struct {
	Key        string           `json:"key"`
	RUC        string           `json:"ruc"`
	Tipo       string           `json:"tipo"`
	TipoNombre string           `json:"tipo_nombre"`
	Serie      string           `json:"serie"`
	Numero     string           `json:"numero"`
	Company    string           `json:"company"`
	Book       string           `json:"book"`
	Period     string           `json:"period"`
	HasXML     bool             `json:"has_xml"`
	HasCDR     bool             `json:"has_cdr"`
	HasPDF     bool             `json:"has_pdf"`
	XMLFile    *ArchiveFileInfo `json:"xml_file,omitempty"`
	CDRFile    *ArchiveFileInfo `json:"cdr_file,omitempty"`
	PDFFile    *ArchiveFileInfo `json:"pdf_file,omitempty"`
	ModifiedAt time.Time        `json:"modified_at"`
}

// ArchiveFileInfo metadatos de un archivo específico de un comprobante
type ArchiveFileInfo struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	RelativePath string `json:"relative_path"`
	Size         int64  `json:"size"`
}

// ArchiveFileItem representa un archivo individual en el archivador con metadatos SUNAT
type ArchiveFileItem struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	RelativePath string    `json:"relative_path"`
	Company      string    `json:"company"`
	Book         string    `json:"book"`
	Period       string    `json:"period"`
	RUC          string    `json:"ruc"`
	Tipo         string    `json:"tipo"`
	TipoNombre   string    `json:"tipo_nombre"`
	Serie        string    `json:"serie"`
	Numero       string    `json:"numero"`
	Format       string    `json:"format"` // XML, CDR, PDF, OTRO
	Size         int64     `json:"size"`
	ModifiedAt   time.Time `json:"modified_at"`
}

// ArchiveFilesResponse representa la lista paginada y filtrada de comprobantes
type ArchiveFilesResponse struct {
	Success       bool                 `json:"success"`
	Company       string               `json:"company"`
	Book          string               `json:"book"`
	Period        string               `json:"period"`
	Total         int                  `json:"total"`
	TotalVouchers int                  `json:"total_vouchers"`
	Page          int                  `json:"page"`
	PageSize      int                  `json:"page_size"`
	TotalPages    int                  `json:"total_pages"`
	Files         []ArchiveFileItem    `json:"files"`
	Vouchers      []ArchiveVoucherItem `json:"vouchers"`
	Summary       ArchiveSummary       `json:"summary"`
}

// ArchiveSummary contabiliza totales de la consulta actual
type ArchiveSummary struct {
	TotalFiles int   `json:"total_files"`
	XMLCount   int   `json:"xml_count"`
	CDRCount   int   `json:"cdr_count"`
	PDFCount   int   `json:"pdf_count"`
	TotalBytes int64 `json:"total_bytes"`
}

var (
	rxStandardDoc = regexp.MustCompile(`(?i)^([0-9]{11})-([0-9]{2})-([A-Za-z0-9]+)-([0-9]+)\.(xml|pdf|zip)$`)
	rxCdrDoc      = regexp.MustCompile(`(?i)^R-([0-9]{11})-([0-9]{2})-([A-Za-z0-9]+)-([0-9]+)\.(xml|zip)$`)
	rxAltDoc      = regexp.MustCompile(`(?i)^(FACTURA|BOLETA|NOTA_CREDITO|NOTA_DEBITO)([A-Za-z0-9]{4})-([0-9]+)([0-9]{11})\.(xml|pdf)$`)
)

// HandleArchiveTree escanea la carpeta de descargas estructurada
func (s *Server) HandleArchiveTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	cpeRoot := filepath.Join(s.fileManager.BaseDir, "APP DESCARGAS", "CPE")
	resp := ArchiveTreeResponse{
		Success:   true,
		BaseDir:   s.fileManager.BaseDir,
		Companies: make([]ArchiveCompany, 0),
		Periods:   make([]ArchivePeriodItem, 0),
	}

	compDirs, err := os.ReadDir(cpeRoot)
	if err != nil {
		// Carpeta no existe aún o vacía
		respondJSON(w, http.StatusOK, resp)
		return
	}

	for _, compEntry := range compDirs {
		if !compEntry.IsDir() {
			continue
		}
		compFolderName := compEntry.Name()
		compPath := filepath.Join(cpeRoot, compFolderName)

		// Parsear RUC y Razón Social de "20490304101 HOTELES CBC S.A.C"
		ruc := ""
		name := compFolderName
		parts := strings.SplitN(compFolderName, " ", 2)
		if len(parts) >= 1 && len(parts[0]) == 11 && isNumeric(parts[0]) {
			ruc = parts[0]
			if len(parts) >= 2 {
				name = parts[1]
			}
		}

		companyItem := ArchiveCompany{
			Folder: compFolderName,
			RUC:    ruc,
			Name:   name,
			Books:  make([]ArchiveBook, 0),
		}

		bookDirs, err := os.ReadDir(compPath)
		if err != nil {
			continue
		}

		for _, bookEntry := range bookDirs {
			if !bookEntry.IsDir() {
				continue
			}
			bookName := bookEntry.Name()
			bookPath := filepath.Join(compPath, bookName)

			bookItem := ArchiveBook{
				Name:    bookName,
				Periods: make([]ArchivePeriod, 0),
			}

			periodDirs, err := os.ReadDir(bookPath)
			if err != nil {
				continue
			}

			for _, periodEntry := range periodDirs {
				if !periodEntry.IsDir() {
					continue
				}
				periodName := periodEntry.Name()
				periodPath := filepath.Join(bookPath, periodName)

				periodItem := ArchivePeriod{
					Period: periodName,
					Path:   periodPath,
				}

				files, err := os.ReadDir(periodPath)
				if err != nil {
					continue
				}

				for _, fileEntry := range files {
					if fileEntry.IsDir() {
						continue
					}
					info, err := fileEntry.Info()
					if err != nil {
						continue
					}
					periodItem.TotalFiles++
					periodItem.TotalBytes += info.Size()

					ext := strings.ToLower(filepath.Ext(fileEntry.Name()))
					isCdr := strings.HasPrefix(strings.ToUpper(fileEntry.Name()), "R-") || ext == ".zip"
					if isCdr {
						periodItem.CDRCount++
					} else if ext == ".xml" {
						periodItem.XMLCount++
					} else if ext == ".pdf" {
						periodItem.PDFCount++
					}
				}

				bookItem.Periods = append(bookItem.Periods, periodItem)
				bookItem.TotalFiles += periodItem.TotalFiles
				bookItem.TotalBytes += periodItem.TotalBytes

				relPeriodPath, _ := filepath.Rel(s.fileManager.BaseDir, periodPath)
				resp.Periods = append(resp.Periods, ArchivePeriodItem{
					CompanyFolder: compFolderName,
					CompanyRUC:    ruc,
					CompanyName:   name,
					Book:          bookName,
					Period:        periodName,
					PeriodLabel:   formatPeriodLabel(periodName),
					XMLCount:      periodItem.XMLCount,
					CDRCount:      periodItem.CDRCount,
					PDFCount:      periodItem.PDFCount,
					TotalFiles:    periodItem.TotalFiles,
					TotalBytes:    periodItem.TotalBytes,
					Path:          filepath.ToSlash(relPeriodPath),
				})
			}

			// Ordenar periodos descendentemente
			sort.Slice(bookItem.Periods, func(i, j int) bool {
				return bookItem.Periods[i].Period > bookItem.Periods[j].Period
			})

			companyItem.Books = append(companyItem.Books, bookItem)
			companyItem.TotalFiles += bookItem.TotalFiles
			companyItem.TotalBytes += bookItem.TotalBytes
		}

		resp.Companies = append(resp.Companies, companyItem)
		resp.TotalFiles += companyItem.TotalFiles
		resp.TotalBytes += companyItem.TotalBytes
	}

	// Ordenar empresas alfabéticamente
	sort.Slice(resp.Companies, func(i, j int) bool {
		return resp.Companies[i].Name < resp.Companies[j].Name
	})

	// Ordenar períodos cronológicamente descendente
	sort.Slice(resp.Periods, func(i, j int) bool {
		if resp.Periods[i].Period == resp.Periods[j].Period {
			return resp.Periods[i].CompanyName < resp.Periods[j].CompanyName
		}
		return resp.Periods[i].Period > resp.Periods[j].Period
	})

	respondJSON(w, http.StatusOK, resp)
}

// HandleArchiveFiles lista y filtra los comprobantes físicos del archivador
func (s *Server) HandleArchiveFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	filterCompany := strings.TrimSpace(r.URL.Query().Get("company"))
	filterBook := strings.TrimSpace(r.URL.Query().Get("book"))
	filterPeriod := strings.TrimSpace(r.URL.Query().Get("period"))
	filterFormat := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("format")))
	searchQuery := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 1000 {
		pageSize = 200
	}

	cpeRoot := filepath.Join(s.fileManager.BaseDir, "APP DESCARGAS", "CPE")
	var matchedFiles []ArchiveFileItem
	var summary ArchiveSummary

	// Escanear árbol
	compDirs, err := os.ReadDir(cpeRoot)
	if err == nil {
		for _, compEntry := range compDirs {
			if !compEntry.IsDir() {
				continue
			}
			compName := compEntry.Name()
			if filterCompany != "" && !strings.Contains(strings.ToLower(compName), strings.ToLower(filterCompany)) {
				continue
			}

			compPath := filepath.Join(cpeRoot, compName)
			bookDirs, err := os.ReadDir(compPath)
			if err != nil {
				continue
			}

			for _, bookEntry := range bookDirs {
				if !bookEntry.IsDir() {
					continue
				}
				bookName := bookEntry.Name()
				if filterBook != "" && !strings.EqualFold(bookName, filterBook) {
					continue
				}

				bookPath := filepath.Join(compPath, bookName)
				periodDirs, err := os.ReadDir(bookPath)
				if err != nil {
					continue
				}

				for _, periodEntry := range periodDirs {
					if !periodEntry.IsDir() {
						continue
					}
					periodName := periodEntry.Name()
					if filterPeriod != "" && !strings.EqualFold(periodName, filterPeriod) {
						continue
					}

					periodPath := filepath.Join(bookPath, periodName)
					files, err := os.ReadDir(periodPath)
					if err != nil {
						continue
					}

					for _, fileEntry := range files {
						if fileEntry.IsDir() {
							continue
						}
						info, err := fileEntry.Info()
						if err != nil {
							continue
						}

						parsed := parseArchiveFileName(fileEntry.Name())
						parsed.Size = info.Size()
						parsed.ModifiedAt = info.ModTime()
						fullPath := filepath.Join(periodPath, fileEntry.Name())
						parsed.Path = fullPath

						if rel, err := filepath.Rel(s.fileManager.BaseDir, fullPath); err == nil {
							parsed.RelativePath = filepath.ToSlash(rel)
						} else {
							parsed.RelativePath = fileEntry.Name()
						}
						parsed.Company = compName
						parsed.Book = bookName
						parsed.Period = periodName

						// Filtro de formato
						if filterFormat != "" && filterFormat != "TODOS" && parsed.Format != filterFormat {
							continue
						}

						// Búsqueda de texto
						if searchQuery != "" {
							match := strings.Contains(strings.ToLower(parsed.Name), searchQuery) ||
								strings.Contains(strings.ToLower(parsed.RUC), searchQuery) ||
								strings.Contains(strings.ToLower(parsed.Serie), searchQuery) ||
								strings.Contains(strings.ToLower(parsed.Numero), searchQuery) ||
								strings.Contains(strings.ToLower(parsed.TipoNombre), searchQuery)
							if !match {
								continue
							}
						}

						// Acumular métricas
						summary.TotalFiles++
						summary.TotalBytes += parsed.Size
						switch parsed.Format {
						case "XML":
							summary.XMLCount++
						case "CDR":
							summary.CDRCount++
						case "PDF":
							summary.PDFCount++
						}

						matchedFiles = append(matchedFiles, parsed)
					}
				}
			}
		}
	}

	// Ordenar por fecha de modificación más reciente
	sort.Slice(matchedFiles, func(i, j int) bool {
		return matchedFiles[i].ModifiedAt.After(matchedFiles[j].ModifiedAt)
	})

	// Consolidar por comprobante único (RUC|Tipo|Serie|Numero)
	voucherMap := make(map[string]*ArchiveVoucherItem)
	voucherKeys := make([]string, 0)

	for _, file := range matchedFiles {
		key := fmt.Sprintf("%s|%s|%s|%s", file.RUC, file.Tipo, file.Serie, file.Numero)
		if file.Serie == "" && file.Numero == "" {
			key = file.Name
		}
		v, exists := voucherMap[key]
		if !exists {
			v = &ArchiveVoucherItem{
				Key:        key,
				RUC:        file.RUC,
				Tipo:       file.Tipo,
				TipoNombre: file.TipoNombre,
				Serie:      file.Serie,
				Numero:     file.Numero,
				Company:    file.Company,
				Book:       file.Book,
				Period:     file.Period,
				ModifiedAt: file.ModifiedAt,
			}
			voucherMap[key] = v
			voucherKeys = append(voucherKeys, key)
		}
		if file.ModifiedAt.After(v.ModifiedAt) {
			v.ModifiedAt = file.ModifiedAt
		}
		fileInfo := &ArchiveFileInfo{
			Name:         file.Name,
			Path:         file.Path,
			RelativePath: file.RelativePath,
			Size:         file.Size,
		}
		switch file.Format {
		case "XML":
			v.HasXML = true
			v.XMLFile = fileInfo
		case "CDR":
			v.HasCDR = true
			v.CDRFile = fileInfo
		case "PDF":
			v.HasPDF = true
			v.PDFFile = fileInfo
		}
	}

	matchedVouchers := make([]ArchiveVoucherItem, 0, len(voucherKeys))
	for _, k := range voucherKeys {
		matchedVouchers = append(matchedVouchers, *voucherMap[k])
	}
	sort.Slice(matchedVouchers, func(i, j int) bool {
		return matchedVouchers[i].ModifiedAt.After(matchedVouchers[j].ModifiedAt)
	})

	total := len(matchedFiles)
	totalVouchers := len(matchedVouchers)
	totalPages := (totalVouchers + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	vStart := (page - 1) * pageSize
	vEnd := vStart + pageSize
	if vStart > totalVouchers {
		vStart = totalVouchers
	}
	if vEnd > totalVouchers {
		vEnd = totalVouchers
	}

	respondJSON(w, http.StatusOK, ArchiveFilesResponse{
		Success:       true,
		Company:       filterCompany,
		Book:          filterBook,
		Period:        filterPeriod,
		Total:         total,
		TotalVouchers: totalVouchers,
		Page:          page,
		PageSize:      pageSize,
		TotalPages:    totalPages,
		Files:         matchedFiles[start:end],
		Vouchers:      matchedVouchers[vStart:vEnd],
		Summary:       summary,
	})
}

// HandleArchiveOpen abre la carpeta indicada en el Explorador de Windows
func (s *Server) HandleArchiveOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	targetPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if targetPath == "" {
		targetPath = s.fileManager.BaseDir
	} else {
		// Si es una ruta relativa o un subdirectorio
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(s.fileManager.BaseDir, targetPath)
		}
		// Validar que esté dentro de BaseDir
		baseAbs, err := filepath.Abs(s.fileManager.BaseDir)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Error obteniendo ruta base")
			return
		}
		targetAbs, err := filepath.Abs(targetPath)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Ruta inválida")
			return
		}
		rel, err := filepath.Rel(baseAbs, targetAbs)
		if err != nil || strings.HasPrefix(rel, "..") {
			respondError(w, http.StatusForbidden, "Acceso fuera de la carpeta de descargas no permitido")
			return
		}
		targetPath = targetAbs
	}

	// Si es un archivo regular, abrir su carpeta contenedora
	if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
		targetPath = filepath.Dir(targetPath)
	}

	err := s.fileManager.OpenInExplorer(targetPath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "No se pudo abrir la carpeta: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Carpeta abierta en el Explorador",
		"path":    targetPath,
	})
}

// HandleArchiveDownloadZip empaqueta y descarga como ZIP la carpeta solicitada
func (s *Server) HandleArchiveDownloadZip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Método no permitido")
		return
	}

	company := strings.TrimSpace(r.URL.Query().Get("company"))
	book := strings.TrimSpace(r.URL.Query().Get("book"))
	period := strings.TrimSpace(r.URL.Query().Get("period"))

	targetDir := filepath.Join(s.fileManager.BaseDir, "APP DESCARGAS", "CPE")
	if company != "" {
		targetDir = filepath.Join(targetDir, company)
		if book != "" {
			targetDir = filepath.Join(targetDir, book)
			if period != "" {
				targetDir = filepath.Join(targetDir, period)
			}
		}
	}

	info, err := os.Stat(targetDir)
	if err != nil || !info.IsDir() {
		respondError(w, http.StatusNotFound, "La carpeta solicitada no existe")
		return
	}

	zipName := fmt.Sprintf("Archivador_%s_%s_%s.zip", cleanFileName(company), cleanFileName(book), cleanFileName(period))
	zipName = strings.Trim(zipName, "_")
	if zipName == ".zip" || zipName == "Archivador__.zip" {
		zipName = "Archivador_Digital.zip"
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", zipName))

	zw := zip.NewWriter(w)
	defer zw.Close()

	walker := func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(targetDir, path)
		if err != nil {
			return nil
		}
		header, err := zip.FileInfoHeader(fi)
		if err != nil {
			return nil
		}
		header.Name = filepath.ToSlash(relPath)
		header.Method = zip.Deflate

		writer, err := zw.CreateHeader(header)
		if err != nil {
			return nil
		}

		source, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer source.Close()

		_, _ = io.Copy(writer, source)
		return nil
	}

	_ = filepath.Walk(targetDir, walker)
}

func parseArchiveFileName(name string) ArchiveFileItem {
	item := ArchiveFileItem{
		Name:   name,
		Format: "OTRO",
	}

	ext := strings.ToLower(filepath.Ext(name))

	// Caso CDR: R-{RUC}-{TIPO}-{SERIE}-{NUMERO}
	if m := rxCdrDoc.FindStringSubmatch(name); len(m) == 6 {
		item.RUC = m[1]
		item.Tipo = m[2]
		item.TipoNombre = tipoDocumentoNombre(m[2])
		item.Serie = m[3]
		item.Numero = m[4]
		item.Format = "CDR"
		return item
	}

	// Caso Estándar SUNAT: {RUC}-{TIPO}-{SERIE}-{NUMERO}
	if m := rxStandardDoc.FindStringSubmatch(name); len(m) == 6 {
		item.RUC = m[1]
		item.Tipo = m[2]
		item.TipoNombre = tipoDocumentoNombre(m[2])
		item.Serie = m[3]
		item.Numero = m[4]
		if ext == ".pdf" {
			item.Format = "PDF"
		} else if ext == ".zip" || strings.HasPrefix(strings.ToUpper(name), "R-") {
			item.Format = "CDR"
		} else {
			item.Format = "XML"
		}
		return item
	}

	// Caso Alternativo: FACTURAE001-11020610234471
	if m := rxAltDoc.FindStringSubmatch(name); len(m) == 6 {
		docWord := strings.ToUpper(m[1])
		switch docWord {
		case "FACTURA":
			item.Tipo = "01"
			item.TipoNombre = "Factura"
		case "BOLETA":
			item.Tipo = "03"
			item.TipoNombre = "Boleta"
		case "NOTA_CREDITO":
			item.Tipo = "07"
			item.TipoNombre = "Nota de Crédito"
		case "NOTA_DEBITO":
			item.Tipo = "08"
			item.TipoNombre = "Nota de Débito"
		}
		item.Serie = m[2]
		item.Numero = m[3]
		item.RUC = m[4]
		if ext == ".pdf" {
			item.Format = "PDF"
		} else {
			item.Format = "XML"
		}
		return item
	}

	// Fallback por extensión
	if ext == ".xml" {
		item.Format = "XML"
	} else if ext == ".pdf" {
		item.Format = "PDF"
	} else if ext == ".zip" {
		item.Format = "CDR"
	}
	item.TipoNombre = "Comprobante"
	return item
}

func tipoDocumentoNombre(tipo string) string {
	switch tipo {
	case "01":
		return "Factura"
	case "03":
		return "Boleta"
	case "07":
		return "Nota Crédito"
	case "08":
		return "Nota Débito"
	case "09":
		return "Guía Remisión"
	case "20":
		return "Retención"
	case "40":
		return "Percepción"
	case "30":
		return "Comprobante Esp."
	default:
		return "Tipo " + tipo
	}
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func cleanFileName(name string) string {
	name = strings.TrimSpace(name)
	invalidChars := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	for _, char := range invalidChars {
		name = strings.ReplaceAll(name, char, "_")
	}
	return name
}

func formatPeriodLabel(period string) string {
	if len(period) == 6 {
		y := period[:4]
		m := period[4:]
		meses := map[string]string{
			"01": "Ene", "02": "Feb", "03": "Mar", "04": "Abr",
			"05": "May", "06": "Jun", "07": "Jul", "08": "Ago",
			"09": "Set", "10": "Oct", "11": "Nov", "12": "Dic",
		}
		if mesNombre, ok := meses[m]; ok {
			return fmt.Sprintf("%s-%s", mesNombre, y)
		}
	}
	return period
}
