package company

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	ID              int64  `json:"id"`
	RUC             string `json:"ruc"`
	BusinessName    string `json:"razon_social"`
	SOLUsername     string `json:"usuario_sol"`
	SOLPassword     string `json:"clave_sol,omitempty"`
	ClientID        string `json:"client_id"`
	ClientSecret    string `json:"client_secret,omitempty"`
	CpeClientID     string `json:"cpe_client_id,omitempty"`
	CpeClientSecret string `json:"cpe_client_secret,omitempty"`
	Regimen         string `json:"regimen,omitempty"`
	Whatsapp        string `json:"whatsapp,omitempty"`
	Selected        bool   `json:"seleccionada"`
}

type Summary struct {
	ID             int64  `json:"id"`
	RUC            string `json:"ruc"`
	BusinessName   string `json:"razon_social"`
	SOLUsername    string `json:"usuario_sol"`
	ClientID       string `json:"client_id"`
	Regimen        string `json:"regimen"`
	Whatsapp       string `json:"whatsapp"`
	UltDigito      string `json:"ult_digito"`
	Selected       bool   `json:"seleccionada"`
	HasCredentials bool   `json:"tiene_credenciales"`
	EstadoToken    int    `json:"estado_token"` // 2=Sí, 1=A medias, 0=No
	TokenTexto     string `json:"token_texto"`  // "✔ Sí", "◑ A medias", "✖ No"
	VenceSire      string `json:"vence_sire"`   // "—" o fecha
	VencePdt       string `json:"vence_pdt"`    // "—" o fecha
	Estado         string `json:"estado"`       // "Sin fecha", etc.
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

	// Migraciones incrementales de columnas
	_, _ = db.ExecContext(ctx, `ALTER TABLE companies ADD COLUMN regimen TEXT DEFAULT '';`)
	_, _ = db.ExecContext(ctx, `ALTER TABLE companies ADD COLUMN whatsapp TEXT DEFAULT '';`)
	_, _ = db.ExecContext(ctx, `ALTER TABLE companies ADD COLUMN cpe_client_id TEXT DEFAULT '';`)
	_, _ = db.ExecContext(ctx, `ALTER TABLE companies ADD COLUMN cpe_client_secret BLOB;`)

	s := &Store{
		db:        db,
		protector: protector,
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) List(ctx context.Context) ([]Summary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, ruc, business_name, sol_username, client_id, is_selected,
		       length(sol_password) > 0 AND length(client_secret) > 0,
		       COALESCE(regimen, ''), COALESCE(whatsapp, '')
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
			&item.Regimen,
			&item.Whatsapp,
		); err != nil {
			return nil, fmt.Errorf("leyendo empresa: %w", err)
		}

		// Último dígito de RUC
		cleanRuc := strings.TrimSpace(item.RUC)
		if len(cleanRuc) > 0 {
			item.UltDigito = string(cleanRuc[len(cleanRuc)-1])
		} else {
			item.UltDigito = ""
		}

		// Cálculo de estado del Token (idéntico al macro FrmSeleccionEmpresa.cs)
		hasID := strings.TrimSpace(item.ClientID) != ""
		if hasID && item.HasCredentials {
			item.EstadoToken = 2
			item.TokenTexto = "✔ Sí"
		} else if hasID || item.HasCredentials {
			item.EstadoToken = 1
			item.TokenTexto = "◑ A medias"
		} else {
			item.EstadoToken = 0
			item.TokenTexto = "✖ No"
		}

		item.VenceSire = "—"
		item.VencePdt = "—"
		item.Estado = "Sin fecha"

		companies = append(companies, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recorriendo empresas: %w", err)
	}
	return companies, nil
}

