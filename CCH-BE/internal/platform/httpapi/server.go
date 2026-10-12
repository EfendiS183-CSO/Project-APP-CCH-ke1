package httpapi

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/health"
	healthcontroller "cch.local/backend/internal/modules/health/controller"
)

const serviceName = "cch-be"

type VersionedRouteRegistrar func(fiber.Router)

func NewServer(database health.ReadinessChecker, routeRegistrars ...VersionedRouteRegistrar) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      serviceName,
		Immutable:    true,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		ErrorHandler: handleError,
	})
	app.Use(requestIDMiddleware)

	healthcontroller.NewController(serviceName, database).Register(app)
	api := app.Group("/api/v1")
	api.Get("/", func(ctx fiber.Ctx) error {
		return ctx.JSON(map[string]string{
			"name":    serviceName,
			"version": "v1",
		})
	})
	for _, registerRoutes := range routeRegistrars {
		if registerRoutes != nil {
			registerRoutes(api)
		}
	}

	return app
}
