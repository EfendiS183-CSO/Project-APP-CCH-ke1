package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestVersionedAPIRoot(t *testing.T) {
	response, err := NewServer(nil).Test(httptest.NewRequest(http.MethodGet, "/api/v1/", nil))
	if err != nil {
		t.Fatalf("request API root: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["name"] != "cch-be" || body["version"] != "v1" {
		t.Fatalf("body = %#v", body)
	}
}

func TestNotFoundUsesJSONErrorEnvelope(t *testing.T) {
	response, err := NewServer(nil).Test(httptest.NewRequest(http.MethodGet, "/missing", nil))
	if err != nil {
		t.Fatalf("request missing route: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	var body errorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "not_found" || body.RequestID == "" {
		t.Fatalf("body = %#v", body)
	}
	if got := response.Header.Get(requestIDHeader); got != body.RequestID {
		t.Fatalf("request ID header = %q, body = %q", got, body.RequestID)
	}
}

func TestInternalErrorsDoNotExposeDetails(t *testing.T) {
	app := NewServer(nil)
	app.Get("/failure", func(fiber.Ctx) error {
		return fiber.NewError(http.StatusInternalServerError, "database password leaked")
	})
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/failure", nil))
	if err != nil {
		t.Fatalf("request failing route: %v", err)
	}
	defer response.Body.Close()

	var body errorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Message != "Internal server error" {
		t.Fatalf("error message exposed internal detail: %q", body.Error.Message)
	}
}
