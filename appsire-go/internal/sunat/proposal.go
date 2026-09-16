package sunat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const sireBaseURL = "https://api-sire.sunat.gob.pe"

type ProposalBook string

const (
	ProposalRCE  ProposalBook = "RCE"
	ProposalRVIE ProposalBook = "RVIE"
)

type ProposalDownload struct {
	Book      ProposalBook
	Period    string
	Ticket    string
	FileName  string
	Content   []byte
	Reused    bool
	Generated string
}

type proposalDefinition struct {
	book         ProposalBook
	bookCode     string
	ticketPath   string
	pollAttempts int
}

type reportFile struct {
	Name string `json:"nomArchivoReporte"`
	Type string `json:"codTipoArchivoReporte"`
	Typo string `json:"codTipoAchivoReporte"`
}

type ticketDetail struct {
	LoadDate string `json:"fecCargaImportacion"`
	LoadTime string `json:"horaCargaImportacion"`
}

type proposalTicket struct {
	Number      string       `json:"numTicket"`
	ProcessCode string       `json:"codProceso"`
	StatusCode  string       `json:"codEstadoProceso"`
	Status      string       `json:"desEstadoProceso"`
	StartedAt   string       `json:"fecInicioProceso"`
	Files       []reportFile `json:"archivoReporte"`
	Detail      ticketDetail `json:"detalleTicket"`
}

type ticketListResponse struct {
	Records []proposalTicket `json:"registros"`
}

type ProposalClient struct {
	clientMu      sync.RWMutex
	httpClient    *http.Client
	timeout       time.Duration
	tokenProvider TokenProvider
	baseURL       string
}

func NewProposalClient(tp TokenProvider, timeout time.Duration) *ProposalClient {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	return &ProposalClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout:       timeout,
		tokenProvider: tp,
		baseURL:       sireBaseURL,
	}
}

// ResetTransport evita compartir conexiones SIRE entre contribuyentes.
func (c *ProposalClient) ResetTransport() {
	c.clientMu.Lock()
	previous := c.httpClient
	c.httpClient = &http.Client{Timeout: c.timeout}
	c.clientMu.Unlock()
	previous.CloseIdleConnections()
}

func (c *ProposalClient) currentHTTPClient() *http.Client {
	c.clientMu.RLock()
	defer c.clientMu.RUnlock()
	return c.httpClient
}

func (c *ProposalClient) Download(
	ctx context.Context,
	book ProposalBook,
	period string,
	reuseExisting bool,
) (*ProposalDownload, error) {
	definition, err := proposalDefinitionFor(book, period)
	if err != nil {
		return nil, err
	}

	ticketNumber, err := c.requestTicket(ctx, definition.ticketPath)
	if err != nil {
		return nil, err
	}

	ticket, waitErr := c.waitForReport(
		ctx,
		definition,
		period,
		ticketNumber,
	)
	reused := false
	if waitErr != nil {
		if !reuseExisting {
			return nil, waitErr
		}

		ticket, err = c.latestFinishedTicket(ctx, definition, period)
		if err != nil {
			return nil, fmt.Errorf("%v; tampoco se encontró una propuesta anterior: %w", waitErr, err)
		}
		reused = true
	}

	file := downloadableFile(ticket.Files)
	if file.Name == "" {
		return nil, errors.New("SUNAT no devolvió un archivo de propuesta descargable")
	}

	content, err := c.downloadArchive(ctx, definition, period, ticket, file)
	if err != nil {
		return nil, err
	}

	generated := strings.TrimSpace(ticket.Detail.LoadDate + " " + ticket.Detail.LoadTime)
	if generated == "" {
		generated = ticket.StartedAt
	}

	return &ProposalDownload{
		Book:      definition.book,
		Period:    period,
		Ticket:    ticket.Number,
		FileName:  file.Name,
		Content:   content,
		Reused:    reused,
		Generated: generated,
	}, nil
}

