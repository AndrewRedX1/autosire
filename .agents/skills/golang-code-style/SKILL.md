---
name: golang-code-style
description: >-
  Estándares de estilo, formato, claridad, declaraciones y control de flujo en Go (Golang)
  siguiendo Effective Go y Uber Go Style Guide. Usar al escribir, dar formato o revisar código Go.
---

# Go Code Style & Clarity Guide

Esta guía establece las directrices de estilo, legibilidad y convenciones idiomáticas para el desarrollo en Go.

---

## 1. Formato y Herramientas
- **Formateo automático:** Todo el código debe estar formateado con `gofmt` y `goimports`.
- **Líneas de código:** Evitar líneas excesivamente largas (>120 caracteres) cuando perjudique la legibilidad. Dividir argumentos o estructuras en múltiples líneas con coma final obligatoria.

---

## 2. Convenciones de Nomenclatura
- **Paquetes:** Nombres cortos, en minúsculas, de una sola palabra (`company`, `cpe`, `auth`). Sin guiones bajos ni camelCase.
- **Evitar nombres redundantes:**
  - `cpe.Client` en vez de `cpe.CpeClient`.
  - `sunat.Credentials` en vez de `sunat.SunatCredentials` cuando el paquete ya aporta contexto.
- **Variables y Constantes:**
  - Ámbitos pequeños: nombres cortos (`ctx`, `err`, `req`, `w`, `r`, `it`, `idx`).
  - Ámbitos globales o paquetes: nombres descriptivos (`TokenMarginExpiry`, `DefaultTimeout`).
  - Acrónimos y siglas en mayúsculas completas: `RUC`, `CPE`, `URL`, `ID`, `HTTP`, `SOL` (ej: `RUCEmisor`, `ClientID`).
- **Getters y Setters:**
  - Getters sin prefijo `Get` cuando corresponda al idiomático Go (ej: `c.Name()` en vez de `c.GetName()`). Se permite `Get` si realiza I/O o red (`GetToken(ctx)`).

---

## 3. Control de Flujo y Happy Path
- **Happy path a la izquierda:** Los casos de error o condiciones límite deben terminar en `return` temprano (guard clauses).
- **Evitar bloques `else` innecesarios:**
  ```go
  // BIEN: Retorno temprano, sin else
  if err != nil {
      return nil, fmt.Errorf("operación falló: %w", err)
  }
  return data, nil

  // MAL: Anidamiento innecesario
  if err == nil {
      return data, nil
  } else {
      return nil, err
  }
  ```

---

## 4. Declaración e Inicialización
- **Variables cortas:** Usar `:=` para variables locales con inferencia clara.
- **Slices y Maps:**
  - Pre-alocar memoria cuando se conozca la capacidad: `items := make([]Item, 0, len(source))`.
  - Diferenciar entre slice nulo y vacío según la semántica requerida en serialización JSON.
- **Structs:**
  - Usar nombres explícitos de campos en literales de estructuras (`comp := Company{RUC: ruc, BusinessName: name}`).

---

## 5. Interfaces Idiomáticas
- **Interfaces pequeñas:** Preferir interfaces de 1 a 3 métodos (`io.Reader`, `io.Closer`, `http.Flusher`).
- **Definir en el consumidor:** Las interfaces deben declararse donde se consumen, no donde se implementan, para reducir el acoplamiento entre paquetes.
