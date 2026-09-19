package exchangerate

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func TestService_SaveAndGetRates(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("abriendo sqlite en memoria: %v", err)
	}
	defer db.Close()

	svc, err := NewService(db)
	if err != nil {
		t.Fatalf("creando servicio: %v", err)
	}

	rates := []Rate{
		{
			Fecha:  "2024-09-01",
			Anio:   2024,
			Mes:    9,
			Dia:    1,
			Compra: 3.739,
			Venta:  3.750,
			Moneda: "USD",
			Origen: "SUNAT",
		},
		{
			Fecha:  "2024-09-02",
			Anio:   2024,
			Mes:    9,
			Dia:    2,
			Compra: 3.745,
			Venta:  3.755,
			Moneda: "USD",
			Origen: "SUNAT",
		},
	}

	ctx := context.Background()
	if err := svc.SaveRates(ctx, rates); err != nil {
		t.Fatalf("guardando tasas: %v", err)
	}

	count, err := svc.CountMonth(ctx, 2024, 9)
	if err != nil {
		t.Fatalf("contando mes: %v", err)
	}
	if count != 2 {
		t.Errorf("esperaba 2 registros, obtuvo %d", count)
	}

	ratesMap, err := svc.GetRatesForMonths(ctx, []YearMonth{{Year: 2024, Month: 9}})
	if err != nil {
		t.Fatalf("obteniendo tasas: %v", err)
	}

	r1, ok := ratesMap["2024-09-01"]
	if !ok {
		t.Fatalf("no se encontró tasa para 2024-09-01")
	}
	if r1.Venta != 3.750 {
		t.Errorf("esperaba venta 3.750, obtuvo %.4f", r1.Venta)
	}

	// Probar lookup con formato DD/MM/YYYY
	r2, ok := ratesMap["02/09/2024"]
	if !ok {
		t.Fatalf("no se encontró tasa para 02/09/2024")
	}
	if r2.Compra != 3.745 {
		t.Errorf("esperaba compra 3.745, obtuvo %.4f", r2.Compra)
	}
}

func TestService_FetchMonthMock(t *testing.T) {
	mockItems := []apiResponseItem{
		{
			Fecha:  "2024-09-05",
			Compra: 3.793,
			Venta:  3.797,
			Moneda: "USD",
			Origen: "SUNAT",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockItems)
	}))
	defer server.Close()

	svc := &Service{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	rates, err := svc.FetchMonth(context.Background(), 2024, 9)
	if err != nil {
		t.Fatalf("error en FetchMonth: %v", err)
	}

	if len(rates) != 1 {
		t.Fatalf("esperaba 1 tasa, obtuvo %d", len(rates))
	}
	if rates[0].Fecha != "2024-09-05" || rates[0].Venta != 3.797 {
		t.Errorf("tasa inesperada: %+v", rates[0])
	}
}
