# AppSire CPE - Replicador de Descarga Masiva SUNAT en Go

Aplicación web funcional y directa para la **descarga masiva de comprobantes de pago electrónicos (PDF, XML, CDR)** desde los servicios web de SUNAT, replicando la lógica analizada de la macro/VSTO .NET.

---

## Características Implementadas

1. **Acceso y licencia AutoSire**:
   - Pantalla de login vinculada al servidor central de licencias AutoSire.
   - Sesión local mediante cookie `HttpOnly` y validación periódica de vigencia o revocación.
   - Opción de recordar el usuario y la licencia sin guardar la contraseña de AutoSire.

2. **Empresas en SQLite con credenciales protegidas**:
   - Catálogo local para guardar, seleccionar y eliminar empresas.
   - Clave SOL y Client Secret cifrados con Windows DPAPI; no se devuelven al navegador.
   - La empresa seleccionada se conecta automáticamente con SUNAT al abrir AutoSire.
   - Base local en `%LOCALAPPDATA%\AutoSire\data\autosire.db`.

3. **Autenticación OAuth2 SUNAT en memoria**:
   - Endpoint: `POST https://api-seguridad.sunat.gob.pe/v1/clientessol/{clientId}/oauth2/token/`
   - Parámetros: `grant_type=password`, `scope=https://api-cpe.sunat.gob.pe/`, usuario (`RUC + UsuarioSOL`) y clave SOL.
   - Decodificación y verificación de JWT en memoria con renovación automática antes de expirar (margen de 5 min).
   - Sin base de datos: las credenciales se usan en la sesión activa y opcionalmente se recuerdan en el navegador vía LocalStorage.

4. **Descarga Masiva Concurrente**:
   - Pool de workers concurrentes (por defecto 6 hilos, configurable de 1 a 12).
   - Reintentos automáticos con backoff exponencial y jitter aleatorio para errores transitorios de SUNAT (429, 500, 502, 503, 504, 408).
   - Descarga de **PDF** (`/consultacpe/comprobantes/{cpe}/01`).
   - Descarga de **XML** (`/consultacpe/comprobantes/{cpe}/02` con fallback automático a `/controlcpe/consultaxml/{cpe}`).
   - Descarga de **CDR** (`/consultacpe/comprobantes/{cpe}/03`).
   - Detección y extracción automática de archivos comprimidos ZIP (magic bytes `PK`).

5. **Lector Inteligente de Excel (.xlsx)**:
   - Compatible con las hojas generadas por AppSireCPE y SIRE:
     - `rvie` (Registro de Ventas e Ingresos Electrónicos)
     - `cpe` (Comprobantes de Pago / Compras)
     - `rxh` (Recibos por Honorarios)
     - `liqcom` (Liquidaciones de Compra)
   - Extracción de metadatos de cabecera (RUC, Razón Social, Periodo tributario).
   - Autodetección de columnas si el archivo no sigue el orden exacto.

6. **Organización de Archivos en Disco**:
   - Mantiene la estructura del sistema original:
     ```
     downloads/APP DESCARGAS/CPE/{RUC} {RazonSocial}/{Compras|Ventas}/{Periodo}/
     ```
   - Nombres estándar: `{RUC}-{Tipo}-{Serie}-{Numero}.pdf/.xml` y `R-{RUC}-{Tipo}-{Serie}-{Numero}.zip`.

7. **Interfaz Web Directa**:
   - Carga de Excel con Drag & Drop.
   - Visualización de estadísticas en tiempo real (Progreso, Exitosos, Errores).
   - Tabla de resultados en vivo.
   - Terminal de eventos/logs en vivo.
   - Botón para **Abrir Carpeta en Windows** (`explorer.exe`).
   - Botón para **Descargar Todo en un solo archivo ZIP** desde el navegador.

8. **Descarga de propuestas SUNAT SIRE**:
   - Generación y descarga directa de la propuesta de compras **RCE** y ventas **RVIE**.
   - Sigue el flujo de la macro: solicitud de ticket, espera del proceso masivo y descarga del ZIP oficial.
   - Reintenta respuestas temporales de SUNAT y puede reutilizar la última propuesta terminada del periodo.
   - Guarda los archivos en `downloads/APP DESCARGAS/SIRE/{RUC}/{RCE|RVIE}/{Periodo}/`.

---

## Cómo Ejecutar la Aplicación

### Opción 1: Con el archivo ejecutable compilado
Haga doble clic sobre el archivo:
```
c:\Users\USER\AppData\Local\Programs\AppSireCPE\appsire-go\iniciar.bat
```
Esto abrirá automáticamente su navegador en `http://localhost:8080` y comenzará el servidor.

### Opción 2: Desde la consola PowerShell / CMD
```powershell
cd c:\Users\USER\AppData\Local\Programs\AppSireCPE\appsire-go
.\appsire-server.exe -port 8080
```
Luego ingrese a [http://localhost:8080](http://localhost:8080) en su navegador web.
