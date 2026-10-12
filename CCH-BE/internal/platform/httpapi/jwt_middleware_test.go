package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/platform/authentication"
)

func TestJWTAuthenticationRequiresHeaderToken(t *testing.T) {
	app := fiber.New()
	app.Use(JWTAuthentication(fakeTokenVerifier{}))
	app.Get("/private", func(ctx fiber.Ctx) error { return ctx.SendStatus(http.StatusOK) })

	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/private", nil),
		httptest.NewRequest(http.MethodGet, "/private?jwt=valid-token", nil),
	}
	for _, request := range requests {
		response, err := app.Test(request)
		if err != nil {
			t.Fatalf("request private route: %v", err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
		}
	}
}

func TestJWTAuthenticationPassesVerifiedIdentity(t *testing.T) {
	app := fiber.New()
	app.Use(JWTAuthentication(fakeTokenVerifier{}))
	app.Get("/private", func(ctx fiber.Ctx) error {
		identity, ok := VerifiedIdentityFromContext(ctx)
		if !ok {
			return fiber.ErrInternalServerError
		}
		return ctx.JSON(map[string]string{
			"username": identity.Username,
			"office":   identity.OfficeCode,
		})
	})

	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.Header.Set(legacyJWTHeader, "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request private route: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
}

func TestJWTAuthenticationStoresOnlyTheVerifiedBearerToken(t *testing.T) {
	var capturedToken string
	var tokenAvailable bool
	app := fiber.New()
	app.Use(JWTAuthentication(fakeTokenVerifier{}))
	app.Get("/private", func(ctx fiber.Ctx) error {
		capturedToken, tokenAvailable = VerifiedBearerTokenFromContext(ctx)
		return ctx.SendStatus(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.Header.Set(legacyJWTHeader, "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request private route: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !tokenAvailable || capturedToken != "valid-token" {
		t.Fatalf("status=%d tokenAvailable=%v token=%q", response.StatusCode, tokenAvailable, capturedToken)
	}

	capturedToken = ""
	tokenAvailable = false
	request = httptest.NewRequest(http.MethodGet, "/private", nil)
	request.Header.Set(legacyJWTHeader, "invalid-token")
	response, err = app.Test(request)
	if err != nil {
		t.Fatalf("request with invalid token: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || tokenAvailable || capturedToken != "" {
		t.Fatalf("invalid token reached handler: status=%d tokenAvailable=%v token=%q", response.StatusCode, tokenAvailable, capturedToken)
	}
}

func TestJWTAuthenticationRejectsMalformedAuthorizationHeader(t *testing.T) {
	app := fiber.New()
	app.Use(JWTAuthentication(fakeTokenVerifier{}))
	app.Get("/private", func(ctx fiber.Ctx) error { return ctx.SendStatus(http.StatusOK) })

	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.Header.Set(legacyJWTHeader, "Bearer token extra")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request private route: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}
}

func TestJWTAuthenticationFailsClosedWhenVerifierIsMissing(t *testing.T) {
	app := fiber.New()
	app.Use(JWTAuthentication(nil))
	app.Get("/private", func(ctx fiber.Ctx) error { return ctx.SendStatus(http.StatusOK) })
	request := httptest.NewRequest(http.MethodGet, "/private", nil)
	request.Header.Set(legacyJWTHeader, "valid-token")

	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request private route: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusServiceUnavailable)
	}
}

type fakeTokenVerifier struct{}

func (fakeTokenVerifier) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "40005CC"}, nil
}
