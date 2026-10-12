package httpapi

import (
	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/platform/authentication"
)

type VersionedRouteModule interface {
	Register(fiber.Router)
}

type VersionedRouteModuleGroup []VersionedRouteModule

func (group VersionedRouteModuleGroup) Register(router fiber.Router) {
	for _, module := range group {
		if module != nil {
			module.Register(router)
		}
	}
}

func AuthenticatedRouteModule(verifier authentication.TokenVerifier, module VersionedRouteModule) VersionedRouteRegistrar {
	return func(api fiber.Router) {
		if module == nil {
			return
		}
		authenticated := api.Group("", JWTAuthentication(verifier))
		module.Register(authenticated)
	}
}
