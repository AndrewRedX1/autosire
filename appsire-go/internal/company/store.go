package company

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"appsire-go/internal/secrets"
	"appsire-go/internal/sunat"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("empresa no encontrada")

//go:embed migrations/001_companies.sql
var initialMigration string

type Company struct {
	ID           int64  `json:"id"`
	RUC          string `json:"ruc"`
	BusinessName string `json:"razon_social"`
	SOLUsername  string `json:"usuario_sol"`
	SOLPassword  string `json:"clave_sol,omitempty"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	Selected     bool   `json:"seleccionada"`
}

type Summary struct {
	ID             int64  `json:"id"`
	RUC            string `json:"ruc"`
	BusinessName   string `json:"razon_social"`
	SOLUsername    string `json:"usuario_sol"`
	ClientID       string `json:"client_id"`
	Selected       bool   `json:"seleccionada"`
	HasCredentials bool   `json:"tiene_credenciales"`
}

type Store struct {
	db        *sql.DB
	protector *secrets.Protector
}

func Open(ctx context.Context, path string, protector *secrets.Protector) (*Store, error) {
	if protector == nil {
		return nil, errors.New("protector de credenciales requerido")
	}
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abriendo SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("conectando SQLite: %w", err)
	}
	if _, err := db.ExecContext(ctx, initialMigration); err != nil {
		db.Close()
		return nil, fmt.Errorf("aplicando estructura SQLite: %w", err)
	}

	return &Store{
		db:        db,
		protector: protector,
	}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) List(ctx context.Context) ([]Summary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, ruc, business_name, sol_username, client_id, is_selected,
		       length(sol_password) > 0 AND length(client_secret) > 0
		FROM companies
		ORDER BY is_selected DESC, business_name COLLATE NOCASE, ruc`)
	if err != nil {
		return nil, fmt.Errorf("listando empresas: %w", err)
	}
	defer rows.Close()

	companies := []Summary{}
	for rows.Next() {
		var item Summary
		if err := rows.Scan(
			&item.ID,
			&item.RUC,
			&item.BusinessName,
			&item.SOLUsername,
			&item.ClientID,
			&item.Selected,
			&item.HasCredentials,
		); err != nil {
			return nil, fmt.Errorf("leyendo empresa: %w", err)
		}
		companies = append(companies, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recorriendo empresas: %w", err)
	}
	return companies, nil
}

func (s *Store) Save(ctx context.Context, company Company) (int64, error) {
	if err := validate(company); err != nil {
		return 0, err
	}

	password, err := s.protector.Encrypt(company.SOLPassword)
	if err != nil {
		return 0, err
	}
	secret, err := s.protector.Encrypt(company.ClientSecret)
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if company.ID == 0 {
		result, err := s.db.ExecContext(ctx, `
			INSERT INTO companies (
				ruc, business_name, sol_username, sol_password,
				client_id, client_secret, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			company.RUC,
			company.BusinessName,
			company.SOLUsername,
			password,
			company.ClientID,
			secret,
			now,
			now,
		)
		if err != nil {
			return 0, fmt.Errorf("guardando empresa: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("obteniendo identificador de empresa: %w", err)
		}
		return id, nil
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE companies
		SET ruc = ?, business_name = ?, sol_username = ?, sol_password = ?,
		    client_id = ?, client_secret = ?, updated_at = ?
		WHERE id = ?`,
		company.RUC,
		company.BusinessName,
		company.SOLUsername,
		password,
		company.ClientID,
		secret,
		now,
		company.ID,
	)
	if err != nil {
		return 0, fmt.Errorf("actualizando empresa: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("confirmando actualización de empresa: %w", err)
	}
	if rowsAffected == 0 {
		return 0, ErrNotFound
	}
	return company.ID, nil
}

func (s *Store) Credentials(ctx context.Context, id int64) (sunat.SunatCredentials, Summary, error) {
	var summary Summary
	var encryptedPassword []byte
	var encryptedSecret []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, ruc, business_name, sol_username, client_id, is_selected,
		       sol_password, client_secret
		FROM companies
		WHERE id = ?`, id).Scan(
		&summary.ID,
		&summary.RUC,
		&summary.BusinessName,
		&summary.SOLUsername,
		&summary.ClientID,
		&summary.Selected,
		&encryptedPassword,
		&encryptedSecret,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return sunat.SunatCredentials{}, Summary{}, ErrNotFound
	}
	if err != nil {
		return sunat.SunatCredentials{}, Summary{}, fmt.Errorf("consultando empresa: %w", err)
	}

	password, err := s.protector.Decrypt(encryptedPassword)
	if err != nil {
		return sunat.SunatCredentials{}, Summary{}, err
	}
	secret, err := s.protector.Decrypt(encryptedSecret)
	if err != nil {
		return sunat.SunatCredentials{}, Summary{}, err
	}
	summary.HasCredentials = password != "" && secret != ""

	return sunat.SunatCredentials{
		RUC:          summary.RUC,
		UsuarioSOL:   summary.SOLUsername,
		ClaveSOL:     password,
		ClientID:     summary.ClientID,
		ClientSecret: secret,
	}, summary, nil
}

func (s *Store) Select(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciando selección de empresa: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "UPDATE companies SET is_selected = 0"); err != nil {
		return fmt.Errorf("limpiando selección anterior: %w", err)
	}
	result, err := tx.ExecContext(ctx, "UPDATE companies SET is_selected = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("seleccionando empresa: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("confirmando selección: %w", err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("guardando selección: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM companies WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("eliminando empresa: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("confirmando eliminación: %w", err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func validate(company Company) error {
	company.RUC = strings.TrimSpace(company.RUC)
	if len(company.RUC) != 11 {
		return errors.New("el RUC debe tener 11 dígitos")
	}
	for _, digit := range company.RUC {
		if digit < '0' || digit > '9' {
			return errors.New("el RUC debe contener solo números")
		}
	}
	if strings.TrimSpace(company.BusinessName) == "" {
		return errors.New("la razón social es obligatoria")
	}
	if strings.TrimSpace(company.SOLUsername) == "" || strings.TrimSpace(company.SOLPassword) == "" {
		return errors.New("el usuario y la clave SOL son obligatorios")
	}
	if strings.TrimSpace(company.ClientID) == "" || strings.TrimSpace(company.ClientSecret) == "" {
		return errors.New("el Client ID y Client Secret son obligatorios")
	}
	return nil
}
