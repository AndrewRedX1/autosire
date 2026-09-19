package cpe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"appsire-go/internal/sirepreview"
)

func TestParseValidationResponse(t *testing.T) {
	// 1. Respuesta ACEPTADO, ACTIVO, HABIDO
	jsonSuccess := []byte(`{
		"success": true,
		"message": "Operation Success",
		"data": {
			"estadoCp": "1",
			"estadoRuc": "00",
			"condDomiRuc": "00",
			"observaciones": ["Comprobante validado conforme"]
		}
	}`)

	res, err := ParseValidationResponse(200, jsonSuccess)
	if err != nil {
		t.Fatalf("ParseValidationResponse fallo: %v", err)
	}
	if !res.Success {
		t.Errorf("esperaba Success=true, obtuvo false")
	}
	if res.EstadoComprobante != "ACEPTADO" {
		t.Errorf("esperaba ACEPTADO, obtuvo %s", res.EstadoComprobante)
	}
	if res.EstadoRuc != "ACTIVO" {
		t.Errorf("esperaba ACTIVO, obtuvo %s", res.EstadoRuc)
	}
	if res.CondicionDomi != "HABIDO" {
		t.Errorf("esperaba HABIDO, obtuvo %s", res.CondicionDomi)
	}

	// 2. Respuesta NO EXISTE
	jsonNoExiste := []byte(`{
		"success": true,
		"message": "Comprobante no informado",
		"data": {
			"estadoCp": "0",
			"estadoRuc": "11",
			"condDomiRuc": "12",
			"observaciones": []
		}
	}`)

	res2, err := ParseValidationResponse(200, jsonNoExiste)
	if err != nil {
		t.Fatalf("ParseValidationResponse fallo: %v", err)
	}
	if res2.EstadoComprobante != "NO EXISTE" {
		t.Errorf("esperaba NO EXISTE, obtuvo %s", res2.EstadoComprobante)
	}
	if res2.EstadoRuc != "BAJA DE OFICIO" {
		t.Errorf("esperaba BAJA DE OFICIO, obtuvo %s", res2.EstadoRuc)
	}
	if res2.CondicionDomi != "NO HABIDO" {
		t.Errorf("esperaba NO HABIDO, obtuvo %s", res2.CondicionDomi)
	}

	// 3. Respuesta ANULADO
	jsonAnulado := []byte(`{
		"success": true,
		"data": {
			"estadoCp": "2",
			"estadoRuc": "00",
			"condDomiRuc": "00"
		}
	}`)

	res3, _ := ParseValidationResponse(200, jsonAnulado)
	if res3.EstadoComprobante != "ANULADO" {
		t.Errorf("esperaba ANULADO, obtuvo %s", res3.EstadoComprobante)
	}

	// 4. Respuesta Error 400
	jsonError := []byte(`{
		"success": false,
		"message": "RUC no corresponde a emisor electrónico",
		"errorCode": "ERR-001"
	}`)

	res4, _ := ParseValidationResponse(400, jsonError)
	if res4.EstadoComprobante != "ERROR" {
		t.Errorf("esperaba ERROR, obtuvo %s", res4.EstadoComprobante)
	}
	if res4.ErrorCode != "ERR-001" {
		t.Errorf("esperaba ERR-001, obtuvo %s", res4.ErrorCode)
	}
}

