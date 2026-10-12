package legacycch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	loginPath           = "/api/login"
	createComplaintPath = "/api/add/cchentri"
	maxResponseBytes    = 1 << 20
)

var (
	ErrInvalidBaseURL      = errors.New("invalid legacy CCH API base URL")
	ErrInvalidTimeout      = errors.New("invalid legacy CCH API timeout")
	ErrInvalidToken        = errors.New("verified CCH token is required")
	ErrInvalidCredentials  = errors.New("CCH intake service credentials are not configured")
	ErrInvalidPayload      = errors.New("invalid CCH complaint payload")
	ErrUpstreamUnavailable = errors.New("legacy CCH API is unavailable")
	ErrUpstreamRejected    = errors.New("legacy CCH API rejected the complaint")
	ErrInvalidResponse     = errors.New("legacy CCH API returned an invalid response")
)

type ComplaintPayload struct {
	CustomerID            int64  `json:"ID_Pelanggan"`
	CustomerName          string `json:"Nama_Pelanggan"`
	CustomerEmail         string `json:"Email_Pelanggan,omitempty"`
	CustomerPhone         string `json:"Nomor_HP_Pelanggan"`
	CustomerAddress       string `json:"Alamat_Pelanggan"`
	OriginOfficeCode      string `json:"Kantor_Asal"`
	DestinationOfficeCode string `json:"Kantor_Tujuan"`
	ComplaintTypeCode     int64  `json:"Jenis_Pengaduan"`
	ComplaintSourceCode   int64  `json:"Sumber_Pengaduan"`
	ShippingServiceCode   string `json:"Jenis_Kiriman"`
	ShipmentNumber        string `json:"Nomor_Kiriman"`
	ShipmentOriginOffice  string `json:"Kantor_Asal_Kiriman"`
	ShipmentTargetOffice  string `json:"Kantor_Tujuan_Kiriman"`
	ComplaintSummary      string `json:"Ringkasan_Pengaduan"`
	ComplaintDescription  string `json:"Uraian_Pengaduan"`
}

type CreateComplaintResult struct {
	TicketID string
}

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewClient(baseURL string, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		return nil, ErrInvalidTimeout
	}
	return newClient(baseURL, &http.Client{Timeout: timeout})
}

func newClient(baseURL string, httpClient *http.Client) (*Client, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsedURL == nil || parsedURL.Scheme != "https" || parsedURL.Hostname() == "" ||
		parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return nil, ErrInvalidBaseURL
	}
	if httpClient == nil {
		return nil, ErrUpstreamUnavailable
	}
	clientCopy := *httpClient
	clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/")
	return &Client{baseURL: parsedURL, httpClient: &clientCopy}, nil
}

func (client *Client) CreateComplaint(ctx context.Context, verifiedToken string, payload ComplaintPayload) (CreateComplaintResult, error) {
	if client == nil || client.baseURL == nil || client.httpClient == nil {
		return CreateComplaintResult{}, ErrUpstreamUnavailable
	}
	if verifiedToken == "" || strings.ContainsAny(verifiedToken, " \t\r\n") {
		return CreateComplaintResult{}, ErrInvalidToken
	}
	if err := payload.Validate(); err != nil {
		return CreateComplaintResult{}, err
	}
	return client.createWithBearerToken(ctx, verifiedToken, payload)
}

func (client *Client) CreateComplaintAsService(ctx context.Context, username string, password string, payload ComplaintPayload) (CreateComplaintResult, error) {
	if client == nil || client.baseURL == nil || client.httpClient == nil {
		return CreateComplaintResult{}, ErrUpstreamUnavailable
	}
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return CreateComplaintResult{}, ErrInvalidCredentials
	}
	if err := payload.Validate(); err != nil {
		return CreateComplaintResult{}, err
	}
	verifiedToken, err := client.Login(ctx, username, password)
	if err != nil {
		return CreateComplaintResult{}, err
	}
	return client.CreateComplaint(ctx, verifiedToken, payload)
}

