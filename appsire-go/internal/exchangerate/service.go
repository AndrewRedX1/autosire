package exchangerate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://api.apis.net.pe/v1/tipo-cambio-sunat"
	defaultToken   = "apis-token-14049.qKBbeJMEetCdBUGRlg0iyiKFi8bPzCgJ"
)

type Rate struct {
	Fecha   string  `json:"fecha"`   // YYYY-MM-DD
	Anio    int     `json:"anio"`
	Mes     int     `json:"mes"`
	Dia     int     `json:"dia"`
	Compra  float64 `json:"compra"`
	Venta   float64 `json:"venta"`
	Moneda  string  `json:"moneda"`
	Origen  string  `json:"origen"`
}

type YearMonth struct {
	Year  int
	Month int
}

type Service struct {
	db         *sql.DB
	httpClient *http.Client
	baseURL    string
	token      string
	mu         sync.RWMutex
}

func NewService(db *sql.DB) (*Service, error) {
	s := &Service{
		db: db,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		baseURL: defaultBaseURL,
		token:   defaultToken,
	}

	if db != nil {
		if err := s.migrate(context.Background()); err != nil {
			return nil, fmt.Errorf("migrando tabla de tipo de cambio: %w", err)
		}
	}

	return s, nil
}

func (s *Service) migrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tipo_cambio_sunat (
		fecha TEXT PRIMARY KEY,
		anio INTEGER NOT NULL,
		mes INTEGER NOT NULL,
		dia INTEGER NOT NULL,
		compra REAL NOT NULL,
		venta REAL NOT NULL,
		moneda TEXT NOT NULL DEFAULT 'USD',
		origen TEXT NOT NULL DEFAULT 'SUNAT',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_tc_anio_mes ON tipo_cambio_sunat(anio, mes);
	`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

type apiResponseItem struct {
	Fecha        string  `json:"fecha"`
	Compra       float64 `json:"compra"`
	Venta        float64 `json:"venta"`
	PrecioCompra float64 `json:"precioCompra"`
	PrecioVenta  float64 `json:"precioVenta"`
	Moneda       string  `json:"moneda"`
	Origen       string  `json:"origen"`
}

func (s *Service) FetchMonth(ctx context.Context, year, month int) ([]Rate, error) {
	url := fmt.Sprintf("%s?month=%02d&year=%d", s.baseURL, month, year)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creando petición HTTP TC: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AutoSire/1.0 (Windows NT 10.0)")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("consultando API SUNAT TC: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API SUNAT respondió HTTP %d", resp.StatusCode)
	}

	var items []apiResponseItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decodificando JSON de tipo de cambio: %w", err)
	}

	rates := make([]Rate, 0, len(items))
	for _, it := range items {
		compra := it.Compra
		if compra <= 0 {
			compra = it.PrecioCompra
		}
		venta := it.Venta
		if venta <= 0 {
			venta = it.PrecioVenta
		}
		if compra <= 0 || venta <= 0 {
			continue
		}

		t, err := time.Parse("2006-01-02", strings.TrimSpace(it.Fecha))
		if err != nil {
			continue
		}

		rates = append(rates, Rate{
			Fecha:  t.Format("2006-01-02"),
			Anio:   t.Year(),
			Mes:    int(t.Month()),
			Dia:    t.Day(),
			Compra: compra,
			Venta:  venta,
			Moneda: "USD",
			Origen: "SUNAT",
		})
	}

	sort.Slice(rates, func(i, j int) bool {
		return rates[i].Fecha < rates[j].Fecha
	})

	return rates, nil
}

func (s *Service) SaveRates(ctx context.Context, rates []Rate) error {
	if s.db == nil || len(rates) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO tipo_cambio_sunat (fecha, anio, mes, dia, compra, venta, moneda, origen, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(fecha) DO UPDATE SET
			compra = excluded.compra,
			venta = excluded.venta,
			moneda = excluded.moneda,
			origen = excluded.origen,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range rates {
		if _, err := stmt.ExecContext(ctx, r.Fecha, r.Anio, r.Mes, r.Dia, r.Compra, r.Venta, r.Moneda, r.Origen); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Service) CountMonth(ctx context.Context, year, month int) (int, error) {
	if s.db == nil {
		return 0, errors.New("base de datos no disponible")
	}
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tipo_cambio_sunat WHERE anio = ? AND mes = ?`, year, month).Scan(&count)
	return count, err
}

func (s *Service) EnsureMonths(ctx context.Context, months []YearMonth) error {
	now := time.Now()
	for _, ym := range months {
		if ym.Year <= 0 || ym.Month < 1 || ym.Month > 12 {
			continue
		}
		if ym.Year > now.Year() || (ym.Year == now.Year() && ym.Month > int(now.Month())) {
			continue
		}

		isCurrentMonth := ym.Year == now.Year() && ym.Month == int(now.Month())
		count, _ := s.CountMonth(ctx, ym.Year, ym.Month)

		// Si es mes pasado y ya tiene registros (normalmente 20+ días), no volver a descargar
		if !isCurrentMonth && count >= 20 {
			continue
		}

		// Descargar de API SUNAT
		rates, err := s.FetchMonth(ctx, ym.Year, ym.Month)
		if err != nil {
			// Si falla la red pero ya tenemos datos en SQLite, no bloquear
			if count > 0 {
				continue
			}
			return fmt.Errorf("descargando TC %02d/%d: %w", ym.Month, ym.Year, err)
		}

		if err := s.SaveRates(ctx, rates); err != nil {
			return fmt.Errorf("guardando TC %02d/%d: %w", ym.Month, ym.Year, err)
		}
	}
	return nil
}

func (s *Service) GetRatesForMonths(ctx context.Context, months []YearMonth) (map[string]Rate, error) {
	if s.db == nil {
		return nil, errors.New("base de datos no disponible")
	}

	result := make(map[string]Rate)
	for _, ym := range months {
		rows, err := s.db.QueryContext(ctx, `
			SELECT fecha, anio, mes, dia, compra, venta, moneda, origen
			FROM tipo_cambio_sunat
			WHERE anio = ? AND mes = ?
		`, ym.Year, ym.Month)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var r Rate
			if err := rows.Scan(&r.Fecha, &r.Anio, &r.Mes, &r.Dia, &r.Compra, &r.Venta, &r.Moneda, &r.Origen); err != nil {
				rows.Close()
				return nil, err
			}
			result[r.Fecha] = r

			// También mapear formato DD/MM/YYYY para coincidencia directa rápida
			ddmmyyyy := fmt.Sprintf("%02d/%02d/%04d", r.Dia, r.Mes, r.Anio)
			result[ddmmyyyy] = r
		}
		rows.Close()
	}

	return result, nil
}
