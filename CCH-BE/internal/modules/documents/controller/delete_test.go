package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/documents"
	"cch.local/backend/internal/modules/documents/storage"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type stubDeleter struct {
	request documents.DeleteRequest
	result  documents.DeleteResult
	err     error
	called  bool
}

func (service *stubDeleter) Delete(_ context.Context, _ authentication.VerifiedIdentity, request documents.DeleteRequest) (documents.DeleteResult, error) {
	service.called = true
	service.request = request
	return service.result, service.err
}

func deleteTestApp(identity authentication.VerifiedIdentity, service *stubDeleter) *fiber.App {
	registrar := httpapi.AuthenticatedRouteModule(stubVerifier{identity: identity}, NewDeleteController(service))
	return httpapi.NewServer(nil, registrar)
}

func TestControllerDeletesOfficeOwnedDocumentAsNoContent(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test"}
	documentID := "0123456789abcdef0123456789abcdef"
	service := &stubDeleter{result: documents.DeleteResult{ComplaintID: "TKT-001", DocumentID: documentID}}
	app := deleteTestApp(identity, service)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/complaints/call-center/TKT-001/documents/"+documentID, nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent || !service.called || service.request.ComplaintID != "TKT-001" || service.request.DocumentID != documentID {
		t.Fatalf("delete response/service unexpected: status=%d called=%t request=%#v", response.StatusCode, service.called, service.request)
	}
	if response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("Cache-Control = %q, want private, no-store", response.Header.Get("Cache-Control"))
	}
}

func TestControllerMapsDeleteErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid request", err: documents.ErrInvalidQuery, want: http.StatusBadRequest},
		{name: "missing or not visible", err: documents.ErrNotFoundOrNotVisible, want: http.StatusNotFound},
		{name: "foreign office document", err: documents.ErrDeleteUnavailable, want: http.StatusConflict},
		{name: "unavailable storage", err: storage.ErrStorageUnavailable, want: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &stubDeleter{err: test.err}
			app := deleteTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
			request := httptest.NewRequest(http.MethodDelete, "/api/v1/complaints/call-center/TKT-001/documents/0123456789abcdef0123456789abcdef", nil)
			request.Header.Set("X-Authorization", "Bearer valid-token")
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != test.want {
				t.Fatalf("status = %d, want %d for %s", response.StatusCode, test.want, test.name)
			}
		})
	}
}