func (s *Store) Get(ctx context.Context, id int64) (Company, error) {
	var c Company
	var encPass, encSec, encCpeSec []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, ruc, business_name, sol_username, sol_password,
		       client_id, client_secret,
		       COALESCE(cpe_client_id, ''), COALESCE(cpe_client_secret, X''),
		       COALESCE(regimen, ''), COALESCE(whatsapp, ''), is_selected
		FROM companies
		WHERE id = ?`, id).Scan(
		&c.ID,
		&c.RUC,
		&c.BusinessName,
		&c.SOLUsername,
		&encPass,
		&c.ClientID,
		&encSec,
		&c.CpeClientID,
		&encCpeSec,
		&c.Regimen,
		&c.Whatsapp,
		&c.Selected,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Company{}, ErrNotFound
	}
	if err != nil {
		return Company{}, fmt.Errorf("consultando empresa por id: %w", err)
	}

	if len(encPass) > 0 {
		c.SOLPassword, _ = s.protector.Decrypt(encPass)
	}
	if len(encSec) > 0 {
		c.ClientSecret, _ = s.protector.Decrypt(encSec)
	}
	if len(encCpeSec) > 0 {
		c.CpeClientSecret, _ = s.protector.Decrypt(encCpeSec)
	}
	return c, nil
}

func (s *Store) Save(ctx context.Context, company Company) (int64, error) {
	if err := s.validateForSave(ctx, &company); err != nil {
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

	var cpeSecret []byte
	if company.CpeClientSecret != "" {
		cpeSecret, _ = s.protector.Encrypt(company.CpeClientSecret)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	if company.ID == 0 {
		// Verificar si ya existe por RUC
		var existingID int64
		err := s.db.QueryRowContext(ctx, "SELECT id FROM companies WHERE ruc = ?", company.RUC).Scan(&existingID)
		if err == nil && existingID > 0 {
			company.ID = existingID
		}
	}

	if company.ID == 0 {
		result, err := s.db.ExecContext(ctx, `
			INSERT INTO companies (
				ruc, business_name, sol_username, sol_password,
				client_id, client_secret, cpe_client_id, cpe_client_secret,
				regimen, whatsapp, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			company.RUC,
			company.BusinessName,
			company.SOLUsername,
			password,
			company.ClientID,
			secret,
			company.CpeClientID,
			cpeSecret,
			company.Regimen,
			company.Whatsapp,
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
		    client_id = ?, client_secret = ?, cpe_client_id = ?, cpe_client_secret = ?,
		    regimen = ?, whatsapp = ?, updated_at = ?
		WHERE id = ?`,
		company.RUC,
		company.BusinessName,
		company.SOLUsername,
		password,
		company.ClientID,
		secret,
		company.CpeClientID,
		cpeSecret,
		company.Regimen,
		company.Whatsapp,
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
		       sol_password, client_secret, COALESCE(regimen, ''), COALESCE(whatsapp, '')
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
		&summary.Regimen,
		&summary.Whatsapp,
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

func (s *Store) DeleteMultiple(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "DELETE FROM companies WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, id := range ids {
		if _, err := stmt.ExecContext(ctx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) validateForSave(ctx context.Context, company *Company) error {
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
	if strings.TrimSpace(company.SOLUsername) == "" {
		return errors.New("el usuario SOL es obligatorio")
	}
	if strings.TrimSpace(company.ClientID) == "" {
		return errors.New("el Client ID es obligatorio")
	}

	if company.ID == 0 {
		if strings.TrimSpace(company.SOLPassword) == "" {
			return errors.New("la clave SOL es obligatoria")
		}
		if strings.TrimSpace(company.ClientSecret) == "" {
			return errors.New("el Client Secret es obligatorio")
		}
	} else {
		// Si se omite la clave o el secreto en edición, mantener los existentes
		if strings.TrimSpace(company.SOLPassword) == "" || strings.TrimSpace(company.ClientSecret) == "" {
			existing, err := s.Get(ctx, company.ID)
			if err == nil {
				if strings.TrimSpace(company.SOLPassword) == "" {
					company.SOLPassword = existing.SOLPassword
				}
				if strings.TrimSpace(company.ClientSecret) == "" {
					company.ClientSecret = existing.ClientSecret
				}
			}
		}
	}
	return nil
}

type accessRecord struct {
	Id              int    `json:"Id"`
	Nombre          string `json:"Nombre"`
	Ruc             string `json:"Ruc"`
	UsuarioSol      string `json:"UsuarioSol"`
	ClaveSol        string `json:"ClaveSol"`
	ClientId        string `json:"ClientId"`
	ClientSecret    string `json:"ClientSecret"`
	CpeClientId     string `json:"CpeClientId"`
	CpeClientSecret string `json:"CpeClientSecret"`
	Regimen         string `json:"Regimen"`
	Whatsapp        string `json:"Whatsapp"`
}

// SyncFromAccess importa las empresas registradas en bdEmpresas.accdb
func (s *Store) SyncFromAccess(ctx context.Context, accdbPath string) (int, error) {
	if _, err := os.Stat(accdbPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("el archivo %s no existe", accdbPath)
	}

	psScript := fmt.Sprintf(`
$p = [System.IO.Path]::GetFullPath('%s');
if (-not (Test-Path $p)) { exit 0 };
$c = New-Object System.Data.OleDb.OleDbConnection('Provider=Microsoft.ACE.OLEDB.12.0;Data Source=' + $p + ';');
$c.Open();
$cmd = $c.CreateCommand();
$cmd.CommandText = 'SELECT Id, Nombre, Ruc, UsuarioSol, ClaveSol, ClientId, ClientSecret, CpeClientId, CpeClientSecret, Regimen, Whatsapp FROM Empresas';
$r = $cmd.ExecuteReader();
$list = @();
while($r.Read()) {
    $list += [PSCustomObject]@{
        Id = $r['Id'];
        Nombre = '' + $r['Nombre'];
        Ruc = '' + $r['Ruc'];
        UsuarioSol = '' + $r['UsuarioSol'];
        ClaveSol = '' + $r['ClaveSol'];
        ClientId = '' + $r['ClientId'];
        ClientSecret = '' + $r['ClientSecret'];
        CpeClientId = '' + $r['CpeClientId'];
        CpeClientSecret = '' + $r['CpeClientSecret'];
        Regimen = '' + $r['Regimen'];
        Whatsapp = '' + $r['Whatsapp'];
    }
};
$c.Close();
if ($list.Count -gt 0) {
    @($list) | ConvertTo-Json -Compress
} else {
    Write-Output '[]'
}
`, strings.ReplaceAll(accdbPath, `'`, `''`))

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ejecutando consulta Access: %w", err)
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" || raw == "[]" {
		return 0, nil
	}

	var records []accessRecord
	if err := json.Unmarshal([]byte(raw), &records); err != nil {
		// Probar si vino como objeto único
		var single accessRecord
		if err2 := json.Unmarshal([]byte(raw), &single); err2 == nil {
			records = []accessRecord{single}
		} else {
			return 0, fmt.Errorf("decodificando empresas de Access: %w (raw: %s)", err, raw)
		}
	}

	imported := 0
	for _, r := range records {
		ruc := strings.TrimSpace(r.Ruc)
		if len(ruc) != 11 {
			continue
		}
		comp := Company{
			RUC:             ruc,
			BusinessName:    strings.TrimSpace(r.Nombre),
			SOLUsername:     strings.TrimSpace(r.UsuarioSol),
			SOLPassword:     strings.TrimSpace(r.ClaveSol),
			ClientID:        strings.TrimSpace(r.ClientId),
			ClientSecret:    strings.TrimSpace(r.ClientSecret),
			CpeClientID:     strings.TrimSpace(r.CpeClientId),
			CpeClientSecret: strings.TrimSpace(r.CpeClientSecret),
			Regimen:         strings.TrimSpace(r.Regimen),
			Whatsapp:        strings.TrimSpace(r.Whatsapp),
		}
		if comp.BusinessName == "" {
			comp.BusinessName = "RUC " + comp.RUC
		}
		if _, err := s.Save(ctx, comp); err == nil {
			imported++
		}
	}

	return imported, nil
}
