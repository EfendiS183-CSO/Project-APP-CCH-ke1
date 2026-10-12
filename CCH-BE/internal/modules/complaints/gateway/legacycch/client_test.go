package legacycch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientPostsAllowlistedPayloadWithVerifiedToken(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != createComplaintPath || request.Method != http.MethodPost {
			t.Errorf("request = %s %s, want POST %s", request.Method, request.URL.Path, createComplaintPath)
		}
		if request.Header.Get("X-Authorization") != "verified-user-token" {
			t.Errorf("X-Authorization = %q, want caller token", request.Header.Get("X-Authorization"))
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", request.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if !strings.Contains(string(body), `"Kantor_Asal":"40005CC"`) || strings.Contains(string(body), "ID_Pengaduan") {
			t.Errorf("unexpected ticket payload fields: %s", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"success":true,"cchentri":{"ID_Pengaduan":"40005CC202609300001"}}`))
	}))
	defer server.Close()

	client, err := newClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.CreateComplaint(context.Background(), "verified-user-token", validPayload())
	if err != nil {
		t.Fatal(err)
	}
	if result.TicketID != "40005CC202609300001" {
		t.Fatalf("TicketID = %q, want created ticket ID", result.TicketID)
	}
}

func TestClientLogsInAsConfiguredServiceThenCreatesComplaint(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case loginPath:
			if request.Header.Get("X-Authorization") != "" {
				t.Error("login request must not carry a bearer token")
			}
			var credentials struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			if err := json.NewDecoder(request.Body).Decode(&credentials); err != nil {
				t.Errorf("decode login credentials: %v", err)
			}
			if credentials.Username != "intake-bot" || credentials.Password != "test-only-password" {
				t.Error("login request did not use configured service credentials")
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"JWT":"service-user-jwt"}`))
		case createComplaintPath:
			if request.Header.Get("X-Authorization") != "service-user-jwt" {
				t.Errorf("Add token = %q, want JWT returned by CCH login", request.Header.Get("X-Authorization"))
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"success":true,"cchentri":{"ID_Pengaduan":"40005CC202609300002"}}`))
		default:
			t.Errorf("unexpected legacy CCH path %q", request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := newClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.CreateComplaintAsService(context.Background(), "intake-bot", "test-only-password", validPayload())
	if err != nil {
		t.Fatal(err)
	}
	if result.TicketID != "40005CC202609300002" {
		t.Fatalf("TicketID = %q, want legacy-created ticket ID", result.TicketID)
	}
}

func TestClientRejectsInvalidServiceCredentialsAndPayloadBeforeNetwork(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("invalid credentials or payload must not reach CCH")
	}))
	defer server.Close()
	client, err := newClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateComplaintAsService(context.Background(), " ", "password", validPayload()); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("missing username error = %v, want invalid credentials", err)
	}
	invalidPayload := validPayload()
	invalidPayload.CustomerID = 0
	if _, err := client.CreateComplaintAsService(context.Background(), "intake-bot", "password", invalidPayload); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("invalid payload error = %v, want invalid payload", err)
	}
}

func TestClientMapsServiceLoginFailureWithoutLeakingResponseBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != loginPath {
			t.Errorf("unexpected request path %q", request.URL.Path)
		}
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"error":"private login detail"}`))
	}))
	defer server.Close()
	client, err := newClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateComplaintAsService(context.Background(), "intake-bot", "password", validPayload())
	if !errors.Is(err, ErrUpstreamRejected) || strings.Contains(err.Error(), "private login detail") {
		t.Fatalf("login error = %v, want generic upstream rejection", err)
	}
}

func TestClientRejectsUnsafeBaseURLsAndMissingVerifiedToken(t *testing.T) {
	for _, baseURL := range []string{
		"http://cch.example.test",
		"https://user:password@cch.example.test",
		"https://cch.example.test?target=internal",
		"https://cch.example.test#fragment",
	} {
		if _, err := newClient(baseURL, &http.Client{}); !errors.Is(err, ErrInvalidBaseURL) {
			t.Errorf("newClient(%q) error = %v, want invalid base URL", baseURL, err)
		}
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("request must not be sent without a verified token")
	}))
	defer server.Close()
	client, err := newClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"", " ", "verified\r\nX-Injected: yes"} {
		if _, err := client.CreateComplaint(context.Background(), token, validPayload()); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("CreateComplaint() token %q error = %v, want invalid token", token, err)
		}
	}
}

func TestClientDoesNotFollowRedirects(t *testing.T) {
	redirected := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/redirect-target" {
			redirected = true
			return
		}
		http.Redirect(writer, request, "/redirect-target", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client, err := newClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateComplaint(context.Background(), "verified-user-token", validPayload()); !errors.Is(err, ErrUpstreamRejected) {
		t.Fatalf("CreateComplaint() error = %v, want rejected redirect response", err)
	}
	if redirected {
		t.Fatal("client followed upstream redirect")
	}
}

func TestClientReturnsGenericErrorsWithoutUpstreamBody(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     int
		body       string
		wantErr    error
		secretBody string
	}{
		{name: "non-json", status: http.StatusOK, body: "internal php stack trace", wantErr: ErrInvalidResponse, secretBody: "internal php stack trace"},
		{name: "rejected", status: http.StatusBadRequest, body: `{"failureMessage":"customer private detail"}`, wantErr: ErrUpstreamRejected, secretBody: "customer private detail"},
		{name: "missing ticket id", status: http.StatusOK, body: `{"success":true,"cchentri":{}}`, wantErr: ErrInvalidResponse},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := newClient(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.CreateComplaint(context.Background(), "verified-user-token", validPayload())
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("CreateComplaint() error = %v, want %v", err, test.wantErr)
			}
			if test.secretBody != "" && strings.Contains(err.Error(), test.secretBody) {
				t.Fatal("upstream response body leaked in error")
			}
		})
	}
}

func TestClientBoundsUpstreamResponseBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", maxResponseBytes+1)))
	}))
	defer server.Close()
	client, err := newClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateComplaint(context.Background(), "verified-user-token", validPayload()); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("CreateComplaint() error = %v, want invalid response", err)
	}
}

func TestNewClientRequiresPositiveTimeout(t *testing.T) {
	if _, err := NewClient("https://cch.example.test", 0); !errors.Is(err, ErrInvalidTimeout) {
		t.Fatalf("NewClient() error = %v, want configuration error", err)
	}
	if _, err := NewClient("https://cch.example.test", time.Second); err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
}

func validPayload() ComplaintPayload {
	return ComplaintPayload{
		CustomerID:            301734,
		CustomerName:          "CUSTOMER TEST",
		CustomerEmail:         "customer@example.test",
		CustomerPhone:         "0812345678",
		CustomerAddress:       "ADDRESS TEST",
		OriginOfficeCode:      "40005CC",
		DestinationOfficeCode: "40006CC",
		ComplaintTypeCode:     1,
		ComplaintSourceCode:   1,
		ShippingServiceCode:   "REG",
		ShipmentNumber:        "TEST-WAYBILL-001",
		ShipmentOriginOffice:  "40007CC",
		ShipmentTargetOffice:  "40008CC",
		ComplaintSummary:      "TEST SUMMARY",
		ComplaintDescription:  "TEST DESCRIPTION",
	}
}
