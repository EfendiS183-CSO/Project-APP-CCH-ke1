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

func TestFollowUpStatusMasterRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := followUpStatusMasterRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled follow-up status master registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{
		"/api/v1/masters/follow-up-statuses",
		"/api/v1/masters/follow-up-statuses/options",
		"/api/v1/masters/follow-up-statuses/001",
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

func TestFollowUpStatusMasterRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{FollowUpStatusMasterAPIEnabled: true}
	if _, err := followUpStatusMasterRoutes(enabled, nil); !errors.Is(err, errFollowUpStatusMasterRouteRequiresDatabase) {
		t.Fatalf("enabled follow-up status route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := followUpStatusMasterRoutes(enabled, database); err == nil {
		t.Fatal("enabled follow-up status route should require valid JWT settings")
	}
	settings := config.Settings{
		FollowUpStatusMasterAPIEnabled: true,
		JWT:                            config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := followUpStatusMasterRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/follow-up-statuses", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated follow-up status route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
