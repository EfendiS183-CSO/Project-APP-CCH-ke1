package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/platform/config"
	"cch.local/backend/internal/platform/httpapi"
)

func TestCustomerProfileRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := customerProfileRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled customer profile registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{"/api/v1/customers", "/api/v1/customers/12"} {
		response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404 while disabled", path, response.StatusCode)
		}
	}
}

func TestCustomerProfileRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{CustomerProfileAPIEnabled: true}
	if _, err := customerProfileRoutes(enabled, nil); !errors.Is(err, errCustomerProfileRouteRequiresDatabase) {
		t.Fatalf("enabled customer profile route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := customerProfileRoutes(enabled, database); err == nil {
		t.Fatal("enabled customer profile route should require valid JWT settings")
	}
	settings := config.Settings{CustomerProfileAPIEnabled: true, JWT: config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"}}
	registrar, err := customerProfileRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated customer profile route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
