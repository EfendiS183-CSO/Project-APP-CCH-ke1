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

func TestLanguagesMasterRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := languagesMasterRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled languages registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{
		"/api/v1/masters/languages",
		"/api/v1/masters/languages/ID",
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

func TestLanguagesMasterRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{LanguagesMasterAPIEnabled: true}
	if _, err := languagesMasterRoutes(enabled, nil); !errors.Is(err, errLanguagesMasterRouteRequiresDatabase) {
		t.Fatalf("enabled languages route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := languagesMasterRoutes(enabled, database); err == nil {
		t.Fatal("enabled languages route should require valid JWT settings")
	}
	settings := config.Settings{
		LanguagesMasterAPIEnabled: true,
		JWT:                       config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := languagesMasterRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/languages", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated languages route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
