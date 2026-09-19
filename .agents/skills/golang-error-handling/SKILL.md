---
name: golang-error-handling
description: >-
  Directrices para creación, propagación, inspección, envoltorio y registro estructurado de errores
  en Go (Golang). Usar al diseñar APIs, implementar control de fallos y diagnosticar errores.
---

# Go Error Handling Standards & Patterns

Esta guía detalla el manejo idiomático de errores en proyectos Go de producción.

---

## 1. Principios Fundamentales
- **Los errores son valores:** Tratar los errores como datos regulares de primera clase.
- **Manejo explícito obligatorio:** Nunca ignorar errores con `_ = err` en llamadas a I/O, red o base de datos sin justificación técnica comentada.
- **Manejar el error una sola vez:** O se registra en log con contexto suficiente, o se devuelve hacia arriba en la pila; nunca registrar y retornar el mismo error repetidamente.

---

## 2. Envoltorio y Preservación de Contexto
- Usar `%w` con `fmt.Errorf` para preservar la cadena de causas y permitir introspección:
  ```go
  if err := c.doRequest(req); err != nil {
      return fmt.Errorf("solicitando token a SUNAT para RUC %s: %w", ruc, err)
  }
  ```
- **Textos de error:**
  - Deben iniciar en minúsculas y sin puntuación final (`"conexion rechazada"`, no `"Conexión rechazada."`).
  - Incluir datos de valor que faciliten el diagnóstico (identificadores, códigos de estado, parámetros clave).

---

## 3. Inspección Idiomática (`errors.Is` y `errors.As`)
- **Errores centinela:** Declarar con prefijo `Err` en el paquete correspondiente:
  ```go
  var (
      ErrNotFound      = errors.New("recurso no encontrado")
      ErrUnauthorized  = errors.New("credenciales inválidas")
  )
  ```
- **Comprobación:** Utilizar `errors.Is` para verificar si un error envuelto coincide con un centinela:
  ```go
  if errors.Is(err, sql.ErrNoRows) {
      return Company{}, ErrNotFound
  }
  ```
- **Tipos de error personalizados:** Utilizar `errors.As` para extraer metadatos de un error tipado:
  ```go
  var sunatErr *SunatAPIError
  if errors.As(err, &sunatErr) {
      log.Printf("Código SUNAT: %s, Mensaje: %s", sunatErr.Code, sunatErr.Message)
  }
  ```

---

## 4. Errores Amigables para el Usuario vs Logs Internos
- **Separación de capas:**
  - El motor interno produce errores técnicos precisos con traza completa.
  - La capa HTTP/API traduce estos errores a mensajes claros, accionables y en el idioma del usuario, indicando exactamente qué debe corregir (ej. *"Credencial no autorizada por SUNAT. Revise el Client ID en Empresas"* en lugar de un crudo `400 Bad Request`).
