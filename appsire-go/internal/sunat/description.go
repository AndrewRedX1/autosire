package sunat

import (
	"bytes"
	"encoding/xml"
	"strings"

	"golang.org/x/net/html/charset"
)

// DescriptionResult contiene el detalle visible que la macro incorpora a la fila.
type DescriptionResult struct {
	Description  string
	VehiclePlate string
}

type descriptionDocument struct {
	InvoiceLines []descriptionLine `xml:"InvoiceLine"`
	CreditLines  []descriptionLine `xml:"CreditNoteLine"`
	DebitLines   []descriptionLine `xml:"DebitNoteLine"`
}

type descriptionLine struct {
	Item             descriptionItem        `xml:"Item"`
	SubInvoiceLines  []descriptionSubLine   `xml:"SubInvoiceLine"`
	AllowanceCharges []descriptionAllowance `xml:"AllowanceCharge"`
}

type descriptionSubLine struct {
	Item descriptionItem `xml:"Item"`
}

type descriptionAllowance struct {
	ID     string `xml:"ID"`
	Reason string `xml:"AllowanceChargeReason"`
}

type descriptionItem struct {
	Descriptions []string              `xml:"Description"`
	Properties   []descriptionProperty `xml:"AdditionalItemProperty"`
}

type descriptionProperty struct {
	NameCode string `xml:"NameCode"`
	Name     string `xml:"Name"`
	Value    string `xml:"Value"`
}

// ExtractUBLDescription replica la lectura local de la macro para facturas y
// notas UBL. Las descripciones se conservan en el orden del comprobante.
func ExtractUBLDescription(data []byte) (DescriptionResult, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charset.NewReaderLabel
	var document descriptionDocument
	if err := decoder.Decode(&document); err != nil {
		return DescriptionResult{}, err
	}

	lines := document.InvoiceLines
	if len(lines) == 0 {
		lines = document.CreditLines
	}
	if len(lines) == 0 {
		lines = document.DebitLines
	}

	descriptions := make([]string, 0, len(lines))
	plates := make([]string, 0)
	for _, line := range lines {
		collectDescriptionItem(line.Item, &descriptions, &plates)
		for _, subLine := range line.SubInvoiceLines {
			collectDescriptionItem(subLine.Item, &descriptions, &plates)
		}
		for _, allowance := range line.AllowanceCharges {
			appendUnique(&descriptions, allowance.ID)
			appendUnique(&descriptions, allowance.Reason)
		}
	}

	return DescriptionResult{
		Description:  strings.Join(descriptions, "; "),
		VehiclePlate: strings.Join(plates, "; "),
	}, nil
}

func collectDescriptionItem(item descriptionItem, descriptions, plates *[]string) {
	for _, description := range item.Descriptions {
		appendUnique(descriptions, description)
	}
	for _, property := range item.Properties {
		name := strings.ToUpper(strings.TrimSpace(property.Name))
		if strings.TrimSpace(property.NameCode) == "7000" || strings.Contains(name, "PLACA") {
			appendUnique(plates, property.Value)
		}
	}
}

func appendUnique(values *[]string, candidate string) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return
	}
	for _, value := range *values {
		if strings.EqualFold(value, candidate) {
			return
		}
	}
	*values = append(*values, candidate)
}
