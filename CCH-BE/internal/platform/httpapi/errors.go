package httpapi

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error     errorDetail `json:"error"`
	RequestID string      `json:"request_id"`
}

func handleError(ctx fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	message := "Internal server error"
	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		status = fiberError.Code
		if status < fiber.StatusInternalServerError {
			message = fiberError.Message
		}
	}

	requestID, _ := ctx.Locals(requestIDLocal).(string)
	return ctx.Status(status).JSON(errorResponse{
		Error: errorDetail{
			Code:    errorCode(status),
			Message: message,
		},
		RequestID: requestID,
	})
}

func errorCode(status int) string {
	switch status {
	case fiber.StatusBadRequest:
		return "bad_request"
	case fiber.StatusUnauthorized:
		return "unauthorized"
	case fiber.StatusForbidden:
		return "forbidden"
	case fiber.StatusNotFound:
		return "not_found"
	case fiber.StatusMethodNotAllowed:
		return "method_not_allowed"
	case fiber.StatusTooManyRequests:
		return "rate_limited"
	case fiber.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		if status >= fiber.StatusInternalServerError {
			return "internal_error"
		}
		return "http_error"
	}
}
