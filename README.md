# 🚀 AutoSire v1.0 — Suite de Gestión y Descarga Masiva SUNAT

Sistema integral de escritorio y servidor central para la **gestión, consulta y descarga masiva de comprobantes de pago electrónicos (CPE)** y **propuestas SIRE (RCE y RVIE)** de SUNAT Perú.

---

## 🏗️ Arquitectura del Proyecto

El repositorio está organizado en dos componentes principales:

```
autosire/
├── appsire-go/         # 💻 Aplicativo de Escritorio (Cliente Windows en Go)
│   ├── cmd/server/     # Punto de entrada y servidor local integrado
│   ├── internal/       # Motores de SUNAT, Auth, Licencias, Excel y Cifrado
│   ├── web/static/     # Interfaz moderna de escritorio (HTML5, CSS3, JS)
│   ├── go.mod
│   └── iniciar.bat     # Lanzador auxiliar
│
├── autosire/           # 🛡️ Servidor Central de Licencias y Control (Docker / VPS)
│   ├── cmd/server/     # API central de licencias y panel administrativo
│   ├── internal/db/    # Persistencia de clientes, planes y vigencia
│   ├── web/static/     # Panel Web de Administración de Licencias
│   ├── Dockerfile      # Empaquetado ligero para producción (Alpine Linux)
│   ├── docker-compose.yml
│   └── go.mod
│
├── AutoSire-App.ico    # Icono oficial de la aplicación
└── plantilla_comprobantes.xlsx # Plantilla oficial de importación Excel
```

---

## 💻 1. Aplicativo de Escritorio (`appsire-go`)

El aplicativo cliente para contadores y empresas:

* **Modo Ventana Nativa:** Se ejecuta como aplicación de escritorio independiente (sin barra de direcciones ni pestañas, usando Chromium/Edge App Mode).
* **Cero Consola Negra:** Compilado como GUI nativa de Windows (`-H windowsgui`), con cierre automático del proceso local al cerrar la ventana.
* **Tolerancia a Fallos de Puertos:** Asignación dinámica de puerto con fallback automático para evitar conflictos con otros servicios (Docker, IIS, etc.).
* **Seguridad Local:** Claves SOL y credenciales de API protegidas con cifrado **Windows DPAPI** (las credenciales nunca se transmiten al exterior).
* **Descarga Masiva Concurrente:** Pool de workers (6 hilos por defecto) con reintentos exponenciales y jitter para PDF, XML y CDR.
* **Propuestas SIRE SUNAT:** Descarga oficial automatizada de propuestas de Compras (**RCE**) y Ventas (**RVIE**) mediante tickets masivos.

### Compilar el Ejecutable de Escritorio:
```powershell
cd appsire-go
go build -ldflags="-H windowsgui -s -w" -o AutoSire.exe ./cmd/server
```

---

## 🛡️ 2. Servidor Central de Licencias (`autosire`)

Servidor para desplegar en tu VPS (o Docker local):

* **Panel de Administración Web:** Gestión de clientes, asignación de planes y vigencia en días.
* **Bloqueo por Hardware (Machine ID):** Vincula cada licencia al hardware físico de la PC del cliente para impedir la redistribución no autorizada.
* **Control en Tiempo Real:** Activación, verificación periódica y revocación instantánea de licencias.
* **Listo para Docker:**
  ```bash
  cd autosire
  docker compose up -d --build
  ```
  Disponible en el puerto `8090` (`http://tuvps:8090`).

---

## 👥 Créditos y Autoría
* **Desarrollado por:** [AndrewRedX1](https://github.com/AndrewRedX1)
* **Versión:** 1.0.0
* **Licencia:** Propietaria
