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

func TestAppSettingsRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := appSettingsRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled app settings registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{
		"/api/v1/app-settings",
		"/api/v1/app-settings/1",
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

func TestAppSettingsRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{AppSettingsAPIEnabled: true}
	if _, err := appSettingsRoutes(enabled, nil); !errors.Is(err, errAppSettingsRouteRequiresDatabase) {
		t.Fatalf("enabled app settings route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := appSettingsRoutes(enabled, database); err == nil {
		t.Fatal("enabled app settings route should require valid JWT settings")
	}
	settings := config.Settings{
		AppSettingsAPIEnabled: true,
		JWT:                   config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := appSettingsRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/app-settings", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated app settings route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
