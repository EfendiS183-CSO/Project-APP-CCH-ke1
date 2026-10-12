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

func TestAnnouncementRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := announcementRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled announcement registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{"/api/v1/announcements", "/api/v1/announcements/options", "/api/v1/announcements/12"} {
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

func TestAnnouncementRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{AnnouncementAPIEnabled: true}
	if _, err := announcementRoutes(enabled, nil); !errors.Is(err, errAnnouncementRouteRequiresDatabase) {
		t.Fatalf("enabled announcement route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := announcementRoutes(enabled, database); err == nil {
		t.Fatal("enabled announcement route should require valid JWT settings")
	}
	settings := config.Settings{AnnouncementAPIEnabled: true, JWT: config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"}}
	registrar, err := announcementRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/announcements", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated announcement route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
