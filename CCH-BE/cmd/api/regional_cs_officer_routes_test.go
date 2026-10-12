package main

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/platform/config"
	"cch.local/backend/internal/platform/httpapi"
)

func TestRegionalAndCSOfficerRoutesAreDisabledIndependently(t *testing.T) {
	regional, err := regionalAdminRoutes(config.Settings{}, nil)
	if err != nil || regional != nil {
		t.Fatalf("default regional registrar/error = %v/%v", regional, err)
	}
	csOfficers, err := csOfficerStatusRoutes(config.Settings{}, nil)
	if err != nil || csOfficers != nil {
		t.Fatalf("default CS Officer registrar/error = %v/%v", csOfficers, err)
	}
	userLevels, err := userLevelCatalogRoutes(config.Settings{}, nil)
	if err != nil || userLevels != nil {
		t.Fatalf("default user-level catalog registrar/error = %v/%v", userLevels, err)
	}
	for _, path := range []string{"/api/v1/admin/regional-offices", "/api/v1/admin/regional-offices/options", "/api/v1/admin/cs-officers", "/api/v1/admin/cs-officers/cs.user", "/api/v1/admin/user-levels", "/api/v1/admin/user-levels/1"} {
		response, err := httpapi.NewServer(nil, regional, csOfficers, userLevels).Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want default-off 404", path, response.StatusCode)
		}
	}

	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	regionalOnly, err := regionalAdminRoutes(config.Settings{RegionalAdminAPIEnabled: true, JWT: config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"}}, database)
	if err != nil {
		t.Fatal(err)
	}
	csOfficers, err = csOfficerStatusRoutes(config.Settings{}, database)
	if err != nil || csOfficers != nil || regionalOnly == nil {
		t.Fatalf("regional-only setting should produce only a regional registrar: regional=%v CS=%v err=%v", regionalOnly != nil, csOfficers != nil, err)
	}
	userLevels, err = userLevelCatalogRoutes(config.Settings{UserLevelCatalogAPIEnabled: true, JWT: config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"}}, database)
	if err != nil || userLevels == nil {
		t.Fatalf("user-level catalog should enable independently: registrar=%v err=%v", userLevels != nil, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRegionalAndCSOfficerRoutesRequireDatabaseAndJWT(t *testing.T) {
	if _, err := regionalAdminRoutes(config.Settings{RegionalAdminAPIEnabled: true}, nil); !errors.Is(err, errRegionalAdminRouteRequiresDatabase) {
		t.Fatalf("regional route without DB = %v", err)
	}
	if _, err := csOfficerStatusRoutes(config.Settings{CSOfficerStatusAPIEnabled: true}, nil); !errors.Is(err, errCSOfficerStatusRouteRequiresDatabase) {
		t.Fatalf("CS Officer route without DB = %v", err)
	}
	if _, err := userLevelCatalogRoutes(config.Settings{UserLevelCatalogAPIEnabled: true}, nil); !errors.Is(err, errUserLevelCatalogRouteRequiresDatabase) {
		t.Fatalf("user-level catalog route without DB = %v", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	for _, test := range []struct {
		name     string
		settings config.Settings
		register func(config.Settings, *sql.DB) (httpapi.VersionedRouteRegistrar, error)
		path     string
	}{
		{name: "regional admin", settings: config.Settings{RegionalAdminAPIEnabled: true}, register: regionalAdminRoutes, path: "/api/v1/admin/regional-offices"},
		{name: "CS Officer", settings: config.Settings{CSOfficerStatusAPIEnabled: true}, register: csOfficerStatusRoutes, path: "/api/v1/admin/cs-officers"},
		{name: "user-level catalog", settings: config.Settings{UserLevelCatalogAPIEnabled: true}, register: userLevelCatalogRoutes, path: "/api/v1/admin/user-levels"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.register(test.settings, database); err == nil {
				t.Fatal("enabled route should require a valid JWT verifier")
			}
			settings := test.settings
			settings.JWT = config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"}
			registrar, err := test.register(settings, database)
			if err != nil {
				t.Fatal(err)
			}
			response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, test.path, nil))
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("unauthenticated route status = %d, want 401", response.StatusCode)
			}
		})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
