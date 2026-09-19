---
name: security-best-practices
description: >-
  Revisiones integrales y directrices de seguridad para Python, JavaScript/TypeScript y Go.
  Prevención de OWASP Top 10 (XSS, CSRF, Inyecciones, exposición de datos sensibles, CORS, cabeceras seguras).
---

# Security Best Practices (Multi-Language: Go, JS/TS, Python)

Esta skill define la postura y directrices de seguridad para prevenir vulnerabilidades en la arquitectura completa (frontend, backend y persistencia).

---

## 1. Seguridad en Frontend (JavaScript / HTML / CSS)

### Prevención de XSS (Cross-Site Scripting)
- **Nunca inyectar HTML arbitrario:** Evitar `element.innerHTML = userControlledData`. Utilizar `textContent` o `innerText`.
- Si se deben renderizar plantillas dinámicas, emplear escapado HTML estricto antes de la inserción.
- **Sanitización:** Validar y desinfectar cualquier entrada del usuario antes de reflejarla en la interfaz.

### Cabeceras de Seguridad y Aislamiento Web
- Configurar cabeceras de respuesta HTTP robustas:
  - `Content-Security-Policy (CSP)`: Restringir orígenes de scripts y estilos.
  - `X-Content-Type-Options: nosniff`: Evitar MIME-sniffing.
  - `X-Frame-Options: DENY` o `SAMEORIGIN`: Prevenir ataques de clickjacking.

---

## 2. Seguridad en Backend (Go & APIs)

### Prevención de Inyecciones (SQL, NoSQL, Shell)
- **Consultas parametrizadas en todo momento:** Ninguna variable dinámica debe concatenarse directamente en queries SQL.
- **Validación de tipos y esquemas:** Validar los payloads entrantes (JSON/Form) mediante esquemas y reglas estrictas (longitud de RUC, formatos de fecha, límites numéricos).

### Principio de Mínimo Privilegio
- Los procesos deben correr con los privilegios mínimos necesarios del sistema operativo.
- Los tokens y credenciales de acceso solo deben solicitar los permisos estrictamente requeridos para la operación (scopes reducidos).

---

## 3. Manejo y Almacenamiento de Secretos

- **Separación de configuración y credenciales:**
  - Nunca commitear credenciales, Client Secrets o claves privadas a repositorios Git.
  - Utilizar variables de entorno, almacenes seguros locales o archivos de configuración excluidos en `.gitignore`.
- **Enmascaramiento en Interfaz de Usuario y Logs:**
  - Los campos de contraseñas y claves de API deben usar inputs de tipo `password` por defecto, con opción explícita de visibilidad.
  - Nunca registrar contraseñas o tokens en los logs del servidor.
