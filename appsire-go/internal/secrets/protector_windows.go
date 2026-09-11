//go:build windows

package secrets

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Protector struct {
	entropy []byte
}

func NewProtector(machineID string) *Protector {
	return &Protector{
		entropy: []byte("AutoSire/company-credentials/" + machineID),
	}
}

func (p *Protector) Encrypt(plaintext string) ([]byte, error) {
	if plaintext == "" {
		return []byte{}, nil
	}
	input := []byte(plaintext)
	inputBlob := dataBlob(input)
	entropyBlob := dataBlob(p.entropy)
	var outputBlob windows.DataBlob

	err := windows.CryptProtectData(
		&inputBlob,
		nil,
		&entropyBlob,
		0,
		nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN,
		&outputBlob,
	)
	runtime.KeepAlive(input)
	runtime.KeepAlive(p.entropy)
	if err != nil {
		return nil, fmt.Errorf("protegiendo credencial con Windows DPAPI: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outputBlob.Data)))

	return append([]byte(nil), unsafe.Slice(outputBlob.Data, outputBlob.Size)...), nil
}

func (p *Protector) Decrypt(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", nil
	}
	inputBlob := dataBlob(ciphertext)
	entropyBlob := dataBlob(p.entropy)
	var outputBlob windows.DataBlob

	err := windows.CryptUnprotectData(
		&inputBlob,
		nil,
		&entropyBlob,
		0,
		nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN,
		&outputBlob,
	)
	runtime.KeepAlive(ciphertext)
	runtime.KeepAlive(p.entropy)
	if err != nil {
		return "", fmt.Errorf("abriendo credencial con Windows DPAPI: %w", err)
	}
	if outputBlob.Data == nil {
		return "", errors.New("Windows DPAPI devolvió una credencial vacía")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(outputBlob.Data)))

	plaintext := append([]byte(nil), unsafe.Slice(outputBlob.Data, outputBlob.Size)...)
	return string(plaintext), nil
}

func dataBlob(data []byte) windows.DataBlob {
	if len(data) == 0 {
		return windows.DataBlob{}
	}
	return windows.DataBlob{
		Size: uint32(len(data)),
		Data: &data[0],
	}
}
