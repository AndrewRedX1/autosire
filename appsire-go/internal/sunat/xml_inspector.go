package sunat

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"strings"
)

// CdrInfo contiene el estado oficial extraído del CDR emitido por SUNAT
type CdrInfo struct {
	ResponseCode string
	Description  string
	Estado       string // "ACEPTADO", "RECHAZADO", "OBSERVADO"
}

// IsXMLContent reconoce XML UTF-8, UTF-8 con BOM y UTF-16 con BOM. SUNAT
// devuelve las tres variantes según el emisor del comprobante.
func IsXMLContent(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	trimmed = bytes.TrimPrefix(trimmed, []byte{0xEF, 0xBB, 0xBF})
	trimmed = bytes.TrimSpace(trimmed)
	if bytes.HasPrefix(trimmed, []byte("<")) {
		return true
	}

	if len(data) < 4 {
		return false
	}
	var order binary.ByteOrder
	switch {
	case data[0] == 0xFF && data[1] == 0xFE:
		order = binary.LittleEndian
	case data[0] == 0xFE && data[1] == 0xFF:
		order = binary.BigEndian
	default:
		return false
	}
	for offset := 2; offset+1 < len(data); offset += 2 {
		char := order.Uint16(data[offset : offset+2])
		switch char {
		case ' ', '\t', '\r', '\n':
			continue
		case '<':
			return true
		default:
			return false
		}
	}
	return false
}

// InspectCDR analiza el XML del CDR en memoria y extrae el código y descripción oficial de SUNAT
func InspectCDR(xmlData []byte) CdrInfo {
	info := CdrInfo{
		Estado: "DESCONOCIDO",
	}

	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var currentElement string

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch se := token.(type) {
		case xml.StartElement:
			currentElement = se.Name.Local
		case xml.CharData:
			text := strings.TrimSpace(string(se))
			if text == "" {
				continue
			}

			if currentElement == "ResponseCode" && info.ResponseCode == "" {
				info.ResponseCode = text
				switch {
				case text == "0":
					info.Estado = "ACEPTADO"
				case strings.HasPrefix(text, "2") || strings.HasPrefix(text, "3"):
					info.Estado = "RECHAZADO"
				case strings.HasPrefix(text, "4"):
					info.Estado = "OBSERVADO"
				default:
					info.Estado = "PROCESADO"
				}
			} else if currentElement == "Description" && info.Description == "" {
				info.Description = text
			}
		}
	}

	return info
}

// ExtractDigestValue extrae la firma hash del comprobante XML sin cargar todo el DOM en memoria
func ExtractDigestValue(xmlData []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var currentElement string

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch se := token.(type) {
		case xml.StartElement:
			currentElement = se.Name.Local
		case xml.CharData:
			text := strings.TrimSpace(string(se))
			if text != "" && currentElement == "DigestValue" {
				return text
			}
		}
	}

	return ""
}

// ValidateContentIntegrity comprueba si los bytes devueltos por SUNAT corresponden legítimamente al formato
// y no a una página HTML de error de SUNAT disfrazada de HTTP 200 OK
func ValidateContentIntegrity(data []byte, expected TipoDescarga) error {
	if len(data) == 0 {
		return errors.New("contenido vacío recibido de SUNAT")
	}

	// Comprobar si SUNAT devolvió HTML de error (falso 200)
	sample := strings.ToLower(string(data[:min(len(data), 512)]))
	if strings.Contains(sample, "<!doctype html") ||
		strings.Contains(sample, "<html") ||
		strings.Contains(sample, "ha ocurrido un error") ||
		strings.Contains(sample, "servicio no disponible") ||
		strings.Contains(sample, "502 bad gateway") ||
		strings.Contains(sample, "500 internal server error") {
		return errors.New("SUNAT devolvió página HTML de error en lugar del comprobante")
	}

	switch expected {
	case DescargaPDF:
		if !bytes.HasPrefix(data, []byte("%PDF")) && !IsZipContent(data) {
			// A veces el JSON Base64 ya fue decodificado, debe ser %PDF
			return errors.New("el archivo no tiene la cabecera válida de PDF (%PDF)")
		}
	case DescargaXML:
		if !IsZipContent(data) && !IsXMLContent(data) {
			return errors.New("el archivo no contiene estructura válida de XML ni ZIP")
		}
	case DescargaCDR:
		if !IsZipContent(data) && !IsXMLContent(data) {
			return errors.New("el CDR no contiene estructura válida de ZIP o XML")
		}
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
