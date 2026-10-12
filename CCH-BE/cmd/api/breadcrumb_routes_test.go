package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/platform/config"
	"cch.local/backend/internal/platform/httpapi"
)

func TestBreadcrumbLinksRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := breadcrumbLinksRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("default breadcrumb registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{
		"/api/v1/admin/breadcrumbs",
		"/api/v1/admin/breadcrumbs/Home",
		"/api/v1/admin/breadcrumbs/Home/ancestors",
	} {
		response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want default-off 404", path, response.StatusCode)
		}
	}
}

func TestBreadcrumbLinksRoutesRequireDatabaseAndJWT(t *testing.T) {
	if _, err := breadcrumbLinksRoutes(config.Settings{BreadcrumbLinksAPIEnabled: true}, nil); err == nil {
		t.Fatal("enabled breadcrumb routes should require a database")
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := breadcrumbLinksRoutes(config.Settings{BreadcrumbLinksAPIEnabled: true}, database); err == nil {
		t.Fatal("enabled breadcrumb routes should require a valid JWT verifier")
	}
	registrar, err := breadcrumbLinksRoutes(config.Settings{
		BreadcrumbLinksAPIEnabled: true,
		JWT:                       config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/admin/breadcrumbs", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated breadcrumb route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
