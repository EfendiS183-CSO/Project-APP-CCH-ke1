package authorization

import (
	"context"
	"errors"

	"cch.local/backend/internal/platform/authentication"
)

var (
	ErrAccessServiceNotConfigured     = errors.New("authorization access loader is not configured")
	ErrPermissionDenied               = errors.New("table action permission denied")
	ErrInvalidAction                  = errors.New("invalid authorization action")
	ErrAccountNotFound                = errors.New("CCH account not found")
	ErrAccountInactive                = errors.New("CCH account is inactive")
	ErrAccountLocked                  = errors.New("CCH account is locked")
	ErrIdentityMismatch               = errors.New("token identity does not match current CCH account")
	ErrSystemAdministratorUnsupported = errors.New("system administrator access is not configured")
	ErrInvalidUserLevels              = errors.New("CCH account has invalid user levels")
	ErrInvalidTableVariable           = errors.New("invalid CCH table variable")
)

type AccountAccess struct {
	Username       string
	OfficeCode     string
	ReportTo       string
	UserLevelIDs   []int64
	PermissionMask int64
}

type AccountAccessLoader interface {
	Load(context.Context, authentication.VerifiedIdentity, string) (AccountAccess, error)
}

type AccessService struct {
	accessLoader AccountAccessLoader
}

func NewAccessService(accessLoader AccountAccessLoader) *AccessService {
	return &AccessService{accessLoader: accessLoader}
}

func (service *AccessService) Authorize(ctx context.Context, identity authentication.VerifiedIdentity, tableVariable string, action Action) (AccountAccess, error) {
	if service == nil || service.accessLoader == nil {
		return AccountAccess{}, ErrAccessServiceNotConfigured
	}
	if action <= 0 || int64(action)&^AllPermissions != 0 {
		return AccountAccess{}, ErrInvalidAction
	}

	access, err := service.accessLoader.Load(ctx, identity, tableVariable)
	if err != nil {
		return AccountAccess{}, err
	}
	if access.PermissionMask < 0 || access.PermissionMask&^AllPermissions != 0 || !Allows(access.PermissionMask, action) {
		return AccountAccess{}, ErrPermissionDenied
	}
	return access, nil
}
