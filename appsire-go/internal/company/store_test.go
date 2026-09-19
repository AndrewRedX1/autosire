package company

import (
	"context"
	"path/filepath"
	"testing"

	"appsire-go/internal/secrets"
)

func TestStore_GetSelected(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_get_selected.db")
	protector := secrets.NewProtector("test-key")
	store, err := Open(ctx, dbPath, protector)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

	// 1. Guardar empresa con credenciales CPE
	comp := Company{
		RUC:             "20123456789",
		BusinessName:    "EMPRESA PRUEBA S.A.C.",
		SOLUsername:     "MODDATOS",
		SOLPassword:     "moddatos",
		ClientID:        "client-sire-id",
		ClientSecret:    "client-sire-secret",
		CpeClientID:     "client-cpe-id",
		CpeClientSecret: "client-cpe-secret",
		Selected:        true,
	}

	id, err := store.Save(ctx, comp)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := store.Select(ctx, id); err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	// 2. Obtener la seleccionada
	selected, err := store.GetSelected(ctx)
	if err != nil {
		t.Fatalf("GetSelected failed: %v", err)
	}

	if selected.RUC != "20123456789" {
		t.Errorf("RUC esperado 20123456789, obtuvo %s", selected.RUC)
	}
	if selected.CpeClientID != "client-cpe-id" {
		t.Errorf("CpeClientID esperado client-cpe-id, obtuvo %s", selected.CpeClientID)
	}
	if selected.CpeClientSecret != "client-cpe-secret" {
		t.Errorf("CpeClientSecret esperado client-cpe-secret, obtuvo %s", selected.CpeClientSecret)
	}
	if selected.ClientID != "client-sire-id" {
		t.Errorf("ClientID esperado client-sire-id, obtuvo %s", selected.ClientID)
	}
}
