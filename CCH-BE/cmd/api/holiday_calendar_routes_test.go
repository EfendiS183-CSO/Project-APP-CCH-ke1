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

func TestHolidayCalendarMasterRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := holidayCalendarMasterRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled holiday calendar registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{
		"/api/v1/masters/holiday-calendar",
		"/api/v1/masters/holiday-calendar/2026-08-17",
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

func TestHolidayCalendarMasterRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{HolidayCalendarMasterAPIEnabled: true}
	if _, err := holidayCalendarMasterRoutes(enabled, nil); !errors.Is(err, errHolidayCalendarMasterRouteRequiresDatabase) {
		t.Fatalf("enabled holiday calendar route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := holidayCalendarMasterRoutes(enabled, database); err == nil {
		t.Fatal("enabled holiday calendar route should require valid JWT settings")
	}
	settings := config.Settings{
		HolidayCalendarMasterAPIEnabled: true,
		JWT:                             config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := holidayCalendarMasterRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/holiday-calendar", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated holiday calendar route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
