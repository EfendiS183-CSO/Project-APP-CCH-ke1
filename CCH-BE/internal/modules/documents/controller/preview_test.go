package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/documents"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type stubPreviewer struct {
	result documents.DownloadResult
	err    error
	called bool
}

func (service *stubPreviewer) Download(_ context.Context, _ authentication.VerifiedIdentity, _ documents.DownloadRequest) (documents.DownloadResult, error) {
	service.called = true
	return service.result, service.err
}

func previewTestApp(identity authentication.VerifiedIdentity, service *stubPreviewer) *fiber.App {
	registrar := httpapi.AuthenticatedRouteModule(stubVerifier{identity: identity}, NewPreviewController(service))
	return httpapi.NewServer(nil, registrar)
}

func TestControllerServesVerifiedDocumentInline(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test"}
	documentID := "0123456789abcdef0123456789abcdef"
	content := []byte("%PDF-1.7\npreview bytes")
	service := &stubPreviewer{result: documents.DownloadResult{Metadata: documents.DownloadMetadata{
		DocumentID: documentID, MediaType: "application/pdf", Extension: ".pdf", FileSize: int64(len(content)),
	}, Content: content}}
	app := previewTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents/"+documentID+"/preview", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !service.called || string(body) != string(content) {
		t.Fatalf("preview response/service unexpected: status=%d called=%t body=%q", response.StatusCode, service.called, body)
	}
	if disposition := response.Header.Get("Content-Disposition"); !strings.HasPrefix(disposition, "inline;") {
		t.Fatalf("Content-Disposition = %q, want inline", disposition)
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", contentType)
	}
}

func TestControllerMapsPreviewErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid request", err: documents.ErrInvalidQuery, want: http.StatusBadRequest},
		{name: "missing or not visible", err: documents.ErrNotFoundOrNotVisible, want: http.StatusNotFound},
		{name: "unavailable content", err: documents.ErrDownloadUnavailable, want: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &stubPreviewer{err: test.err}
			app := previewTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/documents/0123456789abcdef0123456789abcdef/preview", nil)
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