func proposalDefinitionFor(book ProposalBook, period string) (proposalDefinition, error) {
	if len(period) != 6 {
		return proposalDefinition{}, errors.New("el periodo debe tener formato AAAAMM")
	}
	month, err := strconv.Atoi(period[4:])
	if err != nil || month < 1 || month > 12 {
		return proposalDefinition{}, errors.New("el periodo debe tener formato AAAAMM")
	}
	if _, err := strconv.Atoi(period[:4]); err != nil {
		return proposalDefinition{}, errors.New("el periodo debe tener formato AAAAMM")
	}

	switch ProposalBook(strings.ToUpper(string(book))) {
	case ProposalRCE:
		return proposalDefinition{
			book:     ProposalRCE,
			bookCode: "080000",
			ticketPath: "/v1/contribuyente/migeigv/libros/rce/propuesta/web/propuesta/" + period +
				"/exportacioncomprobantepropuesta?codTipoArchivo=0&codOrigenEnvio=2",
			pollAttempts: 15,
		}, nil
	case ProposalRVIE:
		return proposalDefinition{
			book:     ProposalRVIE,
			bookCode: "140000",
			ticketPath: "/v1/contribuyente/migeigv/libros/rvie/propuesta/web/propuesta/" + period +
				"/exportapropuesta?codTipoArchivo=0",
			pollAttempts: 25,
		}, nil
	default:
		return proposalDefinition{}, errors.New("tipo de propuesta inválido; use RCE o RVIE")
	}
}

func (c *ProposalClient) requestTicket(ctx context.Context, path string) (string, error) {
	body, err := c.getWithRetries(ctx, c.baseURL+path, 5)
	if err != nil {
		return "", fmt.Errorf("solicitando ticket de propuesta: %w", err)
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("leyendo ticket de SUNAT: %w", err)
	}
	ticket := findStringField(payload, "numTicket")
	if ticket == "" {
		return "", errors.New("SUNAT no devolvió numTicket")
	}
	return ticket, nil
}

func (c *ProposalClient) waitForReport(
	ctx context.Context,
	definition proposalDefinition,
	period string,
	ticketNumber string,
) (proposalTicket, error) {
	var lastStatus string
	for attempt := 0; attempt < definition.pollAttempts; attempt++ {
		ticket, err := c.findTicket(ctx, period, ticketNumber, "")
		if err == nil {
			lastStatus = ticket.StatusCode
			if downloadableFile(ticket.Files).Name != "" {
				return ticket, nil
			}
			if ticket.StatusCode != "" && ticket.StatusCode != "05" {
				return proposalTicket{}, fmt.Errorf(
					"SUNAT terminó el ticket %s en estado %s sin generar archivo",
					ticketNumber,
					ticket.StatusCode,
				)
			}
		}

		if attempt+1 < definition.pollAttempts {
			if err := waitForRetry(ctx, 2*time.Second); err != nil {
				return proposalTicket{}, err
			}
		}
	}

	return proposalTicket{}, fmt.Errorf(
		"SUNAT no terminó de generar la propuesta (ticket %s, estado %s)",
		ticketNumber,
		lastStatus,
	)
}

func (c *ProposalClient) latestFinishedTicket(
	ctx context.Context,
	definition proposalDefinition,
	period string,
) (proposalTicket, error) {
	ticket, err := c.findTicket(ctx, period, "", definition.bookCode)
	if err != nil {
		return proposalTicket{}, err
	}
	return ticket, nil
}

func (c *ProposalClient) findTicket(
	ctx context.Context,
	period string,
	ticketNumber string,
	bookCode string,
) (proposalTicket, error) {
	params := url.Values{}
	params.Set("perIni", period)
	params.Set("perFin", period)
	params.Set("page", "1")
	params.Set("perPage", "50")
	if ticketNumber != "" {
		params.Set("numTicket", ticketNumber)
	}
	if bookCode != "" {
		params.Set("codLibro", bookCode)
		params.Set("codOrigenEnvio", "2")
	}

	endpoint := c.baseURL +
		"/v1/contribuyente/migeigv/libros/rvierce/gestionprocesosmasivos/web/masivo/consultaestadotickets?" +
		params.Encode()
	body, err := c.getWithRetries(ctx, endpoint, 2)
	if err != nil {
		return proposalTicket{}, fmt.Errorf("consultando estado del ticket: %w", err)
	}

	var response ticketListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return proposalTicket{}, fmt.Errorf("leyendo estado del ticket: %w", err)
	}
	for _, ticket := range response.Records {
		isRequested := ticketNumber == "" || ticket.Number == ticketNumber
		if isRequested && ticket.ProcessCode == "10" && downloadableFile(ticket.Files).Name != "" {
			return ticket, nil
		}
		if ticketNumber != "" && ticket.Number == ticketNumber {
			return ticket, nil
		}
	}
	return proposalTicket{}, errors.New("SUNAT no devolvió información del ticket")
}

