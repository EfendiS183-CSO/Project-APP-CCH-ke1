package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/intake/session/controller"
	"cch.local/backend/internal/modules/intake/submission"
)

const (
	maxRequestBodyBytes = 64 * 1024
	intakeSessionHeader = "X-Intake-Session"
)

var (
	ErrServiceNotConfigured = errors.New("intake submission controller is not configured")
	ErrInvalidOriginPolicy  = errors.New("intake submission origin policy is invalid")
)

type Submitter interface {
	Submit(context.Context, string, submission.Request) (submission.Result, error)
}

type Controller struct {
	service        Submitter
	allowedOrigins map[string]struct{}
	rateLimiter    *controller.RateLimiter
}

type submitResponse struct {
	Status   string `json:"status"`
	TicketID string `json:"ticket_id,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

func New(service Submitter, allowedOrigins []string, limiter *controller.RateLimiter) (*Controller, error) {
	if service == nil {
		return nil, ErrServiceNotConfigured
	}
	if limiter == nil {
		return nil, controller.ErrInvalidRateLimit
	}
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			return nil, ErrInvalidOriginPolicy
		}
		originSet[normalized] = struct{}{}
	}
	if len(originSet) == 0 {
		return nil, ErrInvalidOriginPolicy
	}
	return &Controller{service: service, allowedOrigins: originSet, rateLimiter: limiter}, nil
}

func (controller *Controller) Register(router fiber.Router) {
	router.Options("/intake/submissions", controller.Preflight)
	router.Post("/intake/submissions", controller.Submit)
}

func (controller *Controller) Preflight(ctx fiber.Ctx) error {
	if controller == nil {
		return fiber.ErrServiceUnavailable
	}
	if !controller.setAllowedOrigin(ctx) {
		return fiber.ErrForbidden
	}
	ctx.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	ctx.Set("Access-Control-Allow-Headers", "Content-Type, "+intakeSessionHeader)
	ctx.Set("Access-Control-Max-Age", "600")
	return ctx.SendStatus(fiber.StatusNoContent)
}

func (controller *Controller) Submit(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil || controller.rateLimiter == nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "no-store")
	ctx.Set("Pragma", "no-cache")
	if !controller.setAllowedOrigin(ctx) {
		return fiber.ErrForbidden
	}
	if !controller.rateLimiter.Allow(ctx.IP()) {
		return fiber.ErrTooManyRequests
	}
	if rawLength := ctx.Get("Content-Length"); rawLength != "" {
		contentLength, err := strconv.ParseInt(rawLength, 10, 64)
		if err != nil || contentLength < 0 || contentLength > maxRequestBodyBytes {
			return fiber.ErrRequestEntityTooLarge
		}
	}
	body := ctx.Body()
	if len(body) == 0 {
		return fiber.ErrBadRequest
	}
	if len(body) > maxRequestBodyBytes {
		return fiber.ErrRequestEntityTooLarge
	}
	mediaType, _, err := mime.ParseMediaType(ctx.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fiber.ErrUnsupportedMediaType
	}
	var request submission.Request
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return fiber.ErrBadRequest
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fiber.ErrBadRequest
	}
	conversationToken := strings.TrimSpace(ctx.Get(intakeSessionHeader))
	if conversationToken == "" {
		return fiber.ErrUnauthorized
	}
	result, err := controller.service.Submit(ctx.Context(), conversationToken, request)
	if err != nil {
		return serviceError(err)
	}
	response := submitResponse{Status: result.Status}
	switch result.Status {
	case "created":
		response.TicketID = result.TicketID
		return ctx.Status(fiber.StatusCreated).JSON(response)
	case "pending_cs", "creating":
		response.Reason = result.Reason
		return ctx.Status(fiber.StatusAccepted).JSON(response)
	default:
		return fiber.ErrServiceUnavailable
	}
}

func (controller *Controller) setAllowedOrigin(ctx fiber.Ctx) bool {
	origin := ctx.Get("Origin")
	if _, allowed := controller.allowedOrigins[origin]; !allowed {
		return false
	}
	ctx.Set("Access-Control-Allow-Origin", origin)
	ctx.Set("Vary", "Origin")
	return true
}

func serviceError(err error) error {
	switch {
	case errors.Is(err, submission.ErrInvalidSession):
		return fiber.ErrUnauthorized
	case errors.Is(err, submission.ErrConsentRequired), errors.Is(err, submission.ErrInvalidSubmission):
		return fiber.ErrBadRequest
	case errors.Is(err, submission.ErrDraftState):
		return fiber.NewError(fiber.StatusConflict, "Submission state conflict")
	case errors.Is(err, submission.ErrSubmissionProcessing):
		return fiber.NewError(fiber.StatusServiceUnavailable, "Submission is being reconciled")
	default:
		return fiber.ErrServiceUnavailable
	}
}

func normalizeOrigin(origin string) (string, error) {
	if origin == "" || origin != strings.TrimSpace(origin) {
		return "", ErrInvalidOriginPolicy
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed == nil || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", ErrInvalidOriginPolicy
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return "", ErrInvalidOriginPolicy
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func isLoopbackHost(host string) bool {
	return strings.EqualFold(host, "localhost") || strings.EqualFold(host, "127.0.0.1") || strings.EqualFold(host, "[::1]")
}
