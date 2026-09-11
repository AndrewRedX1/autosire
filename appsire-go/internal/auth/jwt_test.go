package auth

import (
	"testing"
	"time"

	"appsire-go/internal/sunat"
)

func TestParseJWT(t *testing.T) {
	// Token de prueba de SUNAT (ejemplo obtenido de la hoja tk)
	testToken := "eyJraWQiOiJhcGkuc3VuYXQuZ29iLnBlLmtpZDAwMSIsInR5cCI6IkpXVCIsImFsZyI6IlJTMjU2In0.eyJzdWIiOiIyMDUyMjEyODcxNyIsIm5iZiI6MTc2MjU4NDY4OSwiZXhwIjoxNzYyNTg4Mjg5LCJ1c2VyZGF0YSI6eyJudW1SVUMiOiIyMDUyMjEyODcxNyIsInVzdWFyaW9TT0wiOiJBSk9HUlVOQSJ9fQ.signature"

	payload, err := ParseJWT(testToken)
	if err != nil {
		t.Fatalf("ParseJWT falló: %v", err)
	}

	if payload.GetRUC() != "20522128717" {
		t.Errorf("Se esperaba RUC 20522128717, se obtuvo: %s", payload.GetRUC())
	}

	exp := payload.GetExpiresAt()
	if exp.Before(time.Now()) {
		// En este token de prueba exp es unix 1762588289 (año 2025/2026)
	}
}

func TestTokenServiceInvalidateTokenDoesNotDropNewToken(t *testing.T) {
	t.Parallel()

	service := NewTokenService(0)
	service.cachedResp = &sunat.TokenResponse{
		AccessToken: "token-nuevo",
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	service.InvalidateToken("token-antiguo")
	if service.cachedResp == nil {
		t.Fatal("un 401 atrasado eliminó el token recién renovado")
	}

	service.InvalidateToken("token-nuevo")
	if service.cachedResp != nil {
		t.Fatal("el token rechazado no fue invalidado")
	}
}
