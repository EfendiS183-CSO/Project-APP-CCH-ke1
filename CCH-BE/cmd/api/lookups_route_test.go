package main

import (
	"log"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/platform/config"
	"cch.local/backend/internal/platform/httpapi"
)

// Regression check: when the Call Center read flag is enabled, the lookups
// route must be mounted (and protected by auth), not 404.
func TestLookupsRouteMountedWhenEnabled(t *testing.T) {
	enabled := config.Settings{
		CallCenterComplaintReadAPIEnabled: true,
		JWT:                               config.JWTSettings{Secret: "0123456789abcdef0123456789abcdef", Issuer: "cch"},
	}
	database, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	registrar, err := callCenterComplaintRoutes(enabled, database)
	if err != nil {
		t.Fatal(err)
	}
	server := httpapi.NewServer(nil, registrar)

	req := httptest.NewRequest("GET", "/api/v1/complaints/call-center/lookups", nil)
	response, err := server.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode == 404 {
		t.Fatalf("lookups route returned 404 when enabled; route is not mounted")
	}
	if response.StatusCode == 200 || response.StatusCode == 401 {
		t.Logf("lookups route mounted correctly (status=%d)", response.StatusCode)
	} else {
		t.Fatalf("lookups status = %d, want 401/200/other non-404", response.StatusCode)
	}
}

// Print the exact registered order of call-center routes to see whether the
// static /lookups path exists in the tree and where it sits relative to :id.
func TestLookupsRouteRegisteredInTree(t *testing.T) {
	enabled := config.Settings{
		CallCenterComplaintReadAPIEnabled: true,
		JWT:                               config.JWTSettings{Secret: "0123456789abcdef0123456789abcdef", Issuer: "cch"},
	}
	database, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	registrar, err := callCenterComplaintRoutes(enabled, database)
	if err != nil {
		t.Fatal(err)
	}
	server := httpapi.NewServer(nil, registrar)

	found := false
	for _, route := range server.GetRoutes() {
		if strings.Contains(route.Path, "/complaints/call-center") {
			log.Printf("route: %-6s %s", route.Method, route.Path)
			// GetRoutes() returns fully-prefixed paths, e.g.
			// /api/v1/complaints/call-center/lookups.
			if strings.HasSuffix(route.Path, "/complaints/call-center/lookups") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("lookups route is NOT present in the registered tree")
	}
}

// Regression guard for the Fiber v3 routing rule that caused the original 404:
// a static path registered AFTER a ":id" pattern is shadowed by the parameter
// route, so /complaints/call-center/lookups must be registered BEFORE
// /complaints/call-center/:id.
func TestFiberStaticVsParamOrdering(t *testing.T) {
	matches := func(staticFirst bool) int {
		app := fiber.New()
		api := app.Group("/api/v1")
		group := api.Group("", func(ctx fiber.Ctx) error { return ctx.Next() })

		static := func(ctx fiber.Ctx) error {
			return ctx.JSON(fiber.Map{"ok": true})
		}
		param := func(ctx fiber.Ctx) error {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"code": "not_found"})
		}

		if staticFirst {
			group.Get("/things/lookups", static)
			group.Get("/things/:id", param)
		} else {
			group.Get("/things/:id", param)
			group.Get("/things/lookups", static)
		}

		response, err := app.Test(httptest.NewRequest("GET", "/api/v1/things/lookups", nil))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}

	if status := matches(true); status != fiber.StatusOK {
		t.Fatalf("static route registered BEFORE :id status = %d, want 200", status)
	}
	if status := matches(false); status != fiber.StatusNotFound {
		t.Fatalf("static route registered AFTER :id status = %d, want 404 (documents why ordering matters)", status)
	}
}