func downloadableFile(files []reportFile) reportFile {
	for _, file := range files {
		isZip := strings.HasSuffix(strings.ToLower(file.Name), ".zip")
		if file.Name != "" && isZip && !strings.Contains(strings.ToUpper(file.Name), "PCW") {
			return file
		}
	}
	for _, file := range files {
		if file.Name != "" {
			return file
		}
	}
	return reportFile{}
}

func (c *ProposalClient) downloadArchive(
	ctx context.Context,
	definition proposalDefinition,
	period string,
	ticket proposalTicket,
	file reportFile,
) ([]byte, error) {
	fileType := file.Type
	if fileType == "" {
		fileType = file.Typo
	}
	if fileType == "" {
		fileType = "00"
	}

	params := url.Values{}
	params.Set("nomArchivoReporte", file.Name)
	params.Set("codTipoArchivoReporte", fileType)
	params.Set("codLibro", definition.bookCode)
	params.Set("perTributario", period)
	params.Set("codProceso", "10")
	params.Set("numTicket", ticket.Number)
	endpoint := c.baseURL +
		"/v1/contribuyente/migeigv/libros/rvierce/gestionprocesosmasivos/web/masivo/archivoreporte?" +
		params.Encode()

	body, err := c.getWithRetries(ctx, endpoint, 5)
	if err != nil {
		return nil, fmt.Errorf("descargando ZIP de propuesta %s: %w", definition.book, err)
	}
	if !IsZipContent(body) {
		return nil, errors.New("SUNAT respondió sin un archivo ZIP válido")
	}
	return body, nil
}

func (c *ProposalClient) getWithRetries(
	ctx context.Context,
	endpoint string,
	maxAttempts int,
) ([]byte, error) {
	var lastErr error
	delay := 4 * time.Second
	refreshed := false

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		token, err := c.tokenProvider.GetValidToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("obteniendo token SUNAT: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("creando solicitud SIRE: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.currentHTTPClient().Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				err = readErr
			} else if resp.StatusCode == http.StatusOK {
				return body, nil
			} else if resp.StatusCode == http.StatusUnauthorized && !refreshed {
				c.tokenProvider.InvalidateToken(token)
				if _, refreshErr := c.tokenProvider.ForceRefresh(ctx); refreshErr != nil {
					return nil, fmt.Errorf("renovando token SIRE: %w", refreshErr)
				}
				refreshed = true
				attempt--
				continue
			} else {
				statusErr := &HTTPStatusError{
					StatusCode: resp.StatusCode,
					Body:       strings.TrimSpace(string(body)),
				}
				if !IsTransientError(resp.StatusCode) {
					return nil, statusErr
				}
				lastErr = statusErr
				if retryDelay := retryAfter(resp); retryDelay > 0 {
					delay = retryDelay
				}
			}
		}
		if err != nil {
			lastErr = err
		}
		if attempt == maxAttempts {
			break
		}
		if err := waitForRetry(ctx, delay); err != nil {
			return nil, err
		}
		delay *= 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
	}

	return nil, fmt.Errorf("SUNAT SIRE falló tras %d intentos: %w", maxAttempts, lastErr)
}

func retryAfter(resp *http.Response) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After")))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func findStringField(value any, field string) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if strings.EqualFold(key, field) {
				if result, ok := child.(string); ok {
					return result
				}
			}
			if result := findStringField(child, field); result != "" {
				return result
			}
		}
	case []any:
		for _, child := range typed {
			if result := findStringField(child, field); result != "" {
				return result
			}
		}
	}
	return ""
}
