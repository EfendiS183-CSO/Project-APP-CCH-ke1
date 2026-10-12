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

func TestTimezoneMasterRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := timezoneMasterRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled timezone registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{
		"/api/v1/masters/timezones",
		"/api/v1/masters/timezones/1",
	} {
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

func TestTimezoneMasterRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{TimezoneMasterAPIEnabled: true}
	if _, err := timezoneMasterRoutes(enabled, nil); !errors.Is(err, errTimezoneMasterRouteRequiresDatabase) {
		t.Fatalf("enabled timezone route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := timezoneMasterRoutes(enabled, database); err == nil {
		t.Fatal("enabled timezone route should require valid JWT settings")
	}
	settings := config.Settings{
		TimezoneMasterAPIEnabled: true,
		JWT:                      config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := timezoneMasterRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/timezones", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated timezone route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
