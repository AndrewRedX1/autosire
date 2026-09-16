package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"appsire-go/internal/api"
	"appsire-go/internal/auth"
	"appsire-go/internal/company"
	"appsire-go/internal/engine"
	"appsire-go/internal/excel"
	"appsire-go/internal/filemanager"
	"appsire-go/internal/license"
	"appsire-go/internal/secrets"
	"appsire-go/internal/session"
	"appsire-go/internal/sunat"
)

func main() {
	port := flag.Int("port", 8085, "Puerto de escucha del servidor web")
	downloadDir := flag.String("downloads", "./downloads", "Directorio base donde se guardarán los archivos descargados")
	dataDir := flag.String("data", defaultDataDir(), "Directorio de datos locales de AutoSire")
	noBrowser := flag.Bool("no-browser", false, "No abrir automáticamente el navegador")
	flag.Parse()

	// Resolver ruta absoluta de downloads
	absDownloads, err := filepath.Abs(*downloadDir)
	if err != nil {
		absDownloads = *downloadDir
	}
	_ = os.MkdirAll(absDownloads, 0755)
	if err := os.MkdirAll(*dataDir, 0700); err != nil {
		log.Fatalf("Error creando directorio de datos: %v", err)
	}
	if filepath.Clean(*dataDir) == filepath.Clean(defaultDataDir()) {
		migrateLegacyLicense(*dataDir)
	}

	// Inicializar dependencias y servicios
	tokenService := auth.NewTokenService(30 * time.Second)
	// El macro limita la recepción a 30 segundos. Un timeout mayor deja los
	// seis workers bloqueados y degrada fuertemente la cola al final del lote.
	sunatClient := sunat.NewSunatClient(tokenService, 30*time.Second)
	fileMgr := filemanager.NewFileManager(absDownloads)
	excelRdr := excel.NewExcelReader()
	dlEngine := engine.NewDownloadEngine(sunatClient, fileMgr)
	licMgr := license.NewLicenseManager(*dataDir)
	proposalClient := sunat.NewProposalClient(tokenService, 5*time.Minute)
	protector := secrets.NewProtector(licMgr.GetMachineID())
	companyStore, err := company.Open(
		context.Background(),
		filepath.Join(*dataDir, "autosire.db"),
		protector,
	)
	if err != nil {
		log.Fatalf("Error inicializando empresas: %v", err)
	}
	defer companyStore.Close()
	sessionMgr := session.NewManager()

	server := api.NewServer(
		tokenService,
		excelRdr,
		dlEngine,
		fileMgr,
		licMgr,
		proposalClient,
		companyStore,
		sessionMgr,
	)

	mux := http.NewServeMux()

	// Rutas de Licencia AutoSire
	mux.HandleFunc("/api/session", server.HandleSession)
	mux.HandleFunc("/api/session/login", server.HandleSessionLogin)
	mux.HandleFunc("/api/session/logout", server.RequireSession(server.HandleSessionLogout))
	mux.HandleFunc("/api/license/status", server.HandleLicenseStatus)
	mux.HandleFunc("/api/license/activate", server.HandleSessionLogin)

	// Rutas API REST
	mux.HandleFunc("/api/companies", server.RequireSession(server.HandleCompanies))
	mux.HandleFunc("/api/companies/select", server.RequireSession(server.HandleCompanySelect))
	mux.HandleFunc("/api/companies/token", server.RequireSession(server.HandleCompanyToken))
	mux.HandleFunc("/api/companies/delete", server.RequireSession(server.HandleCompanyDelete))
	mux.HandleFunc("/api/companies/delete-multiple", server.RequireSession(server.HandleCompanyDeleteMultiple))
	mux.HandleFunc("/api/auth/token", server.RequireSession(server.HandleAuthToken))
	mux.HandleFunc("/api/auth/status", server.RequireSession(server.HandleAuthStatus))
	mux.HandleFunc("/api/excel/upload", server.RequireSession(server.HandleExcelUpload))
	mux.HandleFunc("/api/excel/template", server.RequireSession(server.HandleDownloadTemplate))
	mux.HandleFunc("/api/sire/proposal/download", server.RequireSession(server.HandleDownloadSireProposal))
	mux.HandleFunc("/api/download/start", server.RequireSession(server.HandleStartDownload))
	mux.HandleFunc("/api/download/item", server.RequireSession(server.HandleDownloadItem))
	mux.HandleFunc("/api/download/status", server.RequireSession(server.HandleDownloadStatus))
	mux.HandleFunc("/api/download/events", server.RequireSession(server.HandleDownloadEvents))
	mux.HandleFunc("/api/download/cancel", server.RequireSession(server.HandleCancelDownload))
	mux.HandleFunc("/api/files/open-folder", server.RequireSession(server.HandleOpenFolder))
	mux.HandleFunc("/api/files/download-zip", server.RequireSession(server.HandleDownloadZip))
	mux.HandleFunc("/api/files/view", server.RequireSession(server.HandleViewFile))
	mux.HandleFunc("/api/files/xml-preview", server.RequireSession(server.HandleXMLPreview))
	// Rutas de Ciclo de Vida de la Ventana (Watchdog de escritorio)
	var (
		shutdownMu    sync.Mutex
		shutdownTimer *time.Timer
	)

	resetShutdownTimer := func(d time.Duration) {
		shutdownMu.Lock()
		defer shutdownMu.Unlock()
		if shutdownTimer != nil {
			shutdownTimer.Stop()
		}
		shutdownTimer = time.AfterFunc(d, func() {
			os.Exit(0)
		})
	}

	// 120 segundos de gracia inicial para que la ventana abra y cargue
	resetShutdownTimer(120 * time.Second)

	mux.HandleFunc("/api/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		resetShutdownTimer(120 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		resetShutdownTimer(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	// Servir archivos estáticos del Frontend
	fs := http.FileServer(resolveStaticDir())
	mux.Handle("/", fs)

	// Middleware de actividad: refresca el temporizador en cualquier interacción
	activityHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resetShutdownTimer(120 * time.Second)
		mux.ServeHTTP(w, r)
	})

	listener, activePort, err := createListener(*port)
	if err != nil {
		log.Fatalf("Error asignando puerto de red: %v", err)
	}

	appURL := fmt.Sprintf("http://127.0.0.1:%d", activePort)
	fmt.Println("=================================================================")
	fmt.Printf("   🚀 AutoSire - Gestión y descarga SUNAT (Modo Escritorio)\n")
	fmt.Println("=================================================================")
	fmt.Printf("   🌐 Aplicación activa en: %s\n", appURL)
	fmt.Printf("   📁 Carpeta de descargas: %s\n", absDownloads)
	fmt.Println("   💡 Para cerrar la aplicación, cierre la ventana principal")
	fmt.Println("=================================================================")

	// Abrir la ventana nativa de escritorio independiente (Modo App)
	if !*noBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openDesktopApp(appURL)
		}()
	}

	httpServer := &http.Server{
		Handler:           api.SecurityHeaders(api.SameOrigin(activityHandler)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      6 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
	if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Error iniciando servidor web: %v", err)
	}
}

