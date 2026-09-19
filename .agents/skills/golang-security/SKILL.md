---
name: golang-security
description: >-
  Prácticas y estándares de seguridad para aplicaciones en Go (Golang).
  Prevención de vulnerabilidades (SQL injection, SSRF, Command Injection, Memory Leaks, DoS),
  manejo seguro de credenciales, límites en I/O, configuración segura de HTTP servers/clients y criptografía.
---

# Go Security Best Practices Guide

Esta skill proporciona las directrices y estándares obligatorios de seguridad en el ecosistema Go (Golang).

---

## 1. Prevención de Ataques de Inyección

### Inyecciones SQL / Consultas
- **Siempre usar consultas parametrizadas:** Nunca concatenar cadenas en sentencias SQL (`fmt.Sprintf("SELECT ... WHERE id = '%s'", id)` está estrictamente prohibido).
  ```go
  // BIEN: Argumentos parametrizados (?) o ($1)
  row := db.QueryRowContext(ctx, "SELECT id, name FROM companies WHERE ruc = ?", ruc)
  ```

### Inyecciones de Comandos del Sistema
- **Evitar la invocación directa de shells:** Utilizar `exec.CommandContext(ctx, name, args...)` pasando los argumentos como elementos separados del slice, nunca ejecutar a través de `cmd.exe /c` o `sh -c` con datos no confiables.
- **Validación estricta de rutas y binarios:** Validar que los ejecutables provengan de rutas conocidas y permitidas.

---

## 2. Protección contra DoS (Denial of Service) y Consumo Excesivo de Memoria

### Límites en el Cuerpo de Peticiones HTTP
- **Nunca leer `r.Body` sin límite:** Usar `http.MaxBytesReader` para proteger los endpoints de desbordamiento de memoria por peticiones maliciosas de gran tamaño.
  ```go
  r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // Límite de 10 MB
  ```

### Descompresión Segura (Zip Bomb Protection)
- Limitar el tamaño total extraído y la cantidad de ficheros al procesar archivos ZIP o GZIP (como los CDRs o XMLs de SUNAT).
- Usar `io.LimitReader` para restringir la lectura acumulada.

---

## 3. Timeouts Obligatorios en HTTP y Contextos

### Servidores HTTP
- **Nunca usar `http.ListenAndServe` con configuración por defecto:** Configurar siempre tiempos límite explícitos:
  ```go
  srv := &http.Server{
      Addr:         ":8080",
      Handler:      router,
      ReadTimeout:  15 * time.Second,
      WriteTimeout: 60 * time.Second,
      IdleTimeout:  120 * time.Second,
  }
  ```

### Clientes HTTP
- **Nunca usar `http.DefaultClient`:** Configurar un `&http.Client{}` con `Timeout` configurado y `Transport` con límites de conexiones inactivas e handshake.
- Pasar siempre `context.WithTimeout` a cada petición.

---

## 4. Manejo Seguro de Secretos y Credenciales

- **No almacenar secretos en texto plano en repositorios ni logs:**
  - Enmascarar contraseñas, tokens y claves de API antes de enviarlos a logs estructurados (`slog`).
  - Implementar la interfaz `fmt.Stringer` en tipos de datos sensibles para evitar impresiones accidentales (`return "[REDACTED]"`).
- **Almacenamiento seguro:**
  - Almacenar credenciales en almacenamiento local protegido o bases de datos con permisos de archivo restringidos (`0600`).
  - Limpiar buffers y tokens en memoria cuando ya no sean requeridos si contienen material criptográfico sensible.

---

## 5. Prevención de SSRF (Server-Side Request Forgery)

- Validar y restringir los endpoints externos a los cuales la aplicación realiza peticiones HTTP.
- Para integración con APIs externas (como SUNAT):
  - Mantener listas blancas de dominios/URLs base autorizados.
  - No permitir que el usuario pase URLs arbitrarias completas como parámetros no validados.

---

## 6. Criptografía y Generación de Números Aleatorios

- Para tokens de seguridad, identificadores de sesión o claves: usar siempre `crypto/rand`, nunca `math/rand`.
- Para hashing de contraseñas: utilizar `bcrypt` o `argon2id` con factores de coste adecuados.
