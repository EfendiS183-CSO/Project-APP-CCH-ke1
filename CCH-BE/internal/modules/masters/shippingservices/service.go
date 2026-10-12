package shippingservices

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type Service struct {
	authorization *authorization.AccessService
	repository    Repository
	now           func() time.Time
}

func NewService(accessService *authorization.AccessService, repository Repository) *Service {
	return &Service{authorization: accessService, repository: repository, now: time.Now}
}

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity) ([]ShippingService, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return nil, err
	}
	return service.repository.List(ctx)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, code string) (ShippingService, error) {
	if err := service.validateDependencies(); err != nil {
		return ShippingService{}, err
	}
	code, err := normalizeCode(code)
	if err != nil {
		return ShippingService{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return ShippingService{}, err
	}
	value, err := service.repository.Get(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return ShippingService{}, ErrNotFound
	}
	return value, err
}

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (ShippingService, error) {
	if err := service.validateDependencies(); err != nil {
		return ShippingService{}, err
	}
	code, err := normalizeCode(command.Code)
	if err != nil {
		return ShippingService{}, err
	}
	description, err := normalizeDescription(command.Description)
	if err != nil {
		return ShippingService{}, err
	}
	trackingURL, err := normalizeTrackingURL(command.TrackingURL)
	if err != nil {
		return ShippingService{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return ShippingService{}, err
	}
	updatedAt := service.now().UTC()
	value := ShippingService{
		Code: code, Description: description, TrackingURL: trackingURL, Active: command.Active, UpdatedAt: &updatedAt,
	}
	if err := service.repository.Insert(ctx, value); err != nil {
		return ShippingService{}, err
	}
	return value, nil
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (ShippingService, error) {
	if err := service.validateDependencies(); err != nil {
		return ShippingService{}, err
	}
	code, err := normalizeCode(command.Code)
	if err != nil {
		return ShippingService{}, err
	}
	description, err := normalizeDescription(command.Description)
	if err != nil {
		return ShippingService{}, err
	}
	trackingURL, err := normalizeTrackingURL(command.TrackingURL)
	if err != nil {
		return ShippingService{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return ShippingService{}, err
	}
	updatedAt := service.now().UTC()
	value := ShippingService{Code: code, Description: description, TrackingURL: trackingURL, Active: command.Active, UpdatedAt: &updatedAt}
	if err := service.repository.Update(ctx, value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ShippingService{}, ErrNotFound
		}
		return ShippingService{}, err
	}
	return value, nil
}

func (service *Service) Deactivate(ctx context.Context, identity authentication.VerifiedIdentity, code string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	code, err := normalizeCode(code)
	if err != nil {
		return err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionDelete); err != nil {
		return err
	}
	return service.repository.Deactivate(ctx, code, service.now().UTC())
}

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil || service.now == nil {
		return ErrServiceNotReady
	}
	return nil
}
