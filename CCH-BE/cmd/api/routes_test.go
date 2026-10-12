package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"

	intakesession "cch.local/backend/internal/modules/intake/session"
	"cch.local/backend/internal/platform/config"
	"cch.local/backend/internal/platform/httpapi"
)

func TestCallCenterComplaintRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := callCenterComplaintRoutes(config.Settings{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if registrar != nil {
		t.Fatal("disabled complaint routes returned a registrar")
	}
	for _, path := range []string{
		"/api/v1/complaints/call-center/open",
		"/api/v1/complaints/call-center/open/export",
		"/api/v1/complaints/dashboard/open/count",
		"/api/v1/complaints/dashboard/received-open/count",
		"/api/v1/complaints/dashboard/sent-open/count",
		"/api/v1/complaints/dashboard/completed/count",
		"/api/v1/complaints/dashboard/received-open",
		"/api/v1/complaints/dashboard/sent-open",
		"/api/v1/complaints/reports/received-open",
		"/api/v1/complaints/reports/received-open/duration-buckets",
		"/api/v1/complaints/reports/sent-open",
		"/api/v1/complaints/call-center/TKT-001",
		"/api/v1/complaints/call-center/TKT-001/history",
		"/api/v1/complaints/call-center/TKT-001/status-options",
		"/api/v1/complaints/call-center/TKT-001/follow-ups",
		"/api/v1/complaints/reports/sent-open",
		"/api/v1/follow-ups/call-center/office",
		"/api/v1/follow-ups/call-center/office/due-today",
		"/api/v1/complaints/call-center/lookups",
	} {
		response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatalf("request disabled complaint route %s: %v", path, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want %d when feature is disabled", path, response.StatusCode, http.StatusNotFound)
		}
	}
}

func TestCallCenterComplaintRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{CallCenterComplaintReadAPIEnabled: true}
	if _, err := callCenterComplaintRoutes(enabled, nil); err == nil {
		t.Fatal("enabled complaint routes should require a database")
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := callCenterComplaintRoutes(enabled, database); err == nil {
		t.Fatal("enabled complaint routes should require configured JWT verification")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterStatusWriteRouteIsDisabledByDefault(t *testing.T) {
	registrar, err := callCenterStatusWriteRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled status route registrar/error = %v/%v", registrar, err)
	}
}

func TestCallCenterStatusWriteRouteRequiresLegacyFenceAndDatabase(t *testing.T) {
	settings := config.Settings{CallCenterStatusWriteAPIEnabled: true}
	if _, err := callCenterStatusWriteRoutes(settings, nil); !errors.Is(err, errStatusWriteRequiresLegacyFence) {
		t.Fatalf("route error = %v, want legacy writer fence requirement", err)
	}
	settings.LegacyStatusWritesFenced = true
	if _, err := callCenterStatusWriteRoutes(settings, nil); !errors.Is(err, errStatusWriteRequiresDatabase) {
		t.Fatalf("route error = %v, want database requirement", err)
	}
}

func TestCallCenterStatusWriteRouteRequiresMigrationAndStaffJWT(t *testing.T) {
	settings := config.Settings{CallCenterStatusWriteAPIEnabled: true, LegacyStatusWritesFenced: true}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT idempotency_key FROM cch_be_status_transition_idempotency LIMIT 0")).
		WillReturnRows(sqlmock.NewRows([]string{"idempotency_key"}))
	if _, err := callCenterStatusWriteRoutes(settings, database); err == nil {
		t.Fatal("status route should require a valid staff JWT verifier")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterStatusWriteRouteIsIndependentAndJWTProtected(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT idempotency_key FROM cch_be_status_transition_idempotency LIMIT 0")).
		WillReturnRows(sqlmock.NewRows([]string{"idempotency_key"}))
	settings := config.Settings{
		CallCenterStatusWriteAPIEnabled: true,
		LegacyStatusWritesFenced:        true,
		JWT:                             config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := callCenterStatusWriteRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.NewServer(nil, registrar)
	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/status", strings.NewReader(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status mutation route status = %d, want unauthorized without JWT", response.StatusCode)
	}
	readRequest := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/status-options", nil)
	readRequest.Header.Set("X-Authorization", "Bearer "+signTestJWT(t, settings.JWT.Secret, settings.JWT.Issuer))
	response, err = app.Test(readRequest)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("read route status = %d, want not found when only write flag is enabled", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterFollowUpPlanRouteIsDisabledByDefault(t *testing.T) {
	registrar, err := callCenterFollowUpPlanRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled follow-up plan registrar/error = %v/%v", registrar, err)
	}
}

func TestCallCenterFollowUpPlanRouteRequiresDatabaseAndJWT(t *testing.T) {
	settings := config.Settings{CallCenterFollowUpPlanAPIEnabled: true}
	if _, err := callCenterFollowUpPlanRoutes(settings, nil); !errors.Is(err, errFollowUpPlanRequiresDatabase) {
		t.Fatalf("route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := callCenterFollowUpPlanRoutes(settings, database); err == nil {
		t.Fatal("follow-up plan route should require valid staff JWT settings")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterFollowUpPlanRouteIsIndependentAndJWTProtected(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	settings := config.Settings{
		CallCenterFollowUpPlanAPIEnabled: true,
		JWT:                              config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := callCenterFollowUpPlanRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.NewServer(nil, registrar)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/follow-ups", strings.NewReader(`{}`))
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated plan route status = %d, want unauthorized", response.StatusCode)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/follow-ups/call-center/office", nil)
	request.Header.Set("X-Authorization", "Bearer "+signTestJWT(t, settings.JWT.Secret, settings.JWT.Issuer))
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("follow-up read route status = %d, want not found when only plan flag is enabled", response.StatusCode)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/open", nil)
	request.Header.Set("X-Authorization", "Bearer "+signTestJWT(t, settings.JWT.Secret, settings.JWT.Issuer))
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("complaint read route status = %d, want not found when only plan flag is enabled", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterFollowUpRealizationRouteIsDisabledAndRequiresPrerequisites(t *testing.T) {
	registrar, err := callCenterFollowUpRealizationRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled realization registrar/error = %v/%v", registrar, err)
	}
	settings := config.Settings{CallCenterFollowUpRealizeAPIEnabled: true}
	if _, err := callCenterFollowUpRealizationRoutes(settings, nil); !errors.Is(err, errFollowUpRealizationRequiresDatabase) {
		t.Fatalf("route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := callCenterFollowUpRealizationRoutes(settings, database); err == nil {
		t.Fatal("realization route should require valid staff JWT settings")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterFollowUpRealizationRouteIsIndependentAndJWTProtected(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	settings := config.Settings{
		CallCenterFollowUpRealizeAPIEnabled: true,
		JWT:                                 config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := callCenterFollowUpRealizationRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.NewServer(nil, registrar)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/follow-ups/FU1/realize", strings.NewReader(`{"outcome":"done"}`))
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated realization status = %d, want unauthorized", response.StatusCode)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/follow-ups", strings.NewReader(`{}`))
	request.Header.Set("X-Authorization", "Bearer "+signTestJWT(t, settings.JWT.Secret, settings.JWT.Issuer))
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("plan route status = %d, want not found when only realization flag is enabled", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterDocumentMetadataRouteIsDisabledAndRequiresPrerequisites(t *testing.T) {
	registrar, err := callCenterDocumentMetadataRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled document metadata registrar/error = %v/%v", registrar, err)
	}
	settings := config.Settings{CallCenterDocumentMetadataAPIEnabled: true}
	if _, err := callCenterDocumentMetadataRoutes(settings, nil); !errors.Is(err, errDocumentMetadataRequiresDatabase) {
		t.Fatalf("route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id FROM cch_be_document_files LIMIT 0")).
		WillReturnRows(sqlmock.NewRows([]string{"document_id"}))
	if _, err := callCenterDocumentMetadataRoutes(settings, database); err == nil {
		t.Fatal("document metadata route should require valid staff JWT settings")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterDocumentMetadataRouteIsIndependentAndJWTProtected(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id FROM cch_be_document_files LIMIT 0")).
		WillReturnRows(sqlmock.NewRows([]string{"document_id"}))
	settings := config.Settings{
		CallCenterDocumentMetadataAPIEnabled: true,
		JWT:                                  config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := callCenterDocumentMetadataRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.NewServer(nil, registrar)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated document metadata status = %d, want unauthorized", response.StatusCode)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/open", nil)
	request.Header.Set("X-Authorization", "Bearer "+signTestJWT(t, settings.JWT.Secret, settings.JWT.Issuer))
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("complaint read status = %d, want not found when only document metadata flag is enabled", response.StatusCode)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents/file", nil)
	request.Header.Set("X-Authorization", "Bearer "+signTestJWT(t, settings.JWT.Secret, settings.JWT.Issuer))
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("file route status = %d, want not found because binary serving is not registered", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterDocumentUploadRouteIsDisabledAndRequiresSchemaAndInfrastructure(t *testing.T) {
	registrar, err := callCenterDocumentUploadRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled upload registrar/error = %v/%v", registrar, err)
	}
	settings := config.Settings{CallCenterDocumentUploadAPIEnabled: true}
	if _, err := callCenterDocumentUploadRoutes(settings, nil); !errors.Is(err, errDocumentUploadRequiresDatabase) {
		t.Fatalf("route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id FROM cch_be_document_files LIMIT 0")).
		WillReturnError(errors.New("missing migration"))
	if _, err := callCenterDocumentUploadRoutes(settings, database); err == nil {
		t.Fatal("upload route should fail if the document migration is missing")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterDocumentUploadRouteRequiresPrivateRootScannerAndStaffJWT(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id FROM cch_be_document_files LIMIT 0")).
		WillReturnRows(sqlmock.NewRows([]string{"document_id"}))
	settings := config.Settings{
		CallCenterDocumentUploadAPIEnabled: true,
		DocumentPublicRoot:                 t.TempDir(),
		DocumentStorageRoot:                filepath.Join(t.TempDir(), "private"),
		DocumentClamAVAddress:              "127.0.0.1:3310",
	}
	if _, err := callCenterDocumentUploadRoutes(settings, database); err == nil {
		t.Fatal("upload route should require valid staff JWT settings")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	settings.JWT = config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"}
	settings.DocumentStorageRoot = filepath.Join(settings.DocumentPublicRoot, "public-child")
	if _, err := callCenterDocumentUploadRoutes(settings, database); err == nil {
		t.Fatal("upload route should reject storage under the public root")
	}
}

func TestCallCenterDocumentUploadRouteIsJWTProtectedAndDoesNotServeDownloads(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id FROM cch_be_document_files LIMIT 0")).
		WillReturnRows(sqlmock.NewRows([]string{"document_id"}))
	baseRoot := t.TempDir()
	publicRoot := filepath.Join(baseRoot, "www")
	if err := os.Mkdir(publicRoot, 0700); err != nil {
		t.Fatal(err)
	}
	settings := config.Settings{
		CallCenterDocumentUploadAPIEnabled: true,
		DocumentPublicRoot:                 publicRoot,
		DocumentStorageRoot:                filepath.Join(baseRoot, "private"),
		DocumentClamAVAddress:              "127.0.0.1:3310",
		JWT:                                config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := callCenterDocumentUploadRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.NewServer(nil, registrar)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/documents", strings.NewReader("%PDF-1.7"))
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated upload status = %d, want unauthorized", response.StatusCode)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents/file", nil)
	request.Header.Set("X-Authorization", "Bearer "+signTestJWT(t, settings.JWT.Secret, settings.JWT.Issuer))
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("download route status = %d, want not found because download is not registered", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterDocumentDownloadRouteIsIndependentFromUploadScanner(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id FROM cch_be_document_files LIMIT 0")).
		WillReturnRows(sqlmock.NewRows([]string{"document_id"}))
	baseRoot := t.TempDir()
	publicRoot := filepath.Join(baseRoot, "www")
	if err := os.Mkdir(publicRoot, 0700); err != nil {
		t.Fatal(err)
	}
	settings := config.Settings{
		CallCenterDocumentDownloadAPIEnabled: true,
		DocumentPublicRoot:                   publicRoot,
		DocumentStorageRoot:                  filepath.Join(baseRoot, "private"),
		JWT:                                  config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := callCenterDocumentDownloadRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.NewServer(nil, registrar)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents/0123456789abcdef0123456789abcdef/file", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated download status = %d, want unauthorized", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterDocumentDeleteRouteIsDisabledAndRequiresPrerequisites(t *testing.T) {
	registrar, err := callCenterDocumentDeleteRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled document delete route registrar/error = %v/%v", registrar, err)
	}
	settings := config.Settings{CallCenterDocumentDeleteAPIEnabled: true}
	if _, err := callCenterDocumentDeleteRoutes(settings, nil); !errors.Is(err, errDocumentDeleteRequiresDatabase) {
		t.Fatalf("route error = %v, want database requirement", err)
	}
}

func TestCallCenterDocumentPreviewRouteIsDisabledAndRequiresPrerequisites(t *testing.T) {
	registrar, err := callCenterDocumentPreviewRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled document preview route registrar/error = %v/%v", registrar, err)
	}
	settings := config.Settings{CallCenterDocumentPreviewAPIEnabled: true}
	if _, err := callCenterDocumentPreviewRoutes(settings, nil); !errors.Is(err, errDocumentPreviewRequiresDatabase) {
		t.Fatalf("route error = %v, want database requirement", err)
	}
}

func TestCallCenterDocumentPreviewRouteIsIndependentAndJWTProtected(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id FROM cch_be_document_files LIMIT 0")).
		WillReturnRows(sqlmock.NewRows([]string{"document_id"}))
	baseRoot := t.TempDir()
	publicRoot := filepath.Join(baseRoot, "www")
	if err := os.Mkdir(publicRoot, 0700); err != nil {
		t.Fatal(err)
	}
	settings := config.Settings{
		CallCenterDocumentPreviewAPIEnabled: true,
		DocumentPublicRoot:                  publicRoot,
		DocumentStorageRoot:                 filepath.Join(baseRoot, "private"),
		JWT:                                 config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := callCenterDocumentPreviewRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.NewServer(nil, registrar)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents/0123456789abcdef0123456789abcdef/preview", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated document preview status = %d, want unauthorized", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterDocumentDeleteRouteIsIndependentAndJWTProtected(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id FROM cch_be_document_files LIMIT 0")).
		WillReturnRows(sqlmock.NewRows([]string{"document_id"}))
	baseRoot := t.TempDir()
	publicRoot := filepath.Join(baseRoot, "www")
	if err := os.Mkdir(publicRoot, 0700); err != nil {
		t.Fatal(err)
	}
	settings := config.Settings{
		CallCenterDocumentDeleteAPIEnabled: true,
		DocumentPublicRoot:                 publicRoot,
		DocumentStorageRoot:                filepath.Join(baseRoot, "private"),
		JWT:                                config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := callCenterDocumentDeleteRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.NewServer(nil, registrar)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/complaints/call-center/TKT-001/documents/0123456789abcdef0123456789abcdef", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated document delete status = %d, want unauthorized", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnabledCallCenterComplaintRoutesAreJWTProtected(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	settings := config.Settings{
		CallCenterComplaintReadAPIEnabled: true,
		JWT: config.JWTSettings{
			Secret: "test-only-signing-key-that-is-long-enough",
			Issuer: "cch-test",
		},
	}
	registrar, err := callCenterComplaintRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.NewServer(nil, registrar)
	for _, path := range []string{
		"/api/v1/complaints/call-center/open",
		"/api/v1/complaints/call-center/TKT-001",
		"/api/v1/complaints/call-center/TKT-001/history",
		"/api/v1/complaints/call-center/TKT-001/status-options",
		"/api/v1/complaints/call-center/TKT-001/follow-ups",
		"/api/v1/follow-ups/call-center/office",
		"/api/v1/follow-ups/call-center/office/due-today",
		"/api/v1/complaints/call-center/lookups",
	} {
		response, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatalf("request protected complaint route %s: %v", path, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want %d without JWT", path, response.StatusCode, http.StatusUnauthorized)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestComplaintReadFlagDoesNotEnableCustomerSearch(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	settings := config.Settings{
		CallCenterComplaintReadAPIEnabled: true,
		JWT: config.JWTSettings{
			Secret: "test-only-signing-key-that-is-long-enough",
			Issuer: "cch-test",
		},
	}
	registrar, err := callCenterComplaintRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/customers/search?q=customer", nil)
	request.Header.Set("X-Authorization", signTestJWT(t, settings.JWT.Secret, settings.JWT.Issuer))
	response, err := httpapi.NewServer(nil, registrar).Test(request)
	if err != nil {
		t.Fatalf("request customer search with complaint flag only: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("customer search status = %d, want %d until its own flag is enabled", response.StatusCode, http.StatusNotFound)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerSearchFlagDoesNotEnableComplaintRoutes(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	settings := config.Settings{
		CallCenterCustomerSearchAPIEnabled: true,
		JWT: config.JWTSettings{
			Secret: "test-only-signing-key-that-is-long-enough",
			Issuer: "cch-test",
		},
	}
	registrar, err := callCenterCustomerSearchRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/open", nil)
	request.Header.Set("X-Authorization", signTestJWT(t, settings.JWT.Secret, settings.JWT.Issuer))
	response, err := httpapi.NewServer(nil, registrar).Test(request)
	if err != nil {
		t.Fatalf("request complaint route with customer flag only: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("complaint route status = %d, want %d until complaint flag is enabled", response.StatusCode, http.StatusNotFound)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func signTestJWT(t *testing.T, secret string, issuer string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss": issuer,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Minute).Unix(),
		"security": map[string]any{
			"username":     "agent.test",
			"userid":       "40005CC",
			"parentuserid": "40005",
			"userlevelid":  "3",
			"permission":   256,
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign test JWT: %v", err)
	}
	return token
}

func TestCallCenterCustomerSearchRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := callCenterCustomerSearchRoutes(config.Settings{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if registrar != nil {
		t.Fatal("disabled customer search route returned a registrar")
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/customers/search?q=customer", nil))
	if err != nil {
		t.Fatalf("request disabled customer search route: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d while customer search is disabled", response.StatusCode, http.StatusNotFound)
	}
}

func TestComplaintTypeMasterRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := complaintTypeMasterRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled master route registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{
		"/api/v1/masters/complaint-types",
		"/api/v1/masters/complaint-types/1",
	} {
		response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatalf("request disabled master route: %v", err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404 while disabled", path, response.StatusCode)
		}
	}
}

func TestComplaintSourceMasterRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := complaintSourceMasterRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled complaint source master registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{
		"/api/v1/masters/complaint-sources",
		"/api/v1/masters/complaint-sources/1",
	} {
		response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404 while source master is disabled", path, response.StatusCode)
		}
	}
}

func TestShippingServiceMasterRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := shippingServiceMasterRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled shipping service master registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{
		"/api/v1/masters/shipping-services",
		"/api/v1/masters/shipping-services/REG",
	} {
		response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404 while shipping service master is disabled", path, response.StatusCode)
		}
	}
}

func TestOfficeMasterRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := officeMasterRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled office master registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{"/api/v1/masters/offices", "/api/v1/masters/offices/options", "/api/v1/masters/offices/40005CC"} {
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

func TestOfficeMasterRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{OfficeMasterAPIEnabled: true}
	if _, err := officeMasterRoutes(enabled, nil); !errors.Is(err, errOfficeMasterRouteRequiresDatabase) {
		t.Fatalf("enabled office route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := officeMasterRoutes(enabled, database); err == nil {
		t.Fatal("enabled office route should require valid JWT settings")
	}
	settings := config.Settings{OfficeMasterAPIEnabled: true, JWT: config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"}}
	registrar, err := officeMasterRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/offices", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated office route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHandlingStatusMasterRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := handlingStatusMasterRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled handling status registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{"/api/v1/masters/handling-statuses", "/api/v1/masters/handling-statuses/103"} {
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

func TestHandlingStatusMasterRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{HandlingStatusMasterAPIEnabled: true}
	if _, err := handlingStatusMasterRoutes(enabled, nil); !errors.Is(err, errHandlingStatusMasterRouteRequiresDatabase) {
		t.Fatalf("enabled handling status route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := handlingStatusMasterRoutes(enabled, database); err == nil {
		t.Fatal("enabled handling status route should require valid JWT settings")
	}
	settings := config.Settings{HandlingStatusMasterAPIEnabled: true, JWT: config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"}}
	registrar, err := handlingStatusMasterRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/handling-statuses", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated handling status route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentTypeMasterRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := documentTypeMasterRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled document type registrar/error = %v/%v", registrar, err)
	}
	for _, path := range []string{"/api/v1/masters/document-types", "/api/v1/masters/document-types/PDF"} {
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

func TestDocumentTypeMasterRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{DocumentTypeMasterAPIEnabled: true}
	if _, err := documentTypeMasterRoutes(enabled, nil); !errors.Is(err, errDocumentTypeMasterRouteRequiresDatabase) {
		t.Fatalf("enabled document type route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := documentTypeMasterRoutes(enabled, database); err == nil {
		t.Fatal("enabled document type route should require valid JWT settings")
	}
	settings := config.Settings{DocumentTypeMasterAPIEnabled: true, JWT: config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"}}
	registrar, err := documentTypeMasterRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/document-types", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated document type route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShippingServiceMasterRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{ShippingServiceMasterAPIEnabled: true}
	if _, err := shippingServiceMasterRoutes(enabled, nil); !errors.Is(err, errShippingServiceMasterRouteRequiresDatabase) {
		t.Fatalf("enabled shipping service master route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := shippingServiceMasterRoutes(enabled, database); err == nil {
		t.Fatal("enabled shipping service master route should require valid JWT settings")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnabledShippingServiceMasterRouteRequiresJWT(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	settings := config.Settings{
		ShippingServiceMasterAPIEnabled: true,
		JWT:                             config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := shippingServiceMasterRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/shipping-services", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated shipping service route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestComplaintSourceMasterRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{ComplaintSourceMasterAPIEnabled: true}
	if _, err := complaintSourceMasterRoutes(enabled, nil); !errors.Is(err, errComplaintSourceMasterRouteRequiresDatabase) {
		t.Fatalf("enabled source master route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := complaintSourceMasterRoutes(enabled, database); err == nil {
		t.Fatal("enabled source master route should require valid JWT settings")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnabledComplaintSourceMasterRouteRequiresJWT(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	settings := config.Settings{
		ComplaintSourceMasterAPIEnabled: true,
		JWT:                             config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := complaintSourceMasterRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/complaint-sources", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated source master route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestComplaintTypeMasterRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{ComplaintTypeMasterAPIEnabled: true}
	if _, err := complaintTypeMasterRoutes(enabled, nil); !errors.Is(err, errComplaintTypeMasterRouteRequiresDatabase) {
		t.Fatalf("enabled complaint type master route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := complaintTypeMasterRoutes(enabled, database); err == nil {
		t.Fatal("enabled complaint type master route should require valid JWT configuration")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnabledComplaintTypeMasterRouteRequiresJWT(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	settings := config.Settings{
		ComplaintTypeMasterAPIEnabled: true,
		JWT:                           config.JWTSettings{Secret: "test-only-signing-key-that-is-long-enough", Issuer: "cch-test"},
	}
	registrar, err := complaintTypeMasterRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/complaint-types", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated master route status = %d, want 401", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCallCenterCustomerSearchRoutesRequireDatabaseAndJWT(t *testing.T) {
	enabled := config.Settings{CallCenterCustomerSearchAPIEnabled: true}
	if _, err := callCenterCustomerSearchRoutes(enabled, nil); !errors.Is(err, errCustomerSearchRouteRequiresDatabase) {
		t.Fatalf("enabled customer search route error = %v, want database requirement", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := callCenterCustomerSearchRoutes(enabled, database); err == nil {
		t.Fatal("enabled customer search route should require valid JWT configuration")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnabledCallCenterCustomerSearchRouteRequiresJWT(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	settings := config.Settings{
		CallCenterCustomerSearchAPIEnabled: true,
		JWT: config.JWTSettings{
			Secret: "test-only-signing-key-that-is-long-enough",
			Issuer: "cch-test",
		},
	}
	registrar, err := callCenterCustomerSearchRoutes(settings, database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/customers/search?q=customer", nil))
	if err != nil {
		t.Fatalf("request protected customer search route: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d without JWT", response.StatusCode, http.StatusUnauthorized)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGuestIntakeSessionRouteIsDisabledByDefault(t *testing.T) {
	registrar, err := guestIntakeSessionRoutes(config.GuestIntakeSessionSettings{}, config.JWTSettings{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if registrar != nil {
		t.Fatal("disabled guest intake session route returned a registrar")
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodPost, "/api/v1/intake/sessions", nil))
	if err != nil {
		t.Fatalf("request disabled guest intake route: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d while guest intake is disabled", response.StatusCode, http.StatusNotFound)
	}
}

func TestGuestIntakeSessionRouteRequiresDatabaseAndSeparateSecret(t *testing.T) {
	settings := guestIntakeSessionSettings()
	if _, err := guestIntakeSessionRoutes(settings, config.JWTSettings{}, nil); !errors.Is(err, errGuestIntakeSessionRequiresDatabase) {
		t.Fatalf("route error = %v, want database prerequisite", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	staffJWT := config.JWTSettings{Secret: settings.SigningKey, Issuer: "staff-issuer"}
	if _, err := guestIntakeSessionRoutes(settings, staffJWT, database); !errors.Is(err, errGuestIntakeSessionKeyMustBeSeparate) {
		t.Fatalf("route error = %v, want distinct signing key requirement", err)
	}
	invalidSettings := settings
	invalidSettings.AllowedOrigins = nil
	if _, err := guestIntakeSessionRoutes(invalidSettings, config.JWTSettings{}, database); err == nil {
		t.Fatal("guest intake route should reject an empty origin allowlist")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGuestIntakeSessionRouteIssuesTokenForAllowedOriginWithoutStaffJWT(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	registrar, err := guestIntakeSessionRoutes(guestIntakeSessionSettings(), config.JWTSettings{}, database)
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.NewServer(nil, registrar)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/intake/sessions", nil)
	request.Header.Set("Origin", "https://chat.example.test")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request guest session issuance: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	var body struct {
		ConversationID string `json:"conversation_id"`
		Token          string `json:"token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode guest session response: %v", err)
	}
	if len(body.ConversationID) != 32 || body.Token == "" {
		t.Fatalf("invalid guest session response: %#v", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGuestIntakeSessionRouteRejectsDisallowedOrigin(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	registrar, err := guestIntakeSessionRoutes(guestIntakeSessionSettings(), config.JWTSettings{}, database)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/intake/sessions", nil)
	request.Header.Set("Origin", "https://untrusted.example.test")
	response, err := httpapi.NewServer(nil, registrar).Test(request)
	if err != nil {
		t.Fatalf("request guest session from disallowed origin: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGuestIntakeSubmissionRouteIsDisabledByDefault(t *testing.T) {
	registrar, err := guestIntakeSubmissionRoutes(config.GuestIntakeSubmissionSettings{}, config.GuestIntakeSessionSettings{}, config.JWTSettings{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if registrar != nil {
		t.Fatal("disabled guest intake submission returned a registrar")
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodPost, "/api/v1/intake/submissions", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d while submission is disabled", response.StatusCode, http.StatusNotFound)
	}
}

func TestGuestIntakeSubmissionRouteRequiresDatabaseSessionAndSeparateJWT(t *testing.T) {
	settings := guestIntakeSubmissionSettings()
	if _, err := guestIntakeSubmissionRoutes(settings, guestIntakeSessionSettings(), config.JWTSettings{}, nil); !errors.Is(err, errGuestIntakeSubmissionRequiresDatabase) {
		t.Fatalf("route error = %v, want database prerequisite", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := guestIntakeSubmissionRoutes(settings, config.GuestIntakeSessionSettings{}, guestIntakeStaffJWT(), database); !errors.Is(err, errGuestIntakeSubmissionRequiresSession) {
		t.Fatalf("route error = %v, want enabled guest session configuration", err)
	}
	if _, err := guestIntakeSubmissionRoutes(settings, guestIntakeSessionSettings(), config.JWTSettings{Secret: guestIntakeSessionSettings().SigningKey, Issuer: "staff"}, database); !errors.Is(err, errGuestIntakeSubmissionKeyMustBeSeparate) {
		t.Fatalf("route error = %v, want separate signing key requirement", err)
	}
	invalidGatewaySettings := settings
	invalidGatewaySettings.CCHBaseURL = "http://legacy-cch.example.test"
	if _, err := guestIntakeSubmissionRoutes(invalidGatewaySettings, guestIntakeSessionSettings(), guestIntakeStaffJWT(), database); err == nil {
		t.Fatal("submission route should reject non-HTTPS CCH URL")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGuestIntakeSubmissionRouteRequiresGuestTokenAndAllowedOrigin(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	registrar, err := guestIntakeSubmissionRoutes(guestIntakeSubmissionSettings(), guestIntakeSessionSettings(), guestIntakeStaffJWT(), database)
	if err != nil {
		t.Fatal(err)
	}
	app := httpapi.NewServer(nil, registrar)
	body := `{"consent":true,"summary":"Summary","payload":{"Nama_Pelanggan":"CUSTOMER","Nomor_HP_Pelanggan":"0812345678","Alamat_Pelanggan":"ADDRESS","Jenis_Pengaduan":1,"Jenis_Kiriman":"REG","Nomor_Kiriman":"WAYBILL-1","Kantor_Asal_Kiriman":"40005CC","Kantor_Tujuan_Kiriman":"40006CC","Uraian_Pengaduan":"Complaint"}}`
	for _, test := range []struct {
		origin string
		want   int
	}{
		{origin: "https://chat.example.test", want: http.StatusUnauthorized},
		{origin: "https://untrusted.example.test", want: http.StatusForbidden},
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/intake/submissions", strings.NewReader(body))
		request.Header.Set("Origin", test.origin)
		request.Header.Set("Content-Type", "application/json")
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.want {
			t.Errorf("origin=%q status=%d, want %d", test.origin, response.StatusCode, test.want)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGuestIntakeSubmissionRouteReturnsExistingTicketWithGuestTokenOnly(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	guestSettings := guestIntakeSessionSettings()
	tokenService, err := intakesession.NewService(guestSettings.SigningKey, guestSettings.Issuer, guestSettings.TokenLifetime)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := tokenService.Issue()
	if err != nil {
		t.Fatal(err)
	}
	query := "SELECT conversation_id, status, review_reason, ticket_id, customer_id,\n\tintake_payload, summary, consent_at, created_at\nFROM cch_chatbot_intake_drafts\nWHERE conversation_id = ?\nLIMIT 1"
	createdAt := time.Now().UTC().Add(-time.Minute)
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(issued.ConversationID).
		WillReturnRows(sqlmock.NewRows([]string{"conversation_id", "status", "review_reason", "ticket_id", "customer_id", "intake_payload", "summary", "consent_at", "created_at"}).
			AddRow(issued.ConversationID, "created", nil, "TICKET-REPLAY", int64(42), `{}`, "Summary", createdAt, createdAt))
	registrar, err := guestIntakeSubmissionRoutes(guestIntakeSubmissionSettings(), guestSettings, guestIntakeStaffJWT(), database)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/intake/submissions", strings.NewReader(`{"consent":true,"summary":"Summary","payload":{"Nama_Pelanggan":"CUSTOMER","Nomor_HP_Pelanggan":"0812345678","Alamat_Pelanggan":"ADDRESS","Jenis_Pengaduan":1,"Jenis_Kiriman":"REG","Nomor_Kiriman":"WAYBILL-1","Kantor_Asal_Kiriman":"40005CC","Kantor_Tujuan_Kiriman":"40006CC","Uraian_Pengaduan":"Complaint"}}`))
	request.Header.Set("Origin", "https://chat.example.test")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Intake-Session", issued.Token)
	response, err := httpapi.NewServer(nil, registrar).Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	var body struct {
		Status   string `json:"status"`
		TicketID string `json:"ticket_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "created" || body.TicketID != "TICKET-REPLAY" {
		t.Fatalf("unexpected replay response: %#v", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGuestIntakeReviewRoutesAreDisabledByDefault(t *testing.T) {
	registrar, err := guestIntakeReviewRoutes(config.Settings{}, nil)
	if err != nil || registrar != nil {
		t.Fatalf("disabled review registrar=%v error=%v", registrar, err)
	}
	app := httpapi.NewServer(nil, registrar)
	for _, path := range []string{"/api/v1/intake/review/drafts", "/api/v1/intake/review/drafts/12"} {
		response, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("%s status=%d, want 404 while review flag is off", path, response.StatusCode)
		}
	}
}

func TestGuestIntakeReviewRoutesRequireDatabaseJWTAndIntakeOffice(t *testing.T) {
	settings := guestIntakeReviewSettings()
	if _, err := guestIntakeReviewRoutes(settings, nil); !errors.Is(err, errGuestIntakeReviewRequiresDatabase) {
		t.Fatalf("route error=%v, want database prerequisite", err)
	}
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	withoutJWT := settings
	withoutJWT.JWT = config.JWTSettings{}
	if _, err := guestIntakeReviewRoutes(withoutJWT, database); err == nil {
		t.Fatal("review route should require a valid staff JWT verifier")
	}
	withoutOffice := settings
	withoutOffice.GuestIntakeSubmission.OriginOfficeCode = ""
	if _, err := guestIntakeReviewRoutes(withoutOffice, database); err == nil {
		t.Fatal("review route should require a configured intake office")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnabledGuestIntakeReviewRouteRequiresStaffJWT(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	registrar, err := guestIntakeReviewRoutes(guestIntakeReviewSettings(), database)
	if err != nil {
		t.Fatal(err)
	}
	response, err := httpapi.NewServer(nil, registrar).Test(httptest.NewRequest(http.MethodGet, "/api/v1/intake/review/drafts", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401 without staff JWT", response.StatusCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func guestIntakeSessionSettings() config.GuestIntakeSessionSettings {
	return config.GuestIntakeSessionSettings{
		Enabled:        true,
		SigningKey:     "guest-intake-test-signing-key-is-separate",
		Issuer:         "cch-intake-test",
		TokenLifetime:  20 * time.Minute,
		AllowedOrigins: []string{"https://chat.example.test"},
	}
}

func guestIntakeSubmissionSettings() config.GuestIntakeSubmissionSettings {
	return config.GuestIntakeSubmissionSettings{
		Enabled:          true,
		CCHBaseURL:       "https://legacy-cch.example.test",
		CCHUsername:      "intake-bot",
		CCHPassword:      "test-only-password",
		OriginOfficeCode: "40005CC",
	}
}

func guestIntakeStaffJWT() config.JWTSettings {
	return config.JWTSettings{Secret: "test-only-cch-staff-jwt-signing-key", Issuer: "cch-test"}
}

func guestIntakeReviewSettings() config.Settings {
	return config.Settings{
		GuestIntakeReviewAPIEnabled: true,
		JWT:                         guestIntakeStaffJWT(),
		GuestIntakeSubmission: config.GuestIntakeSubmissionSettings{
			CCHBaseURL:       "https://legacy-cch.example.test",
			CCHUsername:      "intake-bot",
			CCHPassword:      "test-only-password",
			OriginOfficeCode: "40005CC",
		},
	}
}
