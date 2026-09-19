---
name: golang-observability
description: >-
  Logs estructurados (log/slog), métricas, perfiles de rendimiento (pprof), trazas y alertas en Go.
  Buenas prácticas para diagnóstico en producción, detección de cuellos de botella y monitorización.
---

# Go Observability Guide

Esta skill proporciona pautas para instrumentar aplicaciones en Go mediante registro estructurado, perfiles de ejecución y monitorización de salud.

---

## 1. Logs Estructurados con `log/slog`

A partir de Go 1.21, `log/slog` es la biblioteca estándar recomendada para logs con pares clave-valor estructurados:

```go
import "log/slog"

// Configurar handler JSON o Text según el entorno
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
slog.SetDefault(logger)

// Log estructurado con contexto
slog.InfoContext(ctx, "iniciando validación de lote CPE",
    slog.String("ruc", ruc),
    slog.Int("total_docs", len(docs)),
    slog.Duration("timeout", timeout),
)

// Log de error con cadena de causa
if err != nil {
    slog.ErrorContext(ctx, "falló la comunicación con SUNAT",
        slog.Any("error", err),
        slog.String("ruc", ruc),
    )
}
```

---

## 2. Perfilado de Rendimiento con `net/http/pprof`

- **Habilitación segura en servidores backend:**
  ```go
  import _ "net/http/pprof"
  ```
- **Análisis de consumo de memoria y CPU:**
  ```bash
  # Inspeccionar perfil de CPU durante 30 segundos
  go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
  
  # Inspeccionar asignaciones de memoria en vivo
  go tool pprof http://localhost:8080/debug/pprof/heap
  ```
- En producción, restringir el endpoint `/debug/pprof/` a la interfaz de loopback (`localhost`) o mediante autenticación administrativa.

---

## 3. Endpoints de Salud (`Healthchecks`)

- Proveer endpoints `/api/health` o `/api/ready`:
  - `Health`: Verifica que el proceso esté activo y respondiendo.
  - `Ready`: Verifica que las dependencias críticas (base de datos SQLite accesible, conexiones de red listas) estén operativas.
