package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/documents"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type stubLister struct {
	identity authentication.VerifiedIdentity
	query    documents.Query
	items    []documents.Metadata
	err      error
	calls    int
}

func (service *stubLister) ListForCallCenterTicket(_ context.Context, identity authentication.VerifiedIdentity, query documents.Query) ([]documents.Metadata, error) {
	service.calls++
	service.identity = identity
	service.query = query
	return service.items, service.err
}

type stubVerifier struct {
	identity authentication.VerifiedIdentity
}

func (verifier stubVerifier) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return verifier.identity, nil
}

func TestControllerListsMetadataWithoutFileNamesOrPaths(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test"}
	addedAt := time.Date(2026, time.September, 30, 10, 0, 0, 0, time.UTC)
	service := &stubLister{items: []documents.Metadata{{
		ComplaintID: "TKT-001", HistoryID: "HIST-001", DocumentTypeCode: "BA1", DocumentType: "Berita Acara",
		UploadOfficeCode: "40005CC", AddedAt: addedAt, FileCount: 2,
	}}}
	app := testApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents?limit=10&offset=5", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("status/cache = %d/%q", response.StatusCode, response.Header.Get("Cache-Control"))
	}
	if service.calls != 1 || service.identity != identity || service.query.ComplaintID != "TKT-001" || service.query.Limit != 10 || service.query.Offset != 5 {
		t.Fatalf("service request = %#v", service)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	data := body["data"].([]any)
	item := data[0].(map[string]any)
	if item["file_count"] != float64(2) || item["history_id"] != "HIST-001" || item["added_at"] != "2026-09-30T10:00:00Z" {
		t.Fatalf("document response item = %#v", item)
	}
	for _, hiddenField := range []string{"file_name", "folder", "download_url", "path"} {
		if _, exists := item[hiddenField]; exists {
			t.Errorf("response unexpectedly contains %q", hiddenField)
		}
	}
}

func TestControllerRequiresAuthenticationAndValidPagination(t *testing.T) {
	service := &stubLister{}
	app := testApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.StatusCode)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents?limit=1000", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.calls != 0 {
		t.Fatalf("invalid pagination status/calls = %d/%d", response.StatusCode, service.calls)
	}
}

func TestControllerMapsOutOfScopeTicketToNotFound(t *testing.T) {
	service := &stubLister{err: documents.ErrNotFoundOrNotVisible}
	app := testApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want not found", response.StatusCode)
	}
}

func TestControllerMapsUnexpectedServiceFailureWithoutDetails(t *testing.T) {
	service := &stubLister{err: errors.New("database detail")}
	app := testApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want service unavailable", response.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["message"] == "database detail" {
		t.Fatal("internal error detail was exposed")
	}
}

func testApp(identity authentication.VerifiedIdentity, service *stubLister) *fiber.App {
	registrar := httpapi.AuthenticatedRouteModule(stubVerifier{identity: identity}, New(service))
	return httpapi.NewServer(nil, registrar)
}
