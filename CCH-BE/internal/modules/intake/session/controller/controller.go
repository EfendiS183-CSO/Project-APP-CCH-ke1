package controller

import (
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/intake/session"
)

const (
	defaultRequestsPerWindow = 5
	defaultRateLimitWindow   = time.Minute
	maxTrackedClientIPs      = 4096
)

var (
	ErrIssuerNotConfigured = errors.New("intake session issuer is not configured")
	ErrInvalidOrigin       = errors.New("invalid intake session origin allowlist")
	ErrInvalidRateLimit    = errors.New("invalid intake session rate limit")
)

type Issuer interface {
	Issue() (session.IssuedConversation, error)
}

type Controller struct {
	issuer         Issuer
	allowedOrigins map[string]struct{}
	rateLimiter    *RateLimiter
}

type issuedResponse struct {
	ConversationID string    `json:"conversation_id"`
	Token          string    `json:"token"`
	ExpiresAt      time.Time `json:"expires_at"`
}

func New(issuer Issuer, allowedOrigins []string, limiter *RateLimiter) (*Controller, error) {
	if issuer == nil {
		return nil, ErrIssuerNotConfigured
	}
	if limiter == nil {
		return nil, ErrInvalidRateLimit
	}
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			return nil, ErrInvalidOrigin
		}
		originSet[normalized] = struct{}{}
	}
	if len(originSet) == 0 {
		return nil, ErrInvalidOrigin
	}
	return &Controller{issuer: issuer, allowedOrigins: originSet, rateLimiter: limiter}, nil
}

func (controller *Controller) Register(router fiber.Router) {
	router.Options("/intake/sessions", controller.Preflight)
	router.Post("/intake/sessions", controller.Issue)
}

func (controller *Controller) Preflight(ctx fiber.Ctx) error {
	if controller == nil {
		return fiber.ErrServiceUnavailable
	}
	if !controller.setAllowedOrigin(ctx) {
		return fiber.ErrForbidden
	}
	ctx.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	ctx.Set("Access-Control-Allow-Headers", "Content-Type")
	ctx.Set("Access-Control-Max-Age", "600")
	return ctx.SendStatus(fiber.StatusNoContent)
}

func (controller *Controller) Issue(ctx fiber.Ctx) error {
	if controller == nil || controller.issuer == nil || controller.rateLimiter == nil {
		return fiber.ErrServiceUnavailable
	}
	if !controller.setAllowedOrigin(ctx) {
		return fiber.ErrForbidden
	}
	if !controller.rateLimiter.Allow(ctx.IP()) {
		return fiber.ErrTooManyRequests
	}
	issued, err := controller.issuer.Issue()
	if err != nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "no-store")
	ctx.Set("Pragma", "no-cache")
	return ctx.Status(fiber.StatusCreated).JSON(issuedResponse{
		ConversationID: issued.ConversationID,
		Token:          issued.Token,
		ExpiresAt:      issued.ExpiresAt.UTC(),
	})
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

type rateWindow struct {
	startedAt time.Time
	count     int
}

type RateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	clients     map[string]rateWindow
	currentTime func() time.Time
}

func NewRateLimiter(limit int, window time.Duration) (*RateLimiter, error) {
	if limit < 1 || window <= 0 {
		return nil, ErrInvalidRateLimit
	}
	return &RateLimiter{
		limit:       limit,
		window:      window,
		clients:     make(map[string]rateWindow),
		currentTime: time.Now,
	}, nil
}

func (limiter *RateLimiter) Allow(clientIP string) bool {
	if limiter == nil || strings.TrimSpace(clientIP) == "" {
		return false
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := limiter.currentTime()
	window, exists := limiter.clients[clientIP]
	if exists && now.Sub(window.startedAt) < limiter.window {
		if window.count >= limiter.limit {
			return false
		}
		window.count++
		limiter.clients[clientIP] = window
		return true
	}
	if !exists && len(limiter.clients) >= maxTrackedClientIPs {
		limiter.removeExpired(now)
		if len(limiter.clients) >= maxTrackedClientIPs {
			return false
		}
	}
	limiter.clients[clientIP] = rateWindow{startedAt: now, count: 1}
	return true
}

func (limiter *RateLimiter) removeExpired(now time.Time) {
	for clientIP, window := range limiter.clients {
		if now.Sub(window.startedAt) >= limiter.window {
			delete(limiter.clients, clientIP)
		}
	}
}

func normalizeOrigin(origin string) (string, error) {
	if origin == "" || origin != strings.TrimSpace(origin) {
		return "", ErrInvalidOrigin
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed == nil || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", ErrInvalidOrigin
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return "", ErrInvalidOrigin
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func isLoopbackHost(host string) bool {
	return strings.EqualFold(host, "localhost") || strings.EqualFold(host, "127.0.0.1") || strings.EqualFold(host, "[::1]")
}
