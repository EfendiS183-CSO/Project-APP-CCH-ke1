package controller

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/health"
)

type Controller struct {
	serviceName string
	database    health.ReadinessChecker
}

type response struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

type readinessResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	MySQL   string `json:"mysql"`
}

func NewController(serviceName string, database health.ReadinessChecker) *Controller {
	return &Controller{serviceName: serviceName, database: database}
}

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/healthz", controller.serveHealth)
	router.Get("/readyz", controller.serveReadiness)
}

func (controller *Controller) serveHealth(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(response{
		Service: controller.serviceName,
		Status:  "ok",
	})
}

func (controller *Controller) serveReadiness(ctx fiber.Ctx) error {
	if controller.database == nil {
		return ctx.Status(fiber.StatusServiceUnavailable).JSON(readinessResponse{
			Service: controller.serviceName,
			Status:  "not_ready",
			MySQL:   "not_configured",
		})
	}

	checkContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := controller.database.PingContext(checkContext); err != nil {
		return ctx.Status(fiber.StatusServiceUnavailable).JSON(readinessResponse{
			Service: controller.serviceName,
			Status:  "not_ready",
			MySQL:   "unavailable",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(readinessResponse{
		Service: controller.serviceName,
		Status:  "ready",
		MySQL:   "connected",
	})
}
