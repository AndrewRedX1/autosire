//go:build windows

package company

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"appsire-go/internal/secrets"
)

func TestSyncFromAccess(t *testing.T) {
	accessPath := `C:\AppSireCPE\bdEmpresas.accdb`
	if _, err := os.Stat(accessPath); os.IsNotExist(err) {
		t.Skip("C:\\AppSireCPE\\bdEmpresas.accdb no existe en esta máquina")
	}

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_sync.db")
	protector := secrets.NewProtector("test-sync-machine")
	store, err := Open(ctx, dbPath, protector)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

	imported, err := store.SyncFromAccess(ctx, accessPath)
	if err != nil {
		t.Fatalf("SyncFromAccess failed: %v", err)
	}
	if imported == 0 {
		t.Fatalf("Esperaba al menos 1 empresa importada, obtuve 0")
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("Lista vacía tras sincronizar")
	}

	found := false
	for _, c := range list {
		if c.RUC == "20490304101" {
			found = true
			if c.BusinessName != "HOTELES CBC S.A.C." {
				t.Errorf("Esperaba HOTELES CBC S.A.C., obtuve %s", c.BusinessName)
			}
			if c.EstadoToken != 2 {
				t.Errorf("Esperaba estado_token=2, obtuve %d", c.EstadoToken)
			}
			if c.UltDigito != "1" {
				t.Errorf("Esperaba ult_digito='1', obtuve %s", c.UltDigito)
			}
		}
	}
	if !found {
		t.Errorf("No se encontró HOTELES CBC S.A.C. con RUC 20490304101")
	}
}
