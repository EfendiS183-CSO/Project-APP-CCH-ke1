package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/documents"
	"cch.local/backend/internal/modules/documents/storage"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type uploadServiceStub struct {
	identity authentication.VerifiedIdentity
	request  documents.UploadRequest
	result   documents.UploadResult
	err      error
	calls    int
}

func (service *uploadServiceStub) Upload(_ context.Context, identity authentication.VerifiedIdentity, request documents.UploadRequest) (documents.UploadResult, error) {
	service.calls++
	service.identity = identity
	service.request = request
	return service.result, service.err
}

func TestUploadControllerMapsRequiredHeadersAndReturnsNoStorageKey(t *testing.T) {
	for _, test := range []struct {
		name       string
		result     documents.UploadResult
		wantStatus int
	}{
		{
			name:       "created",
			result:     documents.UploadResult{DocumentID: "document-001", ComplaintID: "TKT-001", HistoryID: "HIST-001", DocumentType: "BA1", UploadOffice: "40005CC", MediaType: "application/pdf", FileSize: 16, CreatedAt: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "replay",
			result:     documents.UploadResult{DocumentID: "document-001", ComplaintID: "TKT-001", HistoryID: "HIST-001", DocumentType: "BA1", UploadOffice: "40005CC", MediaType: "application/pdf", FileSize: 16, CreatedAt: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC), Replayed: true},
			wantStatus: http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			identity := authentication.VerifiedIdentity{Username: "agent.test"}
			service := &uploadServiceStub{result: test.result}
			app := uploadTestApp(identity, service)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/documents", strings.NewReader("%PDF-1.7\ncontent"))
			request.Header.Set("X-Authorization", "Bearer valid-token")
			request.Header.Set("Content-Type", "application/pdf")
			request.Header.Set("X-History-ID", "HIST-001")
			request.Header.Set("X-Document-Type", "BA1")
			request.Header.Set("X-File-Name", "report.pdf")
			request.Header.Set("Idempotency-Key", "upload-operation-key-0001")
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus || response.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("status/cache = %d/%q", response.StatusCode, response.Header.Get("Cache-Control"))
			}
			if service.calls != 1 || service.identity != identity || service.request.ComplaintID != "TKT-001" || service.request.HistoryID != "HIST-001" || service.request.DocumentType != "BA1" || service.request.FileName != "report.pdf" || service.request.ContentType != "application/pdf" || service.request.IdempotencyKey != "upload-operation-key-0001" {
				t.Fatalf("service upload request = %#v", service.request)
			}
			var body map[string]any
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			for _, hiddenField := range []string{"storage_key", "folder", "path", "file_name"} {
				if _, exists := body[hiddenField]; exists {
					t.Errorf("upload response unexpectedly contains %q", hiddenField)
				}
			}
		})
	}
}

func TestUploadControllerRequiresAuthAndEnforcesBodyLimit(t *testing.T) {
	service := &uploadServiceStub{}
	app := uploadTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/documents", strings.NewReader("%PDF-1.7"))
	request.Header.Set("Content-Type", "application/pdf")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.StatusCode)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/documents", strings.NewReader(strings.Repeat("x", maxUploadBytes+1)))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/pdf")
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge || service.calls != 0 {
		t.Fatalf("oversize status/calls = %d/%d", response.StatusCode, service.calls)
	}
}

func TestUploadControllerMapsMalwareScopeAndIdempotencyErrors(t *testing.T) {
	for _, test := range []struct {
		serviceError error
		wantStatus   int
	}{
		{serviceError: storage.ErrMalwareDetected, wantStatus: http.StatusUnprocessableEntity},
		{serviceError: documents.ErrUploadNotAvailable, wantStatus: http.StatusNotFound},
		{serviceError: documents.ErrUploadIdempotencyConflict, wantStatus: http.StatusConflict},
		{serviceError: storage.ErrScannerUnavailable, wantStatus: http.StatusServiceUnavailable},
	} {
		service := &uploadServiceStub{err: test.serviceError}
		app := uploadTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/documents", strings.NewReader("%PDF-1.7\ncontent"))
		request.Header.Set("X-Authorization", "Bearer valid-token")
		request.Header.Set("Content-Type", "application/pdf")
		request.Header.Set("X-History-ID", "HIST-001")
		request.Header.Set("X-Document-Type", "BA1")
		request.Header.Set("X-File-Name", "report.pdf")
		request.Header.Set("Idempotency-Key", "upload-operation-key-0001")
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.wantStatus {
			t.Errorf("error %v status = %d, want %d", test.serviceError, response.StatusCode, test.wantStatus)
		}
	}
}

func uploadTestApp(identity authentication.VerifiedIdentity, service *uploadServiceStub) *fiber.App {
	registrar := httpapi.AuthenticatedRouteModule(stubVerifier{identity: identity}, NewUploadController(service))
	return httpapi.NewServer(nil, registrar)
}