func TestClient_GetToken_And_Validate(t *testing.T) {
	// Servidor simulado para token y validación
	tokenHits := 0
	valHits := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			tokenHits++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "token-test-12345",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
			return
		}
		if r.URL.Path == "/validarcomprobante" {
			valHits++
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer token-test-12345" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Operation Success",
				"data": map[string]interface{}{
					"estadoCp":      "1",
					"estadoRuc":     "00",
					"condDomiRuc":   "00",
					"observaciones": []string{"OK"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient()
	client.tokenURLTemplate = server.URL + "/oauth2/token?dummy=%s"
	client.serviceURLTemplate = server.URL + "/validarcomprobante?ruc=%s"

	ctx := context.Background()

	// 1. Obtener token
	tok1, err := client.GetToken(ctx, "test-client", "test-secret")
	if err != nil {
		t.Fatalf("GetToken error: %v", err)
	}
	if tok1 != "token-test-12345" {
		t.Errorf("token inesperado: %s", tok1)
	}
	if tokenHits != 1 {
		t.Errorf("esperaba 1 hit para token, hubo %d", tokenHits)
	}

	// 2. Segunda llamada debe usar el token en caché (sin hacer nuevo HTTP hit)
	tok2, err := client.GetToken(ctx, "test-client", "test-secret")
	if err != nil {
		t.Fatalf("GetToken (cache) error: %v", err)
	}
	if tok2 != tok1 {
		t.Errorf("token de cache no coincide")
	}
	if tokenHits != 1 {
		t.Errorf("tokenHits deberia ser 1 tras usar cache, es %d", tokenHits)
	}

	// 3. Validar Comprobante
	q := VoucherQuery{
		NumRUC:       "20100000001",
		CodComp:      "01",
		NumeroSerie:  "F001",
		Numero:       "100",
		FechaEmision: "15/08/2026",
		Monto:        350.50,
	}

	res, err := client.ValidateVoucher(ctx, "20500000000", q, tok1)
	if err != nil {
		t.Fatalf("ValidateVoucher error: %v", err)
	}
	if res.EstadoComprobante != "ACEPTADO" {
		t.Errorf("esperaba ACEPTADO, obtuvo %s", res.EstadoComprobante)
	}
	if valHits != 1 {
		t.Errorf("esperaba 1 hit de validacion, hubo %d", valHits)
	}
}

func TestValidateBatch_Concurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Simular latencia de red
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)

		cod := "1"
		if body["numero"] == "2" {
			cod = "2" // ANULADO
		} else if body["numero"] == "3" {
			cod = "0" // NO EXISTE
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"estadoCp":    cod,
				"estadoRuc":   "00",
				"condDomiRuc": "00",
			},
		})
	}))
	defer server.Close()

	client := NewClient()
	client.serviceURLTemplate = server.URL + "?ruc=%s"

	items := []sirepreview.ProposalItem{
		{CompPago: "01-F001-1", Tipo: "01", Serie: "F001", Numero: "1", Fecha: "01/08/2026", RUC: "20100000001", ImporteTotal: "100.00", Moneda: "PEN"},
		{CompPago: "01-F001-2", Tipo: "01", Serie: "F001", Numero: "2", Fecha: "02/08/2026", RUC: "20100000002", ImporteTotal: "200.00", Moneda: "PEN"},
		{CompPago: "01-F001-3", Tipo: "01", Serie: "F001", Numero: "3", Fecha: "03/08/2026", RUC: "20100000003", ImporteTotal: "300.00", Moneda: "PEN"},
		{CompPago: "01-F001-4", Tipo: "01", Serie: "F001", Numero: "4", Fecha: "04/08/2026", RUC: "20100000004", ImporteTotal: "400.00", Moneda: "USD", TipoCambio: "3.75"},
	}

	progressCount := 0
	report, err := ValidateBatch(context.Background(), client, "20500000000", "RCE", items, "dummy-token", func(item CPEValidatedItem, current, total int) {
		progressCount++
	})

	if err != nil {
		t.Fatalf("ValidateBatch error: %v", err)
	}

	if report.TotalAuditados != 4 {
		t.Errorf("esperaba 4 auditados, obtuvo %d", report.TotalAuditados)
	}
	if report.TotalAceptados != 2 {
		t.Errorf("esperaba 2 aceptados, obtuvo %d", report.TotalAceptados)
	}
	if report.TotalAnulados != 1 {
		t.Errorf("esperaba 1 anulado, obtuvo %d", report.TotalAnulados)
	}
	if report.TotalNoExiste != 1 {
		t.Errorf("esperaba 1 no existe, obtuvo %d", report.TotalNoExiste)
	}
	if report.TotalConRiesgo != 2 {
		t.Errorf("esperaba 2 con riesgo (anulado + no existe), obtuvo %d", report.TotalConRiesgo)
	}
	if progressCount != 4 {
		t.Errorf("esperaba 4 llamadas a onProgress, hubo %d", progressCount)
	}

	// Verificar calculo de monto original en USD
	usdItem := report.Items[3]
	expectedUsdMonto := 106.67 // 400 / 3.75 = 106.6666... -> 106.67
	if fmt.Sprintf("%.2f", usdItem.MontoOriginal) != fmt.Sprintf("%.2f", expectedUsdMonto) {
		t.Errorf("esperaba monto en USD %.2f, obtuvo %.2f", expectedUsdMonto, usdItem.MontoOriginal)
	}
}

func TestIsTaxRisk(t *testing.T) {
	if isTaxRisk("ACEPTADO", "ACTIVO", "HABIDO") {
		t.Errorf("ACEPTADO/ACTIVO/HABIDO no deberia ser riesgo")
	}
	if !isTaxRisk("NO EXISTE", "ACTIVO", "HABIDO") {
		t.Errorf("NO EXISTE deberia ser riesgo")
	}
	if !isTaxRisk("ANULADO", "ACTIVO", "HABIDO") {
		t.Errorf("ANULADO deberia ser riesgo")
	}
	if !isTaxRisk("ACEPTADO", "BAJA DEFINITIVA", "HABIDO") {
		t.Errorf("BAJA DEFINITIVA deberia ser riesgo")
	}
	if !isTaxRisk("ACEPTADO", "ACTIVO", "NO HABIDO") {
		t.Errorf("NO HABIDO deberia ser riesgo")
	}
}
