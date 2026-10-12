package httpapi

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gofiber/fiber/v3"
)

const requestIDHeader = "X-Request-ID"
const requestIDLocal = "requestID"

func requestIDMiddleware(ctx fiber.Ctx) error {
	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		return fiber.ErrInternalServerError
	}
	requestID := hex.EncodeToString(identifier)
	ctx.Locals(requestIDLocal, requestID)
	ctx.Set(requestIDHeader, requestID)
	return ctx.Next()
}
