package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type stubAccountAccessLoader struct {
	access authorization.AccountAccess
	err    error
}

func (loader stubAccountAccessLoader) Load(context.Context, authentication.VerifiedIdentity, string) (authorization.AccountAccess, error) {
	return loader.access, loader.err
}

func TestRequireTableActionAllowsAuthorizedIdentityAndAddsTrustedAccess(t *testing.T) {
	service := authorization.NewAccessService(stubAccountAccessLoader{
		access: authorization.AccountAccess{
			Username:       "agent.test",
			OfficeCode:     "40005CC",
			ReportTo:       "40005",
			PermissionMask: int64(authorization.ActionList),
		},
	})
	app := fiber.New()
	app.Use(func(ctx fiber.Ctx) error {
		ctx.Locals(verifiedIdentityLocal, authentication.VerifiedIdentity{Username: "agent.test"})
		return ctx.Next()
	})
	app.Get("/complaints", RequireTableAction(service, "cchentri", authorization.ActionList), func(ctx fiber.Ctx) error {
		access, ok := AuthorizedAccountAccessFromContext(ctx)
		if !ok {
			return fiber.ErrInternalServerError
		}
		return ctx.JSON(map[string]string{"office": access.OfficeCode})
	})

	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/complaints", nil))
	if err != nil {
		t.Fatalf("request authorized route: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
}

func TestRequireTableActionRejectsMissingIdentityAndService(t *testing.T) {
	tests := []struct {
		name       string
		service    *authorization.AccessService
		identity   bool
		wantStatus int
	}{
		{name: "missing identity", service: authorization.NewAccessService(stubAccountAccessLoader{}), wantStatus: http.StatusUnauthorized},
		{name: "missing service", identity: true, wantStatus: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := fiber.New()
			if test.identity {
				app.Use(func(ctx fiber.Ctx) error {
					ctx.Locals(verifiedIdentityLocal, authentication.VerifiedIdentity{Username: "agent.test"})
					return ctx.Next()
				})
			}
			app.Get("/private", RequireTableAction(test.service, "cchentri", authorization.ActionList), func(ctx fiber.Ctx) error {
				return ctx.SendStatus(http.StatusOK)
			})
			response, err := app.Test(httptest.NewRequest(http.MethodGet, "/private", nil))
			if err != nil {
				t.Fatalf("request private route: %v", err)
			}
			response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
		})
	}
}

func TestRequireTableActionMapsAuthorizationFailuresWithoutDetails(t *testing.T) {
	tests := []struct {
		name       string
		loaderErr  error
		mask       int64
		wantStatus int
	}{
		{name: "stale account", loaderErr: authorization.ErrIdentityMismatch, wantStatus: http.StatusUnauthorized},
		{name: "permission denied", mask: int64(authorization.ActionView), wantStatus: http.StatusForbidden},
		{name: "database unavailable", loaderErr: errors.New("database password detail"), wantStatus: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := authorization.NewAccessService(stubAccountAccessLoader{
				access: authorization.AccountAccess{PermissionMask: test.mask},
				err:    test.loaderErr,
			})
			app := fiber.New()
			app.Use(func(ctx fiber.Ctx) error {
				ctx.Locals(verifiedIdentityLocal, authentication.VerifiedIdentity{Username: "agent.test"})
				return ctx.Next()
			})
			app.Get("/private", RequireTableAction(service, "cchentri", authorization.ActionList), func(ctx fiber.Ctx) error {
				return ctx.SendStatus(http.StatusOK)
			})

			response, err := app.Test(httptest.NewRequest(http.MethodGet, "/private", nil))
			if err != nil {
				t.Fatalf("request private route: %v", err)
			}
			response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
		})
	}
}
