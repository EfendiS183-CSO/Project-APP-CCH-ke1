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

func TestHelpArticleRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := helpArticleRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled help article registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{
		"/api/v1/help/articles",
		"/api/v1/help/articles/options",
		"/api/v1/help/articles/1",
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

func TestHelpArticleRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{HelpArticleAPIEnabled: true}
	if _, err := helpArticleRoutes(enabled, nil); !errors.Is(err, errHelpArticleRouteRequiresDatabase) {
		t.Fatalf("enabled route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := helpArticleRoutes(enabled, database); err == nil {
		t.Fatal("enabled help article routes should require valid JWT settings")
	}
	enabled.JWT = config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"}
	registrar, err := helpArticleRoutes(enabled, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/help/articles", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated help article route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
