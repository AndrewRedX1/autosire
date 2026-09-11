package excel

import "testing"

func TestGetMappingForSheet_ComprobantesUsesPurchaseBook(t *testing.T) {
	t.Parallel()

	mapping := GetMappingForSheet("Comprobantes")
	if mapping.DefaultLibro != "2" {
		t.Fatalf("DefaultLibro = %q, want %q", mapping.DefaultLibro, "2")
	}
}
