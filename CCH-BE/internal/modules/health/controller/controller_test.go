package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestHealthEndpoint(t *testing.T) {
	app := fiber.New()
	NewController("cch-be", nil).Register(app)
	request, err := http.NewRequest(http.MethodGet, "/healthz", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	httpResponse, err := app.Test(request)
	if err != nil {
		t.Fatalf("test health endpoint: %v", err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", httpResponse.StatusCode, http.StatusOK)
	}
	if got := httpResponse.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}

	var body response
	if err := json.NewDecoder(httpResponse.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Service != "cch-be" || body.Status != "ok" {
		t.Fatalf("body = %#v", body)
	}
}

func TestReadinessReportsMissingMySQLConfiguration(t *testing.T) {
	app := fiber.New()
	NewController("cch-be", nil).Register(app)
	request, err := http.NewRequest(http.MethodGet, "/readyz", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	httpResponse, err := app.Test(request)
	if err != nil {
		t.Fatalf("test readiness endpoint: %v", err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", httpResponse.StatusCode, http.StatusServiceUnavailable)
	}
	var body readinessResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "not_ready" || body.MySQL != "not_configured" {
		t.Fatalf("body = %#v", body)
	}
}

func TestReadinessReportsDatabasePingResults(t *testing.T) {
	tests := []struct {
		name           string
		pingError      error
		wantHTTPStatus int
		wantStatus     string
		wantMySQL      string
	}{
		{name: "connected", wantHTTPStatus: http.StatusOK, wantStatus: "ready", wantMySQL: "connected"},
		{name: "unavailable", pingError: errors.New("connection refused"), wantHTTPStatus: http.StatusServiceUnavailable, wantStatus: "not_ready", wantMySQL: "unavailable"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := fiber.New()
			NewController("cch-be", fakeReadinessChecker{err: test.pingError}).Register(app)
			request, err := http.NewRequest(http.MethodGet, "/readyz", nil)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}

			httpResponse, err := app.Test(request)
			if err != nil {
				t.Fatalf("test readiness endpoint: %v", err)
			}
			defer httpResponse.Body.Close()

			if httpResponse.StatusCode != test.wantHTTPStatus {
				t.Fatalf("status = %d, want %d", httpResponse.StatusCode, test.wantHTTPStatus)
			}
			var body readinessResponse
			if err := json.NewDecoder(httpResponse.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Status != test.wantStatus || body.MySQL != test.wantMySQL {
				t.Fatalf("body = %#v", body)
			}
			if test.pingError != nil && body.MySQL == test.pingError.Error() {
				t.Fatal("response exposed raw database error")
			}
		})
	}
}

type fakeReadinessChecker struct {
	err error
}

func (checker fakeReadinessChecker) PingContext(context.Context) error {
	return checker.err
}
