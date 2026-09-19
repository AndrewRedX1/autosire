---
name: golang-project-layout
description: >-
  Organización de paquetes, módulos, monorepos y aplicaciones CLI en Go.
  Estructura estándar de proyectos Go (/cmd, /internal, /pkg, /web), demarcación de visibilidad y arquitectura limpia.
---

# Go Project Layout Guide

Esta skill define la organización estructural recomendada para proyectos y servicios en Go, basada en el estándar comunitario `golang-standards/project-layout`.

---

## 1. Directorios Principales

```
my-app/
├── cmd/
│   ├── server/           # Punto de entrada para el servidor backend (main.go)
│   └── cli/              # Punto de entrada para utilitarios de línea de comandos
├── internal/             # Código privado de la aplicación; el compilador prohíbe su importación externa
│   ├── api/              # Routers, middlewares y handlers HTTP/REST
│   ├── config/           # Carga de variables de entorno y ficheros de configuración
│   ├── company/          # Lógica de dominio y acceso a datos de empresas
│   ├── cpe/              # Dominio de comprobantes de pago electrónico (SUNAT)
│   ├── db/               # Inicialización de base de datos, SQLite, migraciones
│   └── validation/       # Validadores de tipos de cambio, reglas de negocio
├── pkg/                  # Código público reutilizable por proyectos externos (solo si aplica)
├── web/                  # Activos estáticos, plantillas HTML, CSS y JavaScript embebidos
│   ├── static/           # CSS, JS, imágenes
│   └── templates/        # Plantillas HTML (si aplica)
├── go.mod                # Definición de módulo y dependencias directas
├── go.sum                # Sumas de verificación de dependencias
└── README.md
```

---

## 2. Reglas de Diseño y Paquetes

### `cmd/`
- El directorio `cmd/` contiene exclusivamente puntos de entrada (`main.go`).
- El código dentro de `main()` debe ser mínimo: orquestar la inyección de dependencias, leer la configuración e iniciar el ciclo de vida del servicio.
- Toda lógica de negocio debe delegarse a paquetes dentro de `internal/`.

### `internal/`
- Protege la arquitectura interna impidiendo que otros módulos externos dependan directamente de implementaciones privadas.
- Los subpaquetes deben tener una responsabilidad única y cohesionada (`internal/cpe`, `internal/api`, etc.).
- Las dependencias deben fluir hacia adentro: `api` depende del dominio, pero el dominio nunca depende de `api` ni de HTTP.

### Inserción de Recursos con `embed` (`//go:embed`)
- Para aplicaciones de escritorio y servicios autocontenidos, utilizar el paquete nativo `embed` de Go para empaquetar la carpeta `web/static/` directamente dentro del binario ejecutable (`.exe`).
  ```go
  //go:embed web/static/*
  var staticFiles embed.FS
  ```
