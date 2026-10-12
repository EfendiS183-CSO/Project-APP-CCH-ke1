package httpapi

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/platform/authentication"
)

const legacyJWTHeader = "X-Authorization"
const verifiedIdentityLocal = "verifiedIdentity"
const verifiedTokenLocal = "verifiedToken"

func JWTAuthentication(verifier authentication.TokenVerifier) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		if verifier == nil {
			return fiber.ErrServiceUnavailable
		}

		token := extractHeaderToken(ctx.Get(legacyJWTHeader))
		if token == "" {
			return fiber.ErrUnauthorized
		}

		identity, err := verifier.Verify(token)
		if err != nil {
			return fiber.ErrUnauthorized
		}
		ctx.Locals(verifiedIdentityLocal, identity)
		ctx.Locals(verifiedTokenLocal, token)
		return ctx.Next()
	}
}

func VerifiedIdentityFromContext(ctx fiber.Ctx) (authentication.VerifiedIdentity, bool) {
	identity, ok := ctx.Locals(verifiedIdentityLocal).(authentication.VerifiedIdentity)
	return identity, ok
}

func VerifiedBearerTokenFromContext(ctx fiber.Ctx) (string, bool) {
	token, ok := ctx.Locals(verifiedTokenLocal).(string)
	return token, ok && token != ""
}

func extractHeaderToken(header string) string {
	parts := strings.Fields(header)
	switch len(parts) {
	case 1:
		return parts[0]
	case 2:
		if strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}
	return ""
}
