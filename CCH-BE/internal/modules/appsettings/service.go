package appsettings

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

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity) ([]Settings, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return nil, err
	}
	return service.repository.List(ctx)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, id int64) (Settings, error) {
	if err := service.validateDependencies(); err != nil {
		return Settings{}, err
	}
	if err := validateID(id); err != nil {
		return Settings{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Settings{}, err
	}
	value, err := service.repository.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{}, ErrNotFound
	}
	return value, err
}

func (service *Service) Create(ctx context.Context, identity authentication.VerifiedIdentity, command CreateCommand) (Settings, error) {
	if err := service.validateDependencies(); err != nil {
		return Settings{}, err
	}
	value, err := validateCreate(command)
	if err != nil {
		return Settings{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return Settings{}, err
	}
	if err := service.repository.Insert(ctx, value); err != nil {
		return Settings{}, err
	}
	return value, nil
}

func (service *Service) Update(ctx context.Context, identity authentication.VerifiedIdentity, command UpdateCommand) (Settings, error) {
	if err := service.validateDependencies(); err != nil {
		return Settings{}, err
	}
	value, err := validateUpdate(command)
	if err != nil {
		return Settings{}, err
	}
	if _, err := service.authorization.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return Settings{}, err
	}
	if err := service.repository.Update(ctx, value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Settings{}, ErrNotFound
		}
		return Settings{}, err
	}
	return value, nil
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
	// The default settings row drives the legacy runtime. Physical deletion is
	// deliberately unsupported to avoid breaking LoadApplicationSettings().
	return ErrDeleteRestricted
}

func (service *Service) validateDependencies() error {
	if service == nil || service.authorization == nil || service.repository == nil {
		return ErrServiceNotReady
	}
	return nil
}
