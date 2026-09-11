//go:build integration && windows

package company

import (
	"context"
	"path/filepath"
	"testing"

	"appsire-go/internal/secrets"
)

func TestStoreCompanyLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := Open(
		ctx,
		filepath.Join(t.TempDir(), "autosire-test.db"),
		secrets.NewProtector("company-test-machine"),
	)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	id, err := store.Save(ctx, Company{
		RUC:          "20500000001",
		BusinessName: "Empresa de Prueba SAC",
		SOLUsername:  "MODDATOS",
		SOLPassword:  "clave-secreta",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Select(ctx, id); err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	credentials, summary, err := store.Credentials(ctx, id)
	if err != nil {
		t.Fatalf("Credentials() error = %v", err)
	}
	if credentials.ClaveSOL != "clave-secreta" || credentials.ClientSecret != "client-secret" {
		t.Fatal("Credentials() no recuperó los secretos originales")
	}
	if !summary.Selected {
		t.Fatal("la empresa seleccionada no quedó marcada")
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || !list[0].HasCredentials {
		t.Fatalf("List() = %+v", list)
	}
	if err := store.Delete(ctx, id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
