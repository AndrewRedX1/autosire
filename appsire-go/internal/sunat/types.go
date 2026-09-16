package sunat

import "time"

// TipoDescarga define qué formato descargar
type TipoDescarga string

const (
	DescargaPDF TipoDescarga = "PDF"
	DescargaXML TipoDescarga = "XML"
	DescargaCDR TipoDescarga = "CDR"
)

// SunatCredentials contiene las credenciales requeridas para autenticarse en SUNAT
type SunatCredentials struct {
	RUC          string `json:"ruc"`
	UsuarioSOL   string `json:"usuarioSol"`
	ClaveSOL     string `json:"claveSol"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// TokenResponse respuesta del endpoint OAuth2 de SUNAT
type TokenResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresIn   int64     `json:"expires_in"`
	ExpiresAt   time.Time `json:"expires_at"`
	RUC         string    `json:"ruc,omitempty"`
}

// Comprobante representa un documento a descargar extraído de Excel o API
type Comprobante struct {
	ID           string `json:"id"`                             // Identificador único (RUC-Tipo-Serie-Numero)
	RUC          string `json:"ruc"`                            // RUC del emisor o receptor según tipo de libro
	Tipo         string `json:"tipo"`                           // 01, 03, 07, 08, etc.
	Serie        string `json:"serie"`                          // E001, F001, B001, etc.
	Numero       string `json:"numero"`                         // 1, 00000001, etc.
	Libro        string `json:"libro"`                          // 1=Ventas, 2=Compras
	Periodo      string `json:"periodo"`                        // YYYYMM (ej. 202401)
	RazonSocial  string `json:"razon_social"`                   // Nombre de empresa
	EmpresaRUC   string `json:"empresa_ruc,omitempty"`          // RUC de la empresa dueña del lote
	EmpresaRazon string `json:"empresa_razon_social,omitempty"` // Razón social dueña del lote
	FechaEmision string `json:"fecha_emision"`                  // YYYY-MM-DD
	Monto        string `json:"monto"`
	Moneda       string `json:"moneda"`
	RowIndex     int    `json:"row_index"`  // Fila de origen en el Excel
	SheetName    string `json:"sheet_name"` // Hoja de origen
}

// ApiResponseCpe estructura devuelta por el API de consulta CPE de SUNAT
type ApiResponseCpe struct {
	NomArchivo   string `json:"nomArchivo"`
	ValArchivo   string `json:"valArchivo"` // Base64
	CodRespuesta string `json:"codRespuesta,omitempty"`
	MsgRespuesta string `json:"msgRespuesta,omitempty"`
}

// ItemResult resultado individual de la descarga de un comprobante
type ItemResult struct {
	Comprobante Comprobante  `json:"comprobante"`
	Tipo        TipoDescarga `json:"tipo"`
	Exito       bool         `json:"exito"`
	NomArchivo  string       `json:"nom_archivo"`
	RutaLocal   string       `json:"ruta_local"`
	TamanoBytes int64        `json:"tamano_bytes"`
	Error       string       `json:"error,omitempty"`
	Reintentos  int          `json:"reintentos"`
	Origen      string       `json:"origen,omitempty"`
	Categoria   string       `json:"categoria,omitempty"`
	EstadoCDR   string       `json:"estado_cdr,omitempty"`   // "ACEPTADO", "RECHAZADO", "OBSERVADO"
	CodigoCDR   string       `json:"codigo_cdr,omitempty"`   // "0", "2xxx", etc.
	MensajeCDR  string       `json:"mensaje_cdr,omitempty"`  // Descripción oficial del CDR
	DigestValue string       `json:"digest_value,omitempty"` // Hash digital extraído del XML
}

// BatchStatus estado general de una sesión de descargas
type BatchStatus struct {
	BatchID                string         `json:"batch_id"`
	Estado                 string         `json:"estado"` // "iniciando", "procesando", "completado", "error", "detenido"
	TotalItems             int            `json:"total_items"`
	Procesados             int            `json:"procesados"`
	Exitosos               int            `json:"exitosos"`
	Errores                int            `json:"errores"`
	Porcentaje             float64        `json:"porcentaje"`
	Mensaje                string         `json:"mensaje"`
	IniciadoEn             time.Time      `json:"iniciado_en"`
	FinalizadoEn           *time.Time     `json:"finalizado_en,omitempty"`
	Resultados             []ItemResult   `json:"resultados,omitempty"`
	VelocidadItemsSeg      float64        `json:"velocidad_items_seg"`
	TiempoRestanteEstimado string         `json:"tiempo_restante"`
	HilosActivos           int            `json:"hilos_activos"`
	ManifestPath           string         `json:"manifest_path,omitempty"`
	ResumenCategorias      map[string]int `json:"resumen_categorias,omitempty"`
}