// createListener intenta el puerto preferido, y si está ocupado busca automáticamente otro libre
func createListener(preferredPort int) (net.Listener, int, error) {
	// 1. Probar el puerto preferido
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferredPort))
	if err == nil {
		return l, preferredPort, nil
	}

	// 2. Si está ocupado, probar secuencialmente los siguientes 25 puertos
	for p := preferredPort + 1; p < preferredPort+25; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return l, p, nil
		}
	}

	// 3. Si ninguno está libre, pedir al sistema operativo un puerto efímero garantizado
	l, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, err
	}
	return l, l.Addr().(*net.TCPAddr).Port, nil
}

func resolveStaticDir() http.FileSystem {
	// 1. Probar ruta de trabajo actual
	if fi, err := os.Stat("./web/static"); err == nil && fi.IsDir() {
		return http.Dir("./web/static")
	}
	// 2. Probar relativo a la ubicación física del ejecutable .exe
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidate := filepath.Join(exeDir, "web", "static")
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return http.Dir(candidate)
		}
	}
	return http.Dir("./web/static")
}

// openDesktopApp abre la aplicación en su propia ventana nativa de escritorio independiente (Modo App)
func openDesktopApp(targetURL string) {
	if runtime.GOOS == "windows" {
		var candidates []string

		baseDirs := []string{
			os.Getenv("ProgramFiles(x86)"),
			os.Getenv("ProgramFiles"),
			os.Getenv("LOCALAPPDATA"),
		}

		// 1. Microsoft Edge (presente en 100% de Windows 10 y 11)
		for _, base := range baseDirs {
			if base != "" {
				candidates = append(candidates, filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"))
			}
		}

		// 2. Google Chrome
		for _, base := range baseDirs {
			if base != "" {
				candidates = append(candidates, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
			}
		}

		for _, browserPath := range candidates {
			if fi, err := os.Stat(browserPath); err == nil && !fi.IsDir() {
				cmd := exec.Command(
					browserPath,
					"--app="+targetURL,
					"--window-size=1280,840",
					"--no-first-run",
					"--no-default-browser-check",
				)
				if err := cmd.Start(); err == nil {
					return
				}
			}
		}

		// Fallback: Navegador predeterminado
		_ = exec.Command("cmd", "/c", "start", "", targetURL).Start()
		return
	}

	if runtime.GOOS == "darwin" {
		_ = exec.Command("open", targetURL).Start()
		return
	}

	_ = exec.Command("xdg-open", targetURL).Start()
}

func defaultDataDir() string {
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return filepath.Join(localAppData, "AutoSire", "data")
	}
	return filepath.Join(".", "data")
}

func migrateLegacyLicense(dataDir string) {
	target := filepath.Join(dataDir, license.LicenseFileName)
	if _, err := os.Stat(target); err == nil {
		return
	}
	content, err := os.ReadFile(license.LicenseFileName)
	if err != nil {
		return
	}
	_ = os.WriteFile(target, content, 0600)
}
