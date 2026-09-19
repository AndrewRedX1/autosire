---
name: golang-testing
description: >-
  Pruebas unitarias, integración, mocks, fuzzing, cobertura y detección de goroutines filtradas en Go.
  Enfoque en pruebas idiomáticas basadas en tablas (table-driven tests), httptest, test isolation y -race.
---

# Go Testing & Quality Assurance Guide

Esta skill define la estrategia y patrones recomendados para pruebas unitarias, de integración y detección de regresiones en Go.

---

## 1. Table-Driven Tests (Pruebas Basadas en Tablas)

El patrón estándar en Go consiste en agrupar casos de prueba en un slice anónimo de estructuras:

```go
func TestValidateCPE(t *testing.T) {
    tests := []struct {
        name    string
        input   CPEData
        wantErr bool
        errMsg  string
    }{
        {
            name:    "Válido con datos completos",
            input:   CPEData{Ruc: "20100000001", TipoDoc: "01", Serie: "F001", Correlativo: 1},
            wantErr: false,
        },
        {
            name:    "Error por RUC inválido",
            input:   CPEData{Ruc: "123", TipoDoc: "01"},
            wantErr: true,
            errMsg:  "RUC inválido",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateCPE(tt.input)
            if (err != nil) != tt.wantErr {
                t.Fatalf("ValidateCPE() error = %v, wantErr %v", err, tt.wantErr)
            }
            if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
                t.Errorf("error = %v, esperaba que contuviera %v", err, tt.errMsg)
            }
        })
    }
}
```

---

## 2. Pruebas de Servidores y Clientes HTTP (`net/http/httptest`)

- **Probar Handlers:** Usar `httptest.NewRecorder()` y `httptest.NewRequest()`:
  ```go
  req := httptest.NewRequest(http.MethodPost, "/api/cpe/validate", body)
  rec := httptest.NewRecorder()
  handler.ServeHTTP(rec, req)
  
  if rec.Code != http.StatusOK {
      t.Errorf("esperaba código 200, obtuvo %d", rec.Code)
  }
  ```
- **Simular APIs Externas:** Usar `httptest.NewServer()` para simular respuestas de APIs externas (e.g. SUNAT) sin depender de conexión a internet durante los tests.

---

## 3. Aislamiento y Mocks con Interfaces

- Definir interfaces pequeñas en el lugar donde se consumen (consumidor):
  ```go
  type TokenProvider interface {
      GetToken(ctx context.Context) (string, error)
  }
  ```
- Crear fakes o mocks sencillos en memoria implementando la interfaz, evitando dependencias externas complejas.

---

## 4. Detección de Fugas de Goroutines y Race Detector

- **Ejecución obligatoria con `-race`:**
  ```bash
  go test -race ./...
  ```
- **Detección de filtraciones de goroutines:** Asegurar que todos los workers o canales se cierren adecuadamente cuando finaliza el test, usando contextos con cancelación (`defer cancel()`) o `goleak`.

---

## 5. Fuzzing

- Utilizar el soporte nativo de Fuzzing en Go para encontrar entradas maliciosas o casos de pánico:
  ```go
  func FuzzParseCPE(f *testing.F) {
      f.Add("20100000001|01|F001|1234")
      f.Fuzz(func(t *testing.T, data string) {
          _, _ = ParseCPE(data) // No debe entrar en panic
      })
  }
  ```

---

## 6. Cobertura y Métricas de Prueba

- Generar y revisar el perfil de cobertura:
  ```bash
  go test -coverprofile=coverage.out ./...
  go tool cover -func=coverage.out
  ```