func (client *Client) Login(ctx context.Context, username string, password string) (string, error) {
	if client == nil || client.baseURL == nil || client.httpClient == nil {
		return "", ErrUpstreamUnavailable
	}
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return "", ErrInvalidCredentials
	}
	loginBody, err := json.Marshal(struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: username, Password: password})
	if err != nil {
		return "", ErrInvalidCredentials
	}
	responseBody, statusCode, err := client.postJSON(ctx, loginPath, loginBody, "")
	if err != nil {
		return "", err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return "", ErrUpstreamRejected
	}
	var loginResponse struct {
		JWT string `json:"JWT"`
	}
	if err := json.Unmarshal(responseBody, &loginResponse); err != nil || loginResponse.JWT == "" || strings.ContainsAny(loginResponse.JWT, " \t\r\n") {
		return "", ErrInvalidResponse
	}
	return loginResponse.JWT, nil
}

func (client *Client) createWithBearerToken(ctx context.Context, verifiedToken string, payload ComplaintPayload) (CreateComplaintResult, error) {
	if verifiedToken == "" || strings.ContainsAny(verifiedToken, " \t\r\n") {
		return CreateComplaintResult{}, ErrInvalidToken
	}
	if err := payload.Validate(); err != nil {
		return CreateComplaintResult{}, err
	}
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return CreateComplaintResult{}, ErrInvalidPayload
	}
	responseBody, statusCode, err := client.postJSON(ctx, createComplaintPath, requestBody, verifiedToken)
	if err != nil {
		return CreateComplaintResult{}, err
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return CreateComplaintResult{}, ErrUpstreamRejected
	}
	var apiResponse struct {
		Success bool `json:"success"`
		Ticket  struct {
			ID string `json:"ID_Pengaduan"`
		} `json:"cchentri"`
	}
	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		return CreateComplaintResult{}, ErrInvalidResponse
	}
	if !apiResponse.Success {
		return CreateComplaintResult{}, ErrUpstreamRejected
	}
	if strings.TrimSpace(apiResponse.Ticket.ID) == "" {
		return CreateComplaintResult{}, ErrInvalidResponse
	}
	return CreateComplaintResult{TicketID: apiResponse.Ticket.ID}, nil
}

func (client *Client) postJSON(ctx context.Context, path string, body []byte, bearerToken string) ([]byte, int, error) {
	targetURL := *client.baseURL
	targetURL.Path = strings.TrimRight(targetURL.Path, "/") + path
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, 0, ErrInvalidBaseURL
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		request.Header.Set("X-Authorization", bearerToken)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, 0, ErrUpstreamUnavailable
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(responseBody) > maxResponseBytes {
		return nil, 0, ErrInvalidResponse
	}
	return responseBody, response.StatusCode, nil
}

func (payload ComplaintPayload) Validate() error {
	if payload.CustomerID <= 0 || payload.ComplaintTypeCode <= 0 || payload.ComplaintSourceCode <= 0 {
		return ErrInvalidPayload
	}
	for _, value := range []string{
		payload.CustomerName,
		payload.CustomerPhone,
		payload.CustomerAddress,
		payload.OriginOfficeCode,
		payload.DestinationOfficeCode,
		payload.ShippingServiceCode,
		payload.ShipmentNumber,
		payload.ShipmentOriginOffice,
		payload.ShipmentTargetOffice,
		payload.ComplaintSummary,
		payload.ComplaintDescription,
	} {
		if strings.TrimSpace(value) == "" {
			return ErrInvalidPayload
		}
	}
	if len(payload.OriginOfficeCode) > 7 || len(payload.DestinationOfficeCode) > 7 ||
		len(payload.ShipmentOriginOffice) > 7 || len(payload.ShipmentTargetOffice) > 7 ||
		len([]rune(payload.ComplaintSummary)) > 100 {
		return ErrInvalidPayload
	}
	return nil
}
