//go:build windows

package secrets

import "testing"

func TestProtectorRoundTrip(t *testing.T) {
	t.Parallel()

	protector := NewProtector("test-machine")
	ciphertext, err := protector.Encrypt("clave-sol-secreta")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if string(ciphertext) == "clave-sol-secreta" {
		t.Fatal("Encrypt() guardó el texto original")
	}

	plaintext, err := protector.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "clave-sol-secreta" {
		t.Fatalf("Decrypt() = %q", plaintext)
	}
}
