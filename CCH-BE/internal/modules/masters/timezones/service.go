package timezones

import (
	"context"
	"database/sql"
	"errors"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type Service struct {
	authorization *authorization.AccessService
	repository    Repository
}

func NewService(accessService *authorization.AccessService, repository Repository) *Service {
	return &Service{authorization: accessService, repository: repository}
}

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity) ([]Timezone, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return nil, err
	}
	return service.repository.List(ctx)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, id int64) (Timezone, error) {
	if err := service.validateDependencies(); err != nil {
		return Timezone{}, err
	}
	if err := validateID(id); err != nil {
		return Timezone{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Timezone{}, err
	}
	value, err := service.repository.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Timezone{}, ErrNotFound
	}
	return value, err
}

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (Timezone, error) {
	if err := service.validateDependencies(); err != nil {
		return Timezone{}, err
	}
	value, err := normalizeValue(command.Timezone)
	if err != nil {
		return Timezone{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return Timezone{}, err
	}
	created := Timezone{Timezone: value, IsDefault: command.IsDefault}
	if err := service.repository.Insert(ctx, created); err != nil {
		return Timezone{}, err
	}
	return created, nil
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (Timezone, error) {
	if err := service.validateDependencies(); err != nil {
		return Timezone{}, err
	}
	if err := validateID(command.ID); err != nil {
		return Timezone{}, err
	}
	value, err := normalizeValue(command.Timezone)
	if err != nil {
		return Timezone{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return Timezone{}, err
	}
	updated := Timezone{ID: command.ID, Timezone: value, IsDefault: command.IsDefault}
	if err := service.repository.Update(ctx, updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Timezone{}, ErrNotFound
		}
		return Timezone{}, err
	}
	return updated, nil
}

func (service *Service) Delete(ctx context.Context, identity authentication.VerifiedIdentity, id int64) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	if err := validateID(id); err != nil {
		return err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionDelete); err != nil {
		return err
	}
	return service.repository.Delete(ctx, id)
}

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil {
		return ErrServiceNotReady
	}
	return nil
}
