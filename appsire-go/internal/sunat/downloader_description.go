package sunat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type descriptionResponse struct {
	Comprobantes []struct {
		PlacaVehicular string `json:"placaVehicular"`
		Concepto       string `json:"desConcepto"`
		Observacion    string `json:"desObservacion"`
		Items          []struct {
			Description string `json:"desItem"`
		} `json:"informacionItems"`
	} `json:"comprobantes"`
}

// DownloadDescription consulta el detalle del comprobante. A diferencia de
// PDF, XML y CDR, SUNAT devuelve datos JSON y no un archivo descargable.
func (c *SunatClient) DownloadDescription(ctx context.Context, comp Comprobante) (DescriptionResult, error) {
	tipo := QueryTipo(comp.Tipo, comp.Serie)
	libro := strings.TrimSpace(comp.Libro)
	if libro == "" {
		libro = "1"
	}
	urlStr := fmt.Sprintf(
		"%s/consultacpe/comprobantes/%s-%s-%s-%s-%s",
		c.baseURL,
		comp.RUC,
		tipo,
		comp.Serie,
		comp.Numero,
		libro,
	)

	_, body, err := c.doRequestOnce(ctx, http.MethodGet, urlStr)
	if err != nil {
		return DescriptionResult{}, fmt.Errorf(
			"error descargando descripción de SUNAT (%s-%s-%s): %w",
			comp.RUC,
			comp.Serie,
			comp.Numero,
			err,
		)
	}

	var response descriptionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return DescriptionResult{}, fmt.Errorf("respuesta de descripción inválida: %w", err)
	}
	descriptions := make([]string, 0)
	plates := make([]string, 0)
	for _, comprobante := range response.Comprobantes {
		appendUnique(&plates, comprobante.PlacaVehicular)
		for _, item := range comprobante.Items {
			appendUnique(&descriptions, item.Description)
		}
		appendUnique(&descriptions, comprobante.Concepto)
		if observation := strings.TrimSpace(comprobante.Observacion); observation != "" {
			appendUnique(&descriptions, "Observación: "+observation)
		}
	}
	result := DescriptionResult{
		Description:  strings.Join(descriptions, "; "),
		VehiclePlate: strings.Join(plates, "; "),
	}
	if result.Description == "" && result.VehiclePlate == "" {
		return result, errors.New("SUNAT no devolvió descripción para el comprobante (HTTP 422)")
	}
	return result, nil
}
