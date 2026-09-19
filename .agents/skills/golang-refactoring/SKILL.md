---
name: golang-refactoring
description: >-
  Refactorización segura y progresiva de código Go.
  Principios SOLID aplicados a Go, desacoplamiento con interfaces pequeñas, modularización,
  separación de responsabilidades y migración sin romper contratos ni provocar regresiones.
---

# Go Refactoring Guide

Esta skill establece las pautas para mejorar y modernizar bases de código en Go manteniendo la estabilidad, la compatibilidad hacia atrás y la simplicidad.

---

## 1. Principios de Refactorización en Go

- **Cobertura previa con pruebas:** Nunca refactorizar código crítico sin antes disponer de una suite de pruebas unitarias o de integración que asegure el comportamiento esperado.
- **Paso a paso (Baby Steps):** Realizar cambios atómicos y focalizados. Compilar y ejecutar pruebas tras cada transformación estructural (`go test ./...`).
- **Conservar la simplicidad:** La filosofía de Go premia la legibilidad sobre la abstracción excesiva. Evitar jerarquías profundas de tipos o patrones de diseño innecesarios (e.g. sobreuso de factories o singletons abstractos).

---

## 2. Técnicas Clave de Refactorización

### Reducción de la Complejidad Ciclomática
- Extraer lógica anidada y bucles complejos a funciones auxiliares puras y bien delimitadas.
- Reemplazar múltiples capas de condicionales con **Guard Clauses** tempranas (Happy path a la izquierda).

### Desacoplamiento mediante Interfaces Pequeñas
- Aplicar el Principio de Segregación de Interfaces:
  ```go
  // ANTES: Dependencia rígida de una estructura pesada
  func Process(s *Store) error { ... }

  // DESPUÉS: Declarar sólo lo que la función necesita
  type ItemFetcher interface {
      FetchItem(id string) (*Item, error)
  }
  func Process(f ItemFetcher) error { ... }
  ```

### Extracción de Paquetes y Evitar Ciclos de Dependencia
- Si dos paquetes dependen mutuamente (`import cycle not allowed`), extraer la definición de tipos o interfaces comunes a un paquete base o reevaluar la pertenencia lógica del módulo.
- No crear paquetes "util" o "common" genéricos y masivos: agrupar por dominio funcional (`cpe`, `company`, `validation`, `auth`).

---

## 3. Manejo de Compatibilidad y Deprecación

- Al modificar firmas públicas de funciones, conservar la función original como envoltorio (`wrapper`) si es necesario mantener retrocompatibilidad:
  ```go
  // Nueva API idiomática con Context
  func ValidateCPE(ctx context.Context, cpe *CPE) (*Result, error) { ... }

  // Obsoleta pero funcional para no romper consumidores existentes
  // Deprecated: usar ValidateCPE con context.Context.
  func ValidateCPELegacy(cpe *CPE) (*Result, error) {
      return ValidateCPE(context.Background(), cpe)
  }
  ```
